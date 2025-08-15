package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"openailogger/internal/config"
	"openailogger/internal/server"
	"openailogger/storage"
	"openailogger/storage/memory"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "config.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize storage
	var store storage.Store
	switch cfg.Capture.Store {
	case "memory":
		store = memory.New()
	default:
		log.Fatalf("Unsupported storage type: %s", cfg.Capture.Store)
	}

	// Create and start server
	srv := server.New(cfg, store)

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down server...")
		if err := srv.Close(); err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
		os.Exit(0)
	}()

	// Start server
	if err := srv.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
