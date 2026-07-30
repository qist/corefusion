package system

import (
	"context"

	"corefusion/internal/store/repository"
)

var (
	defaultNotifier *Notifier
	defaultTmplRepo *repository.NotificationTemplateRepository
)

// RegisterNotifier wires the notifier and template repository so business code can
// send template notifications via NotifyTemplate without holding these dependencies.
func RegisterNotifier(n *Notifier, tmplRepo *repository.NotificationTemplateRepository) {
	defaultNotifier = n
	defaultTmplRepo = tmplRepo
}

// NotifyTemplate looks up an enabled template by name and sends it with the given
// variables. It is a no-op (returns nil) when the notifier is unconfigured or no
// enabled template matches, so callers can fire-and-forget from business events.
func NotifyTemplate(ctx context.Context, name string, vars map[string]string) error {
	if defaultNotifier == nil || defaultTmplRepo == nil {
		return nil
	}
	tmpl, err := defaultTmplRepo.GetByName(ctx, name)
	if err != nil {
		return err
	}
	if tmpl == nil || !tmpl.Enabled {
		return nil
	}
	return defaultNotifier.SendTemplateNotification(ctx, tmpl, vars)
}
