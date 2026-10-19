package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/fireflycons/deadmanshandle/internal/doccrypt"
)

// DocumentTransfer downloads and uploads the document. Only the CLI does,
// so it is not part of ports.DocumentStore.
type DocumentTransfer interface {
	Download(ctx context.Context, bucket, key string) (io.ReadCloser, error)
	Upload(ctx context.Context, bucket, key string, body io.Reader) error
}

// PasswordPrompt asks for the document password, showing prompt
type PasswordPrompt func(prompt string) (string, error)

// passwordTries is how many times a wrong password may be typed
const passwordTries = 3

// Documents is what the document commands work with: the config naming the
// document, the stored password, the bucket, and a way to ask for the
// password when the stored one can't be read
type Documents struct {
	Store             ConfigReadWriter
	ConfigParameter   string
	PasswordParameter string
	Transfer          DocumentTransfer
	Bucket            string
	Prompt            PasswordPrompt
}

// GetDocument downloads the document into dir and returns the file's path.
// An encrypted document is decrypted and named as it was before encryption,
// unless raw is set; otherwise the file is named after the key, as the
// recipients receive it. An existing file is replaced only if force is set.
func GetDocument(ctx context.Context, d Documents, dir string, force, raw bool) (string, error) {
	cfg, err := ReadConfig(ctx, d.Store, d.ConfigParameter)
	if err != nil {
		return "", err
	}
	data, err := download(ctx, d.Transfer, d.Bucket, cfg.DocumentKey)
	if err != nil {
		return "", err
	}

	name := path.Base(cfg.DocumentKey)
	if !raw && doccrypt.IsEncrypted(data) {
		password, found, err := ReadDocumentPassword(ctx, d.Store, d.PasswordParameter)
		if err != nil {
			// Not fatal: the password can be typed instead
			found = false
		}
		name, data, err = OpenDocument(data, password, found, d.Prompt)
		if err != nil {
			return "", err
		}
	}

	file := filepath.Join(dir, name)
	if err := writeFile(file, data, force); err != nil {
		return "", err
	}
	return file, nil
}

// OpenDocument decrypts data with the stored password if there is one
// (found), and otherwise, or if it is wrong, with one typed at prompt. The
// name it returns is safe to use as a file name in a directory.
func OpenDocument(data []byte, stored string, found bool, prompt PasswordPrompt) (string, []byte, error) {
	if found {
		if name, content, err := doccrypt.Decrypt(data, stored); err == nil {
			return safeName(name), content, nil
		} else if !errors.Is(err, doccrypt.ErrWrongPassword) {
			return "", nil, err
		}
	}
	if prompt == nil {
		return "", nil, doccrypt.ErrWrongPassword
	}
	question := "Document password: "
	if found {
		question = "The stored password doesn't open it. Document password: "
	}
	for try := 1; ; try++ {
		password, err := prompt(question)
		if err != nil {
			return "", nil, err
		}
		name, content, err := doccrypt.Decrypt(data, password)
		if err == nil {
			return safeName(name), content, nil
		}
		if !errors.Is(err, doccrypt.ErrWrongPassword) || try == passwordTries {
			return "", nil, err
		}
		question = "That password didn't work. Try again: "
	}
}

// safeName keeps only the last element of a name from a decrypted file, so
// that it can't write outside the directory, and replaces the characters
// Windows doesn't allow in file names
func safeName(name string) string {
	name = path.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || strings.ContainsRune(`<>:"|?*`, r) {
			return '_'
		}
		return r
	}, name)
	if name == "." || name == ".." || name == "/" || strings.Trim(name, ". ") == "" {
		return "document"
	}
	return name
}

// writeFile creates file with data, owner-only as the document is
// presumably private. An existing file is replaced only if force is set.
func writeFile(file string, data []byte, force bool) error {
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if force {
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	out, err := os.OpenFile(file, flags, 0o600)
	if errors.Is(err, os.ErrExist) {
		return fmt.Errorf("%s already exists; add -force to replace it", file)
	}
	if err != nil {
		return err
	}
	if _, err := out.Write(data); err != nil {
		_ = out.Close()
		_ = os.Remove(file)
		return err
	}
	return out.Close()
}

// PutResult describes an upload by PutDocument
type PutResult struct {
	Key         string // where the document now is
	PreviousKey string // where the config pointed before; the object there is kept
	Encrypted   bool
}

// KeyChanged reports whether the upload moved the document to a new key
func (r PutResult) KeyChanged() bool {
	return r.Key != r.PreviousKey
}

// PutDocument uploads the file at filePath as the document, and points the
// config at it. It is encrypted with the stored password, under the file's
// name plus doccrypt.Extension unless key is given; with plain set it is
// uploaded as it is, under the file's name. If the password can't be read
// (no access, say), it is asked for; with none stored, the upload is
// refused. It uploads first, so the config never names a document that
// isn't there; if the config cannot then be changed, the upload stays and
// the error says so.
func PutDocument(ctx context.Context, d Documents, filePath, key string, plain bool) (PutResult, error) {
	if key == "" {
		key = filepath.Base(filePath)
		if !plain {
			key += doccrypt.Extension
		}
	}
	if strings.TrimSpace(key) == "" || strings.HasSuffix(key, "/") || path.Base(key) == "." || path.Base(key) == ".." {
		return PutResult{}, fmt.Errorf("%q is not a usable document key", key)
	}
	cfg, err := ReadConfig(ctx, d.Store, d.ConfigParameter)
	if err != nil {
		return PutResult{}, err
	}
	result := PutResult{Key: key, PreviousKey: cfg.DocumentKey, Encrypted: !plain}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return PutResult{}, err
	}
	if !plain {
		password, err := putPassword(ctx, d)
		if err != nil {
			return PutResult{}, err
		}
		if data, err = doccrypt.Encrypt(data, filepath.Base(filePath), password); err != nil {
			return PutResult{}, err
		}
	}
	if err := d.Transfer.Upload(ctx, d.Bucket, key, bytes.NewReader(data)); err != nil {
		return PutResult{}, fmt.Errorf("uploading to s3://%s/%s: %w", d.Bucket, key, err)
	}

	if result.KeyChanged() {
		if _, err := SetConfig(ctx, d.Store, d.ConfigParameter, ConfigChange{DocumentKey: &key}); err != nil {
			return PutResult{}, fmt.Errorf("uploaded to s3://%s/%s, but the config still names %s: %w", d.Bucket, key, cfg.DocumentKey, err)
		}
	}
	return result, nil
}

// putPassword is the stored password, or one typed if it can't be read. A
// typed one must be the stored one, or recipients couldn't open the
// document, so it is asked for twice.
func putPassword(ctx context.Context, d Documents) (string, error) {
	password, found, err := ReadDocumentPassword(ctx, d.Store, d.PasswordParameter)
	if err == nil && !found {
		return "", fmt.Errorf("no document password is set; run dmh document password set first, or add -plain to upload it unencrypted")
	}
	if err == nil {
		return password, nil
	}
	first, perr := d.Prompt(fmt.Sprintf("Couldn't read %s (%v).\nDocument password: ", d.PasswordParameter, err))
	if perr != nil {
		return "", perr
	}
	again, perr := d.Prompt("Again: ")
	if perr != nil {
		return "", perr
	}
	if first != again {
		return "", errors.New("the passwords don't match")
	}
	if len(first) < MinPasswordLength {
		return "", fmt.Errorf("the password must be at least %d characters", MinPasswordLength)
	}
	return first, nil
}
