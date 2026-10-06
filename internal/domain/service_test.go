package domain

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fireflycons/deadmanshandle/internal/config"
)

// testDocument is a document that is present
var testDocument = Document{Location: "s3://bucket/document.pdf", Available: true}

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

	emails, err := service.ProcessScheduledEvent(t.Context(), cfg, testDocument)
	if err != nil {
		t.Fatalf("ProcessScheduledEvent failed: %v", err)
	}

	// One document email per recipient, then one notice to the owner
	if len(emails) != len(cfg.Recipients)+1 {
		t.Fatalf("Expected %d emails, got %d", len(cfg.Recipients)+1, len(emails))
	}

	for i, recipient := range cfg.Recipients {
		if emails[i].To != recipient {
			t.Errorf("Expected email %d to %s, got %s", i, recipient, emails[i].To)
		}
		if !emails[i].AttachDocument() {
			t.Errorf("Expected document attached to email %d", i)
		}
	}

	notice := emails[len(cfg.Recipients)]
	if notice.Kind != EmailTriggerNotice || notice.To != cfg.Owner {
		t.Errorf("Expected trigger notice to owner, got kind %d to %s", notice.Kind, notice.To)
	}
	if notice.AttachDocument() {
		t.Error("Expected no document attached to the owner's notice")
	}
	for _, recipient := range cfg.Recipients {
		if !strings.Contains(notice.Body, recipient) {
			t.Errorf("Expected notice to list %s, got %q", recipient, notice.Body)
		}
	}
}

func TestProcessScheduledEventSkipsCompletedDeliveries(t *testing.T) {
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	service := NewDeadmansHandleServiceWithTime(now)

	cfg := &config.Config{
		Owner:         "owner@example.com",
		Recipients:    []string{"recipient1@example.com", "recipient2@example.com"},
		Timeout:       now.AddDate(0, 0, -1),
		SentTo:        []string{"recipient1@example.com"},
		OwnerNotified: true,
	}

	emails, err := service.ProcessScheduledEvent(t.Context(), cfg, testDocument)
	if err != nil {
		t.Fatalf("ProcessScheduledEvent failed: %v", err)
	}

	if len(emails) != 1 || emails[0].To != "recipient2@example.com" {
		t.Fatalf("Expected only recipient2 to be retried, got %+v", emails)
	}

	// Everything delivered: nothing more to send
	cfg.SentTo = append(cfg.SentTo, "recipient2@example.com")
	emails, err = service.ProcessScheduledEvent(t.Context(), cfg, testDocument)
	if err != nil {
		t.Fatalf("ProcessScheduledEvent failed: %v", err)
	}
	if len(emails) != 0 {
		t.Errorf("Expected no emails once all delivered, got %+v", emails)
	}
}

func TestRecordSent(t *testing.T) {
	service := NewDeadmansHandleServiceWithTime(time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC))
	cfg := &config.Config{}

	document := EmailAction{Kind: EmailDocument, To: "recipient@example.com"}
	if !service.RecordSent(cfg, document) {
		t.Error("Expected first document send to change config")
	}
	if service.RecordSent(cfg, document) {
		t.Error("Expected repeated document send not to change config")
	}
	if len(cfg.SentTo) != 1 || cfg.SentTo[0] != "recipient@example.com" {
		t.Errorf("Expected SentTo to hold the recipient once, got %v", cfg.SentTo)
	}

	notice := EmailAction{Kind: EmailTriggerNotice, To: "owner@example.com"}
	if !service.RecordSent(cfg, notice) || !cfg.OwnerNotified {
		t.Error("Expected trigger notice to set OwnerNotified")
	}

	if service.RecordSent(cfg, EmailAction{Kind: EmailWarning, To: "owner@example.com"}) {
		t.Error("Expected warning not to change config")
	}
}

func TestCheckInClearsDeliveryState(t *testing.T) {
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	service := NewDeadmansHandleServiceWithTime(now)

	cfg := &config.Config{
		ResetDays:     30,
		Timeout:       now.AddDate(0, 0, -1),
		SentTo:        []string{"recipient@example.com"},
		OwnerNotified: true,
	}

	newCfg, err := service.CheckIn(cfg)
	if err != nil {
		t.Fatalf("CheckIn failed: %v", err)
	}

	if newCfg.SentTo != nil || newCfg.OwnerNotified {
		t.Errorf("Expected delivery state cleared, got SentTo=%v OwnerNotified=%v", newCfg.SentTo, newCfg.OwnerNotified)
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

	emails, err := service.ProcessScheduledEvent(t.Context(), cfg, testDocument)
	if err != nil {
		t.Fatalf("ProcessScheduledEvent failed: %v", err)
	}

	if len(emails) > 0 && emails[0].AttachDocument() {
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

func TestProcessScheduledEventDocumentMissing(t *testing.T) {
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	missing := Document{Location: "s3://bucket/document.pdf", Available: false}

	tests := []struct {
		name          string
		timeout       time.Time
		sentTo        []string
		ownerNotified bool
		wantKinds     []EmailKind
		wantInBody    string
	}{
		{"well before timeout", now.AddDate(0, 0, 20), nil, false,
			[]EmailKind{EmailDocumentMissing}, "missing from s3://bucket/document.pdf"},
		{"within warnDays", now.AddDate(0, 0, 5), nil, false,
			[]EmailKind{EmailDocumentMissing, EmailWarning}, "nothing can be sent to your recipients"},
		{"after timeout", now.AddDate(0, 0, -1), nil, false,
			[]EmailKind{EmailDeliveryBlocked}, "could not be sent to:\n\n  recipient1@example.com\n  recipient2@example.com\n"},
		{"after timeout, one recipient already sent", now.AddDate(0, 0, -1), []string{"recipient1@example.com"}, true,
			[]EmailKind{EmailDeliveryBlocked}, "could not be sent to:\n\n  recipient2@example.com\n"},
		{"after timeout, all sent but owner notice pending", now.AddDate(0, 0, -1), []string{"recipient1@example.com", "recipient2@example.com"}, false,
			[]EmailKind{EmailTriggerNotice}, "is being sent to"},
		{"after timeout, all done", now.AddDate(0, 0, -1), []string{"recipient1@example.com", "recipient2@example.com"}, true,
			nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Owner:         "owner@example.com",
				Recipients:    []string{"recipient1@example.com", "recipient2@example.com"},
				ResetDays:     30,
				WarnDays:      7,
				Timeout:       tt.timeout,
				APIKey:        "test-key",
				SentTo:        tt.sentTo,
				OwnerNotified: tt.ownerNotified,
			}

			emails, err := NewDeadmansHandleServiceWithTime(now).ProcessScheduledEvent(t.Context(), cfg, missing)
			if err != nil {
				t.Fatalf("ProcessScheduledEvent failed: %v", err)
			}

			var kinds []EmailKind
			for _, email := range emails {
				kinds = append(kinds, email.Kind)
				if email.To != cfg.Owner || email.AttachDocument() {
					t.Errorf("Expected only owner emails without the document, got %+v", email)
				}
			}
			if !slices.Equal(kinds, tt.wantKinds) {
				t.Fatalf("Expected email kinds %v, got %v", tt.wantKinds, kinds)
			}
			if tt.wantInBody != "" && !strings.Contains(emails[0].Body, tt.wantInBody) {
				t.Errorf("Expected body to contain %q, got %q", tt.wantInBody, emails[0].Body)
			}
		})
	}
}
