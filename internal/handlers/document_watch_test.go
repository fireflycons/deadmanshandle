package handlers

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/fireflycons/deadmanshandle/internal/domain"
	"github.com/fireflycons/deadmanshandle/internal/mocks"
)

const changedSubject = "Deadman's Handle Document Changed"

var testOrigin = &domain.ChangeOrigin{Requester: "123456789012", SourceIP: "192.0.2.1"}

func newDocumentWatchTest(t *testing.T) (*DocumentWatchHandler, *mocks.MockConfigStore, *mocks.MockDocumentStore, *mocks.MockEmailSender) {
	t.Helper()

	configStore := mocks.NewMockConfigStore()
	cfgData, err := triggeredConfig().ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}
	configStore.Data[testParam] = cfgData
	configStore.Data[testETagParam] = []byte(domain.NoRecordedETag)

	documentStore := mocks.NewMockDocumentStore()
	emailSender := mocks.NewMockEmailSender()
	service := domain.NewDeadmansHandleServiceWithTime(testNow)
	watcher := NewDocumentWatcher(configStore, emailSender, service, testETagParam)
	handler := NewDocumentWatchHandler(configStore, documentStore, watcher, testParam, testBucket, testKey)

	return handler, configStore, documentStore, emailSender
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
	handler, configStore, documentStore, emailSender := newDocumentWatchTest(t)
	documentStore.SetDocument(testBucket, testKey, []byte("v1"))
	if err := handler.Handle(context.Background(), testOrigin); err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	recorded := string(configStore.Data[testETagParam])

	// The notice fails, so the new ETag is not recorded
	sendErr := errors.New("SES rejected address")
	emailSender.FailFor["owner@example.com"] = sendErr
	documentStore.SetDocument(testBucket, testKey, []byte("v2"))
	if err := handler.Handle(context.Background(), testOrigin); !errors.Is(err, sendErr) {
		t.Fatalf("Expected the send failure, got %v", err)
	}
	if got := string(configStore.Data[testETagParam]); got != recorded {
		t.Errorf("Expected recorded ETag %s to be kept, got %s", recorded, got)
	}

	// The next check reports the change
	delete(emailSender.FailFor, "owner@example.com")
	if err := handler.Handle(context.Background(), nil); err != nil {
		t.Fatalf("Retry Handle failed: %v", err)
	}
	if len(changeNotices(emailSender)) != 1 {
		t.Errorf("Expected the change to be reported on retry, got %+v", emailSender.SentEmails)
	}
}

func TestScheduledHandlerReportsDocumentChange(t *testing.T) {
	// Before the timeout, so a missing check-in would trigger nothing else
	cfg := triggeredConfig()
	cfg.Timeout = testNow.AddDate(0, 0, 20)
	handler, configStore, emailSender := newScheduledTest(t, cfg)
	configStore.Data[testETagParam] = []byte(`"recorded before the change"`)

	if err := handler.Handle(context.Background()); err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	notices := changeNotices(emailSender)
	if len(emailSender.SentEmails) != 1 || len(notices) != 1 || strings.Contains(notices[0].Body, "AWS account") {
		t.Errorf("Expected one change notice without an origin, got %+v", emailSender.SentEmails)
	}
}

func TestScheduledHandlerSendsDespiteFailedChangeCheck(t *testing.T) {
	handler, configStore, emailSender := newScheduledTest(t, triggeredConfig())
	configStore.Data[testETagParam] = []byte(`"recorded before the change"`)
	sendErr := errors.New("SES rejected address")
	emailSender.FailFor["owner@example.com"] = sendErr

	// The owner's notices fail, but the recipients still get the document
	if err := handler.Handle(context.Background()); !errors.Is(err, sendErr) {
		t.Fatalf("Expected the send failure, got %v", err)
	}
	if len(emailSender.SentEmails) != 2 {
		t.Errorf("Expected the document sent to both recipients, got %+v", emailSender.SentEmails)
	}
}
