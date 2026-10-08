package mocks

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/fireflycons/deadmanshandle/internal/ports"
)

// Compile-time checks that the mocks implement the ports
var (
	_ ports.ConfigStore     = (*MockConfigStore)(nil)
	_ ports.DocumentStore   = (*MockDocumentStore)(nil)
	_ ports.EmailSender     = (*MockEmailSender)(nil)
	_ ports.APIKeyValidator = (*MockAPIKeyValidator)(nil)
)

// MockConfigStore is a mock implementation of ConfigStore
type MockConfigStore struct {
	Data   map[string][]byte
	GetErr error // Returned by GetConfig when set
	SetErr error // Returned by SetConfig when set
}

func NewMockConfigStore() *MockConfigStore {
	return &MockConfigStore{
		Data: make(map[string][]byte),
	}
}

func (m *MockConfigStore) GetConfig(ctx context.Context, parameterName string) ([]byte, error) {
	if m.GetErr != nil {
		return nil, m.GetErr
	}
	return m.Data[parameterName], nil
}

func (m *MockConfigStore) SetConfig(ctx context.Context, parameterName string, data []byte) error {
	if m.SetErr != nil {
		return m.SetErr
	}
	m.Data[parameterName] = data
	return nil
}

// MockDocumentStore is a mock implementation of DocumentStore
type MockDocumentStore struct {
	Documents map[string]map[string][]byte
}

func NewMockDocumentStore() *MockDocumentStore {
	return &MockDocumentStore{
		Documents: make(map[string]map[string][]byte),
	}
}

func (m *MockDocumentStore) GetDocument(ctx context.Context, bucket, key string) ([]byte, error) {
	if docs, ok := m.Documents[bucket]; ok {
		return docs[key], nil
	}
	return nil, nil
}

// DocumentETag derives the ETag from the content, so replacing a document
// with different content changes it
func (m *MockDocumentStore) DocumentETag(ctx context.Context, bucket, key string) (string, bool, error) {
	data, ok := m.Documents[bucket][key]
	if !ok {
		return "", false, nil
	}
	return fmt.Sprintf("\"%x\"", sha256.Sum256(data)), true, nil
}

func (m *MockDocumentStore) SetDocument(bucket, key string, data []byte) {
	if _, ok := m.Documents[bucket]; !ok {
		m.Documents[bucket] = make(map[string][]byte)
	}
	m.Documents[bucket][key] = data
}

// SentEmail records an email passed to MockEmailSender
type SentEmail struct {
	To          string
	Subject     string
	Body        string
	Attachments map[string][]byte
}

// MockEmailSender is a mock implementation of EmailSender
type MockEmailSender struct {
	SentEmails []SentEmail
	FailFor    map[string]error // Recipients whose sends fail with the given error
}

func NewMockEmailSender() *MockEmailSender {
	return &MockEmailSender{
		SentEmails: make([]SentEmail, 0),
		FailFor:    make(map[string]error),
	}
}

func (m *MockEmailSender) SendEmail(ctx context.Context, to, subject, body string, attachments map[string][]byte) error {
	if err := m.FailFor[to]; err != nil {
		return err
	}
	m.SentEmails = append(m.SentEmails, SentEmail{
		To:          to,
		Subject:     subject,
		Body:        body,
		Attachments: attachments,
	})
	return nil
}

// MockAPIKeyValidator is a mock implementation of APIKeyValidator. It
// compares against the key the caller passes, so tests catch a handler that
// checks against the wrong key.
type MockAPIKeyValidator struct{}

func NewMockAPIKeyValidator() *MockAPIKeyValidator {
	return &MockAPIKeyValidator{}
}

func (m *MockAPIKeyValidator) ValidateAPIKey(ctx context.Context, providedKey string, expectedKey string) bool {
	return providedKey == expectedKey
}
