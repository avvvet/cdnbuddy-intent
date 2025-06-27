package prompts

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/avvvet/cdnbuddy-intent/internal/models"
)

const SystemPrompt = `You are an AI assistant for CDNbuddy, a CDN management platform. Your job is to analyze user conversations and determine what CDN-related actions they want to perform.

IMPORTANT RULES:
1. Work on ONE action at a time, even if multiple actions are mentioned
2. If multiple actions are mentioned, pick the first one mentioned
3. Extract parameters from the conversation for the selected action
4. If you need more information, ask specific questions
5. When an action is complete, you can ask "Do you have any other requirements?"

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

Current Conversation:
%s

Analyze the conversation and respond with the JSON format above.`

const FallbackMessage = "I didn't understand your request clearly. Could you please rephrase what you'd like me to help you with regarding CDN setup or management?"

func BuildIntentPrompt(request *models.IntentRequest) string {
	// Build available actions section
	actionsSection := buildActionsSection(request.AvailableActions)

	// Build conversation section
	conversationSection := buildConversationSection(request.ConversationHistory, request.UserMessage)

	return fmt.Sprintf(SystemPrompt, actionsSection, conversationSection)
}

func buildActionsSection(actions []models.ActionSchema) string {
	var builder strings.Builder

	for _, action := range actions {
		builder.WriteString(fmt.Sprintf("- %s: requires [%s]\n",
			action.Action,
			strings.Join(action.Parameters, ", ")))
	}

	return builder.String()
}

func buildConversationSection(history []models.ConversationMessage, currentMessage string) string {
	var builder strings.Builder

	// Add conversation history
	for _, msg := range history {
		builder.WriteString(fmt.Sprintf("%s: %s\n", strings.Title(msg.Role), msg.Message))
	}

	// Add current user message
	builder.WriteString(fmt.Sprintf("User: %s\n", currentMessage))

	return builder.String()
}

func ParseLLMResponse(content string) (*models.IntentResponse, error) {
	// Try to extract JSON from the response
	jsonContent := extractJSON(content)
	if jsonContent == "" {
		return nil, fmt.Errorf("no valid JSON found in response")
	}

	var response models.IntentResponse
	if err := json.Unmarshal([]byte(jsonContent), &response); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Validate required fields
	if response.Status == "" {
		response.Status = models.StatusError
		response.UserMessage = FallbackMessage
	}

	if response.Parameters == nil {
		response.Parameters = make(map[string]*string)
	}

	return &response, nil
}

func extractJSON(content string) string {
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
