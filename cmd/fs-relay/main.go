package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"fs-relay/internal/local"
)

var errUsage = errors.New("usage")

func main() {
	if err := run(); err != nil {
		if errors.Is(err, errUsage) {
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "fs-relay: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	flags := flag.NewFlagSet("fs-relay", flag.ContinueOnError)
	listen := flags.String("listen", "127.0.0.1:8080", "address to listen on")
	data := flags.String("data", "data", "directory that holds one folder per namespace")
	given := flags.String("token", "", "shared secret; printed when omitted")
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
		fmt.Println(versionLine("fs-relay"))
		return nil
	}

	token, err := chooseToken(*given)
	if err != nil {
		return err
	}
	tree := local.New(*data)
	defer tree.Close()
	if err := tree.Prepare(); err != nil {
		return err
	}
	abs, err := filepath.Abs(*data)
	if err != nil {
		return fmt.Errorf("data directory: %w", err)
	}
	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		return fmt.Errorf("listen %s: %w", *listen, err)
	}
	log.Printf("fs-relay: data %s", abs)
	log.Printf("fs-relay: listening on %s", listenURL(ln.Addr(), defaultRouteIP))
	h, raw := handler(tree, newHub(token))
	srv := &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
	}
	srv.RegisterOnShutdown(raw.closeAll)
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-errc:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	case <-sigc:
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		shut := srv.Shutdown(ctx)
		<-errc
		return shut
	}
}
