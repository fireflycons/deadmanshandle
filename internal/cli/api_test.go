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
