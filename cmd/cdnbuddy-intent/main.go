package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/avvvet/cdnbuddy-intent/internal/config"
	"github.com/avvvet/cdnbuddy-intent/internal/handlers"
	"github.com/avvvet/cdnbuddy-intent/internal/llm"
	"github.com/avvvet/cdnbuddy-intent/internal/transport"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if it exists (for development)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	log.Println("Starting CDNbuddy Intent Service...")
	log.Println("Starting CDNbuddy Intent Service...")

	// Load configuration
	cfg := config.Load()
	log.Printf("Service: %s", cfg.ServiceName)
	log.Printf("NATS URL: %s", cfg.NatsURL)
	log.Printf("Anthropic Model: %s", cfg.AnthropicModel)

	// Validate required configuration
	if cfg.AnthropicAPIKey == "" {
		log.Fatal("ANTHROPIC_API_KEY environment variable is required")
	}

	// Initialize LLM provider
	anthropicProvider := llm.NewAnthropicProvider(
		cfg.AnthropicAPIKey,
		cfg.AnthropicModel,
		cfg.AnthropicTimeout,
	)
	log.Println("Anthropic provider initialized")

	// Initialize intent handler
	intentHandler := handlers.NewIntentHandler(anthropicProvider)
	log.Println("Intent handler initialized")

	// Initialize NATS transport
	natsTransport, err := transport.NewNATSTransport(cfg, intentHandler)
	if err != nil {
		log.Fatalf("Failed to initialize NATS transport: %v", err)
	}
	defer natsTransport.Close()

	// Start listening for requests
	if err := natsTransport.Start(); err != nil {
		log.Fatalf("Failed to start NATS transport: %v", err)
	}

	log.Println("CDNbuddy Intent Service is running...")
	log.Printf("Listening on subject: %s", cfg.NatsRequestSubject)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Block until signal received
	sig := <-sigChan
	log.Printf("Received signal: %v", sig)
	log.Println("Shutting down gracefully...")

	// Cleanup
	if err := natsTransport.Close(); err != nil {
		log.Printf("Error during cleanup: %v", err)
	}

	log.Println("CDNbuddy Intent Service stopped")
}
