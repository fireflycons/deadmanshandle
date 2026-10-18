package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/fireflycons/deadmanshandle/internal/ports"
)

// APIStage is the /<app>/api-stage parameter, which Terraform keeps up to
// date: the check-in API's stage and the throttle that opens it
type APIStage struct {
	APIID      string  `json:"apiId"`
	Stage      string  `json:"stage"`
	RateLimit  float64 `json:"rateLimit"`
	BurstLimit int32   `json:"burstLimit"`
	// The flood guard's check rule; empty when the guard is off
	GuardRule string `json:"guardRule"`
}

// ParseAPIStage parses the api-stage parameter
func ParseAPIStage(data []byte) (*APIStage, error) {
	var stage APIStage
	if err := json.Unmarshal(data, &stage); err != nil {
		return nil, err
	}
	if stage.APIID == "" || stage.Stage == "" {
		return nil, errors.New("apiId or stage is missing")
	}
	if stage.Open().Closed() {
		return nil, errors.New("rateLimit and burstLimit are 0, which would leave the API closed")
	}
	return &stage, nil
}

// Open is the throttle that Terraform sets, under which the API is open
func (s *APIStage) Open() ports.Throttle {
	return ports.Throttle{RateLimit: s.RateLimit, BurstLimit: s.BurstLimit}
}

// DisableAPI closes the API: every request is refused with 429 without
// invoking the Lambda. The flood guard's check, if any, is stopped first, so
// that it cannot reopen the API.
func DisableAPI(ctx context.Context, gate ports.APIGate, schedule ports.CheckSchedule) error {
	if err := stopCheck(ctx, schedule); err != nil {
		return err
	}
	if err := gate.SetThrottle(ctx, ports.Throttle{}); err != nil {
		return fmt.Errorf("closing the API: %w", err)
	}
	return nil
}

// EnableAPI reopens the API with the throttle Terraform set, and stops the
// flood guard's check, if any
func EnableAPI(ctx context.Context, gate ports.APIGate, schedule ports.CheckSchedule, stage *APIStage) error {
	if err := stopCheck(ctx, schedule); err != nil {
		return err
	}
	if err := gate.SetThrottle(ctx, stage.Open()); err != nil {
		return fmt.Errorf("opening the API: %w", err)
	}
	return nil
}

// stopCheck stops the flood guard's check; schedule is nil if the guard is off
func stopCheck(ctx context.Context, schedule ports.CheckSchedule) error {
	if schedule == nil {
		return nil
	}
	if err := schedule.Disable(ctx); err != nil {
		return fmt.Errorf("stopping the flood guard's check: %w", err)
	}
	return nil
}

// FormatThrottle describes whether the API is open, and its throttle
func FormatThrottle(throttle ports.Throttle) string {
	if throttle.Closed() {
		return "closed (every request is refused)"
	}
	return "open (rate " + strconv.FormatFloat(throttle.RateLimit, 'g', -1, 64) +
		"/s, burst " + strconv.Itoa(int(throttle.BurstLimit)) + ")"
}

// ErrGuardOff is returned when the flood guard is needed but is off
var ErrGuardOff = errors.New("the flood guard is off (Terraform's api_flood_guard)")

// SetCheckInterval changes how often the flood guard checks whether to
// reopen the API. schedule is nil if the guard is off.
func SetCheckInterval(ctx context.Context, schedule ports.CheckSchedule, minutes int) error {
	if schedule == nil {
		return ErrGuardOff
	}
	if minutes < 1 || minutes > 1440 {
		return errors.New("the interval must be from 1 to 1440 minutes")
	}
	if err := schedule.SetInterval(ctx, minutes); err != nil {
		return fmt.Errorf("changing the check interval: %w", err)
	}
	return nil
}

// GuardStatus describes the flood guard. schedule is nil if the guard is off.
func GuardStatus(ctx context.Context, schedule ports.CheckSchedule) (string, error) {
	if schedule == nil {
		return "off", nil
	}
	minutes, checking, err := schedule.Interval(ctx)
	if err != nil {
		return "", fmt.Errorf("reading the flood guard's check: %w", err)
	}
	every := "every minute"
	if minutes != 1 {
		every = "every " + strconv.Itoa(minutes) + " minutes"
	}
	if checking {
		return "on; it closed the API and checks " + every + " whether to reopen it", nil
	}
	return "on; closes the API in a flood, then checks " + every + " whether to reopen it", nil
}
