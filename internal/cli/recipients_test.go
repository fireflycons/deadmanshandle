package cli

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/fireflycons/deadmanshandle/internal/doccrypt"
	"github.com/fireflycons/deadmanshandle/internal/domain"
	"github.com/fireflycons/deadmanshandle/internal/mocks"
)

func TestCheckRecipients(t *testing.T) {
	tests := []struct {
		name       string
		account    mocks.MockSESAccount
		verifiedBy []string // owner, a, b
		deliver    []bool
	}{
		{
			name:       "out of the sandbox, every address",
			account:    mocks.MockSESAccount{},
			verifiedBy: []string{"", "", ""},
			deliver:    []bool{true, true, true},
		},
		{
			name:       "verified domain",
			account:    mocks.MockSESAccount{InSandbox: true, Identities: map[string]string{"example.com": "SUCCESS"}},
			verifiedBy: []string{"example.com", "example.com", "example.com"},
			deliver:    []bool{true, true, true},
		},
		{
			name: "verified addresses only",
			account: mocks.MockSESAccount{InSandbox: true, Identities: map[string]string{
				"example.com":       "PENDING",
				"owner@example.com": "SUCCESS",
				"a@example.com":     "SUCCESS",
				"b@example.com":     "PENDING",
			}},
			verifiedBy: []string{"owner@example.com", "a@example.com", ""},
			deliver:    []bool{true, true, false},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, err := CheckRecipients(context.Background(), &tt.account, testConfig())
			if err != nil {
				t.Fatalf("CheckRecipients failed: %v", err)
			}
			if report.Sandbox != tt.account.InSandbox {
				t.Errorf("Expected sandbox %v, got %v", tt.account.InSandbox, report.Sandbox)
			}
			if len(report.Addresses) != 3 || !report.Addresses[0].Owner || report.Addresses[0].Address != "owner@example.com" {
				t.Fatalf("Expected the owner then the two recipients, got %+v", report.Addresses)
			}
			for i, address := range report.Addresses {
				if address.VerifiedBy != tt.verifiedBy[i] {
					t.Errorf("%s: expected verified by %q, got %q", address.Address, tt.verifiedBy[i], address.VerifiedBy)
				}
				if got := address.Deliverable(report.Sandbox); got != tt.deliver[i] {
					t.Errorf("%s: expected deliverable %v, got %v", address.Address, tt.deliver[i], got)
				}
			}
		})
	}
}

var testEmailNow = time.Date(2026, 10, 20, 9, 30, 0, 0, time.UTC)

// testEmail returns a TestEmail whose store holds password (none if empty),
// and the pauses it makes
func testEmail(password string, account *mocks.MockSESAccount, sender *mocks.MockEmailSender) (TestEmail, *[]time.Duration) {
	store := mocks.NewMockConfigStore()
	if password != "" {
		store.Data[testPasswordParam] = []byte(password)
	}
	var pauses []time.Duration
	return TestEmail{
		Store:             store,
		PasswordParameter: testPasswordParam,
		Account:           account,
		Sender:            sender,
		Now:               testEmailNow,
		Pause:             func(d time.Duration) { pauses = append(pauses, d) },
	}, &pauses
}

func TestSendTestEmail(t *testing.T) {
	sender := &mocks.MockEmailSender{}
	te, pauses := testEmail(newPassword, &mocks.MockSESAccount{MaxRate: 2}, sender)
	results, err := SendTestEmail(context.Background(), te, testConfig(), nil)
	if err != nil {
		t.Fatalf("SendTestEmail failed: %v", err)
	}

	want := []string{"owner@example.com", "a@example.com", "b@example.com"}
	if len(results) != 3 || len(sender.SentEmails) != 3 {
		t.Fatalf("Expected 3 results and emails, got %+v and %d", results, len(sender.SentEmails))
	}
	for i, email := range sender.SentEmails {
		if email.To != want[i] || results[i].Address != want[i] || results[i].Outcome != TestSent {
			t.Errorf("Expected %s sent, got %s, %+v", want[i], email.To, results[i])
		}
		if email.Subject != domain.TestEmailSubject || email.Body != domain.TestEmailBody("dmh-test.txt.dmh", "owner@example.com") {
			t.Errorf("Unexpected email: %s\n%s", email.Subject, email.Body)
		}
		name, content, err := doccrypt.Decrypt(email.Attachments["dmh-test.txt.dmh"], newPassword)
		if err != nil || name != TestDocumentName || string(content) != string(domain.TestDocument("owner@example.com", testEmailNow)) {
			t.Errorf("Expected the attachment to open with the password, got %q, %q, %v", name, content, err)
		}
	}
	if !results[0].Owner || results[1].Owner {
		t.Errorf("Expected only the first result to be the owner's: %+v", results)
	}
	// 2 a second
	if !slices.Equal(*pauses, []time.Duration{500 * time.Millisecond, 500 * time.Millisecond}) {
		t.Errorf("Expected a pause between sends, got %v", *pauses)
	}
}

func TestSendTestEmailSandbox(t *testing.T) {
	account := &mocks.MockSESAccount{InSandbox: true, MaxRate: 1, Identities: map[string]string{
		"owner@example.com": "SUCCESS",
		"a@example.com":     "PENDING",
		"c@example.com":     "FAILED",
	}}
	cfg := testConfig()
	cfg.Recipients = []string{"a@example.com", "b@example.com", "c@example.com"}
	sender := &mocks.MockEmailSender{}
	te, pauses := testEmail(newPassword, account, sender)
	results, err := SendTestEmail(context.Background(), te, cfg, nil)
	if err != nil {
		t.Fatalf("SendTestEmail failed: %v", err)
	}

	outcomes := []TestOutcome{TestSent, TestAwaitingVerification, TestVerificationSent, TestVerificationSent}
	for i, result := range results {
		if result.Outcome != outcomes[i] {
			t.Errorf("%s: expected outcome %d, got %d", result.Address, outcomes[i], result.Outcome)
		}
	}
	if len(sender.SentEmails) != 1 || sender.SentEmails[0].To != "owner@example.com" {
		t.Errorf("Expected only the owner sent the email, got %+v", sender.SentEmails)
	}
	// An expired link (FAILED) is sent again; a pending one isn't
	if !slices.Equal(account.Verified, []string{"b@example.com", "c@example.com"}) {
		t.Errorf("Expected verification for b and c, got %v", account.Verified)
	}
	if len(*pauses) != 0 {
		t.Errorf("Expected no pause for a single send, got %v", *pauses)
	}
}

func TestSendTestEmailOnly(t *testing.T) {
	sender := &mocks.MockEmailSender{}
	te, _ := testEmail(newPassword, &mocks.MockSESAccount{}, sender)
	if _, err := SendTestEmail(context.Background(), te, testConfig(), []string{"B@Example.com"}); err != nil {
		t.Fatalf("SendTestEmail failed: %v", err)
	}
	if len(sender.SentEmails) != 2 || sender.SentEmails[0].To != "owner@example.com" || sender.SentEmails[1].To != "B@Example.com" {
		t.Errorf("Expected the owner and b, got %+v", sender.SentEmails)
	}

	if _, err := SendTestEmail(context.Background(), te, testConfig(), []string{"stranger@example.com"}); err == nil {
		t.Error("Expected an address that isn't a recipient to be refused")
	}
}

func TestSendTestEmailFailures(t *testing.T) {
	refused := errors.New("MessageRejected")
	sender := &mocks.MockEmailSender{FailFor: map[string]error{"a@example.com": refused}}
	te, _ := testEmail(newPassword, &mocks.MockSESAccount{}, sender)
	results, err := SendTestEmail(context.Background(), te, testConfig(), nil)
	if err == nil {
		t.Fatal("Expected an error when a send fails")
	}
	if results[1].Outcome != TestFailed || !errors.Is(results[1].Err, refused) {
		t.Errorf("Expected a's send to fail, got %+v", results[1])
	}
	if results[2].Outcome != TestSent || len(sender.SentEmails) != 2 {
		t.Errorf("Expected the others still sent, got %+v", results)
	}

	verifyFails := &mocks.MockSESAccount{InSandbox: true, VerifyErr: errors.New("throttled"), Identities: map[string]string{"owner@example.com": "SUCCESS"}}
	te, _ = testEmail(newPassword, verifyFails, &mocks.MockEmailSender{})
	results, err = SendTestEmail(context.Background(), te, testConfig(), nil)
	if err == nil || results[1].Outcome != TestFailed {
		t.Errorf("Expected a failed verification to be an error, got %+v, %v", results, err)
	}

	te, _ = testEmail("", &mocks.MockSESAccount{}, &mocks.MockEmailSender{})
	if _, err := SendTestEmail(context.Background(), te, testConfig(), nil); !errors.Is(err, ErrNoPassword) {
		t.Errorf("Expected ErrNoPassword without a password, got %v", err)
	}
}
