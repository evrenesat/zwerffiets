package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGenerateOperatorMagicLink(t *testing.T) {
	app, _ := newAdminTestServer(t)

	var capturedOpID int
	var capturedHash string
	app.adminCreateOperatorMagicLinkToken = func(ctx context.Context, operatorID int, tokenHash string, expiresAt time.Time) error {
		capturedOpID = operatorID
		capturedHash = tokenHash
		return nil
	}

	url, err := app.generateOperatorMagicLink(context.Background(), 123)
	if err != nil {
		t.Fatalf("failed to generate magic link: %v", err)
	}

	if capturedOpID != 123 {
		t.Errorf("expected opID 123, got %d", capturedOpID)
	}
	if capturedHash == "" {
		t.Error("expected token hash to be generated")
	}
	if !strings.Contains(url, "/api/v1/operator/verify?token=") {
		t.Errorf("unexpected url format: %s", url)
	}
}

func TestVerifyOperatorMagicLinkHandler_Success(t *testing.T) {
	app, router := newAdminTestServer(t)

	muni := "Eindhoven"
	app.adminVerifyOperatorMagicLinkToken = func(ctx context.Context, tokenHash string) (int, error) {
		return 123, nil
	}
	app.adminGetOperatorByID = func(ctx context.Context, id int) (*Operator, error) {
		return &Operator{ID: 123, Email: "op@example.com", Role: "municipality_operator", Municipality: &muni}, nil
	}

	req := httptest.NewRequest("GET", "/api/v1/operator/verify?token=some-token", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/bikeadmin" {
		t.Errorf("expected redirect to /bikeadmin, got %s", loc)
	}

	// Check if cookie is set
	cookies := rec.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == operatorCookieName {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected operator session cookie to be set")
	}
}

func TestUnsubscribeHandler_Success(t *testing.T) {
	app, router := newAdminTestServer(t)

	var capturedOpID int
	app.adminSetUnsubscribeRequested = func(ctx context.Context, operatorID int) error {
		capturedOpID = operatorID
		return nil
	}

	// Generate a valid unsubscribe token
	token, _ := app.generateUnsubscribeURL(123)
	// token is "http://.../api/v1/unsubscribe?token=..."
	parts := strings.Split(token, "token=")
	tokenString := parts[1]

	req := httptest.NewRequest("GET", "/api/v1/unsubscribe?token="+tokenString, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	if capturedOpID != 123 {
		t.Errorf("expected opID 123, got %d", capturedOpID)
	}
	if !strings.Contains(rec.Body.String(), "Afmelding ontvangen") {
		t.Error("expected Dutch confirmation message in body")
	}
}

// Mock generateUnsubscribeURL for package level access (assuming it is exported via App)
// Wait, generateUnsubscribeURL is attached to App.
// But we are calling it via app.
