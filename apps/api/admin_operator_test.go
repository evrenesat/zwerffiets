package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestAdminOperatorListPage_AdminAccess(t *testing.T) {
	app, router := newAdminTestServer(t)

	// Mock list operators
	app.adminListOperators = func(ctx context.Context) ([]Operator, error) {
		return []Operator{
			{ID: 1, Email: "op1@example.com", Role: "municipality_operator", IsActive: true},
		}, nil
	}
	// Mock auth as admin
	app.adminAuthenticateOperator = func(ctx context.Context, email, password string) (string, *string, error) {
		return "admin", nil, nil
	}

	req := authenticatedRequest(t, app, "GET", "/bikeadmin/operators", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "op1@example.com") {
		t.Errorf("expected operator email in body")
	}
}

func TestAdminOperatorCreateSubmit_Success(t *testing.T) {
	app, router := newAdminTestServer(t)

	created := false
	app.adminCreateOperator = func(ctx context.Context, email, name, password string, municipality *string) error {
		if email != "new@example.com" {
			return errors.New("wrong email")
		}
		if name != "New Operator" {
			return errors.New("wrong name")
		}
		if municipality == nil || *municipality != "Amsterdam" {
			return errors.New("wrong municipality")
		}
		created = true
		return nil
	}
	app.adminAuthenticateOperator = func(ctx context.Context, email, password string) (string, *string, error) {
		return "admin", nil, nil
	}

	form := url.Values{}
	form.Set("email", "new@example.com")
	form.Set("name", "New Operator")
	form.Set("password", "secret")
	form.Set("municipality", "Amsterdam")

	req := authenticatedRequest(t, app, "POST", "/bikeadmin/operators", form.Encode())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", rec.Code)
	}
	if !created {
		t.Error("adminCreateOperator mock was not called correctly")
	}
}

func TestAdminOperatorToggle_Success(t *testing.T) {
	app, router := newAdminTestServer(t)

	toggled := false
	app.adminToggleOperatorStatus = func(ctx context.Context, id int) (bool, error) {
		if id != 123 {
			return false, errors.New("wrong id")
		}
		toggled = true
		return false, nil
	}
	app.adminAuthenticateOperator = func(ctx context.Context, email, password string) (string, *string, error) {
		return "admin", nil, nil
	}

	req := authenticatedRequest(t, app, "POST", "/bikeadmin/operators/123/toggle", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", rec.Code)
	}
	if !toggled {
		t.Error("adminToggleOperatorStatus mock was not called")
	}
}

func TestAdminOperatorToggleReports_Success(t *testing.T) {
	app, router := newAdminTestServer(t)

	toggled := false
	app.adminToggleReceivesReports = func(ctx context.Context, id int) (bool, error) {
		if id != 123 {
			return false, errors.New("wrong id")
		}
		toggled = true
		return true, nil
	}
	app.adminAuthenticateOperator = func(ctx context.Context, email, password string) (string, *string, error) {
		return "admin", nil, nil
	}

	req := authenticatedRequest(t, app, "POST", "/bikeadmin/operators/123/toggle-reports", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", rec.Code)
	}
	if !toggled {
		t.Error("adminToggleReceivesReports mock was not called")
	}
}

func TestAdminOperatorEditPage_RendersForm(t *testing.T) {
	app, router := newAdminTestServer(t)

	muni := "Amsterdam"
	app.adminGetOperatorByID = func(ctx context.Context, id int) (*Operator, error) {
		if id != 123 {
			return nil, errors.New("wrong id")
		}
		return &Operator{
			ID:           123,
			Email:        "op@example.com",
			Role:         "municipality_operator",
			Municipality: &muni,
			IsActive:     true,
		}, nil
	}
	app.adminAuthenticateOperator = func(ctx context.Context, email, password string) (string, *string, error) {
		return "admin", nil, nil
	}

	req := authenticatedRequest(t, app, "GET", "/bikeadmin/operators/123/edit", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "op@example.com") {
		t.Errorf("expected operator email in body")
	}
	if !strings.Contains(body, "Amsterdam") {
		t.Errorf("expected municipality in body")
	}
	if !strings.Contains(body, `value="municipality_operator" selected`) {
		t.Errorf("expected role to be pre-selected")
	}
}

func TestAdminOperatorEditSubmit_Success(t *testing.T) {
	app, router := newAdminTestServer(t)

	muni := "Utrecht"
	var capturedID int
	var capturedEmail, capturedName, capturedRole, capturedPassword string
	var capturedMuni *string
	app.adminUpdateOperator = func(ctx context.Context, id int, email, name, role string, municipality *string, password string) error {
		capturedID = id
		capturedEmail = email
		capturedName = name
		capturedRole = role
		capturedMuni = municipality
		capturedPassword = password
		return nil
	}
	app.adminGetOperatorByID = func(ctx context.Context, id int) (*Operator, error) {
		return &Operator{ID: id, Email: "old@example.com", Role: "municipality_operator", Municipality: &muni}, nil
	}
	app.adminAuthenticateOperator = func(ctx context.Context, email, password string) (string, *string, error) {
		return "admin", nil, nil
	}

	form := url.Values{}
	form.Set("email", "new@example.com")
	form.Set("name", "Updated Name")
	form.Set("role", "municipality_operator")
	form.Set("municipality", "Amsterdam")
	form.Set("password", "")

	req := authenticatedRequest(t, app, "POST", "/bikeadmin/operators/123/edit", form.Encode())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "/bikeadmin/operators") {
		t.Errorf("expected redirect to operators list, got %s", loc)
	}
	if capturedID != 123 {
		t.Errorf("wrong id: %d", capturedID)
	}
	if capturedEmail != "new@example.com" {
		t.Errorf("wrong email: %s", capturedEmail)
	}
	if capturedName != "Updated Name" {
		t.Errorf("wrong name: %s", capturedName)
	}
	if capturedRole != "municipality_operator" {
		t.Errorf("wrong role: %s", capturedRole)
	}
	if capturedMuni == nil || *capturedMuni != "Amsterdam" {
		t.Errorf("wrong municipality: %v", capturedMuni)
	}
	if capturedPassword != "" {
		t.Errorf("expected empty password, got non-empty")
	}
}

func TestAdminOperatorEditSubmit_PasswordChange(t *testing.T) {
	app, router := newAdminTestServer(t)

	muni := "Utrecht"
	var capturedPassword string
	app.adminUpdateOperator = func(ctx context.Context, id int, email, name, role string, municipality *string, password string) error {
		capturedPassword = password
		return nil
	}
	app.adminGetOperatorByID = func(ctx context.Context, id int) (*Operator, error) {
		return &Operator{ID: id, Email: "old@example.com", Role: "municipality_operator", Municipality: &muni}, nil
	}
	app.adminAuthenticateOperator = func(ctx context.Context, email, password string) (string, *string, error) {
		return "admin", nil, nil
	}

	form := url.Values{}
	form.Set("email", "old@example.com")
	form.Set("name", "Some Name")
	form.Set("role", "municipality_operator")
	form.Set("municipality", "Utrecht")
	form.Set("password", "newpassword123")

	req := authenticatedRequest(t, app, "POST", "/bikeadmin/operators/123/edit", form.Encode())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", rec.Code)
	}
	if capturedPassword != "newpassword123" {
		t.Errorf("expected password to be passed through, got %q", capturedPassword)
	}
}

func TestAdminOperatorEditSubmit_InvalidMunicipality(t *testing.T) {
	app, router := newAdminTestServer(t)

	updateCalled := false
	app.adminUpdateOperator = func(ctx context.Context, id int, email, name, role string, municipality *string, password string) error {
		updateCalled = true
		return nil
	}
	app.adminGetOperatorByID = func(ctx context.Context, id int) (*Operator, error) {
		return &Operator{ID: id, Email: "op@example.com", Role: "municipality_operator"}, nil
	}
	app.adminAuthenticateOperator = func(ctx context.Context, email, password string) (string, *string, error) {
		return "admin", nil, nil
	}

	form := url.Values{}
	form.Set("email", "op@example.com")
	form.Set("name", "Some Name")
	form.Set("role", "municipality_operator")
	form.Set("municipality", "NotARealMunicipality")
	form.Set("password", "")

	req := authenticatedRequest(t, app, "POST", "/bikeadmin/operators/123/edit", form.Encode())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "error=") {
		t.Errorf("expected error in redirect location, got %s", loc)
	}
	if updateCalled {
		t.Error("adminUpdateOperator should not have been called")
	}
}
