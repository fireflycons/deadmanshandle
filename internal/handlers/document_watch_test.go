package handlers

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fireflycons/deadmanshandle/internal/config"
	"github.com/fireflycons/deadmanshandle/internal/domain"
	"github.com/fireflycons/deadmanshandle/internal/mocks"
)

const changedSubject = "Deadman's Handle Document Changed"

var testOrigin = &domain.ChangeOrigin{Requester: "123456789012", SourceIP: "192.0.2.1"}

func newDocumentWatchTest(t *testing.T) (*DocumentWatchHandler, *mocks.MockStateStore, *mocks.MockDocumentStore, *mocks.MockEmailSender) {
	t.Helper()

	configStore := mocks.NewMockConfigStore()
	cfgData, err := testConfig().ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}
	configStore.Data[testParam] = cfgData

	stateStore := mocks.NewMockStateStore(config.State{Timeout: testNow.AddDate(0, 0, 10)})
	documentStore := mocks.NewMockDocumentStore()
	emailSender := mocks.NewMockEmailSender()
	service := domain.NewDeadmansHandleServiceWithTime(testNow)
	watcher := NewDocumentWatcher(stateStore, emailSender, service)
	handler := NewDocumentWatchHandler(configStore, documentStore, watcher, testParam, testBucket, testKey)

	return handler, stateStore, documentStore, emailSender
}

// changeNotices returns the document change notices sent
func changeNotices(sender *mocks.MockEmailSender) []mocks.SentEmail {
	var notices []mocks.SentEmail
	for _, email := range sender.SentEmails {
		if email.Subject == changedSubject {
			notices = append(notices, email)
		}
	}
	return notices
}

func TestDocumentWatchHandler(t *testing.T) {
	// Each step uploads (or deletes, if nil) the document and then handles
	// an event for it
	type step struct {
		content []byte
		notify  bool
	}
	tests := []struct {
		name  string
		steps []step
	}{
		{"first upload is silent", []step{{[]byte("v1"), false}}},
		{"changed content is reported", []step{{[]byte("v1"), false}, {[]byte("v2"), true}}},
		{"identical content is silent", []step{{[]byte("v1"), false}, {[]byte("v1"), false}}},
		{"each change is reported once", []step{{[]byte("v1"), false}, {[]byte("v2"), true}, {[]byte("v2"), false}, {[]byte("v3"), true}}},
		{"delete then re-upload different content is reported", []step{{[]byte("v1"), false}, {nil, false}, {[]byte("v2"), true}}},
		{"delete then re-upload the same content is silent", []step{{[]byte("v1"), false}, {nil, false}, {[]byte("v1"), false}}},
		{"missing before any upload is silent", []step{{nil, false}, {[]byte("v1"), false}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, _, documentStore, emailSender := newDocumentWatchTest(t)

			for i, s := range tt.steps {
				if s.content == nil {
					delete(documentStore.Documents[testBucket], testKey)
				} else {
					documentStore.SetDocument(testBucket, testKey, s.content)
				}
				emailSender.SentEmails = nil

				if err := handler.Handle(context.Background(), testOrigin); err != nil {
					t.Fatalf("Step %d: Handle failed: %v", i, err)
				}

				notices := changeNotices(emailSender)
				if len(emailSender.SentEmails) != len(notices) {
					t.Errorf("Step %d: expected only change notices, got %+v", i, emailSender.SentEmails)
				}
				if s.notify {
					if len(notices) != 1 || notices[0].To != "owner@example.com" ||
						!strings.Contains(notices[0].Body, "AWS account 123456789012 from IP address 192.0.2.1") {
						t.Errorf("Step %d: expected one change notice to the owner with the origin, got %+v", i, notices)
					}
				} else if len(notices) != 0 {
					t.Errorf("Step %d: expected no change notice, got %+v", i, notices)
				}
			}
		})
	}
}

func TestDocumentWatchHandlerRetriesFailedNotice(t *testing.T) {
	handler, stateStore, documentStore, emailSender := newDocumentWatchTest(t)

	// The first document is recorded silently and is not timed
	documentStore.SetDocument(testBucket, testKey, []byte("v1"))
	if err := handler.Handle(context.Background(), testOrigin); err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if state := stateStore.State; !state.DocumentChangedAt.IsZero() || state.DocumentChangePending {
		t.Errorf("Expected no change recorded for the first document, got %+v", state)
	}

	// The notice fails: the change is still recorded and timed, so the hold
	// runs from now, and the notice stays pending
	sendErr := errors.New("SES rejected address")
	emailSender.FailFor["owner@example.com"] = sendErr
	documentStore.SetDocument(testBucket, testKey, []byte("v2"))
	if err := handler.Handle(context.Background(), testOrigin); !errors.Is(err, sendErr) {
		t.Fatalf("Expected the send failure, got %v", err)
	}
	v2, _, _ := documentStore.DocumentETag(context.Background(), testBucket, testKey)
	if state := stateStore.State; state.DocumentETag != v2 || !state.DocumentChangedAt.Equal(testNow) || !state.DocumentChangePending {
		t.Errorf("Expected the change recorded at %v with its notice pending, got %+v", testNow, state)
	}
	delete(emailSender.FailFor, "owner@example.com")

	// Too soon to resend: the check that found the change may still be running
	if err := handler.Handle(context.Background(), nil); err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if len(changeNotices(emailSender)) != 0 {
		t.Errorf("Expected no resend within %v, got %+v", domain.ChangeNoticeRetryAfter, emailSender.SentEmails)
	}

	// Later, the pending notice is sent, with the original change time, and
	// the change time is not moved
	handler.watcher.service = domain.NewDeadmansHandleServiceWithTime(testNow.Add(domain.ChangeNoticeRetryAfter))
	if err := handler.Handle(context.Background(), testOrigin); err != nil {
		t.Fatalf("Retry Handle failed: %v", err)
	}
	notices := changeNotices(emailSender)
	if len(notices) != 1 || !strings.Contains(notices[0].Body, "noticed on Saturday 1 June 2024 at 12:00 UTC") ||
		strings.Contains(notices[0].Body, "AWS account") {
		t.Errorf("Expected one resent notice with the original time and no origin, got %+v", notices)
	}
	if state := stateStore.State; !state.DocumentChangedAt.Equal(testNow) || state.DocumentChangePending {
		t.Errorf("Expected the notice cleared and the change time kept, got %+v", state)
	}
}

func TestScheduledHandlerReportsDocumentChange(t *testing.T) {
	// Before the timeout, so a missing check-in would trigger nothing else
	handler, _, emailSender := newScheduledTest(t, testConfig(),
		config.State{Timeout: testNow.AddDate(0, 0, 20), DocumentETag: `"recorded before the change"`})

	if err := handler.Handle(context.Background()); err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	notices := changeNotices(emailSender)
	if len(emailSender.SentEmails) != 1 || len(notices) != 1 || strings.Contains(notices[0].Body, "AWS account") {
		t.Errorf("Expected one change notice without an origin, got %+v", emailSender.SentEmails)
	}
}

func TestScheduledHandlerDeliversAfterHoldEvenIfOwnerUnreachable(t *testing.T) {
	state := triggeredState()
	state.DocumentETag = `"recorded before the change"`
	handler, _, emailSender := newScheduledTest(t, testConfig(), state)
	sendErr := errors.New("SES rejected address")
	emailSender.FailFor["owner@example.com"] = sendErr

	// The change is found but the owner cannot be told; delivery is held
	if err := handler.Handle(context.Background()); !errors.Is(err, sendErr) {
		t.Fatalf("Expected the send failure, got %v", err)
	}
	if len(emailSender.SentEmails) != 0 {
		t.Errorf("Expected nothing sent while held, got %+v", emailSender.SentEmails)
	}

	// The failing notice does not extend the hold: once it is over, the
	// recipients get the document
	later := domain.NewDeadmansHandleServiceWithTime(testNow.Add(domain.ChangeHoldPeriod + time.Hour))
	handler.service = later
	handler.watcher.service = later
	if err := handler.Handle(context.Background()); !errors.Is(err, sendErr) {
		t.Fatalf("Expected the owner's send failures, got %v", err)
	}
	if len(emailSender.SentEmails) != 2 {
		t.Errorf("Expected the document sent to both recipients, got %+v", emailSender.SentEmails)
	}
}

func TestDocumentWatcherConcurrentChecks(t *testing.T) {
	// Another invocation records an ETag between this one's read and swap
	tests := []struct {
		name        string
		ours        string // ETag this invocation sees
		theirs      string // ETag the other invocation records first
		wantNotices int
	}{
		{"same change is reported once, by the other", `"v2"`, `"v2"`, 0},
		{"a later change is still reported", `"v3"`, `"v2"`, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateStore := mocks.NewMockStateStore(config.State{Timeout: testNow.AddDate(0, 0, 10), DocumentETag: `"v1"`})
			stateStore.BeforeUpdate = func() {
				stateStore.BeforeUpdate = nil
				if err := stateStore.SwapDocumentETag(context.Background(), `"v1"`, tt.theirs, testNow); err != nil {
					t.Fatalf("Concurrent swap failed: %v", err)
				}
			}
			emailSender := mocks.NewMockEmailSender()
			watcher := NewDocumentWatcher(stateStore, emailSender, domain.NewDeadmansHandleServiceWithTime(testNow))
			doc := domain.Document{Location: "s3://" + testBucket + "/" + testKey, Available: true, ETag: tt.ours}

			if err := watcher.Check(context.Background(), testConfig(), doc, testOrigin); err != nil {
				t.Fatalf("Check failed: %v", err)
			}

			if got := len(changeNotices(emailSender)); got != tt.wantNotices {
				t.Errorf("Expected %d notices, got %d", tt.wantNotices, got)
			}
			if stateStore.State.DocumentETag != tt.ours {
				t.Errorf("Expected recorded ETag %s, got %s", tt.ours, stateStore.State.DocumentETag)
			}
		})
	}
}
