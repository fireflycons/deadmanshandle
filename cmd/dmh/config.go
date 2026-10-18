package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fireflycons/deadmanshandle/internal/cli"
	"github.com/fireflycons/deadmanshandle/internal/config"
	"github.com/fireflycons/deadmanshandle/internal/domain"
)

var configCommands = map[string]command{
	"show": {"show the settings, what Terraform fixed, and the state", runConfigShow},
	"set":  {"change the settings, the timeout or the push token", runConfigSet},
}

func runConfig(ctx context.Context, g globals, args []string) error {
	return subcommand(ctx, g, "config", configCommands, []string{"show", "set"}, args)
}

func runConfigShow(ctx context.Context, g globals, args []string) error {
	fs := newFlagSet("config show", "[flags]")
	showSecrets := fs.Bool("show-secrets", false, "show the API key and push token in full")
	asJSON := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("config show: unexpected argument %q", fs.Arg(0))
	}

	s, err := newSession(ctx, g)
	if err != nil {
		return err
	}
	configData, err := s.ssm.GetConfig(ctx, s.configParameter())
	if err != nil {
		return fmt.Errorf("config show: reading %s: %w", s.configParameter(), err)
	}

	// What can't be found is reported, not fatal: the parameters Terraform
	// publishes are missing until an upgrade is applied
	var problems []string
	lookup := func(name string) string {
		value, err := s.lookup(ctx, name)
		if err != nil {
			problems = append(problems, err.Error())
		}
		return value
	}
	deployment := cli.Deployment{
		Endpoint:       lookup("api-endpoint"),
		DocumentBucket: lookup("document-bucket"),
		StateTable:     lookup("state-table"),
		SenderEmail:    lookup("sender-email"),
	}
	var state *config.State
	if stateStore, err := s.stateStore(ctx); err == nil {
		if state, err = stateStore.GetState(ctx); err != nil {
			problems = append(problems, "reading the state: "+err.Error())
		}
	}

	report := cli.NewReport(s.configParameter(), configData, deployment, state, *showSecrets)
	report.Problems = append(report.Problems, problems...)
	if *asJSON {
		out, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	}
	return report.WriteText(os.Stdout, time.Now())
}

func runConfigSet(ctx context.Context, g globals, args []string) error {
	fs := newFlagSet("config set", "[flags]")
	var change cli.ConfigChange
	fs.Func("owner", "the owner's email address", func(v string) error {
		change.Owner = &v
		return nil
	})
	fs.Func("recipients", "the recipients' email addresses, comma-separated; replaces the list", func(v string) error {
		var list []string
		for r := range strings.SplitSeq(v, ",") {
			if r = strings.TrimSpace(r); r != "" {
				list = append(list, r)
			}
		}
		change.Recipients = &list
		return nil
	})
	fs.Func("reset-days", "days from a check-in to the deadline", intFlag(&change.ResetDays))
	fs.Func("warn-days", "days before the deadline that the warnings start", intFlag(&change.WarnDays))
	fs.Func("api-key", "the API key the app checks in with; the app then needs a new QR code", func(v string) error {
		change.APIKey = &v
		return nil
	})
	fs.Func("document-key", "the document's S3 key; dmh document put sets it too", func(v string) error {
		change.DocumentKey = &v
		return nil
	})
	var timeout time.Time
	fs.Func("timeout", "set the deadline to this RFC 3339 time, clearing the delivery state as a check-in does", func(v string) error {
		var err error
		timeout, err = time.Parse(time.RFC3339, v)
		return err
	})
	clearPushToken := fs.Bool("clear-push-token", false, "forget the app's push token, to stop pushes to a lost phone")
	force := fs.Bool("force", false, "allow a -timeout that has passed, which sends the document on the next daily run")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("config set: unexpected argument %q", fs.Arg(0))
	}
	if change.IsZero() && timeout.IsZero() && !*clearPushToken {
		fs.Usage()
		return errors.New("config set: nothing to set")
	}

	s, err := newSession(ctx, g)
	if err != nil {
		return err
	}

	// The settings first: they need only the config parameter, so they
	// work before an upgrade's terraform apply
	if !change.IsZero() {
		if _, err := cli.SetConfig(ctx, s.ssm, s.configParameter(), change); err != nil {
			return fmt.Errorf("config set: %w", err)
		}
		fmt.Println("Settings saved. Check-ins can take up to 5 minutes to see them.")
		if change.APIKey != nil {
			fmt.Println("The app needs a new QR code for the new API key: dmh qr -aws")
		}
		if change.DocumentKey != nil {
			fmt.Println("The document must be at the new key; dmh document put uploads it.")
		}
	}

	if !timeout.IsZero() || *clearPushToken {
		stateStore, err := s.stateStore(ctx)
		if err != nil {
			return fmt.Errorf("config set: %w", err)
		}
		if !timeout.IsZero() {
			err := cli.SetTimeout(ctx, stateStore, domain.NewDeadmansHandleService(), timeout, *force)
			if errors.Is(err, cli.ErrTimeoutPast) {
				return fmt.Errorf("config set: %w; add -force to set it anyway", err)
			}
			if err != nil {
				return fmt.Errorf("config set: %w", err)
			}
			fmt.Println("Timeout set to", cli.FormatTime(timeout))
		}
		if *clearPushToken {
			if err := stateStore.ClearPushToken(ctx); err != nil {
				return fmt.Errorf("config set: clearing the push token: %w", err)
			}
			fmt.Println("Push token cleared. The next check-in from the app records a new one.")
		}
	}
	return nil
}

// intFlag parses a flag's value into *target
func intFlag(target **int) func(string) error {
	return func(v string) error {
		n, err := strconv.Atoi(v)
		if err != nil {
			return errors.New("not a whole number")
		}
		*target = &n
		return nil
	}
}
