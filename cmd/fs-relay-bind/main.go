package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fs-relay/internal/client"
	"fs-relay/internal/wire"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

var errUsage = errors.New("usage")

func main() {
	if err := run(); err != nil {
		if errors.Is(err, errUsage) {
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "fs-relay-bind: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	flags := flag.NewFlagSet("fs-relay-bind", flag.ContinueOnError)
	url := flags.String("url", "http://127.0.0.1:8080", "fs-relay base URL")
	ns := flags.String("ns", "default", "namespace")
	post := flags.Bool("post", false, "send each operation as its own POST")
	useWS := flags.Bool("ws", false, "one connection for the mount (the default)")
	allowOther := flags.Bool("allow-other", false, "allow other users to access the mount")
	useRaw := flags.Bool("raw", true, "carry large reads and writes on a separate connection")
	debug := flags.Bool("debug", false, "enable FUSE debug logging")
	forceExisting := flags.Bool("force-existing-mnt", false, "mount over a non-empty directory")
	token := flags.String("token", "", "secret printed by fs-relay")
	tokenFile := flags.String("token-file", "", "read the secret from this file (mode 600)")
	showVersion := flags.Bool("version", false, "print the version and exit")
	flags.SetOutput(os.Stderr)
	flags.Usage = func() {}
	if err := flags.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Print(usageText(flags))
			return nil
		}
		fmt.Fprint(os.Stderr, usageText(flags))
		return errUsage
	}
	if *showVersion {
		fmt.Println(versionLine("fs-relay-bind"))
		return nil
	}
	if flags.NArg() != 1 {
		if bad := misplacedFlag(flags.Args()); bad != "" {
			fmt.Fprintf(os.Stderr, "flags must come before the mountpoint: %s\n", bad)
		}
		fmt.Fprint(os.Stderr, usageText(flags))
		return errUsage
	}
	if *post && *useWS {
		return fmt.Errorf("pass -post or -ws, not both")
	}
	if err := checkMountpoint(flags.Arg(0), *forceExisting); err != nil {
		return err
	}
	if err := wire.CheckNS(*ns); err != nil {
		return fmt.Errorf("namespace: %w", err)
	}
	base := normalizeURL(*url)
	var tokenSet, fileSet bool
	flags.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "token":
			tokenSet = true
		case "token-file":
			fileSet = true
		}
	})
	if tokenSet && fileSet {
		return fmt.Errorf("pass -token or -token-file, not both")
	}
	if tokenSet && *token == "" {
		return fmt.Errorf("pass a value for -token, or use -token-file")
	}
	if fileSet && *tokenFile == "" {
		return fmt.Errorf("pass a path for -token-file")
	}
	if !tokenSet && !fileSet {
		return fmt.Errorf("pass -token or -token-file")
	}
	tk, err := resolveToken(*token, *tokenFile)
	if err != nil {
		return err
	}
	open := openSocket
	mode := "websocket"
	if *post {
		open = openHTTP
		mode = "HTTP POST"
	}
	c, events, stop, err := open(base, *ns, tk)
	if err != nil {
		return err
	}
	defer stop()
	k := newKernel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go k.loop(ctx, events)
	root := &node{c: c, k: k, ino: 1}
	defer attachRaw(root, base, *ns, tk, *useRaw)()
	server, err := fs.Mount(flags.Arg(0), root, mountOptions(*allowOther, *debug))
	if err != nil {
		return fmt.Errorf("mount: %w", err)
	}
	log.Printf("fs-relay-bind: mounted on %s, namespace %s, %s, %s", flags.Arg(0), *ns, mode, base)
	go unmountOnSignal(server)
	server.Wait()
	return nil
}

func attachRaw(root *node, url, ns, token string, on bool) func() {
	if !on {
		return func() {}
	}
	raw, err := client.DialRaw(context.Background(), url, ns, token)
	if client.Refused(err) {
		log.Fatalf("fs-relay-bind: %v", err)
	}
	if err != nil {
		log.Printf("fs-relay-bind: large transfers remain on the main connection: %v", err)
		return func() {}
	}
	root.r = raw
	root.rawMin = raw.Min()
	root.spliceMax = spliceCeiling()
	return func() { _ = raw.Close() }
}

func mountOptions(allowOther, debug bool) *fs.Options {
	attr := time.Second
	neg := 500 * time.Millisecond
	opts := &fs.Options{
		AttrTimeout:     &attr,
		EntryTimeout:    &attr,
		NegativeTimeout: &neg,
		UID:             uint32(os.Getuid()),
		GID:             uint32(os.Getgid()),
	}
	opts.AllowOther = allowOther
	opts.Debug = debug
	opts.FsName = "fs-relay"
	opts.Name = "fs-relay"
	opts.DisableXAttrs = true
	opts.EnableLocks = true
	opts.MaxWrite = wire.MaxWrite
	opts.RootStableAttr = &fs.StableAttr{Ino: 1, Mode: syscall.S_IFDIR}
	return opts
}

func openHTTP(url, ns, token string) (caller, <-chan wire.Change, func(), error) {
	c := &client.Client{URL: url, NS: ns, Token: token}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if err := c.Listen(ctx); err != nil {
			log.Fatalf("fs-relay-bind: %v", err)
		}
	}()
	return c, c.Events(), func() { cancel() }, nil
}

func openSocket(url, ns, token string) (caller, <-chan wire.Change, func(), error) {
	sock, err := client.Dial(context.Background(), url, ns, token)
	if err != nil {
		return nil, nil, nil, err
	}
	return sock, sock.Events(), func() { _ = sock.Close() }, nil
}

func unmountOnSignal(server *fuse.Server) {
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	<-sigc
	_ = server.Unmount()
}
