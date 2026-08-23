package config

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port            string
	MongoURI        string
	MongoDB         string
	FrontendURL     string
	PaymentsMode    string
	DodoEnvironment string
	DodoAPIKey      string
	DodoWebhookKey  string
	DodoProductID   string
}

func Load() (Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("read .env: %w", err)
	}

	cfg := Config{
		Port:            env("PORT", "8080"),
		MongoURI:        mongoURI(),
		MongoDB:         env("MONGODB_DB", "saaslb"),
		FrontendURL:     strings.TrimRight(env("FRONTEND_URL", "http://localhost:5173"), "/"),
		PaymentsMode:    strings.ToLower(env("PAYMENTS_MODE", "dodo")),
		DodoEnvironment: env("DODO_ENVIRONMENT", "test_mode"),
		DodoAPIKey:      env("DODO_PAYMENTS_API_KEY", ""),
		DodoWebhookKey:  env("DODO_PAYMENTS_WEBHOOK_KEY", ""),
		DodoProductID:   env("DODO_PRODUCT_ID", ""),
	}

	if cfg.PaymentsMode != "dodo" && cfg.PaymentsMode != "simulate" {
		return Config{}, fmt.Errorf("PAYMENTS_MODE must be dodo or simulate")
	}
	if cfg.DodoEnvironment != "test_mode" && cfg.DodoEnvironment != "live_mode" {
		return Config{}, fmt.Errorf("DODO_ENVIRONMENT must be test_mode or live_mode")
	}

	log.Printf("mongo host %s db=%s", mongoHost(cfg.MongoURI), cfg.MongoDB)
	return cfg, nil
}

func mongoHost(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return "(unset)"
	}
	return parsed.Host
}

func (c Config) DodoBaseURL() string {
	if c.DodoEnvironment == "live_mode" {
		return "https://live.dodopayments.com"
	}
	return "https://test.dodopayments.com"
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func mongoURI() string {
	if value := env("MONGODB_URI", ""); value != "" {
		return value
	}
	if value := env("DATABASE_URL", ""); strings.HasPrefix(value, "mongodb") {
		return value
	}
	return "mongodb://localhost:27017"
}
