package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/fireflycons/deadmanshandle/internal/config"
	"github.com/fireflycons/deadmanshandle/internal/domain"
	"github.com/fireflycons/deadmanshandle/internal/ports"
)

// ScheduledEventHandler handles EventBridge scheduled events
type ScheduledEventHandler struct {
	configStore    ports.ConfigStore
	documentStore  ports.DocumentStore
	emailSender    ports.EmailSender
	service        *domain.DeadmansHandleService
	paramName      string
	documentBucket string
	documentKey    string
}

// NewScheduledEventHandler creates a new scheduled event handler
func NewScheduledEventHandler(
	configStore ports.ConfigStore,
	documentStore ports.DocumentStore,
	emailSender ports.EmailSender,
	service *domain.DeadmansHandleService,
	parameterName string,
	docBucket string,
	docKey string,
) *ScheduledEventHandler {
	return &ScheduledEventHandler{
		configStore:    configStore,
		documentStore:  documentStore,
		emailSender:    emailSender,
		service:        service,
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

	// Process scheduled event
	emails, err := h.service.ProcessScheduledEvent(ctx, cfg)
	if err != nil {
		return err
	}

	if len(emails) == 0 {
		slog.Info("No emails due", "timeout", cfg.Timeout)
		return nil
	}

	// If document needs to be sent, fetch it
	var attachments map[string][]byte
	if slices.ContainsFunc(emails, domain.EmailAction.AttachDocument) {
		doc, err := h.documentStore.GetDocument(ctx, h.documentBucket, h.documentKey)
		if err != nil {
			return err
		}
		attachments = map[string][]byte{
			"document": doc,
		}
	}

	// Send each email individually so that one failure does not stop the
	// rest, recording each success so later runs only retry the failures.
	var errs []error
	changed := false
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

		if h.service.RecordSent(cfg, email) {
			changed = true
		}
	}

	if changed {
		if err := h.saveDeliveryState(ctx, cfg); err != nil {
			errs = append(errs, fmt.Errorf("saving delivery state: %w", err))
		}
	}

	// Report any failures so the invocation shows as an error
	return errors.Join(errs...)
}

// saveDeliveryState writes the delivery fields of cfg back to the config
// store. The stored config is re-read first so that a check-in made while
// emails were being sent is not overwritten with the old timeout.
func (h *ScheduledEventHandler) saveDeliveryState(ctx context.Context, cfg *config.Config) error {
	currentData, err := h.configStore.GetConfig(ctx, h.paramName)
	if err != nil {
		return err
	}

	current, err := config.ParseConfig(currentData)
	if err != nil {
		return err
	}

	if !current.Timeout.Equal(cfg.Timeout) {
		// The owner checked in, which resets the delivery state anyway
		slog.Info("Delivery state not saved: the owner checked in during the run")
		return nil
	}

	current.SentTo = cfg.SentTo
	current.OwnerNotified = cfg.OwnerNotified

	data, err := current.ToJSON()
	if err != nil {
		return err
	}

	return h.configStore.SetConfig(ctx, h.paramName, data)
}
