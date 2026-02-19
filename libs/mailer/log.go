package mailer

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
)

// LogProvider is a fallback provider that logs emails instead of sending them.
type LogProvider struct {
	Logger *slog.Logger
}

// NewLogProvider creates a new log-only provider.
func NewLogProvider(logger *slog.Logger) *LogProvider {
	return &LogProvider{Logger: logger}
}

// Name returns the provider name.
func (l *LogProvider) Name() string {
	return "log"
}

// Send logs the email message and returns a fake message ID.
func (l *LogProvider) Send(msg Message) (SendResult, error) {
	fakeID := uuid.New().String()
	l.Logger.Info("mailer: email logged (not sent)",
		"provider", "log",
		"from", msg.From,
		"to", strings.Join(msg.To, ", "),
		"subject", msg.Subject,
		"html_length", len(msg.HTML),
		"text_length", len(msg.Text),
		"fake_message_id", fakeID,
	)
	if msg.HTML != "" {
		l.Logger.Info("mailer: email HTML body", "html", msg.HTML)
	}
	if msg.Text != "" {
		l.Logger.Info("mailer: email text body", "text", msg.Text)
	}
	return SendResult{ProviderMessageID: fmt.Sprintf("log-%s", fakeID)}, nil
}
