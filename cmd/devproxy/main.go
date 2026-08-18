package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/phy0hk/devproxy/internal/config"
	"github.com/phy0hk/devproxy/internal/server"
)

func main() {
	configPath := flag.String(
		"config",
		"config.yaml",
		"path to configuration file",
	)

	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	srv, err := server.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	serverErr := make(chan error, 1)

	go func() {
		serverErr <- srv.Start()
	}()

	signalCtx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	select {
	case err := <-serverErr:
		if !errors.Is(err, context.Canceled) &&
			!errors.Is(err, os.ErrClosed) {
			log.Fatal(err)
		}

	case <-signalCtx.Done():
		if err := srv.Shutdown(context.Background()); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	}
}
