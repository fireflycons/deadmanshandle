package doccrypt

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/vector.dmh")

// The test vector, shared with tools/decrypt/decrypt.test.mjs so that the
// browser page is checked against the same bytes. Non-ASCII name, password
// and content catch encoding mistakes on either side.
type vector struct {
	Password   string `json:"password"`
	Name       string `json:"name"`
	Content    string `json:"content"`
	Salt       []byte `json:"salt"`  // base64 in the JSON
	Nonce      []byte `json:"nonce"` // base64 in the JSON
	Iterations uint32 `json:"iterations"`
}

func readVector(t *testing.T) vector {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "vector.json"))
	if err != nil {
		t.Fatal(err)
	}
	var v vector
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestVector(t *testing.T) {
	v := readVector(t)
	got, err := encryptWith([]byte(v.Content), v.Name, v.Password, v.Salt, v.Nonce, v.Iterations)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join("testdata", "vector.dmh")
	if *update {
		if err := os.WriteFile(file, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("encrypting the vector no longer gives testdata/vector.dmh: the format has changed")
	}

	name, content, err := Decrypt(want, v.Password)
	if err != nil {
		t.Fatal(err)
	}
	if name != v.Name || string(content) != v.Content {
		t.Errorf("Decrypted %q, %q; want %q, %q", name, content, v.Name, v.Content)
	}
}

func TestRoundTrip(t *testing.T) {
	content := []byte("%PDF-1.7 the will\x00\xff")
	data, err := Encrypt(content, "will.pdf", "apple-banana-cherry")
	if err != nil {
		t.Fatal(err)
	}
	if !IsEncrypted(data) || bytes.Contains(data, []byte("will.pdf")) || bytes.Contains(data, content) {
		t.Error("Expected the format's prefix, with neither the name nor the content in clear")
	}
	name, got, err := Decrypt(data, "apple-banana-cherry")
	if err != nil {
		t.Fatal(err)
	}
	if name != "will.pdf" || !bytes.Equal(got, content) {
		t.Errorf("Decrypted %q, %q", name, got)
	}

	again, err := Encrypt(content, "will.pdf", "apple-banana-cherry")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(data, again) {
		t.Error("Expected a new salt and nonce for each encryption")
	}
}

func TestEmptyContent(t *testing.T) {
	data, err := Encrypt(nil, "empty.txt", "pw")
	if err != nil {
		t.Fatal(err)
	}
	name, content, err := Decrypt(data, "pw")
	if err != nil || name != "empty.txt" || len(content) != 0 {
		t.Errorf("Got %q, %q, %v", name, content, err)
	}
}

func TestDecryptFailures(t *testing.T) {
	v := readVector(t)
	good, err := os.ReadFile(filepath.Join("testdata", "vector.dmh"))
	if err != nil {
		t.Fatal(err)
	}
	modified := func(at int) []byte {
		data := bytes.Clone(good)
		data[at] ^= 1
		return data
	}
	bigIterations := bytes.Clone(good)
	copy(bigIterations[5:9], []byte{0xff, 0xff, 0xff, 0xff})

	tests := []struct {
		name     string
		data     []byte
		password string
		want     error
		wantText string
	}{
		{"wrong password", good, "not it", ErrWrongPassword, ""},
		{"password in another case", good, strings.ToUpper(v.Password), ErrWrongPassword, ""},
		{"changed salt", modified(10), v.Password, ErrWrongPassword, ""},
		{"changed ciphertext", modified(len(good) - 20), v.Password, ErrWrongPassword, ""},
		{"changed tag", modified(len(good) - 1), v.Password, ErrWrongPassword, ""},
		{"truncated", good[:len(good)-1], v.Password, ErrWrongPassword, ""},
		{"header only", good[:headerSize], v.Password, ErrWrongPassword, ""},
		{"short header", good[:10], v.Password, ErrWrongPassword, ""},
		{"absurd iterations", bigIterations, v.Password, ErrWrongPassword, ""},
		{"plain file", []byte("%PDF-1.7"), v.Password, ErrNotEncrypted, ""},
		{"later version", modified(4), v.Password, nil, "version 0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := Decrypt(tt.data, tt.password)
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Errorf("Expected %v, got %v", tt.want, err)
			}
			if tt.wantText != "" && (err == nil || !strings.Contains(err.Error(), tt.wantText)) {
				t.Errorf("Expected an error mentioning %q, got %v", tt.wantText, err)
			}
		})
	}
}

func TestEncryptRefuses(t *testing.T) {
	tests := []struct {
		name, fileName, password string
	}{
		{"empty password", "will.pdf", ""},
		{"empty name", "", "pw"},
		{"invalid UTF-8 name", "will\xff.pdf", "pw"},
		{"name too long", strings.Repeat("a", 1<<16), "pw"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Encrypt([]byte("x"), tt.fileName, tt.password); err == nil {
				t.Error("Expected an error")
			}
		})
	}
}
