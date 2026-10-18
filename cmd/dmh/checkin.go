package main

import (
	"context"
	"fmt"

	"github.com/fireflycons/deadmanshandle/internal/cli"
	"github.com/fireflycons/deadmanshandle/internal/domain"
)

// runCheckIn checks in, as the app does, but straight on the state: it
// needs neither the API key nor the API, and is not throttled
func runCheckIn(ctx context.Context, g globals, args []string) error {
	fs := newFlagSet("checkin", "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("checkin: unexpected argument %q", fs.Arg(0))
	}

	s, err := newSession(ctx, g)
	if err != nil {
		return err
	}
	stateStore, err := s.stateStore(ctx)
	if err != nil {
		return err
	}
	timeout, err := cli.CheckIn(ctx, s.ssm, s.configParameter(), stateStore, domain.NewDeadmansHandleService())
	if err != nil {
		return fmt.Errorf("checkin: %w", err)
	}
	fmt.Println("Checked in. The new deadline is", cli.FormatTime(timeout))
	return nil
}
