package memory

import (
	"context"
	"time"
)

// Message represents a single message in a conversation
type Message struct {
	Role      string    `json:"role"`      // "user" or "assistant"
	Content   string    `json:"content"`   // The actual message text
	Timestamp time.Time `json:"timestamp"` // When the message was sent
}

// SessionData represents all data for a conversation session
type SessionData struct {
	SessionID string    `json:"session_id"`
	UserID    string    `json:"user_id"`
	Messages  []Message `json:"messages"`
	Metadata  Metadata  `json:"metadata"`
}

// Metadata contains session information
type Metadata struct {
	StartedAt    time.Time `json:"started_at"`
	LastActivity time.Time `json:"last_activity"`
	MessageCount int       `json:"message_count"`
}

// Store defines the interface for conversation storage
// This allows us to swap between Redis, PostgreSQL, in-memory, etc.
type Store interface {
	// LoadSession loads a session from storage
	LoadSession(ctx context.Context, sessionID string) (*SessionData, error)

	// SaveMessage appends a message to a session
	SaveMessage(ctx context.Context, sessionID, userID string, msg Message) error

	// GetMessages retrieves all messages for a session
	GetMessages(ctx context.Context, sessionID string) ([]Message, error)

	// ClearSession removes a session from storage
	ClearSession(ctx context.Context, sessionID string) error

	// SessionExists checks if a session exists
	SessionExists(ctx context.Context, sessionID string) (bool, error)

	// UpdateActivity updates the last activity timestamp
	UpdateActivity(ctx context.Context, sessionID string) error
}
