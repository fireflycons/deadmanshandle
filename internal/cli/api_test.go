package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/fireflycons/deadmanshandle/internal/mocks"
	"github.com/fireflycons/deadmanshandle/internal/ports"
)

var openThrottle = ports.Throttle{RateLimit: 0.017, BurstLimit: 1}

func TestParseAPIStage(t *testing.T) {
	stage, err := ParseAPIStage([]byte(`{"apiId":"abc","stage":"$default","rateLimit":0.017,"burstLimit":1,"guardRule":"rule"}`))
	if err != nil {
		t.Fatalf("ParseAPIStage failed: %v", err)
	}
	if stage.Open() != openThrottle || stage.GuardRule != "rule" {
		t.Errorf("Unexpected stage %+v", stage)
	}

	for _, bad := range []string{
		`{"stage":"$default","rateLimit":1,"burstLimit":1}`,
		`{"apiId":"abc","stage":"$default","rateLimit":0,"burstLimit":0}`,
		`not json`,
	} {
		if _, err := ParseAPIStage([]byte(bad)); err == nil {
			t.Errorf("Expected %s refused", bad)
		}
	}
}

func TestDisableAndEnableAPI(t *testing.T) {
	stage := &APIStage{APIID: "abc", Stage: "$default", RateLimit: 0.017, BurstLimit: 1, GuardRule: "rule"}
	tests := []struct {
		name     string
		schedule *mocks.MockCheckSchedule // nil when the guard is off
	}{
		{"guard off", nil},
		{"guard checking", &mocks.MockCheckSchedule{Enabled: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gate := &mocks.MockAPIGate{Current: openThrottle}
			var schedule ports.CheckSchedule
			if tt.schedule != nil {
				schedule = tt.schedule
			}

			if err := DisableAPI(context.Background(), gate, schedule); err != nil {
				t.Fatalf("DisableAPI failed: %v", err)
			}
			if !gate.Current.Closed() {
				t.Errorf("Expected the API closed, got %+v", gate.Current)
			}
			// The guard must not reopen an API closed by hand
			if tt.schedule != nil && tt.schedule.Enabled {
				t.Error("Expected the guard's check stopped")
			}

			if tt.schedule != nil {
				tt.schedule.Enabled = true
			}
			if err := EnableAPI(context.Background(), gate, schedule, stage); err != nil {
				t.Fatalf("EnableAPI failed: %v", err)
			}
			if gate.Current != openThrottle {
				t.Errorf("Expected the published throttle restored, got %+v", gate.Current)
			}
			if tt.schedule != nil && tt.schedule.Enabled {
				t.Error("Expected the guard's check stopped")
			}
		})
	}
}

func TestDisableAPIReportsFailure(t *testing.T) {
	gate := &mocks.MockAPIGate{Current: openThrottle, SetErr: errors.New("access denied")}
	if err := DisableAPI(context.Background(), gate, nil); !errors.Is(err, gate.SetErr) {
		t.Errorf("Expected the failure, got %v", err)
	}
}

func TestFormatThrottle(t *testing.T) {
	if got := FormatThrottle(openThrottle); got != "open (rate 0.017/s, burst 1)" {
		t.Errorf("Got %q", got)
	}
	if got := FormatThrottle(ports.Throttle{}); !strings.HasPrefix(got, "closed") {
		t.Errorf("Got %q", got)
	}
}

func TestSetCheckInterval(t *testing.T) {
	schedule := &mocks.MockCheckSchedule{Enabled: true, Minutes: 15}
	if err := SetCheckInterval(context.Background(), schedule, 1); err != nil {
		t.Fatalf("SetCheckInterval failed: %v", err)
	}
	// A running check keeps running at the new rate
	if schedule.Minutes != 1 || !schedule.Enabled {
		t.Errorf("Expected 1 minute and still checking, got %+v", schedule)
	}
	for _, bad := range []int{0, -5, 1441} {
		if err := SetCheckInterval(context.Background(), schedule, bad); err == nil {
			t.Errorf("Expected %d refused", bad)
		}
	}
	if schedule.Minutes != 1 {
		t.Errorf("Expected the interval left at 1, got %d", schedule.Minutes)
	}
	if err := SetCheckInterval(context.Background(), nil, 5); !errors.Is(err, ErrGuardOff) {
		t.Errorf("Expected ErrGuardOff, got %v", err)
	}
}

func TestGuardStatus(t *testing.T) {
	tests := []struct {
		name     string
		schedule ports.CheckSchedule
		want     string
	}{
		{"off", nil, "off"},
		{"idle", &mocks.MockCheckSchedule{Minutes: 15}, "on; closes the API in a flood, then checks every 15 minutes"},
		{"checking", &mocks.MockCheckSchedule{Enabled: true, Minutes: 1}, "on; it closed the API and checks every minute"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GuardStatus(context.Background(), tt.schedule)
			if err != nil || !strings.HasPrefix(got, tt.want) {
				t.Errorf("Expected %q, got %q (%v)", tt.want, got, err)
			}
		})
	}
}
