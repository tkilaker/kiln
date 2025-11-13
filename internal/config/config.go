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
