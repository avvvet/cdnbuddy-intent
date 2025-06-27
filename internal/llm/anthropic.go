package llm

import (
	"context"
	"net/http"
	"time"

	"github.com/avvvet/cdnbuddy-intent/internal/models"
)

type AnthropicProvider struct {
	apiKey  string
	model   string
	timeout time.Duration
	client  *http.Client
}

func NewAnthropicProvider(apiKey, model string, timeout time.Duration) *AnthropicProvider {
	return &AnthropicProvider{
		apiKey:  apiKey,
		model:   model,
		timeout: timeout,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (a *AnthropicProvider) AnalyzeIntent(ctx context.Context, request *models.IntentRequest) (*models.IntentResponse, error) {
	// TODO: Implement the actual Anthropic API call
	// For now, return a mock response
	return &models.IntentResponse{
		SessionID:   request.SessionID,
		Action:      stringPtr("CREATE_SERVICE"),
		Status:      models.StatusNeedsInfo,
		Parameters:  map[string]*string{},
		UserMessage: "I understand you want to set up a CDN. What would you like to name your service?",
	}, nil
}

func stringPtr(s string) *string {
	return &s
}
