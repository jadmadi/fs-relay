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
	flag.Parse()

	tree := local.New(*data)
	defer tree.Close()
	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		return fmt.Errorf("listen %s: %w", *listen, err)
	}
	log.Printf("fs-relay: listening on %s", listenURL(ln.Addr(), defaultRouteIP))
	h, raw := handler(tree, newHub())
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
