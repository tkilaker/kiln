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

	// Initialize TTS service (optional - requires provider API key)
	var ttsSvc *tts.Service
	var ttsProvider tts.Provider

	switch cfg.TTSProvider {
	case "elevenlabs":
		if cfg.ElevenLabsAPIKey != "" && cfg.ElevenLabsVoiceID != "" {
			ttsProvider = tts.NewElevenLabsProvider(
				cfg.ElevenLabsAPIKey,
				cfg.ElevenLabsModel,
				cfg.ElevenLabsVoiceID,
				cfg.ElevenLabsVoiceName,
			)
			log.Printf("Using ElevenLabs TTS provider (model=%s, voice=%s)", cfg.ElevenLabsModel, cfg.ElevenLabsVoiceName)
		} else {
			log.Println("TTS disabled (ELEVENLABS_API_KEY and ELEVENLABS_VOICE_ID required for elevenlabs provider)")
		}
	default: // "openai"
		if cfg.OpenAIAPIKey != "" {
			ttsProvider = tts.NewOpenAIProvider(cfg.OpenAIAPIKey, cfg.TTSModel, cfg.TTSVoice)
			log.Printf("Using OpenAI TTS provider (model=%s, voice=%s)", cfg.TTSModel, cfg.TTSVoice)
		} else {
			log.Println("TTS disabled (OPENAI_API_KEY not set)")
		}
	}

	if ttsProvider != nil {
		ttsSvc, err = tts.New(ttsProvider, cfg.AudioDir)
		if err != nil {
			return fmt.Errorf("failed to initialize TTS: %w", err)
		}
		log.Printf("Initialized TTS service (provider=%s, dir=%s)", ttsProvider.Name(), cfg.AudioDir)
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
