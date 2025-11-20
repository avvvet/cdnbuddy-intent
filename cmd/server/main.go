package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/avvvet/cdnbuddy-intent/internal/config"
	"github.com/avvvet/cdnbuddy-intent/internal/handlers"
	"github.com/avvvet/cdnbuddy-intent/internal/llm"
	"github.com/avvvet/cdnbuddy-intent/internal/memory"
	"github.com/avvvet/cdnbuddy-intent/internal/transport"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if it exists (for development)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	log.Println("🚀 Starting CDNbuddy Intent Service...")

	// Load configuration
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("❌ Failed to load config: %v", err)
	}
	log.Printf("📋 Service: %s", cfg.ServiceName)
	log.Printf("📡 NATS URL: %s", cfg.NatsURL)
	log.Printf("🤖 Anthropic Model: %s", cfg.AnthropicModel)

	// Validate required configuration
	if cfg.AnthropicAPIKey == "" {
		log.Fatal("❌ ANTHROPIC_API_KEY environment variable is required")
	}

	// Get Redis URL from environment (with default)
	redisURL := getEnv("REDIS_URL", "redis://localhost:6379/0")
	log.Printf("💾 Redis URL: %s", redisURL)

	// Initialize Redis store
	log.Println("🔌 Connecting to Redis...")
	redisStore, err := memory.NewRedisStore(redisURL, 30*time.Minute) // 30 min TTL
	if err != nil {
		log.Fatalf("❌ Failed to connect to Redis: %v", err)
	}
	defer redisStore.Close()
	log.Println("✅ Redis connected")

	// Initialize Memory Manager
	log.Println("🧠 Initializing memory manager...")
	memoryManager := memory.NewManager(redisStore)
	defer memoryManager.Close()
	log.Println("✅ Memory manager initialized")

	// Initialize LLM provider with memory manager
	log.Println("🤖 Initializing Anthropic provider...")
	anthropicProvider := llm.NewAnthropicProvider(
		cfg.AnthropicAPIKey,
		cfg.AnthropicModel,
		cfg.AnthropicTimeout,
		memoryManager, // Pass memory manager here
	)
	log.Println("✅ Anthropic provider initialized")

	// Initialize intent handler
	intentHandler := handlers.NewIntentHandler(anthropicProvider)
	log.Println("✅ Intent handler initialized")

	// Initialize NATS transport
	log.Println("📡 Connecting to NATS...")
	natsTransport, err := transport.NewNATSTransport(cfg, intentHandler)
	if err != nil {
		log.Fatalf("❌ Failed to initialize NATS transport: %v", err)
	}
	defer natsTransport.Close()

	// Start listening for requests
	if err := natsTransport.Start(); err != nil {
		log.Fatalf("❌ Failed to start NATS transport: %v", err)
	}

	log.Println("✅ CDNbuddy Intent Service is running!")
	log.Printf("👂 Listening on subject: %s", cfg.NatsRequestSubject)
	log.Printf("📊 Active sessions: %d", memoryManager.GetActiveSessionCount())

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Block until signal received
	sig := <-sigChan
	log.Printf("🛑 Received signal: %v", sig)
	log.Println("🔄 Shutting down gracefully...")

	// Cleanup
	log.Printf("📊 Final session count: %d", memoryManager.GetActiveSessionCount())

	if err := memoryManager.Close(); err != nil {
		log.Printf("⚠️ Error closing memory manager: %v", err)
	}

	if err := natsTransport.Close(); err != nil {
		log.Printf("⚠️ Error closing NATS transport: %v", err)
	}

	log.Println("👋 CDNbuddy Intent Service stopped")
}

// getEnv gets environment variable with default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
