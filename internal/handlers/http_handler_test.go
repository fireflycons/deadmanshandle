package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
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
	handler := NewHTTPHandler(configStore, mocks.NewMockStateStore(config.State{}), service, validator, "test-param")

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
		APIKey:     "correct-key",
	}
	configStore.Data["test-param"] = configJSON(t, cfg)

	service := domain.NewDeadmansHandleService()
	validator := mocks.NewMockAPIKeyValidator()
	handler := NewHTTPHandler(configStore, mocks.NewMockStateStore(config.State{}), service, validator, "test-param")

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
		APIKey:     "test-key",
	}
	configStore.Data["test-param"] = configJSON(t, cfg)

	// The handle had triggered and delivered to the recipient
	stateStore := mocks.NewMockStateStore(config.State{
		Timeout:       now.AddDate(0, 0, -5),
		SentTo:        []string{"recipient@example.com"},
		OwnerNotified: true,
	})

	service := domain.NewDeadmansHandleServiceWithTime(now)
	validator := mocks.NewMockAPIKeyValidator()
	handler := NewHTTPHandler(configStore, stateStore, service, validator, "test-param")

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

	// Verify the state was updated and the delivery state cleared
	state := stateStore.State
	expectedTimeout := now.AddDate(0, 0, 30)
	if !state.Timeout.Equal(expectedTimeout) {
		t.Errorf("Expected timeout %v, got %v", expectedTimeout, state.Timeout)
	}
	if state.SentTo != nil || state.OwnerNotified || state.CheckIns != 1 {
		t.Errorf("Expected delivery state cleared and CheckIns 1, got %+v", state)
	}
}

func TestHTTPHandlerErrorPaths(t *testing.T) {
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	validCfg := &config.Config{
		Owner:      "owner@example.com",
		Recipients: []string{"recipient@example.com"},
		ResetDays:  30,
		WarnDays:   7,
		APIKey:     "test-key",
	}
	validData := configJSON(t, validCfg)

	invalidCfg := *validCfg
	invalidCfg.Recipients = nil
	invalidData := configJSON(t, &invalidCfg)

	tests := []struct {
		name        string
		apiKey      string
		stored      []byte
		getErr      error
		checkInErr  error
		wantStatus  int
		wantMessage string
	}{
		{"missing API key", "", validData, nil, nil, 401, "Missing API key"},
		{"wrong API key", "wrong-key", validData, nil, nil, 401, "Invalid API key"},
		{"config read fails", "test-key", validData, errors.New("ssm down"), nil, 500, "Failed to retrieve configuration"},
		{"config missing", "test-key", nil, nil, nil, 500, "Failed to parse configuration"},
		{"config invalid", "test-key", invalidData, nil, nil, 500, "Failed to parse configuration"},
		{"check-in save fails", "test-key", validData, nil, errors.New("dynamodb down"), 500, "Failed to save check-in"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configStore := mocks.NewMockConfigStore()
			configStore.Data["test-param"] = tt.stored
			configStore.GetErr = tt.getErr
			initial := config.State{Timeout: now.AddDate(0, 0, 10)}
			stateStore := mocks.NewMockStateStore(initial)
			stateStore.CheckInErr = tt.checkInErr
			handler := NewHTTPHandler(configStore, stateStore, domain.NewDeadmansHandleServiceWithTime(now), mocks.NewMockAPIKeyValidator(), "test-param")

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

			// A failed check-in must not change the state
			if !reflect.DeepEqual(stateStore.State, initial) {
				t.Errorf("State changed: %+v", stateStore.State)
			}
		})
	}
}
