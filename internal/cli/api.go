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
