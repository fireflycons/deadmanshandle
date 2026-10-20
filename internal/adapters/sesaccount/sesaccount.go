// Package sesaccount reads the SES account's sandbox state and identities for
// dmh. It is apart from adapters so that the Lambdas, which only send through
// SES v1, don't link the SES v2 client.
package sesaccount

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

// Account reads the SES account's sandbox state and its identities, and
// starts verifying an address. Only dmh uses it: the Lambdas send through
// SES v1 and never change the account.
type Account struct {
	client *sesv2.Client
}

// New creates an SES account reader
func New(client *sesv2.Client) *Account {
	return &Account{client: client}
}

// Sandbox reports whether the account is in the SES sandbox, and the
// greatest number of emails it may send a second
func (a *Account) Sandbox(ctx context.Context) (bool, float64, error) {
	account, err := a.client.GetAccount(ctx, &sesv2.GetAccountInput{})
	if err != nil {
		return false, 0, err
	}
	var rate float64
	if account.SendQuota != nil {
		rate = account.SendQuota.MaxSendRate
	}
	return !account.ProductionAccessEnabled, rate, nil
}

// IdentityStatus returns the verification status of an identity, an email
// address or a domain, such as "SUCCESS" or "PENDING": empty if there is no
// such identity
func (a *Account) IdentityStatus(ctx context.Context, identity string) (string, error) {
	output, err := a.client.GetEmailIdentity(ctx, &sesv2.GetEmailIdentityInput{EmailIdentity: &identity})
	var notFound *types.NotFoundException
	if errors.As(err, &notFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(output.VerificationStatus), nil
}

// VerifyAddress creates an identity for an email address, so SES emails it
// a link that verifies it. An identity that already exists, such as one
// whose link expired unused, is replaced, since SES only sends the link when
// the identity is created.
func (a *Account) VerifyAddress(ctx context.Context, address string) error {
	_, err := a.client.CreateEmailIdentity(ctx, &sesv2.CreateEmailIdentityInput{EmailIdentity: &address})
	var exists *types.AlreadyExistsException
	if !errors.As(err, &exists) {
		return err
	}
	if _, err := a.client.DeleteEmailIdentity(ctx, &sesv2.DeleteEmailIdentityInput{EmailIdentity: &address}); err != nil {
		return err
	}
	_, err = a.client.CreateEmailIdentity(ctx, &sesv2.CreateEmailIdentityInput{EmailIdentity: &address})
	return err
}
