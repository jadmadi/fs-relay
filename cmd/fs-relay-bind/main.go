package main

import (
	"context"
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

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fs-relay-bind: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	url := flag.String("url", "http://127.0.0.1:8080", "fs-relay base URL")
	ns := flag.String("ns", "default", "namespace")
	post := flag.Bool("post", false, "send each operation as its own POST")
	useWS := flag.Bool("ws", false, "one connection for the mount (the default)")
	allowOther := flag.Bool("allow-other", false, "allow other users to access the mount")
	useRaw := flag.Bool("raw", true, "carry large reads and writes on a separate connection")
	debug := flag.Bool("debug", false, "enable FUSE debug logging")
	token := flag.String("token", "", "secret printed by fs-relay")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "usage: %s [flags] <mountpoint>\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(2)
	}
	if *post && *useWS {
		return fmt.Errorf("pass -post or -ws, not both")
	}
	if err := wire.CheckNS(*ns); err != nil {
		return fmt.Errorf("namespace: %w", err)
	}
	if *token == "" {
		return fmt.Errorf("pass -token")
	}
	if err := wire.CheckToken(*token); err != nil {
		return err
	}
	open := openSocket
	mode := "websocket"
	if *post {
		open = openHTTP
		mode = "HTTP POST"
	}
	c, events, stop, err := open(*url, *ns, *token)
	if err != nil {
		return err
	}
	defer stop()
	k := newKernel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go k.loop(ctx, events)
	root := &node{c: c, k: k, ino: 1}
	defer attachRaw(root, *url, *ns, *token, *useRaw)()
	server, err := fs.Mount(flag.Arg(0), root, mountOptions(*allowOther, *debug))
	if err != nil {
		return fmt.Errorf("mount: %w", err)
	}
	log.Printf("fs-relay-bind: mounted on %s, namespace %s, %s, %s", flag.Arg(0), *ns, mode, *url)
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
