package ports

import (
	"context"
	"errors"
	"time"

	"github.com/fireflycons/deadmanshandle/internal/config"
)

// ErrDocumentChanged is returned by DocumentStore.GetDocument when the
// document no longer has the ETag it was checked with
var ErrDocumentChanged = errors.New("document changed since it was checked")

// ErrConditionFailed is returned by a conditional StateStore update when the
// state has changed since it was read
var ErrConditionFailed = errors.New("state changed concurrently")

// ConfigStore defines the interface for retrieving configuration
type ConfigStore interface {
	GetConfig(ctx context.Context, parameterName string) ([]byte, error)
}

// StateStore holds the handle's mutable state. Each update is atomic, so
// concurrent Lambdas cannot lose each other's writes.
type StateStore interface {
	GetState(ctx context.Context) (*config.State, error)
	// CheckIn sets the timeout, clears the delivery state and increments
	// CheckIns. It is unconditional: a check-in always wins.
	CheckIn(ctx context.Context, timeout time.Time) error
	// RecordSent adds recipient to SentTo, and RecordOwnerNotified sets
	// OwnerNotified, if CheckIns still equals checkIns. Otherwise they return
	// ErrConditionFailed.
	RecordSent(ctx context.Context, checkIns int64, recipient string) error
	RecordOwnerNotified(ctx context.Context, checkIns int64) error
	// SwapDocumentETag sets DocumentETag to current if it still equals
	// previous (empty for none). Otherwise it returns ErrConditionFailed.
	SwapDocumentETag(ctx context.Context, previous, current string) error
}

// DocumentStore defines the interface for retrieving documents from storage
type DocumentStore interface {
	// GetDocument fetches the document only if its ETag is still etag, so
	// that what is sent is what was checked. Otherwise it returns
	// ErrDocumentChanged.
	GetDocument(ctx context.Context, bucket, key, etag string) ([]byte, error)
	// DocumentETag returns the object's ETag, which changes with its content.
	// It reports exists false, with no error, only if there is no such object.
	DocumentETag(ctx context.Context, bucket, key string) (etag string, exists bool, err error)
}

// EmailSender defines the interface for sending emails
type EmailSender interface {
	SendEmail(ctx context.Context, to, subject, body string, attachments map[string][]byte) error
}

// APIKeyValidator defines the interface for validating API keys
type APIKeyValidator interface {
	ValidateAPIKey(ctx context.Context, providedKey string, expectedKey string) bool
}
