package ports

import (
	"context"

	"github.com/fireflycons/deadmanshandle/internal/domain"
)

// ConfigStore defines the interface for storing and retrieving configuration
type ConfigStore interface {
	GetConfig(ctx context.Context, parameterName string) ([]byte, error)
	SetConfig(ctx context.Context, parameterName string, data []byte) error
}

// DocumentStore defines the interface for retrieving documents from storage
type DocumentStore interface {
	GetDocument(ctx context.Context, bucket, key string) ([]byte, error)
}

// EmailSender defines the interface for sending emails
type EmailSender interface {
	SendEmail(ctx context.Context, to, subject, body string, attachments map[string][]byte) error
	SendBatchEmail(ctx context.Context, emails []domain.EmailAction, attachments map[string][]byte) error
}

// APIKeyValidator defines the interface for validating API keys
type APIKeyValidator interface {
	ValidateAPIKey(ctx context.Context, providedKey string, expectedKey string) bool
}
