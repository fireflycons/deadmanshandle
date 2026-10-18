package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fireflycons/deadmanshandle/internal/config"
	"github.com/fireflycons/deadmanshandle/internal/mocks"
)

const testBucket = "bucket"

func TestGetDocument(t *testing.T) {
	store := configStoreWith(t, testConfig())
	docs := mocks.NewMockDocumentStore()
	docs.SetDocument(testBucket, "docs/will.pdf", []byte("the will"))
	dir := t.TempDir()

	// Named after the key's file name, as the recipients receive it
	file, err := GetDocument(context.Background(), store, testParam, docs, testBucket, dir, false)
	if err != nil {
		t.Fatalf("GetDocument failed: %v", err)
	}
	if got, _ := os.ReadFile(file); file != filepath.Join(dir, "will.pdf") || string(got) != "the will" {
		t.Errorf("Expected the document in %s, got %q in %s", filepath.Join(dir, "will.pdf"), got, file)
	}

	// An existing file is kept unless forced
	docs.SetDocument(testBucket, "docs/will.pdf", []byte("the new will"))
	if _, err := GetDocument(context.Background(), store, testParam, docs, testBucket, dir, false); err == nil ||
		!strings.Contains(err.Error(), "already exists") {
		t.Errorf("Expected the existing file kept, got %v", err)
	}
	if got, _ := os.ReadFile(file); string(got) != "the will" {
		t.Errorf("Expected the file unchanged, got %q", got)
	}
	if _, err := GetDocument(context.Background(), store, testParam, docs, testBucket, dir, true); err != nil {
		t.Fatalf("GetDocument with force failed: %v", err)
	}
	if got, _ := os.ReadFile(file); string(got) != "the new will" {
		t.Errorf("Expected the file replaced, got %q", got)
	}
}

func TestGetDocumentMissing(t *testing.T) {
	dir := t.TempDir()
	_, err := GetDocument(context.Background(), configStoreWith(t, testConfig()), testParam, mocks.NewMockDocumentStore(), testBucket, dir, false)
	if err == nil {
		t.Fatal("Expected an error")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("Expected no file written, got %v", entries)
	}
}

// orderCheckingDocs records the config's document key when each upload
// happens, to check that the config is changed after the upload
type orderCheckingDocs struct {
	*mocks.MockDocumentStore
	t           *testing.T
	store       *mocks.MockConfigStore
	keyAtUpload string
}

func (d *orderCheckingDocs) Upload(ctx context.Context, bucket, key string, body io.Reader) error {
	cfg, err := config.ParseConfig(d.store.Data[testParam])
	if err != nil {
		d.t.Fatalf("Config invalid at upload: %v", err)
	}
	d.keyAtUpload = cfg.DocumentKey
	return d.MockDocumentStore.Upload(ctx, bucket, key, body)
}

func TestPutDocument(t *testing.T) {
	tests := []struct {
		name        string
		file        string
		key         string
		wantKey     string
		wantChanged bool
	}{
		{"same name keeps the key", "will.pdf", "docs/will.pdf", "docs/will.pdf", false},
		{"new name changes the key", "new-will.pdf", "", "new-will.pdf", true},
		{"key overrides the name", "scan.pdf", "docs/will-2.pdf", "docs/will-2.pdf", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := configStoreWith(t, testConfig())
			docs := &orderCheckingDocs{MockDocumentStore: mocks.NewMockDocumentStore(), t: t, store: store}
			file := filepath.Join(t.TempDir(), tt.file)
			if err := os.WriteFile(file, []byte("content"), 0o600); err != nil {
				t.Fatal(err)
			}

			result, err := PutDocument(context.Background(), store, testParam, docs, testBucket, file, tt.key)
			if err != nil {
				t.Fatalf("PutDocument failed: %v", err)
			}
			if result.Key != tt.wantKey || result.PreviousKey != "docs/will.pdf" || result.KeyChanged() != tt.wantChanged {
				t.Errorf("Unexpected result %+v", result)
			}
			if string(docs.Documents[testBucket][tt.wantKey]) != "content" {
				t.Errorf("Expected the document at %s, got %v", tt.wantKey, docs.Documents)
			}
			if docs.keyAtUpload != "docs/will.pdf" {
				t.Errorf("Expected the config changed after the upload, but it named %s during it", docs.keyAtUpload)
			}
			cfg, err := config.ParseConfig(store.Data[testParam])
			if err != nil || cfg.DocumentKey != tt.wantKey {
				t.Errorf("Expected the config to name %s, got %+v (%v)", tt.wantKey, cfg, err)
			}
			if wantPuts := map[bool]int{false: 0, true: 1}[tt.wantChanged]; store.Puts != wantPuts {
				t.Errorf("Expected %d config writes, got %d", wantPuts, store.Puts)
			}
		})
	}
}

func TestPutDocumentRefusesBadKey(t *testing.T) {
	store := configStoreWith(t, testConfig())
	docs := mocks.NewMockDocumentStore()
	file := filepath.Join(t.TempDir(), "will.pdf")
	if err := os.WriteFile(file, []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{" ", "docs/", "docs/.."} {
		if _, err := PutDocument(context.Background(), store, testParam, docs, testBucket, file, key); err == nil {
			t.Errorf("Expected key %q refused", key)
		}
	}
	if len(docs.Documents) != 0 || store.Puts != 0 {
		t.Error("Expected nothing uploaded or changed")
	}
}
