package main

import (
	"flag"
	"log"

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

	srv := server.New(cfg)

	if err := srv.Start(); err != nil {
		log.Fatal(err)
	}
}
