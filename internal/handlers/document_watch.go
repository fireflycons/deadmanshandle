package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/fireflycons/deadmanshandle/internal/config"
	"github.com/fireflycons/deadmanshandle/internal/domain"
	"github.com/fireflycons/deadmanshandle/internal/ports"
)

// DocumentWatcher emails the owner when the document's content changes. It
// runs on each S3 upload and on each daily run, which catches missed events
// and records the first ETag of a document uploaded before it was deployed.
type DocumentWatcher struct {
	stateStore  ports.StateStore // Holds the recorded ETag
	emailSender ports.EmailSender
	service     *domain.DeadmansHandleService
}

// NewDocumentWatcher creates a new document watcher
func NewDocumentWatcher(
	stateStore ports.StateStore,
	emailSender ports.EmailSender,
	service *domain.DeadmansHandleService,
) *DocumentWatcher {
	return &DocumentWatcher{
		stateStore:  stateStore,
		emailSender: emailSender,
		service:     service,
	}
}

// swapAttempts bounds the retries when another invocation records an ETag
// between this one's read and its swap
const swapAttempts = 3

// Check compares doc with the recorded ETag and, on a change, records the
// new ETag with the time and a pending notice, then tells the owner and
// clears the pending notice. Recording first, with a conditional swap, means
// only one of several concurrent checks sends the notice. If the notice
// fails it stays pending, and a later check sends it. origin is nil when not
// called for an S3 event.
func (w *DocumentWatcher) Check(ctx context.Context, cfg *config.Config, doc domain.Document, origin *domain.ChangeOrigin) error {
	for range swapAttempts {
		state, err := w.stateStore.GetState(ctx)
		if err != nil {
			return fmt.Errorf("reading the recorded document ETag: %w", err)
		}

		notify, record := w.service.DocumentChanged(state, doc)
		changedAt := state.DocumentChangedAt
		switch {
		case record:
			// The silent first sighting is not a change, so is not timed
			var recordedAt time.Time
			if notify {
				changedAt = w.service.Now()
				recordedAt = changedAt
			}
			err = w.stateStore.SwapDocumentETag(ctx, state.DocumentETag, doc.ETag, recordedAt)
			if errors.Is(err, ports.ErrConditionFailed) {
				// Another check recorded an ETag first; compare with that one
				continue
			}
			if err != nil {
				return fmt.Errorf("recording the document ETag: %w", err)
			}
			if !notify {
				slog.Info("Document ETag recorded", "etag", doc.ETag)
				return nil
			}
		case notify:
			// Resending an earlier notice; this event's origin is not the change's
			origin = nil
		default:
			return nil
		}

		email := w.service.DocumentChangedEmail(cfg, state, doc, changedAt, origin)
		if err := w.emailSender.SendEmail(ctx, email.To, email.Subject, email.Body, nil); err != nil {
			return fmt.Errorf("sending document change notice to %s (it stays pending): %w", email.To, err)
		}
		slog.Info("Document change reported", "to", email.To, "from", state.DocumentETag, "etag", doc.ETag)

		// A failed condition means the ETag has moved on, and the newer
		// change has its own pending notice
		err = w.stateStore.ClearDocumentChangePending(ctx, doc.ETag)
		if err != nil && !errors.Is(err, ports.ErrConditionFailed) {
			return fmt.Errorf("clearing the pending change notice: %w", err)
		}
		return nil
	}
	return fmt.Errorf("recording the document ETag: still changing after %d attempts", swapAttempts)
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
