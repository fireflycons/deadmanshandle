package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fireflycons/deadmanshandle/internal/config"
	"github.com/fireflycons/deadmanshandle/internal/domain"
	"github.com/fireflycons/deadmanshandle/internal/mocks"
)

func ptr[T any](v T) *T { return &v }

func TestSetConfig(t *testing.T) {
	tests := []struct {
		name    string
		change  ConfigChange
		check   func(*config.Config) bool
		wantErr string // the parameter must then be left as it was
	}{
		{"owner", ConfigChange{Owner: ptr("new@example.com")},
			func(c *config.Config) bool { return c.Owner == "new@example.com" }, ""},
		{"recipients replace the list", ConfigChange{Recipients: ptr([]string{"c@example.com"})},
			func(c *config.Config) bool { return len(c.Recipients) == 1 && c.Recipients[0] == "c@example.com" }, ""},
		{"days together", ConfigChange{ResetDays: ptr(10), WarnDays: ptr(3)},
			func(c *config.Config) bool { return c.ResetDays == 10 && c.WarnDays == 3 }, ""},
		{"api key and document key", ConfigChange{APIKey: ptr("new-key"), DocumentKey: ptr("new.pdf")},
			func(c *config.Config) bool { return c.APIKey == "new-key" && c.DocumentKey == "new.pdf" }, ""},
		{"warnDays not below resetDays", ConfigChange{WarnDays: ptr(30)}, nil, "warnDays must be"},
		{"no recipients", ConfigChange{Recipients: ptr([]string{})}, nil, "recipients is empty"},
		{"blank document key", ConfigChange{DocumentKey: ptr(" ")}, nil, "documentKey is empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := configStoreWith(t, testConfig())
			before := string(store.Data[testParam])

			cfg, err := SetConfig(context.Background(), store, testParam, tt.change)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("Expected an error containing %q, got %v", tt.wantErr, err)
				}
				if store.Puts != 0 || string(store.Data[testParam]) != before {
					t.Error("Expected the parameter left as it was")
				}
				return
			}
			if err != nil {
				t.Fatalf("SetConfig failed: %v", err)
			}
			stored, err := config.ParseConfig(store.Data[testParam])
			if err != nil {
				t.Fatalf("Stored config is invalid: %v", err)
			}
			if !tt.check(cfg) || !tt.check(stored) {
				t.Errorf("Change not applied: returned %+v, stored %+v", cfg, stored)
			}
		})
	}
}

// The stored parameter may hold fields the CLI does not know, such as those
// of an uploaded config.json; they must survive a change
func TestSetConfigKeepsUnknownFields(t *testing.T) {
	stored := map[string]any{
		"owner": "owner@example.com", "recipients": []string{"a@example.com"},
		"resetDays": 30, "warnDays": 7, "apiKey": "key", "documentKey": "doc.pdf",
		"timeout": "2030-01-01T00:00:00Z", "deployment": map[string]string{"senderEmail": "noreply@example.com"},
	}
	store := configStoreWith(t, stored)

	if _, err := SetConfig(context.Background(), store, testParam, ConfigChange{Owner: ptr("new@example.com")}); err != nil {
		t.Fatalf("SetConfig failed: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(store.Data[testParam], &got); err != nil {
		t.Fatalf("Stored value is not JSON: %v", err)
	}
	if got["owner"] != "new@example.com" || got["timeout"] != "2030-01-01T00:00:00Z" || got["deployment"] == nil {
		t.Errorf("Expected the owner changed and the other fields kept, got %v", got)
	}
}

// An upgrade adds documentKey to a config that is invalid without it
func TestSetConfigFixesMissingDocumentKey(t *testing.T) {
	cfg := testConfig()
	cfg.DocumentKey = ""
	store := configStoreWith(t, cfg)

	if _, err := SetConfig(context.Background(), store, testParam, ConfigChange{DocumentKey: ptr("document.pdf")}); err != nil {
		t.Fatalf("SetConfig failed: %v", err)
	}
	if _, err := config.ParseConfig(store.Data[testParam]); err != nil {
		t.Errorf("Expected a valid config, got %v", err)
	}
}

func TestSetConfigReportsWriteFailure(t *testing.T) {
	store := configStoreWith(t, testConfig())
	store.PutErr = errors.New("access denied")
	if _, err := SetConfig(context.Background(), store, testParam, ConfigChange{Owner: ptr("new@example.com")}); !errors.Is(err, store.PutErr) {
		t.Errorf("Expected the write failure, got %v", err)
	}
}

func TestSetTimeout(t *testing.T) {
	service := domain.NewDeadmansHandleServiceWithTime(testNow)
	tests := []struct {
		name    string
		timeout time.Time
		force   bool
		wantErr error
	}{
		{"future", testNow.Add(time.Hour), false, nil},
		{"past", testNow.Add(-time.Hour), false, ErrTimeoutPast},
		{"now", testNow, false, ErrTimeoutPast},
		{"past with force", testNow.Add(-time.Hour), true, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateStore := mocks.NewMockStateStore(config.State{
				Timeout: testNow.AddDate(0, 0, 5), SentTo: []string{"a@example.com"}, OwnerNotified: true,
			})
			err := SetTimeout(context.Background(), stateStore, service, tt.timeout, tt.force)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Expected %v, got %v", tt.wantErr, err)
			}
			state := stateStore.State
			if tt.wantErr != nil {
				if state.CheckIns != 0 {
					t.Error("Expected the state left as it was")
				}
				return
			}
			if !state.Timeout.Equal(tt.timeout) || state.SentTo != nil || state.OwnerNotified || state.CheckIns != 1 {
				t.Errorf("Expected the timeout set as a check-in sets it, got %+v", state)
			}
		})
	}
}

func TestReportMasksSecrets(t *testing.T) {
	data, _ := json.Marshal(testConfig())
	token := strings.Repeat("t", 150)
	state := &config.State{Timeout: testNow, PushToken: token}

	masked := NewReport(testParam, data, Deployment{}, state, false)
	if masked.Settings.APIKey != "secr…" || masked.State.PushToken != strings.Repeat("t", 16)+"…" {
		t.Errorf("Expected the API key masked and the token shortened, got %q and %q", masked.Settings.APIKey, masked.State.PushToken)
	}
	shown := NewReport(testParam, data, Deployment{}, state, true)
	if shown.Settings.APIKey != "secret-api-key" || shown.State.PushToken != token {
		t.Errorf("Expected the secrets in full, got %q and %q", shown.Settings.APIKey, shown.State.PushToken)
	}

	short := testConfig()
	short.APIKey = "short"
	data, _ = json.Marshal(short)
	if got := NewReport(testParam, data, Deployment{}, nil, false).Settings.APIKey; got != "********" {
		t.Errorf("Expected a short key hidden entirely, got %q", got)
	}
}

func TestReportShowsInvalidConfig(t *testing.T) {
	cfg := testConfig()
	cfg.DocumentKey = ""
	data, _ := json.Marshal(cfg)

	report := NewReport(testParam, data, Deployment{}, nil, false)
	if report.Settings.Owner != "owner@example.com" || len(report.Problems) != 1 ||
		!strings.Contains(report.Problems[0], "documentKey is empty") {
		t.Errorf("Expected the settings shown with the problem, got %+v", report)
	}

	var out bytes.Buffer
	if err := report.WriteText(&out, testNow); err != nil {
		t.Fatalf("WriteText failed: %v", err)
	}
	if !strings.Contains(out.String(), "Problems") || strings.Contains(out.String(), "State (DynamoDB)") {
		t.Errorf("Expected the problems and no state, got:\n%s", out.String())
	}
}

func TestReportText(t *testing.T) {
	data, _ := json.Marshal(testConfig())
	state := &config.State{Timeout: testNow.AddDate(0, 0, 3), CheckIns: 2}
	deployment := Deployment{Endpoint: "https://api.example.com", DocumentBucket: "bucket"}
	report := NewReport(testParam, data, deployment, state, false)
	report.API = FormatThrottle(openThrottle)

	var out bytes.Buffer
	if err := report.WriteText(&out, testNow); err != nil {
		t.Fatalf("WriteText failed: %v", err)
	}
	for _, want := range []string{
		"a@example.com, b@example.com", "secr…", "docs/will.pdf", "https://api.example.com",
		"Tuesday 4 June 2024 at 12:00 UTC", "in 3 days", "documentChangedAt", "(never)",
		"Check-in API", "open (rate 0.017/s, burst 1)",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("Expected %q in:\n%s", want, out.String())
		}
	}
}
