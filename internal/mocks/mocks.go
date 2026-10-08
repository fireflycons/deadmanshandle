package mocks

import (
	"context"
	"crypto/sha256"
	"fmt"
	"slices"
	"time"

	"github.com/fireflycons/deadmanshandle/internal/config"
	"github.com/fireflycons/deadmanshandle/internal/ports"
)

// Compile-time checks that the mocks implement the ports
var (
	_ ports.ConfigStore     = (*MockConfigStore)(nil)
	_ ports.StateStore      = (*MockStateStore)(nil)
	_ ports.DocumentStore   = (*MockDocumentStore)(nil)
	_ ports.EmailSender     = (*MockEmailSender)(nil)
	_ ports.APIKeyValidator = (*MockAPIKeyValidator)(nil)
)

// MockConfigStore is a mock implementation of ConfigStore
type MockConfigStore struct {
	Data   map[string][]byte
	GetErr error // Returned by GetConfig when set
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

// MockStateStore is a mock implementation of StateStore, applying the same
// conditions as the DynamoDB adapter
type MockStateStore struct {
	State      config.State
	GetErr     error   // Returned by GetState when set
	CheckInErr error   // Returned by CheckIn when set
	RecordErr  error   // Returned by RecordSent and RecordOwnerNotified when set
	SwapErrs   []error // Returned by successive SwapDocumentETag calls, if any remain
	// BeforeUpdate, when set, runs before each conditional update, to
	// simulate another Lambda writing concurrently
	BeforeUpdate func()
}

func NewMockStateStore(state config.State) *MockStateStore {
	return &MockStateStore{State: state}
}

func (m *MockStateStore) GetState(ctx context.Context) (*config.State, error) {
	if m.GetErr != nil {
		return nil, m.GetErr
	}
	state := m.State
	state.SentTo = slices.Clone(m.State.SentTo)
	return &state, nil
}

func (m *MockStateStore) CheckIn(ctx context.Context, timeout time.Time) error {
	if m.CheckInErr != nil {
		return m.CheckInErr
	}
	m.State.Timeout = timeout
	m.State.SentTo = nil
	m.State.OwnerNotified = false
	m.State.CheckIns++
	return nil
}

func (m *MockStateStore) RecordSent(ctx context.Context, checkIns int64, recipient string) error {
	if err := m.recordable(checkIns); err != nil {
		return err
	}
	if !slices.Contains(m.State.SentTo, recipient) {
		m.State.SentTo = append(m.State.SentTo, recipient)
	}
	return nil
}

func (m *MockStateStore) RecordOwnerNotified(ctx context.Context, checkIns int64) error {
	if err := m.recordable(checkIns); err != nil {
		return err
	}
	m.State.OwnerNotified = true
	return nil
}

func (m *MockStateStore) SwapDocumentETag(ctx context.Context, previous, current string) error {
	if m.BeforeUpdate != nil {
		m.BeforeUpdate()
	}
	if len(m.SwapErrs) > 0 {
		err := m.SwapErrs[0]
		m.SwapErrs = m.SwapErrs[1:]
		if err != nil {
			return err
		}
	}
	if m.State.DocumentETag != previous {
		return ports.ErrConditionFailed
	}
	m.State.DocumentETag = current
	return nil
}

func (m *MockStateStore) recordable(checkIns int64) error {
	if m.BeforeUpdate != nil {
		m.BeforeUpdate()
	}
	if m.RecordErr != nil {
		return m.RecordErr
	}
	if m.State.CheckIns != checkIns {
		return ports.ErrConditionFailed
	}
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

func (m *MockDocumentStore) GetDocument(ctx context.Context, bucket, key, etag string) ([]byte, error) {
	if current, _, _ := m.DocumentETag(ctx, bucket, key); current != etag {
		return nil, ports.ErrDocumentChanged
	}
	return m.Documents[bucket][key], nil
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
