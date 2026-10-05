package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/fireflycons/deadmanshandle/internal/config"
)

func TestCheckIn(t *testing.T) {
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	service := NewDeadmansHandleServiceWithTime(now)

	cfg := &config.Config{
		Owner:      "owner@example.com",
		Recipients: []string{"recipient1@example.com"},
		ResetDays:  30,
		WarnDays:   7,
		Timeout:    now.AddDate(0, 0, -5), // 5 days ago
		APIKey:     "test-key",
	}

	newCfg, err := service.CheckIn(cfg)
	if err != nil {
		t.Fatalf("CheckIn failed: %v", err)
	}

	expectedTimeout := now.AddDate(0, 0, 30)
	if !newCfg.Timeout.Equal(expectedTimeout) {
		t.Errorf("Expected timeout %v, got %v", expectedTimeout, newCfg.Timeout)
	}
}

func TestDaysUntilTimeout(t *testing.T) {
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	service := NewDeadmansHandleServiceWithTime(now)

	cfg := &config.Config{
		Timeout: now.AddDate(0, 0, 10),
	}

	days := service.DaysUntilTimeout(cfg)
	if days != 10 {
		t.Errorf("Expected 10 days until timeout, got %d", days)
	}
}

func TestIsTimeoutPassed(t *testing.T) {
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	service := NewDeadmansHandleServiceWithTime(now)

	// Timeout in the past
	cfg := &config.Config{
		Timeout: now.AddDate(0, 0, -1),
	}
	if !service.IsTimeoutPassed(cfg) {
		t.Error("Expected timeout to be passed")
	}

	// Timeout in the future
	cfg.Timeout = now.AddDate(0, 0, 1)
	if service.IsTimeoutPassed(cfg) {
		t.Error("Expected timeout to not be passed")
	}
}

func TestProcessScheduledEventTimeoutPassed(t *testing.T) {
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	service := NewDeadmansHandleServiceWithTime(now)

	cfg := &config.Config{
		Owner:      "owner@example.com",
		Recipients: []string{"recipient1@example.com", "recipient2@example.com"},
		ResetDays:  30,
		WarnDays:   7,
		Timeout:    now.AddDate(0, 0, -1), // Timeout is past
		APIKey:     "test-key",
	}

	emails, attachDocument, err := service.ProcessScheduledEvent(t.Context(), cfg)
	if err != nil {
		t.Fatalf("ProcessScheduledEvent failed: %v", err)
	}

	if !attachDocument {
		t.Error("Expected document to be attached when timeout is passed")
	}

	if len(emails) != len(cfg.Recipients) {
		t.Fatalf("Expected %d emails, got %d", len(cfg.Recipients), len(emails))
	}

	for i, recipient := range cfg.Recipients {
		if emails[i].To != recipient {
			t.Errorf("Expected email %d to %s, got %s", i, recipient, emails[i].To)
		}
	}
}

func TestProcessScheduledEventWarning(t *testing.T) {
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	service := NewDeadmansHandleServiceWithTime(now)

	// Timeout is 5 days away, warn days is 7
	cfg := &config.Config{
		Owner:      "owner@example.com",
		Recipients: []string{"recipient1@example.com"},
		ResetDays:  30,
		WarnDays:   7,
		Timeout:    now.AddDate(0, 0, 5),
		APIKey:     "test-key",
	}

	emails, attachDocument, err := service.ProcessScheduledEvent(t.Context(), cfg)
	if err != nil {
		t.Fatalf("ProcessScheduledEvent failed: %v", err)
	}

	if attachDocument {
		t.Error("Expected no document attachment before timeout")
	}

	// Should send warning email to owner
	if len(emails) == 0 {
		t.Error("Expected warning email to be sent")
	}

	if len(emails) > 0 && emails[0].To != cfg.Owner {
		t.Errorf("Expected warning email to owner %s, got %s", cfg.Owner, emails[0].To)
	}

	if len(emails) > 0 && !strings.Contains(emails[0].Body, "trigger in 5 days") {
		t.Errorf("Expected warning body to state 5 days remaining, got %q", emails[0].Body)
	}
}

func TestWarningBody(t *testing.T) {
	timeout := time.Date(2024, 6, 6, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		remaining time.Duration
		want      string
	}{
		{"several days", 5*24*time.Hour + 3*time.Hour, "will trigger in 5 days, on Thursday 6 June 2024 at 12:00 UTC."},
		{"one day", 30 * time.Hour, "will trigger in 1 day, on"},
		{"final day", 23 * time.Hour, "will trigger within the next 24 hours, on"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := warningBody(tt.remaining, timeout)
			if !strings.Contains(got, tt.want) {
				t.Errorf("Expected body to contain %q, got %q", tt.want, got)
			}
		})
	}
}
