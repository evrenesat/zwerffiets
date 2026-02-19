package mailer

// Message represents an email to send.
type Message struct {
	From    string
	To      []string
	Subject string
	HTML    string
	Text    string
}

// SendResult contains the response from the provider.
type SendResult struct {
	ProviderMessageID string
}

// Provider sends emails via a specific backend.
type Provider interface {
	Name() string
	Send(msg Message) (SendResult, error)
}

// Mailer is the top-level entry point for sending emails.
type Mailer struct {
	provider    Provider
	fromAddress string
}

// New creates a new Mailer with the given provider and default sender address.
func New(provider Provider, fromAddress string) *Mailer {
	return &Mailer{
		provider:    provider,
		fromAddress: fromAddress,
	}
}

// Send sends an email message via the configured provider.
// If msg.From is empty, the default fromAddress is used.
func (m *Mailer) Send(msg Message) (SendResult, error) {
	if msg.From == "" {
		msg.From = m.fromAddress
	}
	return m.provider.Send(msg)
}

// ProviderName returns the name of the configured provider.
func (m *Mailer) ProviderName() string {
	return m.provider.Name()
}
