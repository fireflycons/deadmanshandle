package main

import (
	"context"
	"fmt"

	"github.com/fireflycons/deadmanshandle/internal/cli"
)

var apiCommands = map[string]command{
	"status":  {"show whether the check-in API is open", runAPIStatus},
	"disable": {"close the API: every request is refused with 429", runAPIDisable},
	"enable":  {"reopen the API with the throttle Terraform set", runAPIEnable},
}

func runAPI(ctx context.Context, g globals, args []string) error {
	return subcommand(ctx, g, "api", apiCommands, []string{"status", "disable", "enable"}, args)
}

func runAPIStatus(ctx context.Context, g globals, args []string) error {
	s, err := apiSession(ctx, g, "status", args)
	if err != nil {
		return err
	}
	_, gate, _, err := s.api(ctx)
	if err != nil {
		return fmt.Errorf("api status: %w", err)
	}
	throttle, err := gate.Throttle(ctx)
	if err != nil {
		return fmt.Errorf("api status: %w", err)
	}
	fmt.Println("The check-in API is", cli.FormatThrottle(throttle))
	return nil
}

func runAPIDisable(ctx context.Context, g globals, args []string) error {
	s, err := apiSession(ctx, g, "disable", args)
	if err != nil {
		return err
	}
	_, gate, schedule, err := s.api(ctx)
	if err != nil {
		return fmt.Errorf("api disable: %w", err)
	}
	if err := cli.DisableAPI(ctx, gate, schedule); err != nil {
		return fmt.Errorf("api disable: %w", err)
	}
	fmt.Println("The check-in API is closed: every request is refused with 429, and the")
	fmt.Println("Lambda is not invoked. Check in with dmh checkin; dmh api enable reopens it.")
	return nil
}

func runAPIEnable(ctx context.Context, g globals, args []string) error {
	s, err := apiSession(ctx, g, "enable", args)
	if err != nil {
		return err
	}
	stage, gate, schedule, err := s.api(ctx)
	if err != nil {
		return fmt.Errorf("api enable: %w", err)
	}
	if err := cli.EnableAPI(ctx, gate, schedule, stage); err != nil {
		return fmt.Errorf("api enable: %w", err)
	}
	fmt.Println("The check-in API is", cli.FormatThrottle(stage.Open()))
	return nil
}

// apiSession parses an api subcommand's (lack of) flags and opens a session
func apiSession(ctx context.Context, g globals, name string, args []string) (*session, error) {
	fs := newFlagSet("api "+name, "")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if fs.NArg() > 0 {
		return nil, fmt.Errorf("api %s: unexpected argument %q", name, fs.Arg(0))
	}
	return newSession(ctx, g)
}
