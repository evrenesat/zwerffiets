package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const operatorMagicLinkExpiry = 7 * 24 * time.Hour

func (a *App) generateOperatorMagicLink(ctx context.Context, operatorID int) (string, error) {
	token := createMagicLinkToken()
	hash := hashMagicLinkToken(token)
	expiresAt := time.Now().Add(operatorMagicLinkExpiry)

	if err := a.adminCreateOperatorMagicLinkToken(ctx, operatorID, hash, expiresAt); err != nil {
		return "", err
	}

	return buildPublicURL(a.cfg.PublicBaseURL, fmt.Sprintf("/api/v1/operator/verify?token=%s", token)), nil
}

func (a *App) verifyOperatorMagicLinkHandler(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.String(http.StatusBadRequest, "Missing token")
		return
	}

	hash := hashMagicLinkToken(token)
	operatorID, err := a.adminVerifyOperatorMagicLinkToken(c.Request.Context(), hash)
	if err != nil {
		c.String(http.StatusUnauthorized, "Ongeldige of verlopen link. Vraag een nieuwe aan.")
		return
	}

	op, err := a.adminGetOperatorByID(c.Request.Context(), operatorID)
	if err != nil {
		writeAPIError(c, err)
		return
	}

	if err := a.startOperatorSession(c, OperatorSession{
		Email:        op.Email,
		Role:         op.Role,
		Municipality: op.Municipality,
	}); err != nil {
		writeAPIError(c, err)
		return
	}

	c.Redirect(http.StatusSeeOther, "/bikeadmin")
}

func (a *App) generateUnsubscribeURL(operatorID int) (string, error) {
	claims := jwt.MapClaims{
		"operator_id": strconv.Itoa(operatorID),
		"purpose":     "unsubscribe",
		"iat":         time.Now().Unix(),
		"exp":         time.Now().Add(365 * 24 * time.Hour).Unix(), // Long expiry
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(a.cfg.AppSigningSecret))
	if err != nil {
		return "", err
	}

	return buildPublicURL(a.cfg.PublicBaseURL, fmt.Sprintf("/api/v1/unsubscribe?token=%s", signed)), nil
}

func (a *App) unsubscribeHandler(c *gin.Context) {
	tokenString := c.Query("token")
	if tokenString == "" {
		c.String(http.StatusBadRequest, "Missing token")
		return
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(a.cfg.AppSigningSecret), nil
	})

	if err != nil || !token.Valid {
		c.String(http.StatusUnauthorized, "Invalid token")
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["purpose"] != "unsubscribe" {
		c.String(http.StatusUnauthorized, "Invalid token payload")
		return
	}

	operatorIDStr, ok := claims["operator_id"].(string)
	if !ok {
		c.String(http.StatusUnauthorized, "Invalid operator id")
		return
	}
	operatorID, err := strconv.Atoi(operatorIDStr)
	if err != nil {
		c.String(http.StatusUnauthorized, "Invalid operator id format")
		return
	}

	if err := a.adminSetUnsubscribeRequested(c.Request.Context(), operatorID); err != nil {
		writeAPIError(c, err)
		return
	}

	c.String(http.StatusOK, "Afmelding ontvangen. Je ontvangt geen wekelijkse meldingsmails meer.")
}
