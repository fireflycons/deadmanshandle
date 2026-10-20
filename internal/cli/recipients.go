package cli

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/fireflycons/deadmanshandle/internal/config"
	"github.com/fireflycons/deadmanshandle/internal/doccrypt"
	"github.com/fireflycons/deadmanshandle/internal/domain"
	"github.com/fireflycons/deadmanshandle/internal/ports"
)

// SESAccount reads the SES sandbox state and identities, and starts
// verifying an address. Only the CLI uses SES this way, so it is not a port.
type SESAccount interface {
	// Sandbox reports whether the account is in the SES sandbox, and the
	// greatest number of emails it may send a second
	Sandbox(ctx context.Context) (sandbox bool, maxRate float64, err error)
	// IdentityStatus returns an identity's verification status, such as
	// "SUCCESS" or "PENDING", or "" if there is none
	IdentityStatus(ctx context.Context, identity string) (string, error)
	// VerifyAddress makes SES email the address a verification link
	VerifyAddress(ctx context.Context, address string) error
}

// SES identity verification statuses
const (
	identityVerified = "SUCCESS"
	identityPending  = "PENDING"
)

// TestDocumentName is the test document's name before encryption
const TestDocumentName = "dmh-test.txt"

// AddressStatus is whether SES will deliver to an address. In the sandbox
// it only delivers to verified addresses, or addresses in verified domains.
type AddressStatus struct {
	Address    string
	Owner      bool   // the owner's address, rather than a recipient's
	VerifiedBy string // the verified identity, the address or its domain; empty if neither is
	Status     string // the address's own identity's status, "" if it has none
}

// Deliverable reports whether SES will deliver to the address
func (a AddressStatus) Deliverable(sandbox bool) bool {
	return !sandbox || a.VerifiedBy != ""
}

// RecipientReport is the SES state for the owner and the recipients
type RecipientReport struct {
	Sandbox   bool
	MaxRate   float64 // emails a second
	Addresses []AddressStatus
}

// CheckRecipients reports whether SES will deliver to the owner and each
// recipient. Out of the sandbox it delivers to any address, so the
// identities aren't read.
func CheckRecipients(ctx context.Context, account SESAccount, cfg *config.Config) (RecipientReport, error) {
	return checkAddresses(ctx, account, cfg.Owner, cfg.Recipients)
}

func checkAddresses(ctx context.Context, account SESAccount, owner string, recipients []string) (RecipientReport, error) {
	sandbox, rate, err := account.Sandbox(ctx)
	if err != nil {
		return RecipientReport{}, fmt.Errorf("reading the SES account: %w", err)
	}
	report := RecipientReport{Sandbox: sandbox, MaxRate: rate}
	for i, address := range append([]string{owner}, recipients...) {
		status := AddressStatus{Address: address, Owner: i == 0}
		if sandbox {
			if status, err = checkAddress(ctx, account, status); err != nil {
				return RecipientReport{}, err
			}
		}
		report.Addresses = append(report.Addresses, status)
	}
	return report, nil
}

// checkAddress reads the identities of the address and of its domain
func checkAddress(ctx context.Context, account SESAccount, status AddressStatus) (AddressStatus, error) {
	var err error
	if status.Status, err = account.IdentityStatus(ctx, status.Address); err != nil {
		return status, fmt.Errorf("reading the SES identity %s: %w", status.Address, err)
	}
	if status.Status == identityVerified {
		status.VerifiedBy = status.Address
		return status, nil
	}
	_, domainName, _ := strings.Cut(status.Address, "@")
	domainStatus, err := account.IdentityStatus(ctx, domainName)
	if err != nil {
		return status, fmt.Errorf("reading the SES identity %s: %w", domainName, err)
	}
	if domainStatus == identityVerified {
		status.VerifiedBy = domainName
	}
	return status, nil
}

// TestOutcome is what happened to one address's test email
type TestOutcome int

const (
	// TestSent means the test email was sent
	TestSent TestOutcome = iota
	// TestVerificationSent means the address isn't verified, so SES was asked
	// to send it a verification link instead
	TestVerificationSent
	// TestAwaitingVerification means the address's verification link hasn't
	// been followed yet
	TestAwaitingVerification
	// TestFailed means sending, or asking for verification, failed
	TestFailed
)

// TestResult is the outcome for one address
type TestResult struct {
	Address string
	Owner   bool
	Outcome TestOutcome
	Err     error // set when Outcome is TestFailed
}

// TestEmail is what SendTestEmail needs
type TestEmail struct {
	Store             ports.ConfigStore // holds the document password
	PasswordParameter string
	Account           SESAccount
	Sender            ports.EmailSender
	Now               time.Time
	Pause             func(time.Duration) // between sends, to keep to the SES rate; time.Sleep
}

// SendTestEmail sends the test email, with a test document encrypted with
// the document password, to the owner and to the recipients: all of them, or those in
// only. In the sandbox, an address that isn't verified is sent a
// verification link by SES instead, unless it is still waiting for one to be
// followed. It changes no state. It returns an error if any address failed,
// after trying the others.
func SendTestEmail(ctx context.Context, t TestEmail, cfg *config.Config, only []string) ([]TestResult, error) {
	password, found, err := ReadDocumentPassword(ctx, t.Store, t.PasswordParameter)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrNoPassword
	}
	recipients := cfg.Recipients
	if len(only) > 0 {
		for _, address := range only {
			if !slices.ContainsFunc(cfg.Recipients, func(r string) bool { return strings.EqualFold(r, address) }) {
				return nil, fmt.Errorf("%s isn't a recipient (dmh config show lists them)", address)
			}
		}
		recipients = only
	}
	report, err := checkAddresses(ctx, t.Account, cfg.Owner, recipients)
	if err != nil {
		return nil, err
	}

	encrypted, err := doccrypt.Encrypt(domain.TestDocument(cfg.Owner, t.Now), TestDocumentName, password)
	if err != nil {
		return nil, err
	}
	name := TestDocumentName + doccrypt.Extension
	body := domain.TestEmailBody(name, cfg.Owner)

	interval := time.Second
	if report.MaxRate > 0 {
		interval = time.Duration(float64(time.Second) / report.MaxRate)
	}
	var results []TestResult
	failed, sent := 0, 0
	for _, address := range report.Addresses {
		result := TestResult{Address: address.Address, Owner: address.Owner}
		switch {
		case !address.Deliverable(report.Sandbox) && address.Status == identityPending:
			result.Outcome = TestAwaitingVerification
		case !address.Deliverable(report.Sandbox):
			result.Outcome = TestVerificationSent
			if err := t.Account.VerifyAddress(ctx, address.Address); err != nil {
				result.Outcome, result.Err = TestFailed, fmt.Errorf("asking SES to verify it: %w", err)
			}
		default:
			if sent > 0 {
				t.Pause(interval)
			}
			sent++
			result.Outcome = TestSent
			if err := t.Sender.SendEmail(ctx, address.Address, domain.TestEmailSubject, body, map[string][]byte{name: encrypted}); err != nil {
				result.Outcome, result.Err = TestFailed, err
			}
		}
		if result.Outcome == TestFailed {
			failed++
		}
		results = append(results, result)
	}
	if failed > 0 {
		return results, fmt.Errorf("%d of %d failed", failed, len(results))
	}
	return results, nil
}

// ErrNoPassword is returned when there is no document password to encrypt
// the test document with
var ErrNoPassword = errors.New("there is no document password: set one with dmh document password set")
