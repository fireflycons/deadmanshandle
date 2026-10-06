package domain

import (
	"context"
	"slices"
	"strconv"
	"strings"
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

// EmailKind identifies why an email is being sent
type EmailKind int

const (
	// EmailDocument delivers the document to a recipient
	EmailDocument EmailKind = iota
	// EmailTriggerNotice tells the owner that the document has been sent
	EmailTriggerNotice
	// EmailWarning reminds the owner to check in
	EmailWarning
	// EmailDocumentMissing warns the owner, before the timeout, that the
	// document is missing and could not be sent
	EmailDocumentMissing
	// EmailDeliveryBlocked tells the owner that the timeout has passed but
	// the document is missing, so nothing was sent to the recipients
	EmailDeliveryBlocked
)

// Document describes the document to be sent, as found by the scheduled run
type Document struct {
	Location  string // Where the owner should upload it, e.g. s3://bucket/key
	Available bool
}

// EmailAction represents an email that should be sent
type EmailAction struct {
	Kind    EmailKind
	To      string
	Subject string
	Body    string
}

// AttachDocument reports whether the document should be attached
func (e EmailAction) AttachDocument() bool {
	return e.Kind == EmailDocument
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
	newCfg.Timeout = s.now().AddDate(0, 0, cfg.ResetDays)
	newCfg.SentTo = nil
	newCfg.OwnerNotified = false
	return &newCfg, nil
}

// ProcessScheduledEvent handles the EventBridge scheduled event
// Returns emails that should be sent. All decisions are made from a single
// reading of the clock so they cannot disagree if the timeout falls between
// two readings.
//
// Once the timeout has passed, the document is sent only to recipients not
// yet recorded in cfg.SentTo, and the owner is notified once. Call
// RecordSent after each successful send so that later runs skip it.
//
// If the document is missing, the owner is warned instead: daily before the
// timeout, and with an EmailDeliveryBlocked notice on each run after it while
// recipients are still waiting.
func (s *DeadmansHandleService) ProcessScheduledEvent(ctx context.Context, cfg *config.Config, doc Document) ([]EmailAction, error) {
	var emails []EmailAction
	now := s.now()

	// Check if timeout has passed
	if now.After(cfg.Timeout) {
		var pending []string
		for _, recipient := range cfg.Recipients {
			if !slices.Contains(cfg.SentTo, recipient) {
				pending = append(pending, recipient)
			}
		}

		if len(pending) > 0 && !doc.Available {
			// The usual trigger notice waits until the document is sent
			emails = append(emails, EmailAction{
				Kind:    EmailDeliveryBlocked,
				To:      cfg.Owner,
				Subject: "Deadman's Handle Triggered - Document Missing",
				Body:    deliveryBlockedBody(cfg.Timeout, doc.Location, pending),
			})
		} else {
			// Send document to each recipient that has not yet received it
			for _, recipient := range pending {
				emails = append(emails, EmailAction{
					Kind:    EmailDocument,
					To:      recipient,
					Subject: "Important Document - Deadman's Handle",
					Body:    "Please find the important document attached. This has been sent as per the deadman's handle protocol.",
				})
			}

			if !cfg.OwnerNotified {
				emails = append(emails, EmailAction{
					Kind:    EmailTriggerNotice,
					To:      cfg.Owner,
					Subject: "Deadman's Handle Triggered",
					Body:    triggerNoticeBody(cfg.Timeout, cfg.Recipients),
				})
			}
		}
	} else if !doc.Available {
		emails = append(emails, EmailAction{
			Kind:    EmailDocumentMissing,
			To:      cfg.Owner,
			Subject: "Deadman's Handle Document Missing",
			Body:    documentMissingBody(cfg.Timeout, doc.Location),
		})
	}

	// Check if warning should be sent
	warningThreshold := cfg.Timeout.AddDate(0, 0, -cfg.WarnDays)
	if now.After(warningThreshold) && now.Before(cfg.Timeout) {
		emails = append(emails, EmailAction{
			Kind:    EmailWarning,
			To:      cfg.Owner,
			Subject: "Deadman's Handle Check-In Required",
			Body:    warningBody(cfg.Timeout.Sub(now), cfg.Timeout),
		})
	}

	return emails, nil
}

// RecordSent updates the delivery state in cfg after email was sent
// successfully. It reports whether cfg changed and needs to be saved.
func (s *DeadmansHandleService) RecordSent(cfg *config.Config, email EmailAction) bool {
	switch email.Kind {
	case EmailDocument:
		if slices.Contains(cfg.SentTo, email.To) {
			return false
		}
		cfg.SentTo = append(cfg.SentTo, email.To)
		return true
	case EmailTriggerNotice:
		if cfg.OwnerNotified {
			return false
		}
		cfg.OwnerNotified = true
		return true
	default:
		return false
	}
}

// triggerNoticeBody tells the owner that the document is being sent, so
// that a false trigger can be noticed and followed up.
func triggerNoticeBody(timeout time.Time, recipients []string) string {
	return "Your deadman's handle check-in deadline of " +
		timeout.UTC().Format("Monday 2 January 2006 at 15:04 MST") +
		" passed without a check-in, so your document is being sent to:\n\n" +
		"  " + strings.Join(recipients, "\n  ") + "\n\n" +
		"If this is a mistake, check in to reset the timeout and contact the recipients."
}

// documentMissingBody warns the owner, before the timeout, that there is
// nothing to send.
func documentMissingBody(timeout time.Time, location string) string {
	return "Warning: The document for your deadman's handle is missing from " + location + ". " +
		"If the handle triggers, on " + timeout.UTC().Format("Monday 2 January 2006 at 15:04 MST") +
		", nothing can be sent to your recipients.\n\n" +
		"Upload the document to fix this. You will be reminded daily until you do."
}

// deliveryBlockedBody tells the owner that the handle has triggered but the
// document could not be sent.
func deliveryBlockedBody(timeout time.Time, location string, pending []string) string {
	return "Your deadman's handle check-in deadline of " +
		timeout.UTC().Format("Monday 2 January 2006 at 15:04 MST") +
		" passed without a check-in, but the document is missing from " + location +
		", so it could not be sent to:\n\n" +
		"  " + strings.Join(pending, "\n  ") + "\n\n" +
		"Upload the document and it will be sent on the next daily run. " +
		"If this is a mistake, check in to reset the timeout."
}

// warningBody describes how long the owner has left to check in. The day
// count is rounded down, so the final day reads "within the next 24 hours"
// rather than "in 0 days", and the exact deadline removes any ambiguity.
func warningBody(remaining time.Duration, timeout time.Time) string {
	var when string
	switch days := int(remaining.Hours() / 24); days {
	case 0:
		when = "within the next 24 hours"
	case 1:
		when = "in 1 day"
	default:
		when = "in " + strconv.Itoa(days) + " days"
	}

	return "Warning: Your deadman's handle will trigger " + when +
		", on " + timeout.UTC().Format("Monday 2 January 2006 at 15:04 MST") + ". " +
		"Please check in before then to reset the timeout."
}
