package main

import (
	"flag"
	"log"
	"os"

	"github.com/mahdi/dns-proxy-remote/internal/config"
	"github.com/mahdi/dns-proxy-remote/internal/server"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create and run server
	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	if err := srv.Run(); err != nil {
		log.Printf("Server shutdown: %v", err)
		os.Exit(1)
	}
}
