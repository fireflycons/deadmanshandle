package handlers

import (
	"context"
	"encoding/json"

	"github.com/aws/aws-lambda-go/events"
	"github.com/fireflycons/deadmanshandle/internal/config"
	"github.com/fireflycons/deadmanshandle/internal/domain"
	"github.com/fireflycons/deadmanshandle/internal/ports"
)

// HTTPHandler handles HTTP API Gateway requests
type HTTPHandler struct {
	configStore  ports.ConfigStore
	service      *domain.DeadmansHandleService
	keyValidator ports.APIKeyValidator
	paramName    string
}

// NewHTTPHandler creates a new HTTP handler
func NewHTTPHandler(
	configStore ports.ConfigStore,
	service *domain.DeadmansHandleService,
	keyValidator ports.APIKeyValidator,
	parameterName string,
) *HTTPHandler {
	return &HTTPHandler{
		configStore:  configStore,
		service:      service,
		keyValidator: keyValidator,
		paramName:    parameterName,
	}
}

// HTTPResponse represents the response structure
type HTTPResponse struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
	NewTimeout string `json:"newTimeout,omitempty"`
}

// Handle processes HTTP API Gateway requests
func (h *HTTPHandler) Handle(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Extract and validate API key
	apiKey := request.Headers["x-api-key"]
	if apiKey == "" {
		return h.response(401, "Missing API key"), nil
	}

	// Get configuration
	configData, err := h.configStore.GetConfig(ctx, h.paramName)
	if err != nil {
		return h.response(500, "Failed to retrieve configuration"), nil
	}

	cfg, err := config.ParseConfig(configData)
	if err != nil {
		return h.response(500, "Failed to parse configuration"), nil
	}

	// Validate API key
	if !h.keyValidator.ValidateAPIKey(ctx, apiKey, cfg.APIKey) {
		return h.response(401, "Invalid API key"), nil
	}

	// Process check-in
	newCfg, err := h.service.CheckIn(cfg)
	if err != nil {
		return h.response(500, "Failed to process check-in"), nil
	}

	// Save updated configuration
	newConfigData, err := newCfg.ToJSON()
	if err != nil {
		return h.response(500, "Failed to serialize configuration"), nil
	}

	if err := h.configStore.SetConfig(ctx, h.paramName, newConfigData); err != nil {
		return h.response(500, "Failed to save configuration"), nil
	}

	resp := HTTPResponse{
		StatusCode: 200,
		Message:    "Check-in successful",
		NewTimeout: newCfg.Timeout.String(),
	}

	respBody, _ := json.Marshal(resp)
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       string(respBody),
	}, nil
}

func (h *HTTPHandler) response(statusCode int, message string) events.APIGatewayProxyResponse {
	resp := HTTPResponse{
		StatusCode: statusCode,
		Message:    message,
	}
	body, _ := json.Marshal(resp)
	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Body:       string(body),
	}
}
