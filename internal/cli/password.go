package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/fireflycons/deadmanshandle/internal/doccrypt"
	"github.com/fireflycons/deadmanshandle/internal/ports"
)

// MinPasswordLength is the shortest document password the owner may choose
const MinPasswordLength = 12

// ReadDocumentPassword returns the document password stored in
// parameterName, a SecureString that only the CLI uses; found is false if
// none is stored
func ReadDocumentPassword(ctx context.Context, store ports.ConfigStore, parameterName string) (password string, found bool, err error) {
	data, err := store.GetConfig(ctx, parameterName)
	if errors.Is(err, ports.ErrNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("reading %s: %w", parameterName, err)
	}
	return string(data), true, nil
}

// DocumentPasswordStatus is how `dmh config show` reports the password:
// whether one is set, or the password itself if secrets are shown
func DocumentPasswordStatus(password string, found, showSecrets bool) string {
	switch {
	case !found:
		return "not set (dmh document password set)"
	case showSecrets:
		return password
	default:
		return "set"
	}
}

// PasswordChange describes what SetDocumentPassword found stored
type PasswordChange struct {
	Key         string // the document's key
	Missing     bool   // no document is stored
	Plain       bool   // the document is not encrypted
	Reencrypted bool   // the document was re-encrypted with the new password
}

// SetDocumentPassword stores password as the document password. A stored
// document encrypted with the old password is re-encrypted with the new one
// first, so the stored password always opens it; if the password then can't
// be saved, the error says so.
func SetDocumentPassword(ctx context.Context, store ConfigReadWriter, passwordParameter, configParameter string, docs DocumentTransfer, bucket, password string) (PasswordChange, error) {
	if len(password) < MinPasswordLength {
		return PasswordChange{}, fmt.Errorf("the password must be at least %d characters", MinPasswordLength)
	}
	cfg, err := ReadConfig(ctx, store, configParameter)
	if err != nil {
		return PasswordChange{}, err
	}
	change := PasswordChange{Key: cfg.DocumentKey}

	data, err := download(ctx, docs, bucket, cfg.DocumentKey)
	switch {
	case errors.Is(err, ports.ErrNotFound):
		change.Missing = true
	case err != nil:
		return change, err
	case !doccrypt.IsEncrypted(data):
		change.Plain = true
	default:
		if err := reencrypt(ctx, store, passwordParameter, docs, bucket, cfg.DocumentKey, data, password, &change); err != nil {
			return change, err
		}
	}

	if err := store.PutConfig(ctx, passwordParameter, []byte(password)); err != nil {
		if change.Reencrypted {
			return change, fmt.Errorf("the document is now encrypted with the new password, but saving it in %s failed: %w; "+
				"run dmh document password set -prompt and enter that password", passwordParameter, err)
		}
		return change, fmt.Errorf("saving %s: %w", passwordParameter, err)
	}
	return change, nil
}

// reencrypt encrypts the stored document, data, with password instead of the
// stored password. If it already opens with password, it is left alone.
func reencrypt(ctx context.Context, store ports.ConfigStore, passwordParameter string, docs DocumentTransfer, bucket, key string, data []byte, password string, change *PasswordChange) error {
	old, found, err := ReadDocumentPassword(ctx, store, passwordParameter)
	if err != nil {
		return err
	}
	if found && old != password {
		if name, content, err := doccrypt.Decrypt(data, old); err == nil {
			encrypted, err := doccrypt.Encrypt(content, name, password)
			if err != nil {
				return err
			}
			if err := docs.Upload(ctx, bucket, key, bytes.NewReader(encrypted)); err != nil {
				return fmt.Errorf("uploading the re-encrypted document to s3://%s/%s: %w", bucket, key, err)
			}
			change.Reencrypted = true
			return nil
		}
	}
	if _, _, err := doccrypt.Decrypt(data, password); err == nil {
		return nil
	}
	return fmt.Errorf("s3://%s/%s opens with neither the stored password nor the new one, so nothing was changed; "+
		"set the password it was encrypted with (-prompt), or replace it with dmh document put", bucket, key)
}

// download reads the whole object
func download(ctx context.Context, docs DocumentTransfer, bucket, key string) ([]byte, error) {
	body, err := docs.Download(ctx, bucket, key)
	if err != nil {
		return nil, fmt.Errorf("downloading s3://%s/%s: %w", bucket, key, err)
	}
	defer func() {
		_ = body.Close()
	}()
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("downloading s3://%s/%s: %w", bucket, key, err)
	}
	return data, nil
}
