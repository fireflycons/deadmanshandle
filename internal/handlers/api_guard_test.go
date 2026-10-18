package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/fireflycons/deadmanshandle/internal/config"
	"github.com/fireflycons/deadmanshandle/internal/domain"
	"github.com/fireflycons/deadmanshandle/internal/mocks"
	"github.com/fireflycons/deadmanshandle/internal/ports"
)

const guardAlarm = "deadmanshandle-api-rejected-requests"

var openThrottle = ports.Throttle{RateLimit: 0.017, BurstLimit: 1}

type guardTest struct {
	handler  *APIGuardHandler
	gate     *mocks.MockAPIGate
	schedule *mocks.MockCheckSchedule
	alarms   *mocks.MockAlarmStates
	emails   *mocks.MockEmailSender
	pushes   *mocks.MockPushSender
}

func newGuardTest(t *testing.T, throttle ports.Throttle, checking bool) *guardTest {
	t.Helper()
	configStore := mocks.NewMockConfigStore()
	configStore.Data[testParam] = configJSON(t, testConfig())

	g := &guardTest{
		gate:     &mocks.MockAPIGate{Current: throttle},
		schedule: &mocks.MockCheckSchedule{Enabled: checking, Minutes: 15},
		alarms:   &mocks.MockAlarmStates{InAlarmNames: map[string]bool{}},
		emails:   mocks.NewMockEmailSender(),
		pushes:   mocks.NewMockPushSender(),
	}
	g.handler = NewAPIGuardHandler(configStore, mocks.NewMockStateStore(config.State{PushToken: "token"}),
		g.emails, g.pushes, g.gate, g.schedule, g.alarms, domain.NewDeadmansHandleServiceWithTime(testNow),
		testParam, guardAlarm, openThrottle)
	return g
}

func alarmMessage(t *testing.T, name, state string) string {
	t.Helper()
	data, err := json.Marshal(cloudWatchAlarm{AlarmName: name, NewStateValue: state})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestAPIGuardClosesOnAlarm(t *testing.T) {
	g := newGuardTest(t, openThrottle, false)

	if err := g.handler.HandleAlarm(context.Background(), alarmMessage(t, guardAlarm, "ALARM")); err != nil {
		t.Fatalf("HandleAlarm failed: %v", err)
	}
	if !g.gate.Current.Closed() || !g.schedule.Enabled {
		t.Errorf("Expected the API closed and the check started, got %+v, checking %v", g.gate.Current, g.schedule.Enabled)
	}
	if len(g.emails.SentEmails) != 1 || !strings.Contains(g.emails.SentEmails[0].Body, "checked every 15 minutes") ||
		g.emails.SentEmails[0].To != "owner@example.com" {
		t.Errorf("Expected the closed notice to the owner, got %+v", g.emails.SentEmails)
	}
	if len(g.pushes.Pushes) != 1 || g.pushes.Pushes[0].Kind != "api" {
		t.Errorf("Expected the notice pushed, got %+v", g.pushes.Pushes)
	}
}

func TestAPIGuardIgnoresAlarms(t *testing.T) {
	tests := []struct {
		name     string
		throttle ports.Throttle
		message  string
	}{
		{"another alarm", openThrottle, `{"AlarmName":"deadmanshandle-scheduled-errors","NewStateValue":"ALARM"}`},
		{"back to OK", openThrottle, `{"AlarmName":"` + guardAlarm + `","NewStateValue":"OK"}`},
		{"already closed, by hand or by the guard", ports.Throttle{}, `{"AlarmName":"` + guardAlarm + `","NewStateValue":"ALARM"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := newGuardTest(t, tt.throttle, false)
			if err := g.handler.HandleAlarm(context.Background(), tt.message); err != nil {
				t.Fatalf("HandleAlarm failed: %v", err)
			}
			if g.gate.Sets != 0 || g.schedule.Enabled || len(g.emails.SentEmails) != 0 {
				t.Errorf("Expected nothing done, got %d throttle changes, checking %v, emails %+v",
					g.gate.Sets, g.schedule.Enabled, g.emails.SentEmails)
			}
		})
	}
}

// A failed notice is returned, so the errors alarm fires, but the API stays
// closed and checked
func TestAPIGuardCloseNoticeFails(t *testing.T) {
	g := newGuardTest(t, openThrottle, false)
	sendErr := errors.New("SES rejected address")
	g.emails.FailFor["owner@example.com"] = sendErr

	if err := g.handler.HandleAlarm(context.Background(), alarmMessage(t, guardAlarm, "ALARM")); !errors.Is(err, sendErr) {
		t.Fatalf("Expected the send failure, got %v", err)
	}
	if !g.gate.Current.Closed() || !g.schedule.Enabled {
		t.Error("Expected the API closed and checked despite the failed notice")
	}
}

// If closing fails, the check has started, and finds the API open
func TestAPIGuardCloseFails(t *testing.T) {
	g := newGuardTest(t, openThrottle, false)
	g.gate.SetErr = errors.New("access denied")

	if err := g.handler.HandleAlarm(context.Background(), alarmMessage(t, guardAlarm, "ALARM")); !errors.Is(err, g.gate.SetErr) {
		t.Fatalf("Expected the failure, got %v", err)
	}
	g.gate.SetErr = nil
	if err := g.handler.HandleCheck(context.Background()); err != nil {
		t.Fatalf("HandleCheck failed: %v", err)
	}
	if g.schedule.Enabled || len(g.emails.SentEmails) != 0 {
		t.Errorf("Expected the check stopped with no notice, got checking %v, emails %+v", g.schedule.Enabled, g.emails.SentEmails)
	}
}

func TestAPIGuardCheck(t *testing.T) {
	tests := []struct {
		name         string
		throttle     ports.Throttle
		flooded      bool
		wantThrottle ports.Throttle
		wantChecking bool
		wantNotice   bool
	}{
		{"still flooded: stays closed", ports.Throttle{}, true, ports.Throttle{}, true, false},
		{"flood over: reopens", ports.Throttle{}, false, openThrottle, false, true},
		{"reopened by hand: check stops", openThrottle, true, openThrottle, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := newGuardTest(t, tt.throttle, true)
			g.alarms.InAlarmNames[guardAlarm] = tt.flooded

			if err := g.handler.HandleCheck(context.Background()); err != nil {
				t.Fatalf("HandleCheck failed: %v", err)
			}
			if g.gate.Current != tt.wantThrottle || g.schedule.Enabled != tt.wantChecking {
				t.Errorf("Expected throttle %+v and checking %v, got %+v and %v",
					tt.wantThrottle, tt.wantChecking, g.gate.Current, g.schedule.Enabled)
			}
			gotNotice := len(g.emails.SentEmails) == 1 && strings.Contains(g.emails.SentEmails[0].Subject, "Reopened")
			if gotNotice != tt.wantNotice || len(g.emails.SentEmails) > 1 {
				t.Errorf("Expected reopened notice %v, got %+v", tt.wantNotice, g.emails.SentEmails)
			}
		})
	}
}
