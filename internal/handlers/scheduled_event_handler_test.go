package handlers

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fireflycons/deadmanshandle/internal/config"
	"github.com/fireflycons/deadmanshandle/internal/domain"
	"github.com/fireflycons/deadmanshandle/internal/mocks"
)

const (
	testParam  = "test-param"
	testBucket = "test-bucket"
	testKey    = "docs/document.pdf"
)

var testNow = time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

func newScheduledTest(t *testing.T, cfg *config.Config) (*ScheduledEventHandler, *mocks.MockConfigStore, *mocks.MockEmailSender) {
	t.Helper()

	configStore := mocks.NewMockConfigStore()
	cfgData, err := cfg.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}
	configStore.Data[testParam] = cfgData

	documentStore := mocks.NewMockDocumentStore()
	documentStore.SetDocument(testBucket, testKey, []byte("the document"))

	emailSender := mocks.NewMockEmailSender()
	service := domain.NewDeadmansHandleServiceWithTime(testNow)
	handler := NewScheduledEventHandler(configStore, documentStore, emailSender, service, testParam, testBucket, testKey)

	return handler, configStore, emailSender
}

func storedConfig(t *testing.T, store *mocks.MockConfigStore) *config.Config {
	t.Helper()
	cfg, err := config.ParseConfig(store.Data[testParam])
	if err != nil {
		t.Fatalf("ParseConfig failed: %v", err)
	}
	return cfg
}

func triggeredConfig() *config.Config {
	return &config.Config{
		Owner:      "owner@example.com",
		Recipients: []string{"recipient1@example.com", "recipient2@example.com"},
		ResetDays:  30,
		WarnDays:   7,
		Timeout:    testNow.AddDate(0, 0, -1),
		APIKey:     "test-key",
	}
}

func TestScheduledHandlerSendsDocumentOnce(t *testing.T) {
	handler, configStore, emailSender := newScheduledTest(t, triggeredConfig())

	if err := handler.Handle(context.Background()); err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	if len(emailSender.SentEmails) != 3 {
		t.Fatalf("Expected 2 document emails and 1 owner notice, got %d emails", len(emailSender.SentEmails))
	}
	for _, email := range emailSender.SentEmails {
		attached := email.Attachments["document.pdf"] != nil
		if wantAttached := email.To != "owner@example.com"; attached != wantAttached {
			t.Errorf("Email to %s: expected attachment %v, got %v", email.To, wantAttached, attached)
		}
	}

	cfg := storedConfig(t, configStore)
	if !slices.Equal(cfg.SentTo, []string{"recipient1@example.com", "recipient2@example.com"}) || !cfg.OwnerNotified {
		t.Errorf("Expected delivery recorded, got SentTo=%v OwnerNotified=%v", cfg.SentTo, cfg.OwnerNotified)
	}

	// The next daily run sends nothing
	emailSender.SentEmails = nil
	if err := handler.Handle(context.Background()); err != nil {
		t.Fatalf("Second Handle failed: %v", err)
	}
	if len(emailSender.SentEmails) != 0 {
		t.Errorf("Expected no emails on second run, got %+v", emailSender.SentEmails)
	}
}

func TestScheduledHandlerRetriesOnlyFailedRecipients(t *testing.T) {
	handler, configStore, emailSender := newScheduledTest(t, triggeredConfig())
	sendErr := errors.New("SES rejected address")
	emailSender.FailFor["recipient1@example.com"] = sendErr

	// recipient1 fails, but recipient2 and the owner are still emailed
	err := handler.Handle(context.Background())
	if !errors.Is(err, sendErr) {
		t.Fatalf("Expected send failure to be reported, got %v", err)
	}
	if len(emailSender.SentEmails) != 2 {
		t.Fatalf("Expected 2 emails despite failure, got %d", len(emailSender.SentEmails))
	}

	cfg := storedConfig(t, configStore)
	if !slices.Equal(cfg.SentTo, []string{"recipient2@example.com"}) || !cfg.OwnerNotified {
		t.Errorf("Expected only successes recorded, got SentTo=%v OwnerNotified=%v", cfg.SentTo, cfg.OwnerNotified)
	}

	// Next run retries only the failed recipient
	delete(emailSender.FailFor, "recipient1@example.com")
	emailSender.SentEmails = nil
	if err := handler.Handle(context.Background()); err != nil {
		t.Fatalf("Retry Handle failed: %v", err)
	}
	if len(emailSender.SentEmails) != 1 || emailSender.SentEmails[0].To != "recipient1@example.com" {
		t.Errorf("Expected retry to recipient1 only, got %+v", emailSender.SentEmails)
	}
}

func TestScheduledHandlerWarningLeavesConfigUnchanged(t *testing.T) {
	cfg := triggeredConfig()
	cfg.Timeout = testNow.AddDate(0, 0, 5)
	handler, configStore, emailSender := newScheduledTest(t, cfg)
	before := bytes.Clone(configStore.Data[testParam])

	if err := handler.Handle(context.Background()); err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	if len(emailSender.SentEmails) != 1 || emailSender.SentEmails[0].To != "owner@example.com" {
		t.Errorf("Expected a single warning to the owner, got %+v", emailSender.SentEmails)
	}
	if !bytes.Equal(configStore.Data[testParam], before) {
		t.Error("Expected config not to be rewritten for a warning")
	}
}

// checkInDuringRunStore simulates the owner checking in while the scheduled
// run is sending emails: every read after the first sees a new timeout.
type checkInDuringRunStore struct {
	*mocks.MockConfigStore
	reads int
}

func (s *checkInDuringRunStore) GetConfig(ctx context.Context, parameterName string) ([]byte, error) {
	s.reads++
	if s.reads == 2 {
		cfg, err := config.ParseConfig(s.Data[parameterName])
		if err != nil {
			return nil, err
		}
		cfg.Timeout = testNow.AddDate(0, 0, 30)
		if s.Data[parameterName], err = cfg.ToJSON(); err != nil {
			return nil, err
		}
	}
	return s.MockConfigStore.GetConfig(ctx, parameterName)
}

func TestScheduledHandlerDoesNotOverwriteConcurrentCheckIn(t *testing.T) {
	handler, configStore, _ := newScheduledTest(t, triggeredConfig())
	store := &checkInDuringRunStore{MockConfigStore: configStore}
	handler.configStore = store

	if err := handler.Handle(context.Background()); err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	cfg := storedConfig(t, configStore)
	if !cfg.Timeout.Equal(testNow.AddDate(0, 0, 30)) {
		t.Errorf("Expected check-in timeout to survive, got %v", cfg.Timeout)
	}
	if cfg.SentTo != nil || cfg.OwnerNotified {
		t.Errorf("Expected no delivery state written over the check-in, got SentTo=%v OwnerNotified=%v", cfg.SentTo, cfg.OwnerNotified)
	}
}

func TestScheduledHandlerDocumentMissingBeforeTimeout(t *testing.T) {
	cfg := triggeredConfig()
	cfg.Timeout = testNow.AddDate(0, 0, 20)
	handler, configStore, emailSender := newScheduledTest(t, cfg)
	handler.documentStore = mocks.NewMockDocumentStore()
	before := bytes.Clone(configStore.Data[testParam])

	// The owner's email is the warning, so the run itself succeeds
	if err := handler.Handle(context.Background()); err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	if len(emailSender.SentEmails) != 1 || emailSender.SentEmails[0].To != "owner@example.com" ||
		!strings.Contains(emailSender.SentEmails[0].Body, "missing from s3://"+testBucket+"/"+testKey) {
		t.Errorf("Expected a single missing-document warning to the owner, got %+v", emailSender.SentEmails)
	}
	if !bytes.Equal(configStore.Data[testParam], before) {
		t.Error("Expected config not to be rewritten")
	}
}

func TestScheduledHandlerDocumentMissingAfterTimeout(t *testing.T) {
	handler, configStore, emailSender := newScheduledTest(t, triggeredConfig())
	documentStore := mocks.NewMockDocumentStore()
	handler.documentStore = documentStore

	// Recipients are waiting, so the run errors to raise the alarm
	err := handler.Handle(context.Background())
	if err == nil || !strings.Contains(err.Error(), "is missing") {
		t.Fatalf("Expected a missing-document error, got %v", err)
	}
	if len(emailSender.SentEmails) != 1 || emailSender.SentEmails[0].To != "owner@example.com" ||
		emailSender.SentEmails[0].Subject != "Deadman's Handle Triggered - Document Missing" {
		t.Errorf("Expected a single delivery-blocked notice to the owner, got %+v", emailSender.SentEmails)
	}
	if cfg := storedConfig(t, configStore); cfg.SentTo != nil || cfg.OwnerNotified {
		t.Errorf("Expected no delivery state, got SentTo=%v OwnerNotified=%v", cfg.SentTo, cfg.OwnerNotified)
	}

	// Once the document is uploaded, the next run delivers it as normal
	documentStore.SetDocument(testBucket, testKey, []byte("the document"))
	emailSender.SentEmails = nil
	if err := handler.Handle(context.Background()); err != nil {
		t.Fatalf("Handle after upload failed: %v", err)
	}
	if len(emailSender.SentEmails) != 3 {
		t.Errorf("Expected 2 document emails and 1 owner notice after upload, got %+v", emailSender.SentEmails)
	}
	if cfg := storedConfig(t, configStore); len(cfg.SentTo) != 2 || !cfg.OwnerNotified {
		t.Errorf("Expected delivery recorded, got SentTo=%v OwnerNotified=%v", cfg.SentTo, cfg.OwnerNotified)
	}
}
