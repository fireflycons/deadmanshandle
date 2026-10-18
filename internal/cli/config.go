package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/fireflycons/deadmanshandle/internal/config"
	"github.com/fireflycons/deadmanshandle/internal/domain"
	"github.com/fireflycons/deadmanshandle/internal/ports"
)

// ConfigReadWriter reads and replaces the stored config. Only the CLI
// writes it, so writing is not part of ports.ConfigStore.
type ConfigReadWriter interface {
	ports.ConfigStore
	PutConfig(ctx context.Context, parameterName string, value []byte) error
}

// StateEditor is the state store with the edits that only the CLI makes
type StateEditor interface {
	ports.StateStore
	ClearPushToken(ctx context.Context) error
}

// ConfigChange holds the settings to change; nil fields are left as they are
type ConfigChange struct {
	Owner       *string
	Recipients  *[]string
	ResetDays   *int
	WarnDays    *int
	APIKey      *string
	DocumentKey *string
}

// fields maps each set field to its JSON name in the stored config
func (c ConfigChange) fields() map[string]any {
	fields := map[string]any{}
	if c.Owner != nil {
		fields["owner"] = *c.Owner
	}
	if c.Recipients != nil {
		fields["recipients"] = *c.Recipients
	}
	if c.ResetDays != nil {
		fields["resetDays"] = *c.ResetDays
	}
	if c.WarnDays != nil {
		fields["warnDays"] = *c.WarnDays
	}
	if c.APIKey != nil {
		fields["apiKey"] = *c.APIKey
	}
	if c.DocumentKey != nil {
		fields["documentKey"] = *c.DocumentKey
	}
	return fields
}

// IsZero reports whether nothing is to be changed
func (c ConfigChange) IsZero() bool {
	return len(c.fields()) == 0
}

// SetConfig applies change to the stored config and returns the result. The
// config is written only if the result is valid, so the Lambdas never see an
// invalid one. It is a read-modify-write: fields the CLI does not know are
// kept, but a concurrent change to the same parameter can be lost.
func SetConfig(ctx context.Context, store ConfigReadWriter, parameterName string, change ConfigChange) (*config.Config, error) {
	data, err := store.GetConfig(ctx, parameterName)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", parameterName, err)
	}
	var stored map[string]json.RawMessage
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("%s is not a JSON object: %w", parameterName, err)
	}

	for name, value := range change.fields() {
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		stored[name] = encoded
	}
	updated, err := json.Marshal(stored)
	if err != nil {
		return nil, err
	}
	cfg, err := config.ParseConfig(updated)
	if err != nil {
		return nil, fmt.Errorf("not changed, as the result would be invalid: %w", err)
	}

	if err := store.PutConfig(ctx, parameterName, updated); err != nil {
		return nil, fmt.Errorf("writing %s: %w", parameterName, err)
	}
	return cfg, nil
}

// ErrTimeoutPast is returned by SetTimeout for a time that has passed
var ErrTimeoutPast = errors.New("the timeout has already passed, so the document would be sent on the next daily run")

// SetTimeout sets the timeout as a check-in does, clearing the delivery state.
// A time that has passed is refused unless force is set.
func SetTimeout(ctx context.Context, store ports.StateStore, service *domain.DeadmansHandleService, timeout time.Time, force bool) error {
	if !timeout.After(service.Now()) && !force {
		return ErrTimeoutPast
	}
	if err := store.CheckIn(ctx, timeout, ""); err != nil {
		return fmt.Errorf("saving the timeout: %w", err)
	}
	return nil
}
