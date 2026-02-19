package mailer

import (
	"fmt"

	"github.com/resend/resend-go/v2"
)

// ResendProvider sends emails via the Resend API.
type ResendProvider struct {
	client *resend.Client
}

// NewResendProvider creates a new Resend provider with the given API key.
func NewResendProvider(apiKey string) *ResendProvider {
	return &ResendProvider{
		client: resend.NewClient(apiKey),
	}
}

// Name returns the provider name.
func (r *ResendProvider) Name() string {
	return "resend"
}

// Send sends an email via the Resend API.
func (r *ResendProvider) Send(msg Message) (SendResult, error) {
	params := &resend.SendEmailRequest{
		From:    msg.From,
		To:      msg.To,
		Subject: msg.Subject,
		Html:    msg.HTML,
	}
	if msg.Text != "" {
		params.Text = msg.Text
	}

	sent, err := r.client.Emails.Send(params)
	if err != nil {
		return SendResult{}, fmt.Errorf("resend send failed: %w", err)
	}

	return SendResult{ProviderMessageID: sent.Id}, nil
}
