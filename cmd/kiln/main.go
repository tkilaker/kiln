package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/tim/kiln/internal/config"
	"github.com/tim/kiln/internal/database"
	"github.com/tim/kiln/internal/scraper"
	"github.com/tim/kiln/internal/server"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}

func run() error {
	ctx := context.Background()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	log.Println("Starting Kiln...")

	// Connect to database
	db, err := database.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()
	log.Println("Connected to database")

	// Initialize scraper
	scraper, err := scraper.New(cfg.GasettenUser, cfg.GasettenPass, db)
	if err != nil {
		return fmt.Errorf("failed to initialize scraper: %w", err)
	}
	defer scraper.Close()
	log.Println("Initialized scraper")

	// Create server
	srv := server.New(db, scraper, cfg)
	log.Println("Initialized server")

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down gracefully...")
		os.Exit(0)
	}()

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("Server starting on http://localhost%s", addr)
	return srv.Start(addr)
}
