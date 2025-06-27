package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/avvvet/cdnbuddy-intent/internal/config"
	"github.com/avvvet/cdnbuddy-intent/internal/handlers"
	"github.com/avvvet/cdnbuddy-intent/internal/models"
	"github.com/nats-io/nats.go"
)

type NATSTransport struct {
	conn    *nats.Conn
	config  *config.Config
	handler *handlers.IntentHandler
}

func NewNATSTransport(cfg *config.Config, handler *handlers.IntentHandler) (*NATSTransport, error) {
	// Connect to NATS
	conn, err := nats.Connect(cfg.NatsURL,
		nats.Name(cfg.ServiceName),
		nats.Timeout(cfg.NatsTimeout),
		nats.ReconnectWait(2*time.Second),
		nats.MaxReconnects(-1), // Infinite reconnects
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	log.Printf("Connected to NATS server: %s", cfg.NatsURL)

	return &NATSTransport{
		conn:    conn,
		config:  cfg,
		handler: handler,
	}, nil
}

func (nt *NATSTransport) Start() error {
	// Subscribe to intent analysis requests
	_, err := nt.conn.Subscribe(nt.config.NatsRequestSubject, nt.handleIntentRequest)
	if err != nil {
		return fmt.Errorf("failed to subscribe to %s: %w", nt.config.NatsRequestSubject, err)
	}

	log.Printf("Subscribed to subject: %s", nt.config.NatsRequestSubject)
	return nil
}

func (nt *NATSTransport) handleIntentRequest(msg *nats.Msg) {
	// Parse the request
	var request models.IntentRequest
	if err := json.Unmarshal(msg.Data, &request); err != nil {
		log.Printf("Error parsing request: %v", err)
		nt.sendErrorResponse(msg, &request, models.ErrorParseError, "Invalid request format")
		return
	}

	log.Printf("Processing intent request for session: %s", request.SessionID)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), nt.config.AnthropicTimeout)
	defer cancel()

	// Call the handler
	response, err := nt.handler.ProcessIntent(ctx, &request)
	if err != nil {
		log.Printf("Error processing intent: %v", err)
		nt.sendErrorResponse(msg, &request, models.ErrorLLMFailed, err.Error())
		return
	}

	// Send response
	if err := nt.sendResponse(msg, response); err != nil {
		log.Printf("Error sending response: %v", err)
	}
}

func (nt *NATSTransport) sendResponse(msg *nats.Msg, response *models.IntentResponse) error {
	responseData, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	if err := msg.Respond(responseData); err != nil {
		return fmt.Errorf("failed to send response: %w", err)
	}

	log.Printf("Response sent for session: %s, status: %s", response.SessionID, response.Status)
	return nil
}

func (nt *NATSTransport) sendErrorResponse(msg *nats.Msg, request *models.IntentRequest, errorCode, errorMessage string) {
	response := &models.IntentResponse{
		SessionID:    request.SessionID,
		Status:       models.StatusError,
		Parameters:   make(map[string]*string),
		UserMessage:  "I'm sorry, I encountered an error processing your request. Please try again.",
		ErrorCode:    &errorCode,
		ErrorMessage: &errorMessage,
	}

	if err := nt.sendResponse(msg, response); err != nil {
		log.Printf("Failed to send error response: %v", err)
	}
}

func (nt *NATSTransport) Close() error {
	if nt.conn != nil {
		nt.conn.Close()
		log.Println("NATS connection closed")
	}
	return nil
}
