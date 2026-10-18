// Package cli holds the logic of the dmh command, which manages a deployment
// directly through AWS rather than through the Lambdas
package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/fireflycons/deadmanshandle/internal/config"
	"github.com/fireflycons/deadmanshandle/internal/domain"
	"github.com/fireflycons/deadmanshandle/internal/ports"
)

// deadlineFormat is how deadlines are shown, as in the emails and the app
const deadlineFormat = "Monday 2 January 2006 at 15:04 MST"

// FormatTime shows t in UTC, and also in local time if that differs
func FormatTime(t time.Time) string {
	utc := t.UTC().Format(deadlineFormat)
	if local := t.Local(); localOffset(local) != 0 {
		return utc + " (" + local.Format(deadlineFormat) + ")"
	}
	return utc
}

// localOffset is t's offset from UTC in seconds, which is zero for GMT as
// well as UTC
func localOffset(t time.Time) int {
	_, offset := t.Zone()
	return offset
}

// ReadConfig reads and validates the stored config, as the Lambdas do
func ReadConfig(ctx context.Context, store ports.ConfigStore, parameterName string) (*config.Config, error) {
	data, err := store.GetConfig(ctx, parameterName)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", parameterName, err)
	}
	cfg, err := config.ParseConfig(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", parameterName, err)
	}
	return cfg, nil
}

// CheckIn does what a check-in through the API does: it sets the timeout to
// resetDays from now and clears the delivery state. The recorded push token
// is kept. It returns the new timeout.
func CheckIn(ctx context.Context, configStore ports.ConfigStore, parameterName string, stateStore ports.StateStore, service *domain.DeadmansHandleService) (time.Time, error) {
	cfg, err := ReadConfig(ctx, configStore, parameterName)
	if err != nil {
		return time.Time{}, err
	}
	timeout := service.CheckIn(cfg)
	if err := stateStore.CheckIn(ctx, timeout, ""); err != nil {
		return time.Time{}, fmt.Errorf("saving the new timeout: %w", err)
	}
	return timeout, nil
}
