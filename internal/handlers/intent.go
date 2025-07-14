package handlers

import (
	"context"
	"fmt"
	"log"

	"github.com/avvvet/cdnbuddy-intent/internal/llm"
	"github.com/avvvet/cdnbuddy-intent/internal/models"
	"github.com/avvvet/cdnbuddy-intent/internal/prompts"
)

type IntentHandler struct {
	provider llm.LLMProvider
}

func NewIntentHandler(provider llm.LLMProvider) *IntentHandler {
	return &IntentHandler{
		provider: provider,
	}
}

func (h *IntentHandler) ProcessIntent(ctx context.Context, request *models.IntentRequest) (*models.IntentResponse, error) {
	// Validate request
	if err := h.validateRequest(request); err != nil {
		return h.createErrorResponse(request, models.ErrorParseError, err.Error()), nil
	}

	// Build prompt
	prompt := prompts.BuildIntentPrompt(request)
	log.Printf("Built prompt for session %s", request.SessionID)

	// Create LLM request
	llmRequest := &llm.LLMRequest{
		Prompt:              prompt,
		ConversationHistory: request.ConversationHistory,
		MaxTokens:           1000,
		Temperature:         0.1, // Low temperature for consistent responses
	}

	// Call LLM provider
	llmResponse, err := h.callLLM(ctx, llmRequest)
	if err != nil {
		return h.createErrorResponse(request, models.ErrorLLMFailed, err.Error()), nil
	}

	// Parse LLM response
	response, err := prompts.ParseLLMResponse(llmResponse.Content)
	if err != nil {
		log.Printf("Failed to parse LLM response: %v", err)
		return h.createErrorResponse(request, models.ErrorParseError, "Failed to understand response"), nil
	}

	// Set session ID
	response.SessionID = request.SessionID

	// Validate and clean response
	h.validateAndCleanResponse(response)

	log.Printf("Intent processed for session %s: action=%v, status=%s",
		request.SessionID, response.Action, response.Status)

	return response, nil
}

func (h *IntentHandler) validateRequest(request *models.IntentRequest) error {
	if request.SessionID == "" {
		return fmt.Errorf("session_id is required")
	}
	if request.UserMessage == "" {
		return fmt.Errorf("user_message is required")
	}
	/* on request we don't need action for now
	if len(request.AvailableActions) == 0 {
		return fmt.Errorf("available_actions is required")
	}
	*/
	return nil
}

func (h *IntentHandler) callLLM(ctx context.Context, request *llm.LLMRequest) (*llm.LLMResponse, error) {
	// For now, we'll implement this in the Anthropic provider
	// This is a placeholder that will be replaced when we implement the actual LLM call
	return &llm.LLMResponse{
		Content: `{"action": "CREATE_SERVICE", "status": "NEEDS_INFO", "parameters": {"domain_name": null}, "user_message": "I can help you setup a CDN! What would you like to name your service?"}`,
		Usage: &llm.Usage{
			InputTokens:  100,
			OutputTokens: 50,
		},
	}, nil
}

func (h *IntentHandler) validateAndCleanResponse(response *models.IntentResponse) {
	// Ensure status is valid
	validStatuses := map[string]bool{
		models.StatusNeedsInfo: true,
		models.StatusReady:     true,
		models.StatusError:     true,
	}

	if !validStatuses[response.Status] {
		response.Status = models.StatusError
		response.UserMessage = prompts.FallbackMessage
	}

	// Ensure parameters is not nil
	if response.Parameters == nil {
		response.Parameters = make(map[string]*string)
	}

	// Ensure user_message is not empty
	if response.UserMessage == "" {
		response.UserMessage = "How can I help you with your CDN setup?"
	}
}

func (h *IntentHandler) createErrorResponse(request *models.IntentRequest, errorCode, errorMessage string) *models.IntentResponse {
	return &models.IntentResponse{
		SessionID:    request.SessionID,
		Status:       models.StatusError,
		Parameters:   make(map[string]*string),
		UserMessage:  prompts.FallbackMessage,
		ErrorCode:    &errorCode,
		ErrorMessage: &errorMessage,
	}
}
