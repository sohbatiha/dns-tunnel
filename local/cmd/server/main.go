package main

import (
	"flag"
	"log"
	"os"

	"github.com/mahdi/dns-proxy-local/internal/client"
	"github.com/mahdi/dns-proxy-local/internal/config"
	"github.com/mahdi/dns-proxy-local/internal/crypto"
	"github.com/mahdi/dns-proxy-local/internal/server"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create cipher if encryption is enabled
	var cipher *crypto.Cipher
	if cfg.Security.EncryptionEnabled {
		cipher, err = crypto.NewCipher(cfg.Security.EncryptionKey)
		if err != nil {
			log.Fatalf("Failed to create cipher: %v", err)
		}
	}

	// Create API client
	apiClient := client.NewClient(cfg.API, cipher)

	// Create and run server
	srv := server.New(cfg, apiClient)
	if err := srv.Run(); err != nil {
		log.Printf("Server error: %v", err)
		os.Exit(1)
	}
}
