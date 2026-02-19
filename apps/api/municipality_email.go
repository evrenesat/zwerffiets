package main

import (
	"context"
	"fmt"
	"time"
	"zwerffiets/libs/mailer"
)

func (a *App) buildMunicipalityReportEmail(op Operator, triagedCount int, magicLinkURL, unsubscribeURL string) mailer.Message {
	munName := "de gemeente"
	if op.Municipality != nil {
		munName = *op.Municipality
	}

	subject := fmt.Sprintf("Wekelijks overzicht zwerffietsen - %s", munName)

	html := fmt.Sprintf(`
		<div style="font-family: sans-serif; max-width: 600px; margin: 0 auto; line-height: 1.6; color: #333;">
			<h2>Beste beheerder van %s,</h2>
			<p>Er staan momenteel <strong>%d</strong> meldingen van zwerffietsen voor u klaar die zijn getrieerd en klaarstaan voor verdere afhandeling.</p>
			<p style="margin: 30px 0;">
				<a href="%s" style="background-color: #d32f2f; color: white; padding: 12px 24px; text-decoration: none; border-radius: 4px; font-weight: bold; display: inline-block;">
					Bekijk dashboard
				</a>
			</p>
			<p style="font-size: 14px; color: #666;">
				Gebruik bovenstaande knop om direct in te loggen op het beheerpaneel. Deze link is 7 dagen geldig.
			</p>
			<hr style="margin-top: 40px; border: 0; border-top: 1px solid #eee;" />
			<p style="font-size: 12px; color: #999; text-align: center;">
				Wilt u deze e-mails niet meer ontvangen? <a href="%s" style="color: #999;">Afmelden</a>.
			</p>
		</div>
	`, munName, triagedCount, magicLinkURL, unsubscribeURL)

	text := fmt.Sprintf(
		"Beste beheerder van %s,\n\nEr staan momenteel %d meldingen van zwerffietsen voor u klaar.\n\nBekijk ze in het dashboard via deze link:\n%s\n\nAfmelden: %s",
		munName, triagedCount, magicLinkURL, unsubscribeURL,
	)

	return mailer.Message{
		To:      []string{op.Email},
		Subject: subject,
		HTML:    html,
		Text:    text,
	}
}

func (a *App) sendMunicipalityReports(ctx context.Context) error {
	operators, err := a.adminListReportRecipientOperators(ctx)
	if err != nil {
		return fmt.Errorf("failed to list recipient operators: %w", err)
	}

	for _, op := range operators {
		if op.Municipality == nil {
			a.log.Warn("operator marked to receive reports but has no municipality", "email", op.Email)
			continue
		}

		count, err := a.adminCountTriagedReportsByMunicipality(ctx, *op.Municipality)
		if err != nil {
			a.log.Error("failed to count reports for municipality", "municipality", *op.Municipality, "err", err)
			continue
		}

		if count == 0 {
			a.log.Info("skipping municipality report email (0 triaged reports)", "municipality", *op.Municipality)
			continue
		}

		// Generate links (need gin.Context for magic link generation helpers)
		// Since we're in CLI/batch, we'll manually create a dummy context for the helper
		// or refactor the helper to not depend on gin.Context

		magicLinkURL, err := a.createMagicLinkForBatch(ctx, op.ID)
		if err != nil {
			a.log.Error("failed to generate magic link", "email", op.Email, "err", err)
			continue
		}

		unsubscribeURL, err := a.generateUnsubscribeURL(op.ID) // Unsubscribe doesn't use gin.Context
		if err != nil {
			a.log.Error("failed to generate unsubscribe url", "email", op.Email, "err", err)
			continue
		}

		msg := a.buildMunicipalityReportEmail(op, count, magicLinkURL, unsubscribeURL)

		if _, err := a.mailer.Send(msg); err != nil {
			a.log.Error("failed to send municipality report email", "email", op.Email, "err", err)
			continue
		}

		a.log.Info("sent municipality report email", "email", op.Email, "municipality", *op.Municipality, "count", count)
	}

	return nil
}

// createMagicLinkForBatch is a non-gin version of generateOperatorMagicLink
func (a *App) createMagicLinkForBatch(ctx context.Context, operatorID int) (string, error) {
	token := createMagicLinkToken()
	hash := hashMagicLinkToken(token)
	expiresAt := time.Now().Add(operatorMagicLinkExpiry)

	if err := a.adminCreateOperatorMagicLinkToken(ctx, operatorID, hash, expiresAt); err != nil {
		return "", err
	}

	return buildPublicURL(a.cfg.PublicBaseURL, fmt.Sprintf("/api/v1/operator/verify?token=%s", token)), nil
}
