package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/fireflycons/deadmanshandle/internal/doccrypt"
	"github.com/fireflycons/deadmanshandle/internal/mocks"
)

const (
	testPasswordParam = "/deadmanshandle/document-password"
	oldPassword       = "acid-corn-acre-zone"
	newPassword       = "zoom-yo-yo-acid-corn"
)

func encryptedDoc(t *testing.T, password string) []byte {
	t.Helper()
	data, err := doccrypt.Encrypt([]byte("the will"), "will.pdf", password)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestReadDocumentPassword(t *testing.T) {
	store := configStoreWith(t, testConfig())
	if _, found, err := ReadDocumentPassword(context.Background(), store, testPasswordParam); err != nil || found {
		t.Errorf("Expected none found, got found %v, %v", found, err)
	}

	store.Data[testPasswordParam] = []byte(oldPassword)
	if got, found, err := ReadDocumentPassword(context.Background(), store, testPasswordParam); err != nil || !found || got != oldPassword {
		t.Errorf("Expected %q, got %q, found %v, %v", oldPassword, got, found, err)
	}

	store.GetErr = errors.New("access denied")
	if _, _, err := ReadDocumentPassword(context.Background(), store, testPasswordParam); !errors.Is(err, store.GetErr) {
		t.Errorf("Expected the read error, got %v", err)
	}
}

func TestSetDocumentPassword(t *testing.T) {
	tests := []struct {
		name             string
		stored           []byte // the document, nil for none
		storedPassword   string // empty for none
		wantChange       PasswordChange
		wantOpensWithNew bool
	}{
		{"no document", nil, "", PasswordChange{Missing: true}, false},
		{"plain document", []byte("%PDF the will"), oldPassword, PasswordChange{Plain: true}, false},
		{"encrypted with the old password", encryptedDoc(t, oldPassword), oldPassword, PasswordChange{Reencrypted: true}, true},
		{"already encrypted with the new one", encryptedDoc(t, newPassword), "", PasswordChange{}, true},
		{"the same password again", encryptedDoc(t, newPassword), newPassword, PasswordChange{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := configStoreWith(t, testConfig())
			if tt.storedPassword != "" {
				store.Data[testPasswordParam] = []byte(tt.storedPassword)
			}
			docs := mocks.NewMockDocumentStore()
			if tt.stored != nil {
				docs.SetDocument(testBucket, "docs/will.pdf", tt.stored)
			}

			change, err := SetDocumentPassword(context.Background(), store, testPasswordParam, testParam, docs, testBucket, newPassword)
			if err != nil {
				t.Fatalf("SetDocumentPassword failed: %v", err)
			}
			tt.wantChange.Key = "docs/will.pdf"
			if change != tt.wantChange {
				t.Errorf("Expected %+v, got %+v", tt.wantChange, change)
			}
			if got := string(store.Data[testPasswordParam]); got != newPassword {
				t.Errorf("Expected the new password stored, got %q", got)
			}

			stored := docs.Documents[testBucket]["docs/will.pdf"]
			if tt.wantOpensWithNew {
				if name, content, err := doccrypt.Decrypt(stored, newPassword); err != nil || name != "will.pdf" || string(content) != "the will" {
					t.Errorf("Expected the document to open with the new password, got %q, %q, %v", name, content, err)
				}
			} else if string(stored) != string(tt.stored) {
				t.Error("Expected the document unchanged")
			}
		})
	}
}

func TestSetDocumentPasswordRefuses(t *testing.T) {
	t.Run("too short", func(t *testing.T) {
		store := configStoreWith(t, testConfig())
		if _, err := SetDocumentPassword(context.Background(), store, testPasswordParam, testParam, mocks.NewMockDocumentStore(), testBucket, "short"); err == nil {
			t.Error("Expected an error")
		}
		if _, ok := store.Data[testPasswordParam]; ok {
			t.Error("Expected nothing stored")
		}
	})

	t.Run("opens with neither password", func(t *testing.T) {
		store := configStoreWith(t, testConfig())
		store.Data[testPasswordParam] = []byte(oldPassword)
		docs := mocks.NewMockDocumentStore()
		original := encryptedDoc(t, "some-other-password")
		docs.SetDocument(testBucket, "docs/will.pdf", original)

		_, err := SetDocumentPassword(context.Background(), store, testPasswordParam, testParam, docs, testBucket, newPassword)
		if err == nil || !strings.Contains(err.Error(), "nothing was changed") {
			t.Errorf("Expected a refusal, got %v", err)
		}
		if string(store.Data[testPasswordParam]) != oldPassword || string(docs.Documents[testBucket]["docs/will.pdf"]) != string(original) {
			t.Error("Expected the password and the document unchanged")
		}
	})

	t.Run("saving fails after re-encrypting", func(t *testing.T) {
		store := configStoreWith(t, testConfig())
		store.Data[testPasswordParam] = []byte(oldPassword)
		store.PutErr = errors.New("throttled")
		docs := mocks.NewMockDocumentStore()
		docs.SetDocument(testBucket, "docs/will.pdf", encryptedDoc(t, oldPassword))

		change, err := SetDocumentPassword(context.Background(), store, testPasswordParam, testParam, docs, testBucket, newPassword)
		if !errors.Is(err, store.PutErr) || !strings.Contains(err.Error(), "now encrypted with the new password") || !change.Reencrypted {
			t.Errorf("Expected the failure to say the document was re-encrypted, got %+v, %v", change, err)
		}
	})
}

func TestDocumentPasswordStatus(t *testing.T) {
	if got := DocumentPasswordStatus("", false, true); !strings.HasPrefix(got, "not set") {
		t.Errorf("Expected not set, got %q", got)
	}
	if got := DocumentPasswordStatus(oldPassword, true, false); got != "set" {
		t.Errorf("Expected the password hidden, got %q", got)
	}
	if got := DocumentPasswordStatus(oldPassword, true, true); got != oldPassword {
		t.Errorf("Expected the password shown, got %q", got)
	}
}
