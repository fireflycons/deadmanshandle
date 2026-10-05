package domain

import (
	"context"
	"time"

	"github.com/fireflycons/deadmanshandle/internal/config"
)

// CheckinResult represents the result of a check-in operation
type CheckinResult struct {
	Success         bool
	Message         string
	NewTimeout      time.Time
	WarningRequired bool
}

// EmailAction represents an email that should be sent
type EmailAction struct {
	To      string
	Subject string
	Body    string
}

// DeadmansHandleService contains the core business logic
type DeadmansHandleService struct {
	// now is read on every call rather than captured once, because a warm
	// Lambda container reuses the service across invocations.
	// For testability, allows injection of a fixed time.
	now func() time.Time
}

// NewDeadmansHandleService creates a new service using the system clock (UTC)
func NewDeadmansHandleService() *DeadmansHandleService {
	return &DeadmansHandleService{
		now: func() time.Time { return time.Now().UTC() },
	}
}

// NewDeadmansHandleServiceWithTime creates a new service with a specific time (for testing)
func NewDeadmansHandleServiceWithTime(t time.Time) *DeadmansHandleService {
	return &DeadmansHandleService{
		now: func() time.Time { return t },
	}
}

// CheckIn handles the owner's check-in via HTTP API
// Returns updated configuration and any warning that should be sent
func (s *DeadmansHandleService) CheckIn(cfg *config.Config) (*config.Config, error) {
	newCfg := *cfg
	newCfg.Timeout = cfg.CalculateNewTimeout(s.now(), cfg.ResetDays)
	return &newCfg, nil
}

// ProcessScheduledEvent handles the EventBridge scheduled event
// Returns emails that should be sent, and whether the document must be
// attached to them. Both are decided from a single reading of the clock so
// they cannot disagree if the timeout falls between two readings.
func (s *DeadmansHandleService) ProcessScheduledEvent(ctx context.Context, cfg *config.Config) ([]EmailAction, bool, error) {
	var emails []EmailAction
	now := s.now()

	// Check if timeout has passed
	timeoutPassed := now.After(cfg.Timeout)
	if timeoutPassed {
		// Send document to each recipient
		for _, recipient := range cfg.Recipients {
			emails = append(emails, EmailAction{
				To:      recipient,
				Subject: "Important Document - Deadman's Handle",
				Body:    "Please find the important document attached. This has been sent as per the deadman's handle protocol.",
			})
		}
	}

	// Check if warning should be sent
	warningThreshold := cfg.Timeout.AddDate(0, 0, -cfg.WarnDays)
	if now.After(warningThreshold) && now.Before(cfg.Timeout) {
		daysUntilTimeout := int(cfg.Timeout.Sub(now).Hours() / 24)
		emails = append(emails, EmailAction{
			To:      cfg.Owner,
			Subject: "Deadman's Handle Check-In Required",
			Body: "Warning: Your deadman's handle will trigger in " +
				string(rune(daysUntilTimeout)) + " days. " +
				"Please check in to reset the timeout.",
		})
	}

	return emails, timeoutPassed, nil
}

// DaysUntilTimeout returns the number of days until timeout
func (s *DeadmansHandleService) DaysUntilTimeout(cfg *config.Config) int {
	now := s.now()
	if now.After(cfg.Timeout) {
		return 0
	}
	return int(cfg.Timeout.Sub(now).Hours() / 24)
}

// IsTimeoutPassed checks if the timeout has passed
func (s *DeadmansHandleService) IsTimeoutPassed(cfg *config.Config) bool {
	return s.now().After(cfg.Timeout)
}
