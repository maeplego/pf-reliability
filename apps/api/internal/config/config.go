package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	HTTPAddr       string
	DevAuth        bool
	CORSOrigin     string
	WebhookSecret  string
	IntegrationKey string
	DatabaseURL    string
}

func FromEnv() (Config, error) {
	port := strings.TrimSpace(os.Getenv("RELIABILITY_HTTP_PORT"))
	if port == "" {
		port = "8012"
	}
	devAuth := strings.EqualFold(os.Getenv("RELIABILITY_DEV_AUTH"), "true") || os.Getenv("RELIABILITY_DEV_AUTH") == "1"
	cfg := Config{
		HTTPAddr:       ":" + port,
		DevAuth:        devAuth,
		CORSOrigin:     strings.TrimSpace(os.Getenv("RELIABILITY_CORS_ORIGIN")),
		WebhookSecret:  strings.TrimSpace(os.Getenv("RELIABILITY_WEBHOOK_SECRET")),
		IntegrationKey: strings.TrimSpace(os.Getenv("RELIABILITY_INTEGRATION_KEY")),
		DatabaseURL:    strings.TrimSpace(os.Getenv("RELIABILITY_DATABASE_URL")),
	}
	if cfg.CORSOrigin == "" {
		cfg.CORSOrigin = "http://localhost:3012"
	}
	if cfg.IntegrationKey == "" {
		cfg.IntegrationKey = "dev-inventory"
	}
	if cfg.WebhookSecret == "" {
		return cfg, fmt.Errorf("RELIABILITY_WEBHOOK_SECRET is required (dummy value in .env.example)")
	}
	if !cfg.DevAuth {
		return cfg, fmt.Errorf("RELIABILITY_DEV_AUTH=true is required in this slice (P01 OIDC is not wired yet)")
	}
	return cfg, nil
}
