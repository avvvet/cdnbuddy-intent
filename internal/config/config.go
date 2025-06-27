package config

import (
	"os"
	"time"
)

type Config struct {
	// NATS configuration
	NatsURL            string
	NatsRequestSubject string
	NatsTimeout        time.Duration

	// Anthropic configuration
	AnthropicAPIKey  string
	AnthropicModel   string
	AnthropicTimeout time.Duration

	// Service configuration
	ServiceName string
}

func Load() *Config {
	return &Config{
		// NATS settings
		NatsURL:            getEnv("NATS_URL", "nats://localhost:4222"),
		NatsRequestSubject: getEnv("NATS_REQUEST_SUBJECT", "intent.analyze"),
		NatsTimeout:        getDurationEnv("NATS_TIMEOUT", 30*time.Second),

		// Anthropic settings
		AnthropicAPIKey:  getEnv("ANTHROPIC_API_KEY", ""),
		AnthropicModel:   getEnv("ANTHROPIC_MODEL", "claude-3-5-sonnet-20241022"),
		AnthropicTimeout: getDurationEnv("ANTHROPIC_TIMEOUT", 30*time.Second),

		// Service settings
		ServiceName: getEnv("SERVICE_NAME", "cdnbuddy-intent"),
	}
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
