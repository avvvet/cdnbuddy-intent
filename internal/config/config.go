package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	// Service
	ServiceName string
	Port        string

	// NATS
	NatsURL            string
	NatsRequestSubject string
	NatsTimeout        time.Duration

	// Anthropic
	AnthropicAPIKey  string
	AnthropicModel   string
	AnthropicTimeout time.Duration

	// Redis
	RedisURL string
}

func Load() (*Config, error) {
	cfg := &Config{
		ServiceName:        getEnv("SERVICE_NAME", "cdnbuddy-intent"),
		Port:               getEnv("PORT", "8083"),
		NatsURL:            getEnv("NATS_URL", "nats://localhost:4222"),
		NatsRequestSubject: getEnv("NATS_REQUEST_SUBJECT", "intent.analyze"),
		NatsTimeout:        getDurationEnv("NATS_TIMEOUT", 10*time.Second),
		AnthropicAPIKey:    getEnv("ANTHROPIC_API_KEY", ""),
		AnthropicModel:     getEnv("ANTHROPIC_MODEL", "claude-sonnet-4-20250514"),
		AnthropicTimeout:   getDurationEnv("ANTHROPIC_TIMEOUT", 30*time.Second),
		RedisURL:           getEnv("REDIS_URL", "redis://localhost:6379/0"),
	}

	// Validate
	if cfg.AnthropicAPIKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY is required")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
