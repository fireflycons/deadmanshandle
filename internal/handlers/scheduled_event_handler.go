package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"slices"

	"github.com/fireflycons/deadmanshandle/internal/config"
	"github.com/fireflycons/deadmanshandle/internal/domain"
	"github.com/fireflycons/deadmanshandle/internal/ports"
)

// ScheduledEventHandler handles EventBridge scheduled events
type ScheduledEventHandler struct {
	configStore    ports.ConfigStore
	stateStore     ports.StateStore
	documentStore  ports.DocumentStore
	emailSender    ports.EmailSender
	service        *domain.DeadmansHandleService
	watcher        *DocumentWatcher
	paramName      string
	documentBucket string
	documentKey    string
}

// NewScheduledEventHandler creates a new scheduled event handler
func NewScheduledEventHandler(
	configStore ports.ConfigStore,
	stateStore ports.StateStore,
	documentStore ports.DocumentStore,
	emailSender ports.EmailSender,
	service *domain.DeadmansHandleService,
	watcher *DocumentWatcher,
	parameterName string,
	docBucket string,
	docKey string,
) *ScheduledEventHandler {
	return &ScheduledEventHandler{
		configStore:    configStore,
		stateStore:     stateStore,
		documentStore:  documentStore,
		emailSender:    emailSender,
		service:        service,
		watcher:        watcher,
		paramName:      parameterName,
		documentBucket: docBucket,
		documentKey:    docKey,
	}
}

// Handle processes scheduled events
func (h *ScheduledEventHandler) Handle(ctx context.Context) error {
	// Get configuration
	configData, err := h.configStore.GetConfig(ctx, h.paramName)
	if err != nil {
		return err
	}

	cfg, err := config.ParseConfig(configData)
	if err != nil {
		return err
	}

	// Check for the document on every run, so the owner hears about a
	// missing one before it is needed
	doc, err := lookupDocument(ctx, h.documentStore, h.documentBucket, h.documentKey)
	if err != nil {
		return err
	}
	if !doc.Available {
		slog.Warn("Document missing", "location", doc.Location)
	}

	// Report a change of content missed by the S3 event. A failure here is
	// returned at the end but does not hold up the rest of the run.
	var errs []error
	if err := h.watcher.Check(ctx, cfg, doc, nil); err != nil {
		errs = append(errs, err)
	}

	state, err := h.stateStore.GetState(ctx)
	if err != nil {
		return errors.Join(append(errs, fmt.Errorf("reading state: %w", err))...)
	}

	// Process scheduled event
	emails, err := h.service.ProcessScheduledEvent(ctx, cfg, state, doc)
	if err != nil {
		return errors.Join(append(errs, err)...)
	}

	if len(emails) == 0 {
		slog.Info("No emails due", "timeout", state.Timeout)
		return errors.Join(errs...)
	}

	// If document needs to be sent, fetch it. It must still be the one that
	// was checked for changes; if not, nothing is sent and the error makes
	// Lambda retry the run, which checks the new one first.
	var attachments map[string][]byte
	if slices.ContainsFunc(emails, domain.EmailAction.AttachDocument) {
		content, err := h.documentStore.GetDocument(ctx, h.documentBucket, h.documentKey, doc.ETag)
		if err != nil {
			return errors.Join(append(errs, fmt.Errorf("fetching the document: %w", err))...)
		}
		// Named after the object, e.g. "wills/will.pdf" is attached as "will.pdf"
		attachments = map[string][]byte{
			path.Base(h.documentKey): content,
		}
	}

	// Send each email individually so that one failure does not stop the
	// rest, recording each success straight away so later runs only retry
	// the failures. A record is refused once the owner has checked in, which
	// stops the run.
	for _, email := range emails {
		var emailAttachments map[string][]byte
		if email.AttachDocument() {
			emailAttachments = attachments
		}

		if err := h.emailSender.SendEmail(ctx, email.To, email.Subject, email.Body, emailAttachments); err != nil {
			errs = append(errs, fmt.Errorf("sending email to %s: %w", email.To, err))
			continue
		}
		slog.Info("Email sent", "to", email.To, "subject", email.Subject, "document", email.AttachDocument())

		err := h.recordSent(ctx, state, email)
		if errors.Is(err, ports.ErrConditionFailed) {
			slog.Info("The owner checked in during the run; no more emails sent")
			return errors.Join(errs...)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("recording the email to %s: %w", email.To, err))
		}
	}

	// Recipients are still waiting for a document that isn't there
	if slices.ContainsFunc(emails, func(e domain.EmailAction) bool { return e.Kind == domain.EmailDeliveryBlocked }) {
		errs = append(errs, fmt.Errorf("document %s is missing, so it was not sent", doc.Location))
	}

	// Report any failures so the invocation shows as an error
	return errors.Join(errs...)
}

// recordSent records a delivery in the state, if the email was one. It
// fails with ports.ErrConditionFailed if the owner has checked in since
// state was read.
func (h *ScheduledEventHandler) recordSent(ctx context.Context, state *config.State, email domain.EmailAction) error {
	switch email.Kind {
	case domain.EmailDocument:
		return h.stateStore.RecordSent(ctx, state.CheckIns, email.To)
	case domain.EmailTriggerNotice:
		return h.stateStore.RecordOwnerNotified(ctx, state.CheckIns)
	default:
		return nil
	}
}
