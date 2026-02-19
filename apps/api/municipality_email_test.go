package main

import (
	"context"
	"strings"
	"testing"
	"time"
	"zwerffiets/libs/mailer"
)

func TestBuildMunicipalityReportEmail(t *testing.T) {
	app, _ := newAdminTestServer(t)
	muni := "Eindhoven"
	op := Operator{Email: "op@example.com", Municipality: &muni}

	msg := app.buildMunicipalityReportEmail(op, 5, "http://magic", "http://unsub")

	if msg.To[0] != "op@example.com" {
		t.Errorf("wrong recipient: %s", msg.To[0])
	}
	if !strings.Contains(msg.Subject, "Eindhoven") {
		t.Error("subject should contain municipality name")
	}
	if !strings.Contains(msg.HTML, "5") {
		t.Error("body should contain report count")
	}
	if !strings.Contains(msg.HTML, "http://magic") || !strings.Contains(msg.HTML, "http://unsub") {
		t.Error("body should contain links")
	}
}

type mockProvider struct {
	SentMessages []mailer.Message
}

func (m *mockProvider) Name() string { return "mock" }
func (m *mockProvider) Send(msg mailer.Message) (mailer.SendResult, error) {
	m.SentMessages = append(m.SentMessages, msg)
	return mailer.SendResult{ProviderMessageID: "123"}, nil
}

func TestSendMunicipalityReports(t *testing.T) {
	app, _ := newAdminTestServer(t)

	mockP := &mockProvider{}
	app.mailer = mailer.New(mockP, "test@example.com")

	muni1 := "Eindhoven"
	muni2 := "Utrecht"

	app.adminListReportRecipientOperators = func(ctx context.Context) ([]Operator, error) {
		return []Operator{
			{ID: 1, Email: "op1@example.com", Municipality: &muni1},
			{ID: 2, Email: "op2@example.com", Municipality: &muni2},
		}, nil
	}

	app.adminCountTriagedReportsByMunicipality = func(ctx context.Context, municipality string) (int, error) {
		if municipality == "Eindhoven" {
			return 10, nil
		}
		return 0, nil // Utrecht has 0
	}

	app.adminCreateOperatorMagicLinkToken = func(ctx context.Context, operatorID int, tokenHash string, expiresAt time.Time) error {
		return nil
	}

	err := app.sendMunicipalityReports(context.Background())
	if err != nil {
		t.Fatalf("failed to send reports: %v", err)
	}

	if len(mockP.SentMessages) != 1 {
		t.Errorf("expected 1 email sent, got %d", len(mockP.SentMessages))
	}
	if mockP.SentMessages[0].To[0] != "op1@example.com" {
		t.Errorf("wrong recipient for sent message: %s", mockP.SentMessages[0].To[0])
	}
}
