package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/fireflycons/deadmanshandle/internal/config"
	"github.com/fireflycons/deadmanshandle/internal/domain"
	"github.com/fireflycons/deadmanshandle/internal/mocks"
)

func TestHTTPHandlerMissingAPIKey(t *testing.T) {
	configStore := mocks.NewMockConfigStore()
	service := domain.NewDeadmansHandleService()
	validator := mocks.NewMockAPIKeyValidator()
	handler := NewHTTPHandler(configStore, service, validator, "test-param")

	request := events.APIGatewayV2HTTPRequest{
		Headers: make(map[string]string),
	}

	response, err := handler.Handle(context.Background(), request)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	if response.StatusCode != 401 {
		t.Errorf("Expected status 401, got %d", response.StatusCode)
	}
}

func TestHTTPHandlerInvalidAPIKey(t *testing.T) {
	configStore := mocks.NewMockConfigStore()
	cfg := &config.Config{
		Owner:      "owner@example.com",
		Recipients: []string{"recipient@example.com"},
		ResetDays:  30,
		WarnDays:   7,
		Timeout:    time.Now().AddDate(0, 0, 10),
		APIKey:     "correct-key",
	}
	cfgData, _ := cfg.ToJSON()
	configStore.Data["test-param"] = cfgData

	service := domain.NewDeadmansHandleService()
	validator := mocks.NewMockAPIKeyValidator()
	handler := NewHTTPHandler(configStore, service, validator, "test-param")

	request := events.APIGatewayV2HTTPRequest{
		Headers: map[string]string{
			"x-api-key": "wrong-key",
		},
	}

	response, err := handler.Handle(context.Background(), request)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	if response.StatusCode != 401 {
		t.Errorf("Expected status 401, got %d", response.StatusCode)
	}
}

func TestHTTPHandlerSuccessfulCheckin(t *testing.T) {
	configStore := mocks.NewMockConfigStore()
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	cfg := &config.Config{
		Owner:      "owner@example.com",
		Recipients: []string{"recipient@example.com"},
		ResetDays:  30,
		WarnDays:   7,
		Timeout:    now.AddDate(0, 0, -5),
		APIKey:     "test-key",
	}
	cfgData, _ := cfg.ToJSON()
	configStore.Data["test-param"] = cfgData

	service := domain.NewDeadmansHandleServiceWithTime(now)
	validator := mocks.NewMockAPIKeyValidator()
	handler := NewHTTPHandler(configStore, service, validator, "test-param")

	request := events.APIGatewayV2HTTPRequest{
		Headers: map[string]string{
			"x-api-key": "test-key",
		},
	}

	response, err := handler.Handle(context.Background(), request)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	if response.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", response.StatusCode)
	}

	var respBody HTTPResponse
	if err := json.Unmarshal([]byte(response.Body), &respBody); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if respBody.Message != "Check-in successful" {
		t.Errorf("Expected success message, got %s", respBody.Message)
	}
	if respBody.NewTimeout != "2024-07-01T12:00:00Z" {
		t.Errorf("Expected RFC 3339 newTimeout 2024-07-01T12:00:00Z, got %s", respBody.NewTimeout)
	}

	// Verify config was updated
	updatedCfgData := configStore.Data["test-param"]
	updatedCfg, _ := config.ParseConfig(updatedCfgData)
	expectedTimeout := now.AddDate(0, 0, 30)
	if !updatedCfg.Timeout.Equal(expectedTimeout) {
		t.Errorf("Expected timeout %v, got %v", expectedTimeout, updatedCfg.Timeout)
	}
}

func TestHTTPHandlerErrorPaths(t *testing.T) {
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	validCfg := &config.Config{
		Owner:      "owner@example.com",
		Recipients: []string{"recipient@example.com"},
		ResetDays:  30,
		WarnDays:   7,
		Timeout:    now.AddDate(0, 0, 10),
		APIKey:     "test-key",
	}
	validData, _ := validCfg.ToJSON()

	invalidCfg := *validCfg
	invalidCfg.Recipients = nil
	invalidData, _ := invalidCfg.ToJSON()

	tests := []struct {
		name        string
		apiKey      string
		stored      []byte
		getErr      error
		setErr      error
		wantStatus  int
		wantMessage string
	}{
		{"missing API key", "", validData, nil, nil, 401, "Missing API key"},
		{"wrong API key", "wrong-key", validData, nil, nil, 401, "Invalid API key"},
		{"config read fails", "test-key", validData, errors.New("ssm down"), nil, 500, "Failed to retrieve configuration"},
		{"config missing", "test-key", nil, nil, nil, 500, "Failed to parse configuration"},
		{"config invalid", "test-key", invalidData, nil, nil, 500, "Failed to parse configuration"},
		{"config save fails", "test-key", validData, nil, errors.New("ssm down"), 500, "Failed to save configuration"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configStore := mocks.NewMockConfigStore()
			configStore.Data["test-param"] = tt.stored
			configStore.GetErr = tt.getErr
			configStore.SetErr = tt.setErr
			handler := NewHTTPHandler(configStore, domain.NewDeadmansHandleServiceWithTime(now), mocks.NewMockAPIKeyValidator(), "test-param")

			request := events.APIGatewayV2HTTPRequest{Headers: map[string]string{}}
			if tt.apiKey != "" {
				request.Headers["x-api-key"] = tt.apiKey
			}

			response, err := handler.Handle(context.Background(), request)
			if err != nil {
				t.Fatalf("Handle returned an error instead of a response: %v", err)
			}
			if response.StatusCode != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, response.StatusCode)
			}

			var body HTTPResponse
			if err := json.Unmarshal([]byte(response.Body), &body); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}
			if body.Message != tt.wantMessage || body.NewTimeout != "" {
				t.Errorf("Expected message %q and no newTimeout, got %+v", tt.wantMessage, body)
			}

			// A failed check-in must not change the stored config
			if !bytes.Equal(configStore.Data["test-param"], tt.stored) {
				t.Errorf("Stored config changed:\n%s", configStore.Data["test-param"])
			}
		})
	}
}
