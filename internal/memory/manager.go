package memory

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/avvvet/cdnbuddy-intent/internal/models"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/memory"
)

// Manager orchestrates conversation memory using Redis + LangChainGo
type Manager struct {
	store         Store
	sessions      map[string]*memory.ConversationBuffer // In-memory cache
	defaultUserID string
}

// NewManager creates a new memory manager
func NewManager(store Store) *Manager {
	return &Manager{
		store:         store,
		sessions:      make(map[string]*memory.ConversationBuffer),
		defaultUserID: "default_user",
	}
}

// GetOrCreateSession gets or creates a LangChainGo memory buffer for a session
func (m *Manager) GetOrCreateSession(ctx context.Context, sessionID string) (*memory.ConversationBuffer, error) {
	// Check if we already have it in cache
	if mem, exists := m.sessions[sessionID]; exists {
		return mem, nil
	}

	// Create new LangChainGo conversation buffer
	mem := memory.NewConversationBuffer()

	// Load history from Redis
	sessionData, err := m.store.LoadSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}

	// Load messages into LangChainGo memory
	for _, msg := range sessionData.Messages {
		var chatMsg llms.ChatMessage

		switch msg.Role {
		case "user":
			chatMsg = llms.HumanChatMessage{Content: msg.Content}
		case "assistant":
			chatMsg = llms.AIChatMessage{Content: msg.Content}
		case "system":
			chatMsg = llms.SystemChatMessage{Content: msg.Content}
		default:
			log.Printf("⚠️ Unknown message role: %s, skipping", msg.Role)
			continue
		}

		// Add to LangChainGo memory
		if err := mem.ChatHistory.AddMessage(ctx, chatMsg); err != nil {
			return nil, fmt.Errorf("failed to add message to memory: %w", err)
		}
	}

	// Cache it
	m.sessions[sessionID] = mem

	log.Printf("📚 Loaded session %s with %d messages", sessionID, len(sessionData.Messages))

	return mem, nil
}

// SaveUserMessage saves a user message to both Redis and LangChainGo memory
func (m *Manager) SaveUserMessage(ctx context.Context, sessionID, userID, message string) error {
	// Get or create session
	mem, err := m.GetOrCreateSession(ctx, sessionID)
	if err != nil {
		return err
	}

	// Add to LangChainGo memory
	if err := mem.ChatHistory.AddUserMessage(ctx, message); err != nil {
		return fmt.Errorf("failed to add user message to memory: %w", err)
	}

	// Save to Redis
	msg := Message{
		Role:      "user",
		Content:   message,
		Timestamp: time.Now(),
	}

	if err := m.store.SaveMessage(ctx, sessionID, userID, msg); err != nil {
		return fmt.Errorf("failed to save message to Redis: %w", err)
	}

	log.Printf("💾 Saved user message to session %s", sessionID)

	return nil
}

// SaveAssistantMessage saves an assistant message to both Redis and LangChainGo memory
func (m *Manager) SaveAssistantMessage(ctx context.Context, sessionID, userID, message string) error {
	// Get or create session
	mem, err := m.GetOrCreateSession(ctx, sessionID)
	if err != nil {
		return err
	}

	// Add to LangChainGo memory
	if err := mem.ChatHistory.AddAIMessage(ctx, message); err != nil {
		return fmt.Errorf("failed to add AI message to memory: %w", err)
	}

	// Save to Redis
	msg := Message{
		Role:      "assistant",
		Content:   message,
		Timestamp: time.Now(),
	}

	if err := m.store.SaveMessage(ctx, sessionID, userID, msg); err != nil {
		return fmt.Errorf("failed to save message to Redis: %w", err)
	}

	log.Printf("💾 Saved assistant message to session %s", sessionID)

	return nil
}

// LoadHistoryFromRequest loads conversation history from IntentRequest
// This is useful when API Server sends existing history
func (m *Manager) LoadHistoryFromRequest(ctx context.Context, sessionID string, history []models.ConversationMessage) error {
	// Get or create session
	mem, err := m.GetOrCreateSession(ctx, sessionID)
	if err != nil {
		return err
	}

	// Clear existing memory (in case we're reloading)
	mem.Clear(ctx)

	// Load each message
	for _, msg := range history {
		var chatMsg llms.ChatMessage

		switch msg.Role {
		case "user":
			chatMsg = llms.HumanChatMessage{Content: msg.Message}
		case "assistant":
			chatMsg = llms.AIChatMessage{Content: msg.Message}
		default:
			continue
		}

		if err := mem.ChatHistory.AddMessage(ctx, chatMsg); err != nil {
			return fmt.Errorf("failed to add message: %w", err)
		}
	}

	log.Printf("📥 Loaded %d messages from request into session %s", len(history), sessionID)

	return nil
}

// GetFormattedHistory returns conversation history as a formatted string
// This is used for building prompts
func (m *Manager) GetFormattedHistory(ctx context.Context, sessionID string) (string, error) {
	mem, err := m.GetOrCreateSession(ctx, sessionID)
	if err != nil {
		return "", err
	}

	messages, err := mem.ChatHistory.Messages(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get messages: %w", err)
	}

	if len(messages) == 0 {
		return "No previous conversation.", nil
	}

	// Format messages
	var formatted string
	for _, msg := range messages {
		switch m := msg.(type) {
		case llms.HumanChatMessage:
			formatted += fmt.Sprintf("User: %s\n", m.Content)
		case llms.AIChatMessage:
			formatted += fmt.Sprintf("Assistant: %s\n", m.Content)
		case llms.SystemChatMessage:
			formatted += fmt.Sprintf("System: %s\n", m.Content)
		}
	}

	return formatted, nil
}

// GetMessages returns raw messages from Redis
func (m *Manager) GetMessages(ctx context.Context, sessionID string) ([]Message, error) {
	return m.store.GetMessages(ctx, sessionID)
}

// ClearSession clears a session from both cache and Redis
func (m *Manager) ClearSession(ctx context.Context, sessionID string) error {
	// Remove from cache
	delete(m.sessions, sessionID)

	// Remove from Redis
	if err := m.store.ClearSession(ctx, sessionID); err != nil {
		return fmt.Errorf("failed to clear session from Redis: %w", err)
	}

	log.Printf("🗑️ Cleared session %s", sessionID)

	return nil
}

// SessionExists checks if a session exists in Redis
func (m *Manager) SessionExists(ctx context.Context, sessionID string) (bool, error) {
	return m.store.SessionExists(ctx, sessionID)
}

// UpdateActivity updates the last activity timestamp in Redis
func (m *Manager) UpdateActivity(ctx context.Context, sessionID string) error {
	return m.store.UpdateActivity(ctx, sessionID)
}

// GetActiveSessionCount returns the number of cached sessions
func (m *Manager) GetActiveSessionCount() int {
	return len(m.sessions)
}

// Close closes the underlying store
func (m *Manager) Close() error {
	if closer, ok := m.store.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}
