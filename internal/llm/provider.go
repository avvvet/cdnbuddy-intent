package llm

import (
	"context"

	"github.com/avvvet/cdnbuddy-intent/internal/models"
)

// LLMProvider defines the interface for LLM providers
type LLMProvider interface {
	AnalyzeIntent(ctx context.Context, request *models.IntentRequest) (*models.IntentResponse, error)
}

// LLMRequest represents the structured request to LLM
type LLMRequest struct {
	Prompt              string
	ConversationHistory []models.ConversationMessage
	MaxTokens           int
	Temperature         float64
}

// LLMResponse represents the raw response from LLM
type LLMResponse struct {
	Content string
	Usage   *Usage
}

type Usage struct {
	InputTokens  int
	OutputTokens int
}
