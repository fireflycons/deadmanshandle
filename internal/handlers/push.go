package handlers

import (
	"context"
	"log/slog"

	"github.com/fireflycons/deadmanshandle/internal/domain"
	"github.com/fireflycons/deadmanshandle/internal/ports"
)

// pushToOwner pushes an owner notice to the owner's phone, if push is set up
// and the app has registered a token. Push is a best-effort extra to the
// email: a failure is logged, not returned, so it neither fails the run nor
// makes Lambda's retry resend the emails.
func pushToOwner(ctx context.Context, sender ports.PushSender, token string, email domain.EmailAction) {
	if sender == nil || token == "" {
		return
	}
	if err := sender.Push(ctx, token, email.Kind.String(), email.Subject, email.Body); err != nil {
		slog.Warn("Push failed", "kind", email.Kind.String(), "error", err)
		return
	}
	slog.Info("Push sent", "kind", email.Kind.String())
}
