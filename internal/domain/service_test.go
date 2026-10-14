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
		APIKey:     "test-key",
	}

	expectedTimeout := now.AddDate(0, 0, 30)
	if got := service.CheckIn(cfg); !got.Equal(expectedTimeout) {
		t.Errorf("Expected timeout %v, got %v", expectedTimeout, got)
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
		APIKey:     "test-key",
	}
	state := &config.State{Timeout: now.AddDate(0, 0, -1)} // Timeout is past

	emails, err := service.ProcessScheduledEvent(t.Context(), cfg, state, testDocument)
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
		Owner:      "owner@example.com",
		Recipients: []string{"recipient1@example.com", "recipient2@example.com"},
	}
	state := &config.State{
		Timeout:       now.AddDate(0, 0, -1),
		SentTo:        []string{"recipient1@example.com"},
		OwnerNotified: true,
	}

	emails, err := service.ProcessScheduledEvent(t.Context(), cfg, state, testDocument)
	if err != nil {
		t.Fatalf("ProcessScheduledEvent failed: %v", err)
	}

	if len(emails) != 1 || emails[0].To != "recipient2@example.com" {
		t.Fatalf("Expected only recipient2 to be retried, got %+v", emails)
	}

	// Everything delivered: nothing more to send
	state.SentTo = append(state.SentTo, "recipient2@example.com")
	emails, err = service.ProcessScheduledEvent(t.Context(), cfg, state, testDocument)
	if err != nil {
		t.Fatalf("ProcessScheduledEvent failed: %v", err)
	}
	if len(emails) != 0 {
		t.Errorf("Expected no emails once all delivered, got %+v", emails)
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
		APIKey:     "test-key",
	}
	state := &config.State{Timeout: now.AddDate(0, 0, 5)}

	emails, err := service.ProcessScheduledEvent(t.Context(), cfg, state, testDocument)
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
				Owner:      "owner@example.com",
				Recipients: []string{"recipient1@example.com", "recipient2@example.com"},
				ResetDays:  30,
				WarnDays:   7,
				APIKey:     "test-key",
			}
			state := &config.State{Timeout: tt.timeout, SentTo: tt.sentTo, OwnerNotified: tt.ownerNotified}

			emails, err := NewDeadmansHandleServiceWithTime(now).ProcessScheduledEvent(t.Context(), cfg, state, missing)
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

func TestDocumentChanged(t *testing.T) {
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	service := NewDeadmansHandleServiceWithTime(now)
	present := Document{Location: "s3://bucket/document.pdf", Available: true, ETag: `"new"`}
	missing := Document{Location: "s3://bucket/document.pdf"}

	tests := []struct {
		name         string
		state        config.State
		doc          Document
		notify, save bool
	}{
		{"first document is recorded silently", config.State{}, present, false, true},
		{"same content", config.State{DocumentETag: `"new"`}, present, false, false},
		{"changed content", config.State{DocumentETag: `"old"`}, present, true, true},
		{"missing keeps the recorded ETag", config.State{DocumentETag: `"old"`}, missing, false, false},
		{"missing before any document", config.State{}, missing, false, false},
		{"pending notice, too soon to resend",
			config.State{DocumentETag: `"new"`, DocumentChangePending: true, DocumentChangedAt: now.Add(-ChangeNoticeRetryAfter + time.Second)},
			present, false, false},
		{"pending notice is resent",
			config.State{DocumentETag: `"new"`, DocumentChangePending: true, DocumentChangedAt: now.Add(-ChangeNoticeRetryAfter)},
			present, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notify, save := service.DocumentChanged(&tt.state, tt.doc)
			if notify != tt.notify || save != tt.save {
				t.Errorf("Expected notify=%v record=%v, got notify=%v record=%v", tt.notify, tt.save, notify, save)
			}
		})
	}
}

func TestDocumentChangedEmail(t *testing.T) {
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	service := NewDeadmansHandleServiceWithTime(now)
	cfg := &config.Config{Owner: "owner@example.com"}
	state := &config.State{Timeout: now.AddDate(0, 0, 10)}

	tests := []struct {
		name       string
		origin     *ChangeOrigin
		wantOrigin bool
	}{
		{"from an S3 event", &ChangeOrigin{Requester: "123456789012", SourceIP: "192.0.2.1"}, true},
		{"from the daily run", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			email := service.DocumentChangedEmail(cfg, state, testDocument, now, tt.origin)
			if email.To != "owner@example.com" || email.Kind != EmailDocumentChanged || email.AttachDocument() {
				t.Errorf("Expected an owner notice without attachment, got %+v", email)
			}
			for _, want := range []string{testDocument.Location, "noticed on Saturday 1 June 2024 at 12:00 UTC",
				"triggers on Tuesday 11 June 2024 at 12:00 UTC", "before Sunday 2 June 2024 at 12:00 UTC, delivery to your recipients is held"} {
				if !strings.Contains(email.Body, want) {
					t.Errorf("Expected body to contain %q, got %q", want, email.Body)
				}
			}
			if got := strings.Contains(email.Body, "AWS account 123456789012 from IP address 192.0.2.1"); got != tt.wantOrigin {
				t.Errorf("Expected origin in body %v, got %q", tt.wantOrigin, email.Body)
			}
		})
	}
}

func TestProcessScheduledEventDeliveryHeld(t *testing.T) {
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	cfg := &config.Config{
		Owner:      "owner@example.com",
		Recipients: []string{"recipient1@example.com", "recipient2@example.com"},
		ResetDays:  30,
		WarnDays:   7,
	}
	missing := Document{Location: testDocument.Location}

	tests := []struct {
		name      string
		timeout   time.Time
		changedAt time.Time
		sentTo    []string
		doc       Document
		wantKinds []EmailKind
	}{
		{"never changed", now.AddDate(0, 0, -1), time.Time{}, nil, testDocument,
			[]EmailKind{EmailDocument, EmailDocument, EmailTriggerNotice}},
		{"changed an hour ago", now.AddDate(0, 0, -1), now.Add(-time.Hour), nil, testDocument,
			[]EmailKind{EmailDeliveryHeld}},
		{"changed just under the hold period ago", now.AddDate(0, 0, -1), now.Add(-ChangeHoldPeriod + time.Minute), nil, testDocument,
			[]EmailKind{EmailDeliveryHeld}},
		{"changed over the hold period ago", now.AddDate(0, 0, -1), now.Add(-ChangeHoldPeriod - time.Minute), nil, testDocument,
			[]EmailKind{EmailDocument, EmailDocument, EmailTriggerNotice}},
		{"held with one recipient already sent", now.AddDate(0, 0, -1), now.Add(-time.Hour), []string{"recipient1@example.com"}, testDocument,
			[]EmailKind{EmailDeliveryHeld}},
		{"missing document takes precedence", now.AddDate(0, 0, -1), now.Add(-time.Hour), nil, missing,
			[]EmailKind{EmailDeliveryBlocked}},
		{"no effect before the timeout", now.AddDate(0, 0, 20), now.Add(-time.Hour), nil, testDocument,
			nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &config.State{Timeout: tt.timeout, SentTo: tt.sentTo, DocumentChangedAt: tt.changedAt}
			emails, err := NewDeadmansHandleServiceWithTime(now).ProcessScheduledEvent(t.Context(), cfg, state, tt.doc)
			if err != nil {
				t.Fatalf("ProcessScheduledEvent failed: %v", err)
			}

			var kinds []EmailKind
			for _, email := range emails {
				kinds = append(kinds, email.Kind)
			}
			if !slices.Equal(kinds, tt.wantKinds) {
				t.Fatalf("Expected email kinds %v, got %v", tt.wantKinds, kinds)
			}
			if len(kinds) > 0 && kinds[0] == EmailDeliveryHeld {
				heldUntil := tt.changedAt.Add(ChangeHoldPeriod).Format("Monday 2 January 2006 at 15:04 MST")
				if !strings.Contains(emails[0].Body, "first daily run after "+heldUntil) || !strings.Contains(emails[0].Body, "recipient2@example.com") {
					t.Errorf("Expected the hold's end and pending recipients in the body, got %q", emails[0].Body)
				}
			}
		})
	}
}
