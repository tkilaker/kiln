package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all application configuration
type Config struct {
	// Database
	DatabaseURL string

	// Gasetten credentials
	GasettenUser string
	GasettenPass string

	// Server
	Port int

	// RSS Feed
	FeedTitle       string
	FeedDescription string
	FeedLink        string
	FeedAuthor      string

	// Scraper
	ScraperHeadless bool

	// TTS (Text-to-Speech)
	TTSProvider string // "openai" or "elevenlabs"
	AudioDir    string

	// OpenAI TTS
	OpenAIAPIKey string
	TTSModel     string
	TTSVoice     string

	// ElevenLabs TTS (for custom voice cloning)
	ElevenLabsAPIKey    string
	ElevenLabsVoiceID   string
	ElevenLabsModel     string
	ElevenLabsVoiceName string

	// Podcast
	PodcastLanguage string
	PodcastImageURL string
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:     getEnv("DATABASE_URL", ""),
		GasettenUser:    getEnv("GASETTEN_USER", ""),
		GasettenPass:    getEnv("GASETTEN_PASS", ""),
		Port:            getEnvAsInt("PORT", 8080),
		FeedTitle:       getEnv("FEED_TITLE", "My Personal Kiln Feed"),
		FeedDescription: getEnv("FEED_DESCRIPTION", "Articles from Gasetten"),
		FeedLink:        getEnv("FEED_LINK", "http://localhost:8080"),
		FeedAuthor:      getEnv("FEED_AUTHOR", "Kiln User"),
		ScraperHeadless: getEnvAsBool("SCRAPER_HEADLESS", true),
		TTSProvider:         getEnv("TTS_PROVIDER", "openai"),
		AudioDir:            getEnv("AUDIO_DIR", "/data/audio"),
		OpenAIAPIKey:        getEnv("OPENAI_API_KEY", ""),
		TTSModel:            getEnv("TTS_MODEL", "tts-1"),
		TTSVoice:            getEnv("TTS_VOICE", "alloy"),
		ElevenLabsAPIKey:    getEnv("ELEVENLABS_API_KEY", ""),
		ElevenLabsVoiceID:   getEnv("ELEVENLABS_VOICE_ID", ""),
		ElevenLabsModel:     getEnv("ELEVENLABS_MODEL", "eleven_multilingual_v2"),
		ElevenLabsVoiceName: getEnv("ELEVENLABS_VOICE_NAME", "Custom Voice"),
		PodcastLanguage:     getEnv("PODCAST_LANGUAGE", "sv"),
		PodcastImageURL:     getEnv("PODCAST_IMAGE_URL", ""),
	}

	// Validate required fields
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.GasettenUser == "" {
		return nil, fmt.Errorf("GASETTEN_USER is required")
	}
	if cfg.GasettenPass == "" {
		return nil, fmt.Errorf("GASETTEN_PASS is required")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}
