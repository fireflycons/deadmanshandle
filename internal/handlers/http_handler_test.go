package handlers

import (
	"context"
	"encoding/json"
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
	validator := mocks.NewMockAPIKeyValidator("test-key")
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
	validator := mocks.NewMockAPIKeyValidator("correct-key")
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
	validator := mocks.NewMockAPIKeyValidator("test-key")
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
