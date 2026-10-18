package handlers

import (
	"context"
	"encoding/base64"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/fireflycons/deadmanshandle/internal/config"
	"github.com/fireflycons/deadmanshandle/internal/domain"
	"github.com/fireflycons/deadmanshandle/internal/mocks"
)

const testPushToken = "fcm-token:abc_123"

// newScheduledPushTest is newScheduledTest with push set up
func newScheduledPushTest(t *testing.T, state config.State) (*ScheduledEventHandler, *mocks.MockEmailSender, *mocks.MockPushSender) {
	t.Helper()

	configStore := mocks.NewMockConfigStore()
	configStore.Data[testParam] = configJSON(t, testConfig())
	stateStore := mocks.NewMockStateStore(state)
	documentStore := mocks.NewMockDocumentStore()
	documentStore.SetDocument(testBucket, testKey, []byte("the document"))
	emailSender := mocks.NewMockEmailSender()
	pushSender := mocks.NewMockPushSender()
	service := domain.NewDeadmansHandleServiceWithTime(testNow)
	watcher := NewDocumentWatcher(stateStore, emailSender, pushSender, service)
	handler := NewScheduledEventHandler(configStore, stateStore, documentStore, emailSender, pushSender, service, watcher, testParam, testBucket)

	return handler, emailSender, pushSender
}

func pushKinds(sender *mocks.MockPushSender) []string {
	var kinds []string
	for _, push := range sender.Pushes {
		kinds = append(kinds, push.Kind)
	}
	return kinds
}

// warningState is the state 3 days before the timeout, inside warnDays
func warningState() config.State {
	return config.State{Timeout: testNow.AddDate(0, 0, 3), PushToken: testPushToken}
}

func TestScheduledHandlerPushesOwnerNoticesOnly(t *testing.T) {
	state := triggeredState()
	state.PushToken = testPushToken
	handler, emailSender, pushSender := newScheduledPushTest(t, state)

	if err := handler.Handle(context.Background()); err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	// The recipients' documents are emailed but not pushed
	if len(emailSender.SentEmails) != 3 {
		t.Errorf("Expected 3 emails, got %d", len(emailSender.SentEmails))
	}
	if got := pushKinds(pushSender); !slices.Equal(got, []string{"triggered"}) {
		t.Fatalf("Expected only the trigger notice pushed, got %v", got)
	}
	push := pushSender.Pushes[0]
	if push.Token != testPushToken || push.Title != "Deadman's Handle Triggered" || push.Body == "" {
		t.Errorf("Unexpected push %+v", push)
	}
}

func TestScheduledHandlerPushesWhenEmailFails(t *testing.T) {
	handler, emailSender, pushSender := newScheduledPushTest(t, warningState())
	emailSender.FailFor["owner@example.com"] = errors.New("ses down")

	if err := handler.Handle(context.Background()); err == nil {
		t.Error("Expected the failed email to fail the run")
	}
	if got := pushKinds(pushSender); !slices.Equal(got, []string{"warning"}) {
		t.Errorf("Expected the warning pushed, got %v", got)
	}
}

func TestScheduledHandlerIgnoresPushFailure(t *testing.T) {
	handler, emailSender, pushSender := newScheduledPushTest(t, warningState())
	pushSender.Err = errors.New("sns down")

	if err := handler.Handle(context.Background()); err != nil {
		t.Errorf("Expected a failed push not to fail the run, got %v", err)
	}
	if len(emailSender.SentEmails) != 1 {
		t.Errorf("Expected the warning emailed, got %d emails", len(emailSender.SentEmails))
	}
}

func TestScheduledHandlerNoPushWithoutToken(t *testing.T) {
	state := warningState()
	state.PushToken = ""
	handler, _, pushSender := newScheduledPushTest(t, state)

	if err := handler.Handle(context.Background()); err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if len(pushSender.Pushes) != 0 {
		t.Errorf("Expected no push without a token, got %v", pushKinds(pushSender))
	}
}

func TestDocumentWatcherPushesChangeNotice(t *testing.T) {
	stateStore := mocks.NewMockStateStore(config.State{
		Timeout:      testNow.AddDate(0, 0, 10),
		DocumentETag: `"old"`,
		PushToken:    testPushToken,
	})
	pushSender := mocks.NewMockPushSender()
	watcher := NewDocumentWatcher(stateStore, mocks.NewMockEmailSender(), pushSender, domain.NewDeadmansHandleServiceWithTime(testNow))
	doc := domain.Document{Location: "s3://" + testBucket + "/" + testKey, Available: true, ETag: `"new"`}

	if err := watcher.Check(context.Background(), testConfig(), doc, testOrigin); err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if got := pushKinds(pushSender); !slices.Equal(got, []string{"document-changed"}) {
		t.Fatalf("Expected the change notice pushed, got %v", got)
	}
	if pushSender.Pushes[0].Title != changedSubject {
		t.Errorf("Expected title %q, got %q", changedSubject, pushSender.Pushes[0].Title)
	}
}

func TestHTTPHandlerPushToken(t *testing.T) {
	tooLong := strings.Repeat("a", maxPushTokenLength+1)

	tests := []struct {
		name      string
		body      string
		base64    bool
		wantToken string
	}{
		{"no body keeps the token", "", false, "previous"},
		{"empty object keeps the token", `{}`, false, "previous"},
		{"token recorded", `{"pushToken":"` + testPushToken + `"}`, false, testPushToken},
		{"base64 body", `{"pushToken":"` + testPushToken + `"}`, true, testPushToken},
		{"not JSON ignored", `pushToken`, false, "previous"},
		{"bad characters ignored", `{"pushToken":"a b"}`, false, "previous"},
		{"too long ignored", `{"pushToken":"` + tooLong + `"}`, false, "previous"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configStore := mocks.NewMockConfigStore()
			configStore.Data[testParam] = configJSON(t, testConfig())
			stateStore := mocks.NewMockStateStore(config.State{Timeout: testNow, PushToken: "previous"})
			handler := NewHTTPHandler(configStore, stateStore, domain.NewDeadmansHandleServiceWithTime(testNow), mocks.NewMockAPIKeyValidator(), testParam)

			request := events.APIGatewayV2HTTPRequest{
				Headers: map[string]string{"x-api-key": "test-key"},
				Body:    tt.body,
			}
			if tt.base64 {
				request.Body = base64.StdEncoding.EncodeToString([]byte(tt.body))
				request.IsBase64Encoded = true
			}

			response, err := handler.Handle(context.Background(), request)
			if err != nil || response.StatusCode != 200 {
				t.Fatalf("Expected the check-in to succeed, got %d, %v", response.StatusCode, err)
			}
			if stateStore.State.PushToken != tt.wantToken {
				t.Errorf("Expected push token %q, got %q", tt.wantToken, stateStore.State.PushToken)
			}
			if stateStore.State.CheckIns != 1 {
				t.Errorf("Expected a check-in, got CheckIns %d", stateStore.State.CheckIns)
			}
		})
	}
}
