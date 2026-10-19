// Package opener is the logic of open-document, the program recipients use
// to decrypt the document they were emailed
package opener

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fireflycons/deadmanshandle/internal/cli"
	"github.com/fireflycons/deadmanshandle/internal/doccrypt"
)

// StoredPassword reads the document password from AWS, if the recipient
// has access; found is false if it isn't there
type StoredPassword func(ctx context.Context) (password string, found bool, err error)

// Open decrypts the encrypted document in file and writes the original next
// to it, never replacing a file, and returns the path written. The password
// comes from stored if it is set and works, else from prompt.
func Open(ctx context.Context, file string, stored StoredPassword, prompt cli.PasswordPrompt) (string, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("couldn't read %s: %w", file, err)
	}
	if !doccrypt.IsEncrypted(data) {
		return "", fmt.Errorf("%s isn't an encrypted document from the dead man's handle, so there's nothing to open: try opening it as it is", filepath.Base(file))
	}

	var password string
	found := false
	if stored != nil {
		// Any failure just means asking for the password instead
		password, found, _ = stored(ctx)
	}
	name, content, err := cli.OpenDocument(data, password, found, prompt)
	if errors.Is(err, doccrypt.ErrWrongPassword) {
		return "", errors.New("that password didn't open it. Check it with the person who gave it to you, and try again")
	}
	if err != nil {
		return "", err
	}

	out, err := writeNew(filepath.Dir(file), name, content)
	if err != nil {
		return "", fmt.Errorf("couldn't save the opened document: %w", err)
	}
	return out, nil
}

// writeNew writes content as name in dir, or as "name (1).ext" and so on if
// that is taken
func writeNew(dir, name string, content []byte) (string, error) {
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 0; i < 1000; i++ {
		candidate := name
		if i > 0 {
			candidate = fmt.Sprintf("%s (%d)%s", stem, i, ext)
		}
		path := filepath.Join(dir, candidate)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if _, err := f.Write(content); err != nil {
			_ = f.Close()
			_ = os.Remove(path)
			return "", err
		}
		return path, f.Close()
	}
	return "", fmt.Errorf("too many files named like %s in %s", name, dir)
}

// CleanPath tidies a path typed or dropped onto the console, which may be
// quoted (Windows) or have escaped spaces (macOS Terminal)
func CleanPath(path string) string {
	path = strings.TrimSpace(path)
	if len(path) >= 2 && (path[0] == '"' && path[len(path)-1] == '"' || path[0] == '\'' && path[len(path)-1] == '\'') {
		return path[1 : len(path)-1]
	}
	if filepath.Separator == '/' {
		path = strings.ReplaceAll(path, `\ `, " ")
	}
	return path
}
