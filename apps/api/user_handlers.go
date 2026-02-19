package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"zwerffiets/libs/mailer"

	"github.com/gin-gonic/gin"
)

func (a *App) requestMagicLinkHandler(c *gin.Context) {
	var payload struct {
		Email string `json:"email"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_payload", "message": "Invalid request payload"})
		return
	}

	email := strings.ToLower(strings.TrimSpace(payload.Email))
	if email == "" || !strings.Contains(email, "@") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_email", "message": "Valid email required"})
		return
	}

	user, err := a.findOrCreateUser(c.Request.Context(), email)
	if err != nil {
		a.log.Error("failed to find/create user", "email", email, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to process request"})
		return
	}

	token := createMagicLinkToken()
	tokenHash := hashMagicLinkToken(token)
	expiresAt := time.Now().UTC().Add(magicLinkTokenExpiry)

	_, err = a.db.ExecContext(c.Request.Context(), `
		INSERT INTO magic_link_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, user.ID, tokenHash, expiresAt)
	if err != nil {
		a.log.Error("failed to store magic link token", "user_id", user.ID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to process request"})
		return
	}

	magicLinkURL := fmt.Sprintf("%s/auth/verify?token=%s", a.cfg.PublicBaseURL, token)

	msg := mailer.Message{
		To:      []string{email},
		Subject: "Your ZwerfFiets login link",
		HTML:    fmt.Sprintf(`<p>Click the link below to log in to ZwerfFiets:</p><p><a href="%s">Log in</a></p><p>This link expires in 15 minutes.</p>`, magicLinkURL),
		Text:    fmt.Sprintf("Click this link to log in: %s\n\nThis link expires in 15 minutes.", magicLinkURL),
	}

	result, err := a.mailer.Send(msg)
	if err != nil {
		a.log.Error("failed to send magic link email", "email", email, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to send email"})
		return
	}

	a.log.Info("magic link sent", "email", email, "provider", a.mailer.ProviderName(), "message_id", result.ProviderMessageID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) verifyMagicLinkHandler(c *gin.Context) {
	token := strings.TrimSpace(c.Query("token"))
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing_token", "message": "Token required"})
		return
	}

	tokenHash := hashMagicLinkToken(token)

	var userID int

	err := a.db.QueryRowContext(c.Request.Context(), `
		UPDATE magic_link_tokens
		SET used_at = NOW()
		WHERE token_hash = $1
		  AND used_at IS NULL
		  AND expires_at > NOW()
		RETURNING user_id
	`, tokenHash).Scan(&userID)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_token", "message": "Invalid or expired token"})
			return
		}
		a.log.Error("failed to query magic link token", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to verify token"})
		return
	}

	user, err := a.getUserByID(c.Request.Context(), userID)
	if err != nil || user == nil {
		a.log.Error("failed to get user after token verification", "user_id", userID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to create session"})
		return
	}

	if !user.IsActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "account_inactive", "message": "Account is inactive"})
		return
	}

	session := UserSession{
		UserID: user.ID,
		Email:  user.Email,
	}

	sessionToken, err := a.createUserSessionToken(session)
	if err != nil {
		a.log.Error("failed to create user session token", "user_id", user.ID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to create session"})
		return
	}

	secure := strings.EqualFold(a.cfg.Env, "production")
	c.SetCookie(userCookieName, sessionToken, int(userSessionDuration.Seconds()), "/", "", secure, true)

	_, claimErr := a.db.ExecContext(c.Request.Context(), `
		UPDATE reports
		SET user_id = $1, reporter_email_confirmed = TRUE, updated_at = NOW()
		WHERE reporter_email = $2
		  AND reporter_email_confirmed = FALSE
		  AND user_id IS NULL
	`, user.ID, user.Email)
	if claimErr != nil {
		a.log.Error("failed to claim pending reporter reports", "user_id", user.ID, "err", claimErr)
	}

	c.JSON(http.StatusOK, gin.H{"userId": user.ID, "email": user.Email})
}

func (a *App) userLogoutHandler(c *gin.Context) {
	secure := strings.EqualFold(a.cfg.Env, "production")
	c.SetCookie(userCookieName, "", -1, "/", "", secure, true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) userSessionHandler(c *gin.Context) {
	token, err := c.Cookie(userCookieName)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": "User session required"})
		return
	}

	session, err := a.verifyUserSessionToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": "User session required"})
		return
	}

	c.JSON(http.StatusOK, session)
}

func (a *App) userReportsHandler(c *gin.Context) {
	session, err := getUserSession(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": "User session required"})
		return
	}

	reports, err := a.listReportsByUserID(c.Request.Context(), session.UserID)
	if err != nil {
		a.log.Error("failed to list user reports", "user_id", session.UserID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to load reports"})
		return
	}

	c.JSON(http.StatusOK, reports)
}

func (a *App) findOrCreateUser(ctx context.Context, email string) (*User, error) {
	var user User
	err := a.db.QueryRowContext(ctx, `
		SELECT id, email, display_name, is_active, created_at, updated_at
		FROM users
		WHERE email = $1
	`, email).Scan(&user.ID, &user.Email, &user.DisplayName, &user.IsActive, &user.CreatedAt, &user.UpdatedAt)

	if err == nil {
		return &user, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	err = a.db.QueryRowContext(ctx, `
		INSERT INTO users (email, is_active)
		VALUES ($1, TRUE)
		RETURNING id, email, display_name, is_active, created_at, updated_at
	`, email).Scan(&user.ID, &user.Email, &user.DisplayName, &user.IsActive, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (a *App) getUserByID(ctx context.Context, userID int) (*User, error) {
	var user User
	err := a.db.QueryRowContext(ctx, `
		SELECT id, email, display_name, is_active, created_at, updated_at
		FROM users
		WHERE id = $1
	`, userID).Scan(&user.ID, &user.Email, &user.DisplayName, &user.IsActive, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &user, nil
}

func (a *App) listReportsByUserID(ctx context.Context, userID int) ([]Report, error) {
	rows, err := a.db.QueryContext(ctx, reportSelect+` WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reports := make([]Report, 0)
	for rows.Next() {
		report, err := scanReport(rows)
		if err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}

	return reports, rows.Err()
}
