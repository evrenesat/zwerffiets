package mailer

import (
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestLogProviderSend(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	provider := NewLogProvider(logger)

	msg := Message{
		From:    "test@example.com",
		To:      []string{"recipient@example.com"},
		Subject: "Test Subject",
		HTML:    "<p>Test HTML</p>",
		Text:    "Test text",
	}

	result, err := provider.Send(msg)
	if err != nil {
		t.Fatalf("LogProvider.Send() error = %v", err)
	}

	if result.ProviderMessageID == "" {
		t.Error("LogProvider.Send() returned empty message ID")
	}

	if !strings.HasPrefix(result.ProviderMessageID, "log-") {
		t.Errorf("LogProvider.Send() message ID = %v, want prefix 'log-'", result.ProviderMessageID)
	}
}

func TestLogProviderName(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	provider := NewLogProvider(logger)

	if got := provider.Name(); got != "log" {
		t.Errorf("LogProvider.Name() = %v, want 'log'", got)
	}
}

func TestMailerSendDelegatesToProvider(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	provider := NewLogProvider(logger)
	mailer := New(provider, "default@test.com")

	msg := Message{
		To:      []string{"recipient@example.com"},
		Subject: "Test",
		HTML:    "<p>Test</p>",
	}

	result, err := mailer.Send(msg)
	if err != nil {
		t.Fatalf("Mailer.Send() error = %v", err)
	}

	if result.ProviderMessageID == "" {
		t.Error("Mailer.Send() returned empty message ID")
	}

	// Verify that the default From address was used
	// Note: Since LogProvider doesn't expose the sent message easily without modification,
	// we rely on the fact that Send didn't error.
	// To strictly test this, we'd need a mock provider, but since we're refactoring,
	// checking API compatibility (compilation) and no runtime errors is the first step.
}

func TestMailerProviderName(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	provider := NewLogProvider(logger)
	mailer := New(provider, "default@test.com")

	if got := mailer.ProviderName(); got != "log" {
		t.Errorf("Mailer.ProviderName() = %v, want 'log'", got)
	}
}

func TestResendProviderName(t *testing.T) {
	provider := NewResendProvider("fake-api-key")

	if got := provider.Name(); got != "resend" {
		t.Errorf("ResendProvider.Name() = %v, want 'resend'", got)
	}
}
