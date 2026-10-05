package mocks

import (
	"context"
)

// MockConfigStore is a mock implementation of ConfigStore
type MockConfigStore struct {
	Data map[string][]byte
}

func NewMockConfigStore() *MockConfigStore {
	return &MockConfigStore{
		Data: make(map[string][]byte),
	}
}

func (m *MockConfigStore) GetConfig(ctx context.Context, parameterName string) ([]byte, error) {
	return m.Data[parameterName], nil
}

func (m *MockConfigStore) SetConfig(ctx context.Context, parameterName string, data []byte) error {
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

// MockAPIKeyValidator is a mock implementation of APIKeyValidator
type MockAPIKeyValidator struct {
	ExpectedKey string
}

func NewMockAPIKeyValidator(expectedKey string) *MockAPIKeyValidator {
	return &MockAPIKeyValidator{
		ExpectedKey: expectedKey,
	}
}

func (m *MockAPIKeyValidator) ValidateAPIKey(ctx context.Context, providedKey string, expectedKey string) bool {
	return providedKey == m.ExpectedKey
}
