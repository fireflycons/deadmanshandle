package handlers

import (
	"context"

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
	emails, attachDocument, err := h.service.ProcessScheduledEvent(ctx, cfg)
	if err != nil {
		return err
	}

	// If document needs to be sent, fetch it
	var attachments map[string][]byte
	if attachDocument {
		doc, err := h.documentStore.GetDocument(ctx, h.documentBucket, h.documentKey)
		if err != nil {
			return err
		}
		attachments = map[string][]byte{
			"document": doc,
		}
	}

	// Send emails
	if len(emails) > 0 {
		if err := h.emailSender.SendBatchEmail(ctx, emails, attachments); err != nil {
			return err
		}
	}

	return nil
}
