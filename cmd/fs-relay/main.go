package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fs-relay/internal/local"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fs-relay: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	listen := flag.String("listen", "127.0.0.1:8080", "address to listen on")
	data := flag.String("data", "data", "directory that holds one folder per namespace")
	given := flag.String("token", "", "shared secret; printed when omitted")
	file := flag.String("token-file", "", "read the shared secret from this file (mode 600)")
	flag.Parse()

	var tokenSet, fileSet bool
	flag.Visit(func(f *flag.Flag) {
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
	if tokenSet && *given == "" {
		return fmt.Errorf("pass a value for -token, or omit it and the server prints one")
	}
	if fileSet && *file == "" {
		return fmt.Errorf("pass a path for -token-file")
	}
	token, err := chooseToken(*given, *file)
	if err != nil {
		return err
	}
	tree := local.New(*data)
	defer tree.Close()
	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		return fmt.Errorf("listen %s: %w", *listen, err)
	}
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
