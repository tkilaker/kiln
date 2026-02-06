package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/tkilaker/kiln/internal/config"
	"github.com/tkilaker/kiln/internal/database"
	"github.com/tkilaker/kiln/internal/scraper"
	"github.com/tkilaker/kiln/internal/server"
	"github.com/tkilaker/kiln/internal/tts"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}

func run() error {
	ctx := context.Background()

	// Load .env file (ignore error if it doesn't exist - env vars may be set directly)
	_ = godotenv.Load()

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
	scraper, err := scraper.New(cfg.GasettenUser, cfg.GasettenPass, db, cfg.ScraperHeadless)
	if err != nil {
		return fmt.Errorf("failed to initialize scraper: %w", err)
	}
	defer scraper.Close()
	log.Println("Initialized scraper")

	// Initialize TTS service (optional - only if API key is configured)
	var ttsSvc *tts.Service
	if cfg.OpenAIAPIKey != "" {
		ttsSvc, err = tts.New(cfg.OpenAIAPIKey, cfg.TTSModel, cfg.TTSVoice, cfg.AudioDir)
		if err != nil {
			return fmt.Errorf("failed to initialize TTS: %w", err)
		}
		log.Printf("Initialized TTS service (model=%s, voice=%s, dir=%s)", cfg.TTSModel, cfg.TTSVoice, cfg.AudioDir)
	} else {
		log.Println("TTS disabled (OPENAI_API_KEY not set)")
	}

	// Create server
	srv := server.New(db, scraper, ttsSvc, cfg)
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
