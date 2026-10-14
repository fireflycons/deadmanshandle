package domain

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/fireflycons/deadmanshandle/internal/config"
)

// EmailKind identifies why an email is being sent
type EmailKind int

const (
	// EmailDocument delivers the document to a recipient
	EmailDocument EmailKind = iota
	// EmailTriggerNotice tells the owner that the document has been sent
	EmailTriggerNotice
	// EmailWarning reminds the owner to check in
	EmailWarning
	// EmailDocumentMissing warns the owner, before the timeout, that the
	// document is missing and could not be sent
	EmailDocumentMissing
	// EmailDeliveryBlocked tells the owner that the timeout has passed but
	// the document is missing, so nothing was sent to the recipients
	EmailDeliveryBlocked
	// EmailDocumentChanged tells the owner that the document's content
	// changed, in case someone else changed it
	EmailDocumentChanged
	// EmailDeliveryHeld tells the owner that the timeout has passed but the
	// document changed recently, so it has not been sent yet
	EmailDeliveryHeld
)

// ChangeHoldPeriod is how long after a change of the document its delivery
// is held, so that the owner can react to an unauthorised change before the
// recipients receive it. It runs from when the change was found, whether or
// not the owner could be told, so a failing notice cannot hold delivery
// forever.
const ChangeHoldPeriod = 24 * time.Hour

// ChangeNoticeRetryAfter is how long after a change was found a later check
// may resend a notice that is still pending. It exceeds the Lambda timeouts,
// so the check that found the change has finished, or failed, by then.
const ChangeNoticeRetryAfter = 2 * time.Minute

// Document describes the document to be sent, as found by the scheduled run
type Document struct {
	Location  string // Where the owner should upload it, e.g. s3://bucket/key
	Available bool
	ETag      string // Changes with the content; empty when not Available
}

// ChangeOrigin describes who changed the document, as reported by S3
type ChangeOrigin struct {
	Requester string // AWS account ID (or service) that made the request
	SourceIP  string
}

// EmailAction represents an email that should be sent
type EmailAction struct {
	Kind    EmailKind
	To      string
	Subject string
	Body    string
}

// AttachDocument reports whether the document should be attached
func (e EmailAction) AttachDocument() bool {
	return e.Kind == EmailDocument
}

// DeadmansHandleService contains the core business logic
type DeadmansHandleService struct {
	// now is read on every call rather than captured once, because a warm
	// Lambda container reuses the service across invocations.
	// For testability, allows injection of a fixed time.
	now func() time.Time
}

// NewDeadmansHandleService creates a new service using the system clock (UTC)
func NewDeadmansHandleService() *DeadmansHandleService {
	return &DeadmansHandleService{
		now: func() time.Time { return time.Now().UTC() },
	}
}

// NewDeadmansHandleServiceWithTime creates a new service with a specific time (for testing)
func NewDeadmansHandleServiceWithTime(t time.Time) *DeadmansHandleService {
	return &DeadmansHandleService{
		now: func() time.Time { return t },
	}
}

// CheckIn handles the owner's check-in via HTTP API. It returns the new
// timeout; storing it also clears the delivery state.
func (s *DeadmansHandleService) CheckIn(cfg *config.Config) time.Time {
	return s.now().AddDate(0, 0, cfg.ResetDays)
}

// ProcessScheduledEvent handles the EventBridge scheduled event
// Returns emails that should be sent. All decisions are made from a single
// reading of the clock so they cannot disagree if the timeout falls between
// two readings.
//
// Once the timeout has passed, the document is sent only to recipients not
// yet recorded in state.SentTo, and the owner is notified once. Record each
// successful send in the state so that later runs skip it.
//
// If the document is missing, the owner is warned instead: daily before the
// timeout, and with an EmailDeliveryBlocked notice on each run after it while
// recipients are still waiting. If its content changed within
// ChangeHoldPeriod, the owner gets an EmailDeliveryHeld notice instead, and
// the document is sent on the first run after the hold ends.
func (s *DeadmansHandleService) ProcessScheduledEvent(ctx context.Context, cfg *config.Config, state *config.State, doc Document) ([]EmailAction, error) {
	var emails []EmailAction
	now := s.now()

	// Check if timeout has passed
	if now.After(state.Timeout) {
		var pending []string
		for _, recipient := range cfg.Recipients {
			if !slices.Contains(state.SentTo, recipient) {
				pending = append(pending, recipient)
			}
		}

		if len(pending) > 0 && !doc.Available {
			// The usual trigger notice waits until the document is sent
			emails = append(emails, EmailAction{
				Kind:    EmailDeliveryBlocked,
				To:      cfg.Owner,
				Subject: "Deadman's Handle Triggered - Document Missing",
				Body:    deliveryBlockedBody(state.Timeout, doc.Location, pending),
			})
		} else if heldUntil := state.DocumentChangedAt.Add(ChangeHoldPeriod); len(pending) > 0 && now.Before(heldUntil) {
			// The usual trigger notice waits until the document is sent
			emails = append(emails, EmailAction{
				Kind:    EmailDeliveryHeld,
				To:      cfg.Owner,
				Subject: "Deadman's Handle Triggered - Delivery Held",
				Body:    deliveryHeldBody(state.Timeout, state.DocumentChangedAt, heldUntil, doc.Location, pending),
			})
		} else {
			// Send document to each recipient that has not yet received it
			for _, recipient := range pending {
				emails = append(emails, EmailAction{
					Kind:    EmailDocument,
					To:      recipient,
					Subject: "Important Document - Deadman's Handle",
					Body:    "Please find the important document attached. This has been sent as per the deadman's handle protocol.",
				})
			}

			if !state.OwnerNotified {
				emails = append(emails, EmailAction{
					Kind:    EmailTriggerNotice,
					To:      cfg.Owner,
					Subject: "Deadman's Handle Triggered",
					Body:    triggerNoticeBody(state.Timeout, cfg.Recipients),
				})
			}
		}
	} else if !doc.Available {
		emails = append(emails, EmailAction{
			Kind:    EmailDocumentMissing,
			To:      cfg.Owner,
			Subject: "Deadman's Handle Document Missing",
			Body:    documentMissingBody(state.Timeout, doc.Location),
		})
	}

	// Check if warning should be sent
	warningThreshold := state.Timeout.AddDate(0, 0, -cfg.WarnDays)
	if now.After(warningThreshold) && now.Before(state.Timeout) {
		emails = append(emails, EmailAction{
			Kind:    EmailWarning,
			To:      cfg.Owner,
			Subject: "Deadman's Handle Check-In Required",
			Body:    warningBody(state.Timeout.Sub(now), state.Timeout),
		})
	}

	return emails, nil
}

// DocumentChanged compares the document's current ETag with the one recorded
// in state. The first document seen is recorded without notifying the owner;
// after that, any change of content is recorded and reported. A missing
// document changes nothing, so the recorded ETag survives a deletion and a
// re-upload with different content is still reported. A notice that is still
// pending is reported again (without recording) once ChangeNoticeRetryAfter
// has passed.
func (s *DeadmansHandleService) DocumentChanged(state *config.State, doc Document) (notify, record bool) {
	switch {
	case !doc.Available:
		return false, false
	case doc.ETag == state.DocumentETag:
		return state.DocumentChangePending && s.now().Sub(state.DocumentChangedAt) >= ChangeNoticeRetryAfter, false
	case state.DocumentETag == "":
		return false, true
	default:
		return true, true
	}
}

// Now returns the current time, as used for all of the service's decisions
func (s *DeadmansHandleService) Now() time.Time {
	return s.now()
}

// DocumentChangedEmail tells the owner that the document's content changed
// at changedAt. origin is nil when not known from the S3 event.
func (s *DeadmansHandleService) DocumentChangedEmail(cfg *config.Config, state *config.State, doc Document, changedAt time.Time, origin *ChangeOrigin) EmailAction {
	return EmailAction{
		Kind:    EmailDocumentChanged,
		To:      cfg.Owner,
		Subject: "Deadman's Handle Document Changed",
		Body:    documentChangedBody(changedAt, state.Timeout, doc.Location, origin),
	}
}

// triggerNoticeBody tells the owner that the document is being sent, so
// that a false trigger can be noticed and followed up.
func triggerNoticeBody(timeout time.Time, recipients []string) string {
	return "Your deadman's handle check-in deadline of " +
		timeout.UTC().Format("Monday 2 January 2006 at 15:04 MST") +
		" passed without a check-in, so your document is being sent to:\n\n" +
		"  " + strings.Join(recipients, "\n  ") + "\n\n" +
		"If this is a mistake, check in to reset the timeout and contact the recipients."
}

// documentMissingBody warns the owner, before the timeout, that there is
// nothing to send.
func documentMissingBody(timeout time.Time, location string) string {
	return "Warning: The document for your deadman's handle is missing from " + location + ". " +
		"If the handle triggers, on " + timeout.UTC().Format("Monday 2 January 2006 at 15:04 MST") +
		", nothing can be sent to your recipients.\n\n" +
		"Upload the document to fix this. You will be reminded daily until you do."
}

// deliveryBlockedBody tells the owner that the handle has triggered but the
// document could not be sent.
func deliveryBlockedBody(timeout time.Time, location string, pending []string) string {
	return "Your deadman's handle check-in deadline of " +
		timeout.UTC().Format("Monday 2 January 2006 at 15:04 MST") +
		" passed without a check-in, but the document is missing from " + location +
		", so it could not be sent to:\n\n" +
		"  " + strings.Join(pending, "\n  ") + "\n\n" +
		"Upload the document and it will be sent on the next daily run. " +
		"If this is a mistake, check in to reset the timeout."
}

// documentChangedBody reports a change of content, so that an unauthorised
// change can be put right before the document is sent.
func documentChangedBody(changedAt, timeout time.Time, location string, origin *ChangeOrigin) string {
	body := "The content of the document for your deadman's handle, " + location +
		", changed. This was noticed on " + changedAt.UTC().Format("Monday 2 January 2006 at 15:04 MST") + ".\n\n"
	if origin != nil {
		body += "Changed by AWS account " + origin.Requester + " from IP address " + origin.SourceIP + ".\n\n"
	}
	return body + "If you made this change, no action is needed. If you did not, someone else can " +
		"write to the bucket: check its access and upload the correct document before the handle " +
		"triggers on " + timeout.UTC().Format("Monday 2 January 2006 at 15:04 MST") + ".\n\n" +
		"If the handle triggers before " + changedAt.Add(ChangeHoldPeriod).UTC().Format("Monday 2 January 2006 at 15:04 MST") +
		", delivery to your recipients is held until then."
}

// deliveryHeldBody tells the owner that the handle has triggered but the
// document changed too recently to send.
func deliveryHeldBody(timeout, changedAt, heldUntil time.Time, location string, pending []string) string {
	return "Your deadman's handle check-in deadline of " +
		timeout.UTC().Format("Monday 2 January 2006 at 15:04 MST") +
		" passed without a check-in, but the document at " + location + " changed on " +
		changedAt.UTC().Format("Monday 2 January 2006 at 15:04 MST") +
		", so it has not yet been sent to:\n\n" +
		"  " + strings.Join(pending, "\n  ") + "\n\n" +
		"It will be sent on the first daily run after " +
		heldUntil.UTC().Format("Monday 2 January 2006 at 15:04 MST") + ". " +
		"If this is a mistake, check in to reset the timeout. If the change was not yours, " +
		"upload the correct document; that change holds delivery again."
}

// warningBody describes how long the owner has left to check in. The day
// count is rounded down, so the final day reads "within the next 24 hours"
// rather than "in 0 days", and the exact deadline removes any ambiguity.
func warningBody(remaining time.Duration, timeout time.Time) string {
	var when string
	switch days := int(remaining.Hours() / 24); days {
	case 0:
		when = "within the next 24 hours"
	case 1:
		when = "in 1 day"
	default:
		when = "in " + strconv.Itoa(days) + " days"
	}

	return "Warning: Your deadman's handle will trigger " + when +
		", on " + timeout.UTC().Format("Monday 2 January 2006 at 15:04 MST") + ". " +
		"Please check in before then to reset the timeout."
}
