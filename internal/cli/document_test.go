package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fireflycons/deadmanshandle/internal/config"
	"github.com/fireflycons/deadmanshandle/internal/doccrypt"
	"github.com/fireflycons/deadmanshandle/internal/mocks"
)

const testBucket = "bucket"

// scriptedPrompt answers prompts in turn, failing the test if asked more
// often than it has answers
type scriptedPrompt struct {
	t       *testing.T
	answers []string
	asked   []string
}

func (p *scriptedPrompt) ask(prompt string) (string, error) {
	p.asked = append(p.asked, prompt)
	if len(p.answers) == 0 {
		p.t.Errorf("Unexpected prompt %q", prompt)
		return "", errors.New("no answer")
	}
	answer := p.answers[0]
	p.answers = p.answers[1:]
	return answer, nil
}

func testDocuments(t *testing.T, store *mocks.MockConfigStore, transfer DocumentTransfer, answers ...string) (Documents, *scriptedPrompt) {
	prompt := &scriptedPrompt{t: t, answers: answers}
	return Documents{
		Store:             store,
		ConfigParameter:   testParam,
		PasswordParameter: testPasswordParam,
		Transfer:          transfer,
		Bucket:            testBucket,
		Prompt:            prompt.ask,
	}, prompt
}

func TestGetDocument(t *testing.T) {
	store := configStoreWith(t, testConfig())
	docs := mocks.NewMockDocumentStore()
	docs.SetDocument(testBucket, "docs/will.pdf", []byte("the will"))
	d, _ := testDocuments(t, store, docs)
	dir := t.TempDir()

	// Named after the key's file name, as the recipients receive it
	file, err := GetDocument(context.Background(), d, dir, false, false)
	if err != nil {
		t.Fatalf("GetDocument failed: %v", err)
	}
	if got, _ := os.ReadFile(file); file != filepath.Join(dir, "will.pdf") || string(got) != "the will" {
		t.Errorf("Expected the document in %s, got %q in %s", filepath.Join(dir, "will.pdf"), got, file)
	}

	// An existing file is kept unless forced
	docs.SetDocument(testBucket, "docs/will.pdf", []byte("the new will"))
	if _, err := GetDocument(context.Background(), d, dir, false, false); err == nil ||
		!strings.Contains(err.Error(), "already exists") {
		t.Errorf("Expected the existing file kept, got %v", err)
	}
	if got, _ := os.ReadFile(file); string(got) != "the will" {
		t.Errorf("Expected the file unchanged, got %q", got)
	}
	if _, err := GetDocument(context.Background(), d, dir, true, false); err != nil {
		t.Fatalf("GetDocument with force failed: %v", err)
	}
	if got, _ := os.ReadFile(file); string(got) != "the new will" {
		t.Errorf("Expected the file replaced, got %q", got)
	}
}

func TestGetDocumentMissing(t *testing.T) {
	dir := t.TempDir()
	d, _ := testDocuments(t, configStoreWith(t, testConfig()), mocks.NewMockDocumentStore())
	if _, err := GetDocument(context.Background(), d, dir, false, false); err == nil {
		t.Fatal("Expected an error")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("Expected no file written, got %v", entries)
	}
}

func TestGetEncryptedDocument(t *testing.T) {
	encrypted := encryptedDoc(t, oldPassword)
	tests := []struct {
		name       string
		stored     string // the stored password, empty for none
		readErr    error  // reading the password fails
		raw        bool
		answers    []string
		wantFile   string
		wantPrompt int
		wantErr    error
	}{
		{"with the stored password", oldPassword, nil, false, nil, "will.pdf", 0, nil},
		{"none stored: asks", "", nil, false, []string{oldPassword}, "will.pdf", 1, nil},
		{"can't read it: asks", "", errors.New("access denied"), false, []string{oldPassword}, "will.pdf", 1, nil},
		{"stored one wrong: asks", "not-the-password", nil, false, []string{oldPassword}, "will.pdf", 1, nil},
		{"wrong, then right", "", nil, false, []string{"no", "nope", oldPassword}, "will.pdf", 3, nil},
		{"wrong three times", "", nil, false, []string{"no", "nope", "never"}, "", 3, doccrypt.ErrWrongPassword},
		{"raw: as stored", oldPassword, nil, true, nil, "will.pdf.dmh", 0, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig()
			cfg.DocumentKey = "docs/will.pdf.dmh"
			store := configStoreWith(t, cfg)
			if tt.stored != "" {
				store.Data[testPasswordParam] = []byte(tt.stored)
			}
			docs := mocks.NewMockDocumentStore()
			docs.SetDocument(testBucket, cfg.DocumentKey, encrypted)
			d, prompt := testDocuments(t, store, docs, tt.answers...)
			if tt.readErr != nil {
				d.Store = &failingPasswordStore{MockConfigStore: store, err: tt.readErr}
			}
			dir := t.TempDir()

			file, err := GetDocument(context.Background(), d, dir, false, tt.raw)
			if len(prompt.asked) != tt.wantPrompt {
				t.Errorf("Expected %d prompts, got %q", tt.wantPrompt, prompt.asked)
			}
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Expected %v, got %v", tt.wantErr, err)
				}
				if entries, _ := os.ReadDir(dir); len(entries) != 0 {
					t.Errorf("Expected no file written, got %v", entries)
				}
				return
			}
			if err != nil {
				t.Fatalf("GetDocument failed: %v", err)
			}
			want := "the will"
			if tt.raw {
				want = string(encrypted)
			}
			if got, _ := os.ReadFile(file); file != filepath.Join(dir, tt.wantFile) || string(got) != want {
				t.Errorf("Expected %s with the document, got %s", filepath.Join(dir, tt.wantFile), file)
			}
		})
	}
}

// failingPasswordStore fails to read the document password, as without
// permission for it, but reads the config
type failingPasswordStore struct {
	*mocks.MockConfigStore
	err error
}

func (s *failingPasswordStore) GetConfig(ctx context.Context, parameterName string) ([]byte, error) {
	if parameterName == testPasswordParam {
		return nil, s.err
	}
	return s.MockConfigStore.GetConfig(ctx, parameterName)
}

func TestSafeName(t *testing.T) {
	for name, want := range map[string]string{
		"will.pdf":           "will.pdf",
		"../../etc/passwd":   "passwd",
		`..\..\Windows\evil`: "evil",
		"..":                 "document",
		"/":                  "document",
		"C:evil.txt":         "C_evil.txt",
		"a\x00b?.pdf":        "a_b_.pdf",
	} {
		if got := safeName(name); got != want {
			t.Errorf("safeName(%q) = %q, want %q", name, got, want)
		}
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
		plain       bool
		wantKey     string
		wantChanged bool
	}{
		{"encrypted under the name plus .dmh", "will.pdf", "", false, "will.pdf.dmh", true},
		{"key overrides the name", "scan.pdf", "docs/will.pdf", false, "docs/will.pdf", false},
		{"plain: same name keeps the key", "will.pdf", "docs/will.pdf", true, "docs/will.pdf", false},
		{"plain: new name changes the key", "new-will.pdf", "", true, "new-will.pdf", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := configStoreWith(t, testConfig())
			store.Data[testPasswordParam] = []byte(oldPassword)
			docs := &orderCheckingDocs{MockDocumentStore: mocks.NewMockDocumentStore(), t: t, store: store}
			d, _ := testDocuments(t, store, docs)
			file := filepath.Join(t.TempDir(), tt.file)
			if err := os.WriteFile(file, []byte("content"), 0o600); err != nil {
				t.Fatal(err)
			}
			puts := store.Puts

			result, err := PutDocument(context.Background(), d, file, tt.key, tt.plain)
			if err != nil {
				t.Fatalf("PutDocument failed: %v", err)
			}
			if result.Key != tt.wantKey || result.PreviousKey != "docs/will.pdf" || result.KeyChanged() != tt.wantChanged || result.Encrypted == tt.plain {
				t.Errorf("Unexpected result %+v", result)
			}

			stored := docs.Documents[testBucket][tt.wantKey]
			if tt.plain {
				if string(stored) != "content" {
					t.Errorf("Expected the document as it is at %s, got %q", tt.wantKey, stored)
				}
			} else if name, content, err := doccrypt.Decrypt(stored, oldPassword); err != nil || name != tt.file || string(content) != "content" {
				t.Errorf("Expected %s encrypted at %s, got %q, %q, %v", tt.file, tt.wantKey, name, content, err)
			}

			if docs.keyAtUpload != "docs/will.pdf" {
				t.Errorf("Expected the config changed after the upload, but it named %s during it", docs.keyAtUpload)
			}
			cfg, err := config.ParseConfig(store.Data[testParam])
			if err != nil || cfg.DocumentKey != tt.wantKey {
				t.Errorf("Expected the config to name %s, got %+v (%v)", tt.wantKey, cfg, err)
			}
			if wantPuts := map[bool]int{false: 0, true: 1}[tt.wantChanged]; store.Puts-puts != wantPuts {
				t.Errorf("Expected %d config writes, got %d", wantPuts, store.Puts-puts)
			}
		})
	}
}

func TestPutDocumentPassword(t *testing.T) {
	writeTestFile := func(t *testing.T) string {
		file := filepath.Join(t.TempDir(), "will.pdf")
		if err := os.WriteFile(file, []byte("content"), 0o600); err != nil {
			t.Fatal(err)
		}
		return file
	}

	t.Run("none stored: refused", func(t *testing.T) {
		store := configStoreWith(t, testConfig())
		docs := mocks.NewMockDocumentStore()
		d, _ := testDocuments(t, store, docs)
		if _, err := PutDocument(context.Background(), d, writeTestFile(t), "", false); err == nil || !strings.Contains(err.Error(), "dmh document password set") {
			t.Errorf("Expected a refusal naming the password command, got %v", err)
		}
		if len(docs.Documents) != 0 {
			t.Error("Expected nothing uploaded")
		}
	})

	t.Run("can't read it: asked twice", func(t *testing.T) {
		store := configStoreWith(t, testConfig())
		docs := mocks.NewMockDocumentStore()
		d, prompt := testDocuments(t, store, docs, newPassword, newPassword)
		d.Store = &failingPasswordStore{MockConfigStore: store, err: errors.New("access denied")}
		if _, err := PutDocument(context.Background(), d, writeTestFile(t), "", false); err != nil {
			t.Fatalf("PutDocument failed: %v", err)
		}
		if len(prompt.asked) != 2 || !strings.Contains(prompt.asked[0], "access denied") {
			t.Errorf("Expected two prompts, the first saying why, got %q", prompt.asked)
		}
		if _, _, err := doccrypt.Decrypt(docs.Documents[testBucket]["will.pdf.dmh"], newPassword); err != nil {
			t.Errorf("Expected it encrypted with the typed password: %v", err)
		}
	})

	t.Run("can't read it: typed passwords differ", func(t *testing.T) {
		store := configStoreWith(t, testConfig())
		docs := mocks.NewMockDocumentStore()
		d, _ := testDocuments(t, store, docs, newPassword, oldPassword)
		d.Store = &failingPasswordStore{MockConfigStore: store, err: errors.New("access denied")}
		if _, err := PutDocument(context.Background(), d, writeTestFile(t), "", false); err == nil {
			t.Error("Expected an error")
		}
		if len(docs.Documents) != 0 {
			t.Error("Expected nothing uploaded")
		}
	})
}

func TestPutDocumentRefusesBadKey(t *testing.T) {
	store := configStoreWith(t, testConfig())
	docs := mocks.NewMockDocumentStore()
	d, _ := testDocuments(t, store, docs)
	file := filepath.Join(t.TempDir(), "will.pdf")
	if err := os.WriteFile(file, []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{" ", "docs/", "docs/.."} {
		if _, err := PutDocument(context.Background(), d, file, key, true); err == nil {
			t.Errorf("Expected key %q refused", key)
		}
	}
	if len(docs.Documents) != 0 || store.Puts != 0 {
		t.Error("Expected nothing uploaded or changed")
	}
}
