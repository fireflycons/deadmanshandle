package main

import (
	"context"
	"fmt"
	"time"

	"github.com/fireflycons/deadmanshandle/internal/cli"
)

var recipientsCommands = map[string]command{
	"status": {"show whether SES will deliver to the owner and each recipient", runRecipientsStatus},
	"test":   {"send the owner and the recipients a test email, with a test document", runRecipientsTest},
}

func runRecipients(ctx context.Context, g globals, args []string) error {
	return subcommand(ctx, g, "recipients", recipientsCommands, []string{"status", "test"}, args)
}

func runRecipientsStatus(ctx context.Context, g globals, args []string) error {
	fs := newFlagSet("recipients status", "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("recipients status: unexpected argument %q", fs.Arg(0))
	}

	s, err := newSession(ctx, g)
	if err != nil {
		return err
	}
	cfg, err := cli.ReadConfig(ctx, s.ssm, s.configParameter())
	if err != nil {
		return fmt.Errorf("recipients status: %w", err)
	}
	report, err := cli.CheckRecipients(ctx, s.sesAccount(), cfg)
	if err != nil {
		return fmt.Errorf("recipients status: %w", err)
	}

	if report.Sandbox {
		fmt.Println("SES is in the sandbox: it delivers only to verified addresses, and addresses in")
		fmt.Println("verified domains. dmh recipients test asks SES to verify the others; to leave the")
		fmt.Println("sandbox, request production access in the SES console (Account dashboard).")
	} else {
		fmt.Println("SES is out of the sandbox: it delivers to any address.")
	}
	fmt.Println()
	for _, address := range report.Addresses {
		fmt.Printf("%-10s %-35s %s\n", role(address.Owner), address.Address, addressStatus(address, report.Sandbox))
	}
	return nil
}

// addressStatus describes whether SES will deliver to an address
func addressStatus(a cli.AddressStatus, sandbox bool) string {
	switch {
	case !sandbox:
		return "can receive"
	case a.VerifiedBy == a.Address:
		return "verified"
	case a.VerifiedBy != "":
		return "verified (domain " + a.VerifiedBy + ")"
	case a.Status == "PENDING":
		return "not verified: waiting for its SES verification link to be followed"
	default:
		return "not verified: dmh recipients test asks SES to verify it"
	}
}

func role(owner bool) string {
	if owner {
		return "owner"
	}
	return "recipient"
}

func runRecipientsTest(ctx context.Context, g globals, args []string) error {
	fs := newFlagSet("recipients test", "[recipient...]")
	if err := fs.Parse(args); err != nil {
		return err
	}

	s, err := newSession(ctx, g)
	if err != nil {
		return err
	}
	cfg, err := cli.ReadConfig(ctx, s.ssm, s.configParameter())
	if err != nil {
		return fmt.Errorf("recipients test: %w", err)
	}
	sender, err := s.emailSender(ctx)
	if err != nil {
		return fmt.Errorf("recipients test: %w", err)
	}
	results, err := cli.SendTestEmail(ctx, cli.TestEmail{
		Store:             s.ssm,
		PasswordParameter: s.passwordParameter(),
		Account:           s.sesAccount(),
		Sender:            sender,
		Now:               time.Now(),
		Pause:             time.Sleep,
	}, cfg, fs.Args())

	verifying := false
	for _, result := range results {
		fmt.Printf("%-10s %-35s %s\n", role(result.Owner), result.Address, testOutcome(result))
		verifying = verifying || result.Outcome == cli.TestVerificationSent || result.Outcome == cli.TestAwaitingVerification
	}
	if len(results) > 0 {
		fmt.Println()
		fmt.Println("The test changes nothing in the deadman's handle. Ask the recipients whether it")
		fmt.Println("reached their inbox, and whether the attachment opened with the password.")
	}
	if verifying {
		fmt.Println("SES is in the sandbox, so an address must be verified before it can receive mail:")
		fmt.Println("SES emails it a link (from no-reply-aws@amazon.com). Run this again once it has")
		fmt.Println("been followed, or request SES production access.")
	}
	if err != nil {
		return fmt.Errorf("recipients test: %w", err)
	}
	return nil
}

// testOutcome describes what happened to an address's test email
func testOutcome(r cli.TestResult) string {
	switch r.Outcome {
	case cli.TestSent:
		return "sent"
	case cli.TestVerificationSent:
		return "not sent: not verified, so SES has sent it a verification link"
	case cli.TestAwaitingVerification:
		return "not sent: waiting for its SES verification link to be followed"
	default:
		return "failed: " + r.Err.Error()
	}
}
