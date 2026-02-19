package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestAdminGenerateExportSubmit_PassesFilters(t *testing.T) {
	app, router := newAdminTestServer(t)

	var capturedInput map[string]any
	app.adminGenerateExport = func(ctx context.Context, input map[string]any, session OperatorSession) (*ExportBatch, error) {
		capturedInput = input
		return &ExportBatch{ID: 1}, nil
	}

	form := url.Values{}
	form.Set("period_type", "weekly")
	form.Set("status", "new")
	form.Set("municipality", "Amsterdam")

	rec := httptest.NewRecorder()
	req := authenticatedRequest(t, app, http.MethodPost, "/bikeadmin/exports/generate", form.Encode())
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect %d, got %d. Body: %s", http.StatusSeeOther, rec.Code, rec.Body.String())
	}

	if capturedInput["period_type"] != "weekly" {
		t.Errorf("expected period_type weekly, got %v", capturedInput["period_type"])
	}
	if capturedInput["status"] != "new" {
		t.Errorf("expected status new, got %v", capturedInput["status"])
	}
	if capturedInput["municipality"] != "Amsterdam" {
		t.Errorf("expected municipality Amsterdam, got %v", capturedInput["municipality"])
	}
}

func TestAdminGenerateExportSubmit_AllTime(t *testing.T) {
	app, router := newAdminTestServer(t)

	var capturedInput map[string]any
	app.adminGenerateExport = func(ctx context.Context, input map[string]any, session OperatorSession) (*ExportBatch, error) {
		capturedInput = input
		return &ExportBatch{ID: 2}, nil
	}

	form := url.Values{}
	form.Set("period_type", "all")

	rec := httptest.NewRecorder()
	req := authenticatedRequest(t, app, http.MethodPost, "/bikeadmin/exports/generate", form.Encode())
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect %d, got %d", http.StatusSeeOther, rec.Code)
	}

	if capturedInput["period_type"] != "all" {
		t.Errorf("expected period_type all, got %v", capturedInput["period_type"])
	}
}

func TestExportMunicipalityForSession_AdminKeepsRequestedFilter(t *testing.T) {
	adminMunicipality := "Amsterdam"
	session := OperatorSession{
		Email:        "admin@example.com",
		Role:         "admin",
		Municipality: &adminMunicipality,
	}

	resolved, err := exportMunicipalityForSession(session, "Utrecht")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resolved != "Utrecht" {
		t.Fatalf("expected requested municipality to be preserved for admin, got %q", resolved)
	}
}

func TestExportMunicipalityForSession_MunicipalityOperatorForcesScope(t *testing.T) {
	operatorMunicipality := "Rotterdam"
	session := OperatorSession{
		Email:        "operator@example.com",
		Role:         "municipality_operator",
		Municipality: &operatorMunicipality,
	}

	resolved, err := exportMunicipalityForSession(session, "Utrecht")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resolved != "Rotterdam" {
		t.Fatalf("expected municipality operator scope to be forced, got %q", resolved)
	}
}
