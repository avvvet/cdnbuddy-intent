package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/avvvet/cdnbuddy-intent/internal/memory"
	"github.com/avvvet/cdnbuddy-intent/internal/models"
)

type AnthropicProvider struct {
	apiKey        string
	model         string
	timeout       time.Duration
	client        *http.Client
	memoryManager *memory.Manager
}

// AnthropicRequest represents the request structure for Anthropic's API
type AnthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature,omitempty"`
	Messages    []AnthropicMessage `json:"messages"`
}

// AnthropicMessage represents a message in the conversation
type AnthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AnthropicResponse represents the response from Anthropic's API
type AnthropicResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model string `json:"model"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	StopReason string `json:"stop_reason"`
}

// AnthropicError represents an error response from Anthropic
type AnthropicError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func NewAnthropicProvider(apiKey, model string, timeout time.Duration, memoryManager *memory.Manager) *AnthropicProvider {
	return &AnthropicProvider{
		apiKey:        apiKey,
		model:         model,
		timeout:       timeout,
		memoryManager: memoryManager,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// AnalyzeIntent implements the LLMProvider interface
func (a *AnthropicProvider) AnalyzeIntent(ctx context.Context, request *models.IntentRequest) (*models.IntentResponse, error) {
	// Step 1: Save user message to Redis
	userID := "user_" + request.SessionID // Default user ID (can be improved later)
	if err := a.memoryManager.SaveUserMessage(ctx, request.SessionID, userID, request.UserMessage); err != nil {
		fmt.Printf("⚠️ Warning: Failed to save user message to Redis: %v\n", err)
		// Continue anyway - we can still process without saving
	}

	// Step 2: Load conversation history from Redis
	formattedHistory, err := a.memoryManager.GetFormattedHistory(ctx, request.SessionID)
	if err != nil {
		fmt.Printf("⚠️ Warning: Failed to load history from Redis: %v\n", err)
		formattedHistory = "No previous conversation."
	}

	fmt.Printf("📚 Loaded conversation history for session %s:\n%s\n", request.SessionID, formattedHistory)

	// Step 3: Build the prompt using history from Redis
	prompt := a.buildPromptWithHistory(request, formattedHistory)

	// Step 4: Create a single message with the full prompt
	messages := []AnthropicMessage{
		{
			Role:    "user",
			Content: prompt,
		},
	}

	// Step 5: Prepare the request body
	anthropicReq := AnthropicRequest{
		Model:       a.model,
		MaxTokens:   1000,
		Temperature: 0.1, // Low temperature for consistent responses
		Messages:    messages,
	}

	// Marshal the request
	reqBody, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	fmt.Printf("🤖 Calling Claude API for session: %s\n", request.SessionID)

	// Step 6: Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	// Step 7: Make the request
	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Handle non-200 responses
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("❌ Error response body: %s\n", string(body))

		var anthropicErr AnthropicError
		if err := json.Unmarshal(body, &anthropicErr); err != nil {
			return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("anthropic API error: %s", anthropicErr.Message)
	}

	// Step 8: Parse response
	var anthropicResp AnthropicResponse
	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract content
	var content string
	if len(anthropicResp.Content) > 0 {
		content = anthropicResp.Content[0].Text
	}

	fmt.Printf("✅ Claude response received: %d characters\n", len(content))

	// Step 9: Parse the LLM response
	intentResponse, err := a.parseIntentResponse(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse intent response: %w", err)
	}

	// Set session ID
	intentResponse.SessionID = request.SessionID

	// Step 10: Save assistant response to Redis
	if intentResponse.UserMessage != "" {
		if err := a.memoryManager.SaveAssistantMessage(ctx, request.SessionID, userID, intentResponse.UserMessage); err != nil {
			fmt.Printf("⚠️ Warning: Failed to save assistant message to Redis: %v\n", err)
			// Continue anyway
		}
	}

	return intentResponse, nil
}

// buildPromptWithHistory creates the full prompt using conversation history from Redis
func (a *AnthropicProvider) buildPromptWithHistory(request *models.IntentRequest, formattedHistory string) string {
	// Build available actions section
	actionsSection := a.buildActionsSection(request.AvailableActions)

	const SystemPrompt = `You are an AI assistant for CDNbuddy, a CDN management platform. Your job is to analyze user conversations and determine what CDN-related actions they want to perform.

IMPORTANT RULES:
1. Work on ONE action at a time, even if multiple actions are mentioned
2. If multiple actions are mentioned, pick the first one mentioned
3. Extract parameters from the conversation for the selected action
4. If you need more information, ask specific questions
5. When an action is complete, you can ask "Do you have any other requirements?"
6. IMPORTANT: Review the ENTIRE conversation history before responding - don't ask for information already provided

RESPONSE FORMAT:
You must respond with a valid JSON object in this exact format:
{
 "action": "ACTION_NAME or null",
 "status": "NEEDS_INFO or READY",
 "parameters": {
 "param_name": "extracted_value or null"
 },
 "user_message": "Your response to the user"
}

Available Actions:
%s

Conversation History:
%s

Current User Message: %s

Analyze the FULL conversation history above and respond with the JSON format. Remember to check what information was already provided in previous messages.`

	return fmt.Sprintf(SystemPrompt, actionsSection, formattedHistory, request.UserMessage)
}

func (a *AnthropicProvider) buildActionsSection(actions []models.ActionSchema) string {
	var builder strings.Builder
	for _, action := range actions {
		builder.WriteString(fmt.Sprintf("- %s: requires [%s]\n",
			action.Action,
			strings.Join(action.Parameters, ", ")))
	}
	return builder.String()
}

// parseIntentResponse parses the JSON response from the LLM into an IntentResponse
func (a *AnthropicProvider) parseIntentResponse(content string) (*models.IntentResponse, error) {

	jsonContent := a.extractJSON(content)
	if jsonContent == "" {
		return nil, fmt.Errorf("no valid JSON found in response")
	}

	var response models.IntentResponse
	if err := json.Unmarshal([]byte(jsonContent), &response); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	if response.Status == "" {
		response.Status = models.StatusError
		response.UserMessage = "I didn't understand your request clearly. Could you please rephrase what you'd like me to help you with regarding CDN setup or management?"
	}

	if response.Parameters == nil {
		response.Parameters = make(map[string]*string)
	}

	return &response, nil
}

func (a *AnthropicProvider) extractJSON(content string) string {
	// Look for JSON object in the content
	start := strings.Index(content, "{")
	if start == -1 {
		return ""
	}

	end := strings.LastIndex(content, "}")
	if end == -1 || end <= start {
		return ""
	}

	return content[start : end+1]
}
