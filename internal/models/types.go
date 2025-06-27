package models

// NATS Request from backend
type IntentRequest struct {
	SessionID           string                `json:"session_id"`
	UserMessage         string                `json:"user_message"`
	ConversationHistory []ConversationMessage `json:"conversation_history"`
	AvailableActions    []ActionSchema        `json:"available_actions"`
}

type ConversationMessage struct {
	Role    string `json:"role"` // "user" or "assistant"
	Message string `json:"message"`
}

type ActionSchema struct {
	Action     string   `json:"action"`
	Parameters []string `json:"parameters"`
}

// NATS Response to backend
type IntentResponse struct {
	SessionID    string             `json:"session_id"`
	Action       *string            `json:"action"`
	Status       string             `json:"status"` // "NEEDS_INFO", "READY", "ERROR"
	Parameters   map[string]*string `json:"parameters"`
	UserMessage  string             `json:"user_message"`
	ErrorCode    *string            `json:"error_code,omitempty"`
	ErrorMessage *string            `json:"error_message,omitempty"`
}

// Status constants
const (
	StatusNeedsInfo = "NEEDS_INFO"
	StatusReady     = "READY"
	StatusError     = "ERROR"
)

// Error codes
const (
	ErrorLLMTimeout    = "LLM_API_TIMEOUT"
	ErrorLLMFailed     = "LLM_API_FAILED"
	ErrorParseError    = "PARSE_ERROR"
	ErrorUnknownIntent = "UNKNOWN_INTENT"
)
