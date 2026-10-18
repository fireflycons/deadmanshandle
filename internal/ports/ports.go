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
	// CheckIns. It is unconditional: a check-in always wins. It also records
	// pushToken, unless that is empty, which leaves the recorded one.
	CheckIn(ctx context.Context, timeout time.Time, pushToken string) error
	// RecordSent adds recipient to SentTo, and RecordOwnerNotified sets
	// OwnerNotified, if CheckIns still equals checkIns. Otherwise they return
	// ErrConditionFailed.
	RecordSent(ctx context.Context, checkIns int64, recipient string) error
	RecordOwnerNotified(ctx context.Context, checkIns int64) error
	// SwapDocumentETag sets DocumentETag to current if it still equals
	// previous (empty for none). Unless changedAt is zero, it also sets
	// DocumentChangedAt to it and DocumentChangePending. Otherwise it returns
	// ErrConditionFailed.
	SwapDocumentETag(ctx context.Context, previous, current string, changedAt time.Time) error
	// ClearDocumentChangePending clears DocumentChangePending if DocumentETag
	// is still etag. Otherwise it returns ErrConditionFailed.
	ClearDocumentChangePending(ctx context.Context, etag string) error
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

// PushSender sends a push notification to the owner's phone. kind
// identifies the message, so the app can choose how to show it.
type PushSender interface {
	Push(ctx context.Context, token, kind, title, body string) error
}

// Throttle is the check-in API stage's default route throttle. Rate and
// burst 0 refuse every request with 429, without invoking the Lambda.
type Throttle struct {
	RateLimit  float64 // requests per second
	BurstLimit int32
}

// Closed reports whether the throttle refuses every request
func (t Throttle) Closed() bool {
	return t.RateLimit == 0 && t.BurstLimit == 0
}

// APIGate opens and closes the check-in API through its stage throttle
type APIGate interface {
	Throttle(ctx context.Context) (Throttle, error)
	SetThrottle(ctx context.Context, throttle Throttle) error
}

// CheckSchedule turns on and off the flood guard's periodic check, which
// reopens the API once a flood has stopped
type CheckSchedule interface {
	Enable(ctx context.Context) error
	Disable(ctx context.Context) error
	// Interval returns how often the check runs, in minutes, and whether it
	// is running
	Interval(ctx context.Context) (minutes int, enabled bool, err error)
	// SetInterval changes how often the check runs, leaving it running or
	// not as it was
	SetInterval(ctx context.Context, minutes int) error
}

// AlarmStates reads CloudWatch alarm states
type AlarmStates interface {
	InAlarm(ctx context.Context, alarmName string) (bool, error)
}

// APIKeyValidator defines the interface for validating API keys
type APIKeyValidator interface {
	ValidateAPIKey(ctx context.Context, providedKey string, expectedKey string) bool
}
