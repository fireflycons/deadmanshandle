package handlers

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/fireflycons/deadmanshandle/internal/config"
	"github.com/fireflycons/deadmanshandle/internal/domain"
	"github.com/fireflycons/deadmanshandle/internal/ports"
)

// DocumentWatcher emails the owner when the document's content changes. It
// runs on each S3 upload and on each daily run, which catches missed events
// and records the first ETag of a document uploaded before it was deployed.
type DocumentWatcher struct {
	etagStore   ports.ConfigStore // Holds the recorded ETag
	emailSender ports.EmailSender
	service     *domain.DeadmansHandleService
	etagParam   string
}

// NewDocumentWatcher creates a watcher that records the ETag in etagParam
func NewDocumentWatcher(
	etagStore ports.ConfigStore,
	emailSender ports.EmailSender,
	service *domain.DeadmansHandleService,
	etagParam string,
) *DocumentWatcher {
	return &DocumentWatcher{
		etagStore:   etagStore,
		emailSender: emailSender,
		service:     service,
		etagParam:   etagParam,
	}
}

// Check compares doc with the recorded ETag, tells the owner of a change and
// records the new ETag. If the email fails, nothing is recorded, so the
// next check retries it. origin is nil when not called for an S3 event.
func (w *DocumentWatcher) Check(ctx context.Context, cfg *config.Config, doc domain.Document, origin *domain.ChangeOrigin) error {
	recorded, err := w.etagStore.GetConfig(ctx, w.etagParam)
	if err != nil {
		return fmt.Errorf("reading the recorded document ETag: %w", err)
	}

	notify, record := w.service.DocumentChanged(string(recorded), doc)
	if notify {
		email := w.service.DocumentChangedEmail(cfg, doc, origin)
		if err := w.emailSender.SendEmail(ctx, email.To, email.Subject, email.Body, nil); err != nil {
			return fmt.Errorf("sending document change notice to %s: %w", email.To, err)
		}
		slog.Info("Document change reported", "to", email.To, "from", string(recorded), "etag", doc.ETag)
	}

	if record {
		if err := w.etagStore.SetConfig(ctx, w.etagParam, []byte(doc.ETag)); err != nil {
			return fmt.Errorf("recording the document ETag: %w", err)
		}
		if !notify {
			slog.Info("Document ETag recorded", "etag", doc.ETag)
		}
	}
	return nil
}

// DocumentWatchHandler handles the S3 events for uploads of the document
type DocumentWatchHandler struct {
	configStore    ports.ConfigStore
	documentStore  ports.DocumentStore
	watcher        *DocumentWatcher
	paramName      string
	documentBucket string
	documentKey    string
}

// NewDocumentWatchHandler creates a new document watch handler
func NewDocumentWatchHandler(
	configStore ports.ConfigStore,
	documentStore ports.DocumentStore,
	watcher *DocumentWatcher,
	parameterName string,
	docBucket string,
	docKey string,
) *DocumentWatchHandler {
	return &DocumentWatchHandler{
		configStore:    configStore,
		documentStore:  documentStore,
		watcher:        watcher,
		paramName:      parameterName,
		documentBucket: docBucket,
		documentKey:    docKey,
	}
}

// Handle checks the document as it is now, not as the event describes it,
// since events can arrive out of order.
func (h *DocumentWatchHandler) Handle(ctx context.Context, origin *domain.ChangeOrigin) error {
	configData, err := h.configStore.GetConfig(ctx, h.paramName)
	if err != nil {
		return err
	}

	cfg, err := config.ParseConfig(configData)
	if err != nil {
		return err
	}

	doc, err := lookupDocument(ctx, h.documentStore, h.documentBucket, h.documentKey)
	if err != nil {
		return err
	}

	return h.watcher.Check(ctx, cfg, doc, origin)
}

// lookupDocument finds the document and its current ETag
func lookupDocument(ctx context.Context, store ports.DocumentStore, bucket, key string) (domain.Document, error) {
	etag, exists, err := store.DocumentETag(ctx, bucket, key)
	if err != nil {
		return domain.Document{}, fmt.Errorf("checking for the document: %w", err)
	}
	return domain.Document{
		Location:  "s3://" + bucket + "/" + key,
		Available: exists,
		ETag:      etag,
	}, nil
}
