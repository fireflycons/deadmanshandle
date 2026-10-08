package handlers

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fireflycons/deadmanshandle/internal/config"
	"github.com/fireflycons/deadmanshandle/internal/domain"
	"github.com/fireflycons/deadmanshandle/internal/mocks"
	"github.com/fireflycons/deadmanshandle/internal/ports"
)

const (
	testParam  = "test-param"
	testBucket = "test-bucket"
	testKey    = "docs/document.pdf"
)

var testNow = time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

func newScheduledTest(t *testing.T, cfg *config.Config, state config.State) (*ScheduledEventHandler, *mocks.MockStateStore, *mocks.MockEmailSender) {
	t.Helper()

	configStore := mocks.NewMockConfigStore()
	cfgData, err := cfg.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}
	configStore.Data[testParam] = cfgData

	stateStore := mocks.NewMockStateStore(state)
	documentStore := mocks.NewMockDocumentStore()
	documentStore.SetDocument(testBucket, testKey, []byte("the document"))

	emailSender := mocks.NewMockEmailSender()
	service := domain.NewDeadmansHandleServiceWithTime(testNow)
	watcher := NewDocumentWatcher(stateStore, emailSender, service)
	handler := NewScheduledEventHandler(configStore, stateStore, documentStore, emailSender, service, watcher, testParam, testBucket, testKey)

	return handler, stateStore, emailSender
}

func testConfig() *config.Config {
	return &config.Config{
		Owner:      "owner@example.com",
		Recipients: []string{"recipient1@example.com", "recipient2@example.com"},
		ResetDays:  30,
		WarnDays:   7,
		APIKey:     "test-key",
	}
}

// triggeredState is the state a day after the timeout passed
func triggeredState() config.State {
	return config.State{Timeout: testNow.AddDate(0, 0, -1)}
}

func TestScheduledHandlerSendsDocumentOnce(t *testing.T) {
	handler, stateStore, emailSender := newScheduledTest(t, testConfig(), triggeredState())

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

	state := stateStore.State
	if !slices.Equal(state.SentTo, []string{"recipient1@example.com", "recipient2@example.com"}) || !state.OwnerNotified {
		t.Errorf("Expected delivery recorded, got SentTo=%v OwnerNotified=%v", state.SentTo, state.OwnerNotified)
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
	handler, stateStore, emailSender := newScheduledTest(t, testConfig(), triggeredState())
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

	state := stateStore.State
	if !slices.Equal(state.SentTo, []string{"recipient2@example.com"}) || !state.OwnerNotified {
		t.Errorf("Expected only successes recorded, got SentTo=%v OwnerNotified=%v", state.SentTo, state.OwnerNotified)
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

func TestScheduledHandlerWarningLeavesStateUnchanged(t *testing.T) {
	// The document's ETag is already recorded, so nothing at all is written
	handler, stateStore, emailSender := newScheduledTest(t, testConfig(), config.State{Timeout: testNow.AddDate(0, 0, 5)})
	if err := handler.Handle(context.Background()); err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	before := stateStore.State
	emailSender.SentEmails = nil

	if err := handler.Handle(context.Background()); err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	if len(emailSender.SentEmails) != 1 || emailSender.SentEmails[0].To != "owner@example.com" {
		t.Errorf("Expected a single warning to the owner, got %+v", emailSender.SentEmails)
	}
	if !reflect.DeepEqual(stateStore.State, before) {
		t.Errorf("Expected state not to change for a warning, got %+v", stateStore.State)
	}
}

func TestScheduledHandlerStopsAfterConcurrentCheckIn(t *testing.T) {
	handler, stateStore, emailSender := newScheduledTest(t, testConfig(), triggeredState())

	// The owner checks in while the first email is being sent
	newTimeout := testNow.AddDate(0, 0, 30)
	stateStore.BeforeUpdate = func() {
		if stateStore.State.CheckIns == 0 && len(emailSender.SentEmails) == 1 {
			_ = stateStore.CheckIn(context.Background(), newTimeout)
		}
	}

	if err := handler.Handle(context.Background()); err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	// The email already sent cannot be recalled, but nothing more is sent
	if len(emailSender.SentEmails) != 1 {
		t.Errorf("Expected sending to stop after the check-in, got %+v", emailSender.SentEmails)
	}
	state := stateStore.State
	if !state.Timeout.Equal(newTimeout) || state.SentTo != nil || state.OwnerNotified {
		t.Errorf("Expected the check-in to survive with no delivery state, got %+v", state)
	}
}

func TestScheduledHandlerMergesConcurrentDeliveries(t *testing.T) {
	handler, stateStore, emailSender := newScheduledTest(t, testConfig(), triggeredState())

	// recipient2 fails here, but another run delivers to it concurrently
	emailSender.FailFor["recipient2@example.com"] = errors.New("SES rejected address")
	stateStore.BeforeUpdate = func() {
		if len(emailSender.SentEmails) == 1 {
			stateStore.BeforeUpdate = nil
			_ = stateStore.RecordSent(context.Background(), 0, "recipient2@example.com")
		}
	}

	if err := handler.Handle(context.Background()); err == nil {
		t.Fatal("Expected the send failure to be reported")
	}

	if got := stateStore.State.SentTo; !slices.Contains(got, "recipient1@example.com") || !slices.Contains(got, "recipient2@example.com") {
		t.Errorf("Expected both runs' deliveries recorded, got %v", got)
	}
}

func TestScheduledHandlerDocumentMissingBeforeTimeout(t *testing.T) {
	handler, stateStore, emailSender := newScheduledTest(t, testConfig(), config.State{Timeout: testNow.AddDate(0, 0, 20)})
	handler.documentStore = mocks.NewMockDocumentStore()
	before := stateStore.State

	// The owner's email is the warning, so the run itself succeeds
	if err := handler.Handle(context.Background()); err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	if len(emailSender.SentEmails) != 1 || emailSender.SentEmails[0].To != "owner@example.com" ||
		!strings.Contains(emailSender.SentEmails[0].Body, "missing from s3://"+testBucket+"/"+testKey) {
		t.Errorf("Expected a single missing-document warning to the owner, got %+v", emailSender.SentEmails)
	}
	if !reflect.DeepEqual(stateStore.State, before) {
		t.Errorf("Expected state not to change, got %+v", stateStore.State)
	}
}

func TestScheduledHandlerDocumentMissingAfterTimeout(t *testing.T) {
	handler, stateStore, emailSender := newScheduledTest(t, testConfig(), triggeredState())
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
	if state := stateStore.State; state.SentTo != nil || state.OwnerNotified {
		t.Errorf("Expected no delivery state, got SentTo=%v OwnerNotified=%v", state.SentTo, state.OwnerNotified)
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
	if state := stateStore.State; len(state.SentTo) != 2 || !state.OwnerNotified {
		t.Errorf("Expected delivery recorded, got SentTo=%v OwnerNotified=%v", state.SentTo, state.OwnerNotified)
	}
}

// replacedAfterCheckStore replaces the document straight after its ETag is
// checked, as an upload between the run's HeadObject and GetObject would
type replacedAfterCheckStore struct {
	*mocks.MockDocumentStore
}

func (s *replacedAfterCheckStore) DocumentETag(ctx context.Context, bucket, key string) (string, bool, error) {
	etag, exists, err := s.MockDocumentStore.DocumentETag(ctx, bucket, key)
	s.SetDocument(bucket, key, []byte("replaced"))
	return etag, exists, err
}

func TestScheduledHandlerSendsNothingIfDocumentReplacedDuringRun(t *testing.T) {
	handler, stateStore, emailSender := newScheduledTest(t, testConfig(), triggeredState())
	documentStore := mocks.NewMockDocumentStore()
	documentStore.SetDocument(testBucket, testKey, []byte("the document"))
	handler.documentStore = &replacedAfterCheckStore{documentStore}

	err := handler.Handle(context.Background())
	if !errors.Is(err, ports.ErrDocumentChanged) {
		t.Fatalf("Expected a document-changed error, got %v", err)
	}
	for _, email := range emailSender.SentEmails {
		if email.Attachments != nil {
			t.Errorf("Expected nothing sent with the document, got an email to %s", email.To)
		}
	}
	if stateStore.State.SentTo != nil || stateStore.State.OwnerNotified {
		t.Errorf("Expected no delivery recorded, got %+v", stateStore.State)
	}
}
