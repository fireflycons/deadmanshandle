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
	now time.Time // For testability, allows injection of current time
}

// NewDeadmansHandleService creates a new service with current time
func NewDeadmansHandleService() *DeadmansHandleService {
	return &DeadmansHandleService{
		now: time.Now().UTC(),
	}
}

// NewDeadmansHandleServiceWithTime creates a new service with a specific time (for testing)
func NewDeadmansHandleServiceWithTime(t time.Time) *DeadmansHandleService {
	return &DeadmansHandleService{
		now: t,
	}
}

// CheckIn handles the owner's check-in via HTTP API
// Returns updated configuration and any warning that should be sent
func (s *DeadmansHandleService) CheckIn(cfg *config.Config) (*config.Config, error) {
	newCfg := *cfg
	newCfg.Timeout = cfg.CalculateNewTimeout(s.now, cfg.ResetDays)
	return &newCfg, nil
}

// ProcessScheduledEvent handles the EventBridge scheduled event
// Returns emails that should be sent
func (s *DeadmansHandleService) ProcessScheduledEvent(ctx context.Context, cfg *config.Config) ([]EmailAction, error) {
	var emails []EmailAction

	// Check if timeout has passed
	if s.now.After(cfg.Timeout) {
		// Send document to recipients
		emails = append(emails, EmailAction{
			To:      cfg.Recipients[0], // Will be sent to all via batch API
			Subject: "Important Document - Deadman's Handle",
			Body:    "Please find the important document attached. This has been sent as per the deadman's handle protocol.",
		})
	}

	// Check if warning should be sent
	warningThreshold := cfg.Timeout.AddDate(0, 0, -cfg.WarnDays)
	if s.now.After(warningThreshold) && s.now.Before(cfg.Timeout) {
		daysUntilTimeout := int(cfg.Timeout.Sub(s.now).Hours() / 24)
		emails = append(emails, EmailAction{
			To:      cfg.Owner,
			Subject: "Deadman's Handle Check-In Required",
			Body: "Warning: Your deadman's handle will trigger in " +
				string(rune(daysUntilTimeout)) + " days. " +
				"Please check in to reset the timeout.",
		})
	}

	return emails, nil
}

// DaysUntilTimeout returns the number of days until timeout
func (s *DeadmansHandleService) DaysUntilTimeout(cfg *config.Config) int {
	if s.now.After(cfg.Timeout) {
		return 0
	}
	return int(cfg.Timeout.Sub(s.now).Hours() / 24)
}

// IsTimeoutPassed checks if the timeout has passed
func (s *DeadmansHandleService) IsTimeoutPassed(cfg *config.Config) bool {
	return s.now.After(cfg.Timeout)
}
