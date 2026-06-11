package system

import (
	"context"
	"fmt"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"corefusion/internal/model/entity"
)

// Notifier handles sending notifications via Telegram and Email.
type Notifier struct {
	settingSvc *SettingService
}

// NewNotifier creates a new notifier.
func NewNotifier(settingSvc *SettingService) *Notifier {
	return &Notifier{settingSvc: settingSvc}
}

// SendTelegram sends a message via Telegram bot.
func (n *Notifier) SendTelegram(ctx context.Context, message string) error {
	config, err := n.settingSvc.GetNotificationConfig(ctx)
	if err != nil {
		return fmt.Errorf("get config: %w", err)
	}
	if !config.TelegramEnabled {
		return fmt.Errorf("telegram not enabled")
	}
	if config.TelegramBotToken == "" || config.TelegramChatID == "" {
		return fmt.Errorf("telegram not configured")
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", config.TelegramBotToken)
	payload := fmt.Sprintf(`{"chat_id":"%s","text":"%s"}`, config.TelegramChatID, escapeJSON(message))

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("telegram API returned %d", resp.StatusCode)
	}
	return nil
}

// SendEmail sends an email via SMTP.
func (n *Notifier) SendEmail(ctx context.Context, to, subject, body string) error {
	config, err := n.settingSvc.GetNotificationConfig(ctx)
	if err != nil {
		return fmt.Errorf("get config: %w", err)
	}
	if !config.SMTPEnabled {
		return fmt.Errorf("smtp not enabled")
	}
	if config.SMTPHost == "" {
		return fmt.Errorf("smtp not configured")
	}

	addr := config.SMTPHost + ":" + config.SMTPPort

	var auth smtp.Auth
	if config.SMTPUser != "" && config.SMTPPass != "" {
		auth = smtp.PlainAuth("", config.SMTPUser, config.SMTPPass, config.SMTPHost)
	}

	from := config.SMTPFrom
	if from == "" {
		from = config.SMTPUser
	}

	msg := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s", from, to, subject, body))

	return smtp.SendMail(addr, auth, from, []string{to}, msg)
}

// SendNotification sends notification to all enabled channels.
func (n *Notifier) SendNotification(ctx context.Context, message string) error {
	config, err := n.settingSvc.GetNotificationConfig(ctx)
	if err != nil {
		return err
	}

	var errs []string

	if config.TelegramEnabled {
		if err := n.SendTelegram(ctx, message); err != nil {
			errs = append(errs, "telegram: "+err.Error())
		}
	}

	if config.SMTPEnabled && config.SMTPFrom != "" {
		if err := n.SendEmail(ctx, config.SMTPFrom, "CoreFusion Notification", message); err != nil {
			errs = append(errs, "email: "+err.Error())
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("notification errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// SendTemplateNotification sends a notification using a template.
func (n *Notifier) SendTemplateNotification(ctx context.Context, tmpl *entity.NotificationTemplate, vars map[string]string) error {
	content := tmpl.Content
	for k, v := range vars {
		content = strings.ReplaceAll(content, "{"+k+"}", v)
	}

	if tmpl.Type == "telegram" {
		// Send to configured chat ID
		return n.SendTelegram(ctx, content)
	}
	if tmpl.Type == "email" {
		subject := tmpl.Subject
		if subject == "" {
			subject = "CoreFusion Notification"
		}
		// Get recipients from template
		recipients := parseRecipients(tmpl.Recipients)
		if len(recipients) == 0 {
			// Fallback to SMTP from address
			config, err := n.settingSvc.GetNotificationConfig(ctx)
			if err != nil {
				return err
			}
			recipients = []string{config.SMTPFrom}
		}
		// Send to each recipient
		var errs []string
		for _, to := range recipients {
			to = strings.TrimSpace(to)
			if to == "" {
				continue
			}
			if err := n.SendEmail(ctx, to, subject, content); err != nil {
				errs = append(errs, err.Error())
			}
		}
		if len(errs) > 0 {
			return fmt.Errorf("email errors: %s", strings.Join(errs, "; "))
		}
		return nil
	}

	return fmt.Errorf("unknown template type: %s", tmpl.Type)
}

// parseRecipients splits comma-separated recipients.
func parseRecipients(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// escapeJSON escapes special characters for JSON string.
func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}
