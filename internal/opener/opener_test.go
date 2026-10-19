package opener

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fireflycons/deadmanshandle/internal/doccrypt"
)

const password = "acid-corn-acre-zone"

func writeEncrypted(t *testing.T, dir string) string {
	t.Helper()
	data, err := doccrypt.Encrypt([]byte("the will"), "will.pdf", password)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "will.pdf.dmh")
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return file
}

func answers(t *testing.T, list ...string) (func(string) (string, error), *int) {
	asked := 0
	return func(string) (string, error) {
		asked++
		if len(list) == 0 {
			t.Error("Unexpected prompt")
			return "", errors.New("no answer")
		}
		answer := list[0]
		list = list[1:]
		return answer, nil
	}, &asked
}

func storedAs(password string, found bool, err error) StoredPassword {
	return func(context.Context) (string, bool, error) { return password, found, err }
}

func TestOpen(t *testing.T) {
	tests := []struct {
		name      string
		stored    StoredPassword
		answers   []string
		wantAsked int
	}{
		{"password from AWS", storedAs(password, true, nil), nil, 0},
		{"no AWS: asks", nil, []string{password}, 1},
		{"AWS fails: asks", storedAs("", false, errors.New("no credentials")), []string{password}, 1},
		{"wrong, then right", nil, []string{"nope", password}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			file := writeEncrypted(t, dir)
			prompt, asked := answers(t, tt.answers...)

			out, err := Open(context.Background(), file, tt.stored, prompt)
			if err != nil {
				t.Fatalf("Open failed: %v", err)
			}
			if got, _ := os.ReadFile(out); out != filepath.Join(dir, "will.pdf") || string(got) != "the will" {
				t.Errorf("Expected will.pdf next to the encrypted file, got %s", out)
			}
			if *asked != tt.wantAsked {
				t.Errorf("Expected %d prompts, got %d", tt.wantAsked, *asked)
			}
		})
	}
}

func TestOpenNeverReplaces(t *testing.T) {
	dir := t.TempDir()
	file := writeEncrypted(t, dir)
	for _, existing := range []string{"will.pdf", "will (1).pdf"} {
		if err := os.WriteFile(filepath.Join(dir, existing), []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	out, err := Open(context.Background(), file, storedAs(password, true, nil), nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != filepath.Join(dir, "will (2).pdf") {
		t.Errorf("Expected will (2).pdf, got %s", out)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "will.pdf")); string(got) != "keep" {
		t.Error("Expected the existing file kept")
	}
}

func TestOpenFailures(t *testing.T) {
	t.Run("wrong three times", func(t *testing.T) {
		dir := t.TempDir()
		file := writeEncrypted(t, dir)
		prompt, _ := answers(t, "a", "b", "c")
		_, err := Open(context.Background(), file, nil, prompt)
		if err == nil || !strings.Contains(err.Error(), "didn't open it") {
			t.Errorf("Expected a plain explanation, got %v", err)
		}
		if entries, _ := os.ReadDir(dir); len(entries) != 1 {
			t.Errorf("Expected nothing written, got %v", entries)
		}
	})

	t.Run("not encrypted", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "will.pdf")
		if err := os.WriteFile(file, []byte("%PDF"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Open(context.Background(), file, nil, nil); err == nil || !strings.Contains(err.Error(), "opening it as it is") {
			t.Errorf("Expected an explanation, got %v", err)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		if _, err := Open(context.Background(), filepath.Join(t.TempDir(), "none.dmh"), nil, nil); err == nil {
			t.Error("Expected an error")
		}
	})
}

func TestCleanPath(t *testing.T) {
	tests := map[string]string{
		`  "C:\Users\me\Downloads\will.pdf.dmh"  `: `C:\Users\me\Downloads\will.pdf.dmh`,
		`'/Users/me/will.pdf.dmh'`:                 `/Users/me/will.pdf.dmh`,
		"/tmp/will.pdf.dmh\n":                      "/tmp/will.pdf.dmh",
	}
	if filepath.Separator == '/' {
		tests[`/Users/me/My\ Documents/will.pdf.dmh`] = "/Users/me/My Documents/will.pdf.dmh"
	}
	for in, want := range tests {
		if got := CleanPath(in); got != want {
			t.Errorf("CleanPath(%q) = %q, want %q", in, got, want)
		}
	}
}
