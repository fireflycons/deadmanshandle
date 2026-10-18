package cli

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fireflycons/deadmanshandle/internal/config"
	"github.com/fireflycons/deadmanshandle/internal/domain"
	"github.com/fireflycons/deadmanshandle/internal/mocks"
)

const testParam = "/deadmanshandle/config"

var testNow = time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

func testConfig() *config.Config {
	return &config.Config{
		Owner:       "owner@example.com",
		Recipients:  []string{"a@example.com", "b@example.com"},
		ResetDays:   30,
		WarnDays:    7,
		APIKey:      "secret-api-key",
		DocumentKey: "docs/will.pdf",
	}
}

func configStoreWith(t *testing.T, cfg any) *mocks.MockConfigStore {
	t.Helper()
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	store := mocks.NewMockConfigStore()
	store.Data[testParam] = data
	return store
}

func TestCheckIn(t *testing.T) {
	stateStore := mocks.NewMockStateStore(config.State{
		Timeout:       testNow.Add(-time.Hour),
		CheckIns:      4,
		SentTo:        []string{"a@example.com"},
		OwnerNotified: true,
		PushToken:     "token",
	})
	service := domain.NewDeadmansHandleServiceWithTime(testNow)

	timeout, err := CheckIn(context.Background(), configStoreWith(t, testConfig()), testParam, stateStore, service)
	if err != nil {
		t.Fatalf("CheckIn failed: %v", err)
	}

	want := testNow.AddDate(0, 0, 30)
	state := stateStore.State
	if !timeout.Equal(want) || !state.Timeout.Equal(want) {
		t.Errorf("timeout = %v, state %v, want %v", timeout, state.Timeout, want)
	}
	if state.CheckIns != 5 || state.SentTo != nil || state.OwnerNotified || state.PushToken != "token" {
		t.Errorf("Expected the delivery state cleared and the push token kept, got %+v", state)
	}
}

func TestCheckInRefusesInvalidConfig(t *testing.T) {
	cfg := testConfig()
	cfg.Recipients = nil
	stateStore := mocks.NewMockStateStore(config.State{Timeout: testNow})

	_, err := CheckIn(context.Background(), configStoreWith(t, cfg), testParam, stateStore,
		domain.NewDeadmansHandleServiceWithTime(testNow))
	if err == nil || !strings.Contains(err.Error(), "recipients is empty") {
		t.Errorf("Expected the config's problem, got %v", err)
	}
	if stateStore.State.CheckIns != 0 {
		t.Error("Expected no check-in")
	}
}

func TestCheckInReportsSaveFailure(t *testing.T) {
	saveErr := errors.New("throttled")
	stateStore := mocks.NewMockStateStore(config.State{Timeout: testNow})
	stateStore.CheckInErr = saveErr

	_, err := CheckIn(context.Background(), configStoreWith(t, testConfig()), testParam, stateStore,
		domain.NewDeadmansHandleServiceWithTime(testNow))
	if !errors.Is(err, saveErr) {
		t.Errorf("Expected the save failure, got %v", err)
	}
}
