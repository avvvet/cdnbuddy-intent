package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore implements Store interface using Redis
type RedisStore struct {
	client *redis.Client
	ttl    time.Duration // Session TTL (time to live)
}

// NewRedisStore creates a new Redis-backed store
func NewRedisStore(redisURL string, ttl time.Duration) (*RedisStore, error) {
	// Parse Redis URL
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	// Create Redis client
	client := redis.NewClient(opt)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisStore{
		client: client,
		ttl:    ttl,
	}, nil
}

// sessionKey generates Redis key for a session
func (r *RedisStore) sessionKey(sessionID string) string {
	return fmt.Sprintf("session:%s", sessionID)
}

// LoadSession loads a session from Redis
func (r *RedisStore) LoadSession(ctx context.Context, sessionID string) (*SessionData, error) {
	key := r.sessionKey(sessionID)

	// Get data from Redis
	data, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		// Session doesn't exist - return empty session
		return &SessionData{
			SessionID: sessionID,
			Messages:  []Message{},
			Metadata: Metadata{
				StartedAt:    time.Now(),
				LastActivity: time.Now(),
				MessageCount: 0,
			},
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load session from Redis: %w", err)
	}

	// Parse JSON
	var session SessionData
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		return nil, fmt.Errorf("failed to parse session data: %w", err)
	}

	return &session, nil
}

// SaveMessage appends a message to a session
func (r *RedisStore) SaveMessage(ctx context.Context, sessionID, userID string, msg Message) error {
	// Load existing session
	session, err := r.LoadSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to load session: %w", err)
	}

	// Set user ID if this is a new session
	if session.UserID == "" {
		session.UserID = userID
	}

	// Append message
	session.Messages = append(session.Messages, msg)

	// Update metadata
	session.Metadata.LastActivity = time.Now()
	session.Metadata.MessageCount = len(session.Messages)

	// If this is the first message, set started_at
	if session.Metadata.MessageCount == 1 {
		session.Metadata.StartedAt = msg.Timestamp
	}

	// Save to Redis
	return r.saveSession(ctx, session)
}

// saveSession saves session data to Redis
func (r *RedisStore) saveSession(ctx context.Context, session *SessionData) error {
	key := r.sessionKey(session.SessionID)

	// Marshal to JSON
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	// Save to Redis with TTL
	if err := r.client.Set(ctx, key, data, r.ttl).Err(); err != nil {
		return fmt.Errorf("failed to save session to Redis: %w", err)
	}

	return nil
}

// GetMessages retrieves all messages for a session
func (r *RedisStore) GetMessages(ctx context.Context, sessionID string) ([]Message, error) {
	session, err := r.LoadSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	return session.Messages, nil
}

// ClearSession removes a session from Redis
func (r *RedisStore) ClearSession(ctx context.Context, sessionID string) error {
	key := r.sessionKey(sessionID)

	if err := r.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to clear session: %w", err)
	}

	return nil
}

// SessionExists checks if a session exists in Redis
func (r *RedisStore) SessionExists(ctx context.Context, sessionID string) (bool, error) {
	key := r.sessionKey(sessionID)

	exists, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check session existence: %w", err)
	}

	return exists > 0, nil
}

// UpdateActivity updates the last activity timestamp and refreshes TTL
func (r *RedisStore) UpdateActivity(ctx context.Context, sessionID string) error {
	// Load session
	session, err := r.LoadSession(ctx, sessionID)
	if err != nil {
		return err
	}

	// Update last activity
	session.Metadata.LastActivity = time.Now()

	// Save back (this refreshes TTL)
	return r.saveSession(ctx, session)
}

// Close closes the Redis connection
func (r *RedisStore) Close() error {
	return r.client.Close()
}

// Health check - verify Redis connection is alive
func (r *RedisStore) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}
