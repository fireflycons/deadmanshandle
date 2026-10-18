package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/fireflycons/deadmanshandle/internal/ports"
)

// DocumentTransfer downloads and uploads the document. Only the CLI does,
// so it is not part of ports.DocumentStore.
type DocumentTransfer interface {
	Download(ctx context.Context, bucket, key string) (io.ReadCloser, error)
	Upload(ctx context.Context, bucket, key string, body io.Reader) error
}

// GetDocument downloads the document into dir, named after its key as the
// recipients receive it, and returns the file's path. An existing file is
// replaced only if force is set.
func GetDocument(ctx context.Context, configStore ports.ConfigStore, parameterName string, docs DocumentTransfer, bucket, dir string, force bool) (string, error) {
	cfg, err := ReadConfig(ctx, configStore, parameterName)
	if err != nil {
		return "", err
	}
	body, err := docs.Download(ctx, bucket, cfg.DocumentKey)
	if err != nil {
		return "", fmt.Errorf("downloading s3://%s/%s: %w", bucket, cfg.DocumentKey, err)
	}
	defer func() {
		_ = body.Close()
	}()

	file := filepath.Join(dir, path.Base(cfg.DocumentKey))
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if force {
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	// Owner-only, as the document is presumably private
	out, err := os.OpenFile(file, flags, 0o600)
	if errors.Is(err, os.ErrExist) {
		return "", fmt.Errorf("%s already exists; add -force to replace it", file)
	}
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(out, body); err != nil {
		_ = out.Close()
		_ = os.Remove(file)
		return "", fmt.Errorf("downloading s3://%s/%s: %w", bucket, cfg.DocumentKey, err)
	}
	return file, out.Close()
}

// PutResult describes an upload by PutDocument
type PutResult struct {
	Key         string // where the document now is
	PreviousKey string // where the config pointed before; the object there is kept
}

// KeyChanged reports whether the upload moved the document to a new key
func (r PutResult) KeyChanged() bool {
	return r.Key != r.PreviousKey
}

// PutDocument uploads the file at filePath as the document, under key, or
// the file's name if key is empty, and points the config at it. It uploads
// first, so the config never names a document that isn't there; if the
// config cannot then be changed, the upload stays and the error says so.
func PutDocument(ctx context.Context, store ConfigReadWriter, parameterName string, docs DocumentTransfer, bucket, filePath, key string) (PutResult, error) {
	if key == "" {
		key = filepath.Base(filePath)
	}
	if strings.TrimSpace(key) == "" || strings.HasSuffix(key, "/") || path.Base(key) == "." || path.Base(key) == ".." {
		return PutResult{}, fmt.Errorf("%q is not a usable document key", key)
	}
	cfg, err := ReadConfig(ctx, store, parameterName)
	if err != nil {
		return PutResult{}, err
	}
	result := PutResult{Key: key, PreviousKey: cfg.DocumentKey}

	file, err := os.Open(filePath)
	if err != nil {
		return PutResult{}, err
	}
	defer func() {
		_ = file.Close()
	}()
	if err := docs.Upload(ctx, bucket, key, file); err != nil {
		return PutResult{}, fmt.Errorf("uploading to s3://%s/%s: %w", bucket, key, err)
	}

	if result.KeyChanged() {
		if _, err := SetConfig(ctx, store, parameterName, ConfigChange{DocumentKey: &key}); err != nil {
			return PutResult{}, fmt.Errorf("uploaded to s3://%s/%s, but the config still names %s: %w", bucket, key, cfg.DocumentKey, err)
		}
	}
	return result, nil
}
