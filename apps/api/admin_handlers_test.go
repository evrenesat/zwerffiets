package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func newAdminTestServer(t *testing.T) (*App, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	app := &App{
		cfg: &Config{
			Env:              "test",
			AppSigningSecret: "0123456789abcdef",
		},
		log:            slog.New(slog.NewTextHandler(io.Discard, nil)),
		adminTemplates: newAdminTemplateRenderer("test"),
	}
	app.adminListReportCities = func(ctx context.Context, municipality *string) ([]string, error) {
		return []string{}, nil
	}

	router := gin.New()
	app.registerAdminRoutes(router)
	api := router.Group("/api/v1")
	{
		api.GET("/operator/verify", app.verifyOperatorMagicLinkHandler)
		api.GET("/unsubscribe", app.unsubscribeHandler)
	}
	return app, router
}

func authenticatedRequest(t *testing.T, app *App, method string, target string, body string) *http.Request {
	return authenticatedRequestWithSession(
		t,
		app,
		method,
		target,
		body,
		OperatorSession{Email: "operator@example.com", Role: "admin"},
	)
}

func authenticatedRequestWithSession(t *testing.T, app *App, method string, target string, body string, session OperatorSession) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("content-type", "application/x-www-form-urlencoded")
	}
	token, err := app.createOperatorSessionToken(session)
	if err != nil {
		t.Fatalf("create session token: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: operatorCookieName, Value: token, Path: "/"})
	return req
}

func findResponseCookie(response *http.Response, name string) *http.Cookie {
	for _, cookie := range response.Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

func TestAdminLoginSubmitSuccessSetsCookieAndRedirects(t *testing.T) {
	app, router := newAdminTestServer(t)
	app.adminAuthenticateOperator = func(ctx context.Context, email, password string) (string, *string, error) {
		if email != "operator@example.com" || password != "secret" {
			t.Fatalf("unexpected credentials: %s / %s", email, password)
		}
		return "admin", nil, nil
	}

	form := url.Values{}
	form.Set("email", "operator@example.com")
	form.Set("password", "secret")
	form.Set("next", "/bikeadmin/map")
	form.Set("language", "en")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/bikeadmin/login", strings.NewReader(form.Encode()))
	req.Header.Set("content-type", "application/x-www-form-urlencoded")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected %d, got %d", http.StatusSeeOther, rec.Code)
	}
	if location := rec.Header().Get("Location"); location != "/bikeadmin/map" {
		t.Fatalf("unexpected redirect location: %s", location)
	}

	resp := rec.Result()
	operatorCookie := findResponseCookie(resp, operatorCookieName)
	if operatorCookie == nil {
		t.Fatal("expected operator session cookie")
	}
	if operatorCookie.Value == "" {
		t.Fatal("expected operator cookie value")
	}
}

func TestAdminLoginSubmitInvalidCredentialsRendersError(t *testing.T) {
	app, router := newAdminTestServer(t)
	app.adminAuthenticateOperator = func(ctx context.Context, email, password string) (string, *string, error) {
		return "", nil, &apiError{Status: http.StatusUnauthorized, Code: "invalid_credentials", Message: "Invalid credentials"}
	}

	form := url.Values{}
	form.Set("email", "operator@example.com")
	form.Set("password", "wrong")
	form.Set("next", "/bikeadmin")
	form.Set("language", "en")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/bikeadmin/login", strings.NewReader(form.Encode()))
	req.Header.Set("content-type", "application/x-www-form-urlencoded")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Invalid credentials") {
		t.Fatalf("expected invalid credential message, got body: %s", rec.Body.String())
	}
	resp := rec.Result()
	if findResponseCookie(resp, operatorCookieName) != nil {
		t.Fatal("did not expect operator cookie on failed login")
	}
}

func TestAdminLogoutClearsSessionCookie(t *testing.T) {
	app, router := newAdminTestServer(t)

	rec := httptest.NewRecorder()
	req := authenticatedRequest(t, app, http.MethodPost, "/bikeadmin/logout", "")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected %d, got %d", http.StatusSeeOther, rec.Code)
	}
	if location := rec.Header().Get("Location"); location != "/bikeadmin/login" {
		t.Fatalf("unexpected redirect location: %s", location)
	}

	cookie := findResponseCookie(rec.Result(), operatorCookieName)
	if cookie == nil {
		t.Fatal("expected operator cookie clear instruction")
	}
	if cookie.MaxAge >= 0 {
		t.Fatalf("expected cookie max age to be negative, got %d", cookie.MaxAge)
	}
}

func TestAdminProtectedRouteRedirectsWithoutSession(t *testing.T) {
	_, router := newAdminTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/bikeadmin?status=new", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected %d, got %d", http.StatusSeeOther, rec.Code)
	}
	location := rec.Header().Get("Location")
	if !strings.HasPrefix(location, "/bikeadmin/login?next=") {
		t.Fatalf("unexpected redirect location: %s", location)
	}
}

func TestAdminTriageParsesFiltersAndRenders(t *testing.T) {
	app, router := newAdminTestServer(t)
	capturedFilters := map[string]any{}
	app.adminListPaginatedReports = func(ctx context.Context, filters map[string]any, page, pageSize int) (*PaginatedOperatorReports, error) {
		for key, value := range filters {
			capturedFilters[key] = value
		}
		return &PaginatedOperatorReports{
			Reports:     []OperatorReportView{},
			TotalCount:  0,
			TotalPages:  1,
			CurrentPage: page,
			PageSize:    pageSize,
		}, nil
	}

	rec := httptest.NewRecorder()
	req := authenticatedRequest(
		t,
		app,
		http.MethodGet,
		"/bikeadmin?city=Rotterdam&status=new&signal_strength=none&has_qualifying_reconfirmation=false&strong_only=true&sort=signal",
		"",
	)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	if capturedFilters["status"] != "new" {
		t.Fatalf("expected status filter new, got %#v", capturedFilters["status"])
	}
	if capturedFilters["report_city"] != "Rotterdam" {
		t.Fatalf("expected report_city filter Rotterdam, got %#v", capturedFilters["report_city"])
	}
	if capturedFilters["signal_strength"] != "none" {
		t.Fatalf("expected signal filter none, got %#v", capturedFilters["signal_strength"])
	}
	if capturedFilters["has_qualifying_reconfirmation"] != false {
		t.Fatalf("expected has_qualifying_reconfirmation false, got %#v", capturedFilters["has_qualifying_reconfirmation"])
	}
	if capturedFilters["strong_only"] != true {
		t.Fatalf("expected strong_only true, got %#v", capturedFilters["strong_only"])
	}
	if capturedFilters["sort"] != "signal" {
		t.Fatalf("expected sort signal, got %#v", capturedFilters["sort"])
	}
	if !strings.Contains(rec.Body.String(), "name=\"status\"") {
		t.Fatal("expected triage filters in response body")
	}
	if !strings.Contains(rec.Body.String(), "name=\"city\"") {
		t.Fatal("expected city filter in response body")
	}
}

func TestAdminListCachedReportCities_UsesTTL(t *testing.T) {
	app := &App{
		cityFilterCache: make(map[string]cityFilterCacheEntry),
	}
	calls := 0
	app.adminListReportCities = func(ctx context.Context, municipality *string) ([]string, error) {
		calls++
		return []string{"Rotterdam", "Amsterdam", "amsterdam", ""}, nil
	}

	session := OperatorSession{Email: "admin@example.com", Role: "admin"}
	first, err := app.adminListCachedReportCities(context.Background(), session)
	if err != nil {
		t.Fatalf("first cache read failed: %v", err)
	}
	if len(first) != 2 || first[0] != "Amsterdam" || first[1] != "Rotterdam" {
		t.Fatalf("unexpected first city list: %#v", first)
	}

	second, err := app.adminListCachedReportCities(context.Background(), session)
	if err != nil {
		t.Fatalf("second cache read failed: %v", err)
	}
	if len(second) != 2 || second[0] != "Amsterdam" || second[1] != "Rotterdam" {
		t.Fatalf("unexpected second city list: %#v", second)
	}
	if calls != 1 {
		t.Fatalf("expected one fetch before ttl expiry, got %d", calls)
	}

	app.cityFilterMu.Lock()
	entry := app.cityFilterCache["admin"]
	entry.expiresAt = time.Now().Add(-time.Second)
	app.cityFilterCache["admin"] = entry
	app.cityFilterMu.Unlock()

	_, err = app.adminListCachedReportCities(context.Background(), session)
	if err != nil {
		t.Fatalf("cache read after ttl failed: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected cache refresh after ttl expiry, got %d fetches", calls)
	}
}

func TestAdminTriage_AdminWithMunicipalityIsNotImplicitlyScoped(t *testing.T) {
	app, router := newAdminTestServer(t)
	capturedFilters := map[string]any{}
	app.adminListPaginatedReports = func(ctx context.Context, filters map[string]any, page, pageSize int) (*PaginatedOperatorReports, error) {
		for key, value := range filters {
			capturedFilters[key] = value
		}
		return &PaginatedOperatorReports{
			Reports:     []OperatorReportView{},
			TotalCount:  0,
			TotalPages:  1,
			CurrentPage: page,
			PageSize:    pageSize,
		}, nil
	}

	municipality := "Amsterdam"
	session := OperatorSession{Email: "admin@example.com", Role: "admin", Municipality: &municipality}
	rec := httptest.NewRecorder()
	req := authenticatedRequestWithSession(t, app, http.MethodGet, "/bikeadmin", "", session)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	if _, exists := capturedFilters["city"]; exists {
		t.Fatalf("admin session should not be implicitly scoped by municipality, got city=%#v", capturedFilters["city"])
	}
}

func TestAdminMap_AdminWithMunicipalityIsNotImplicitlyScoped(t *testing.T) {
	app, router := newAdminTestServer(t)
	capturedFilters := map[string]any{}
	app.adminListOperatorReports = func(ctx context.Context, filters map[string]any) ([]OperatorReportView, error) {
		for key, value := range filters {
			capturedFilters[key] = value
		}
		return []OperatorReportView{}, nil
	}

	municipality := "Amsterdam"
	session := OperatorSession{Email: "admin@example.com", Role: "admin", Municipality: &municipality}
	rec := httptest.NewRecorder()
	req := authenticatedRequestWithSession(t, app, http.MethodGet, "/bikeadmin/map", "", session)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	if _, exists := capturedFilters["city"]; exists {
		t.Fatalf("admin map view should not be implicitly scoped by municipality, got city=%#v", capturedFilters["city"])
	}
}

func TestAdminStatusAndMergeSubmitsRedirectWithMessages(t *testing.T) {
	app, router := newAdminTestServer(t)

	var capturedStatus string
	app.adminUpdateReportStatus = func(ctx context.Context, reportID int, status string, session OperatorSession) (*Report, error) {
		if reportID != 1 {
			t.Fatalf("unexpected report id: %d", reportID)
		}
		capturedStatus = status
		return &Report{ID: reportID, Status: status}, nil
	}

	var capturedDuplicates []int
	app.adminMergeDuplicates = func(ctx context.Context, canonicalReportID int, duplicateReportIDs []int, session OperatorSession) (*DedupeGroup, error) {
		capturedDuplicates = duplicateReportIDs
		return &DedupeGroup{ID: 100, CanonicalReportID: canonicalReportID, MergedReportIDs: duplicateReportIDs}, nil
	}

	statusForm := url.Values{}
	statusForm.Set("status", "triaged")
	statusForm.Set("next", "/bikeadmin?sort=signal")

	statusRec := httptest.NewRecorder()
	statusReq := authenticatedRequest(t, app, http.MethodPost, "/bikeadmin/reports/1/status", statusForm.Encode())
	router.ServeHTTP(statusRec, statusReq)

	if statusRec.Code != http.StatusSeeOther {
		t.Fatalf("expected status submit redirect %d, got %d", http.StatusSeeOther, statusRec.Code)
	}
	if capturedStatus != "triaged" {
		t.Fatalf("expected captured status triaged, got %s", capturedStatus)
	}
	statusLocation := statusRec.Header().Get("Location")
	if !strings.HasPrefix(statusLocation, "/bikeadmin?") || !strings.Contains(statusLocation, "notice=") {
		t.Fatalf("expected notice redirect, got %s", statusLocation)
	}

	mergeForm := url.Values{}
	mergeForm.Set("duplicate_report_ids", "2, 3, 2")
	mergeForm.Set("next", "/bikeadmin/reports/1")

	mergeRec := httptest.NewRecorder()
	mergeReq := authenticatedRequest(t, app, http.MethodPost, "/bikeadmin/reports/1/merge", mergeForm.Encode())
	router.ServeHTTP(mergeRec, mergeReq)

	if mergeRec.Code != http.StatusSeeOther {
		t.Fatalf("expected merge submit redirect %d, got %d", http.StatusSeeOther, mergeRec.Code)
	}
	if len(capturedDuplicates) != 2 || capturedDuplicates[0] != 2 || capturedDuplicates[1] != 3 {
		t.Fatalf("unexpected duplicate ids: %#v", capturedDuplicates)
	}
	mergeLocation := mergeRec.Header().Get("Location")
	if !strings.HasPrefix(mergeLocation, "/bikeadmin/reports/1?") || !strings.Contains(mergeLocation, "notice=") {
		t.Fatalf("expected notice redirect, got %s", mergeLocation)
	}
}

func TestAdminBulkStatusSubmit_MunicipalityOperatorCrossMunicipalityRejected(t *testing.T) {
	app, router := newAdminTestServer(t)
	called := 0
	app.adminUpdateReportStatus = func(ctx context.Context, reportID int, status string, session OperatorSession) (*Report, error) {
		called++
		return &Report{ID: reportID, Status: status}, nil
	}
	app.adminGetReportByID = func(ctx context.Context, reportID int) (*Report, error) {
		reportMunicipality := "Rotterdam"
		return &Report{ID: reportID, Municipality: &reportMunicipality}, nil
	}

	operatorMunicipality := "Amsterdam"
	session := OperatorSession{
		Email:        "operator@example.com",
		Role:         "municipality_operator",
		Municipality: &operatorMunicipality,
	}

	form := url.Values{}
	form.Set("status", "triaged")
	form.Set("next", "/bikeadmin")
	form.Add("report_ids", "99")

	rec := httptest.NewRecorder()
	req := authenticatedRequestWithSession(t, app, http.MethodPost, "/bikeadmin/reports/bulk-status", form.Encode(), session)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect %d, got %d", http.StatusSeeOther, rec.Code)
	}
	if called != 0 {
		t.Fatalf("expected no status update calls for cross-municipality report, got %d", called)
	}
	if location := rec.Header().Get("Location"); !strings.Contains(location, "error=") {
		t.Fatalf("expected error redirect, got %s", location)
	}
}

func TestAdminLanguageSelectionCookieAndFallback(t *testing.T) {
	_, router := newAdminTestServer(t)

	englishForm := url.Values{}
	englishForm.Set("language", "en")
	englishForm.Set("next", "/bikeadmin/map")
	englishRec := httptest.NewRecorder()
	englishReq := httptest.NewRequest(http.MethodPost, "/bikeadmin/language", strings.NewReader(englishForm.Encode()))
	englishReq.Header.Set("content-type", "application/x-www-form-urlencoded")
	router.ServeHTTP(englishRec, englishReq)

	if englishRec.Code != http.StatusSeeOther {
		t.Fatalf("expected %d, got %d", http.StatusSeeOther, englishRec.Code)
	}
	if englishRec.Header().Get("Location") != "/bikeadmin/map" {
		t.Fatalf("unexpected redirect location: %s", englishRec.Header().Get("Location"))
	}
	englishCookie := findResponseCookie(englishRec.Result(), adminLanguageCookieName)
	if englishCookie == nil || englishCookie.Value != "en" {
		t.Fatalf("expected language cookie en, got %#v", englishCookie)
	}

	invalidForm := url.Values{}
	invalidForm.Set("language", "de")
	invalidForm.Set("next", "/bikeadmin")
	invalidRec := httptest.NewRecorder()
	invalidReq := httptest.NewRequest(http.MethodPost, "/bikeadmin/language", strings.NewReader(invalidForm.Encode()))
	invalidReq.Header.Set("content-type", "application/x-www-form-urlencoded")
	router.ServeHTTP(invalidRec, invalidReq)

	invalidCookie := findResponseCookie(invalidRec.Result(), adminLanguageCookieName)
	if invalidCookie == nil || invalidCookie.Value != adminDefaultLanguage {
		t.Fatalf("expected fallback language cookie %s, got %#v", adminDefaultLanguage, invalidCookie)
	}
}

func TestAdminReportDetailRendersWithMapData(t *testing.T) {
	app, router := newAdminTestServer(t)
	app.adminGetReportDetails = func(ctx context.Context, reportID int) (*OperatorReportDetails, error) {
		return &OperatorReportDetails{
			Report: Report{
				ID:       1,
				PublicID: "PUB-1",
				Status:   "new",
				Location: ReportLocation{Lat: 52.3676, Lng: 4.9041},
			},
			Photos: []OperatorReportPhotoView{},
			SignalDetails: SignalDetails{
				SignalSummary: ReportSignalSummary{},
				BikeGroup:     BikeGroup{ID: 10},
			},
		}, nil
	}

	rec := httptest.NewRecorder()
	req := authenticatedRequest(t, app, http.MethodGet, "/bikeadmin/reports/1", "")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "data-lat=\"52.3676\"") {
		t.Fatal("expected data-lat attribute in body")
	}
	if !strings.Contains(body, "data-lng=\"4.9041\"") {
		t.Fatal("expected data-lng attribute in body")
	}
	if !strings.Contains(body, "id=\"detail-map\"") {
		t.Fatal("expected detail-map div in body")
	}
	// Check if MapLibre links are present because base.IncludeMapLibre was set
	if !strings.Contains(body, "maplibre-gl.js") {
		t.Fatal("expected maplibre-gl.js script tag in body")
	}
}

func TestAdminTagLabelLocalization(t *testing.T) {
	// Test English translation
	enLabel := adminTagLabel("en", "flat_tires")
	if enLabel != "Flat tires" {
		t.Errorf("expected 'Flat tires', got '%s'", enLabel)
	}

	// Test Dutch translation
	nlLabel := adminTagLabel("nl", "flat_tires")
	if nlLabel != "Lekke banden" {
		t.Errorf("expected 'Lekke banden', got '%s'", nlLabel)
	}

	// Test fallback to default (Dutch) for unknown language
	unknownLabel := adminTagLabel("fr", "flat_tires")
	if unknownLabel != "Lekke banden" {
		t.Errorf("expected fallback 'Lekke banden', got '%s'", unknownLabel)
	}

	// Test fallback to key for unknown tag
	unknownTag := adminTagLabel("en", "unknown_tag")
	if unknownTag != "unknown_tag" {
		t.Errorf("expected 'unknown_tag', got '%s'", unknownTag)
	}
}

func TestCheckMunicipalityScope_Bypass(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	app := &App{}

	// Case 1: Operator with NO municipality (simulating the vulnerability)
	// Theoretically this user should NOT be able to access a report from Amsterdam
	// But current logic might allow it if checkMunicipalityScope returns nil when session.Municipality is nil
	t.Run("MunicipalityOperator_WithNilMunicipality_ShouldBeDenied", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		// Simulate authenticated session
		// Role is municipality_operator, but Municipality is nil (the bug state)
		session := OperatorSession{
			Email:        "bad_actor@operator.com",
			Role:         "municipality_operator",
			Municipality: nil,
		}
		c.Set("operatorSession", session)

		reportMunicipality := "Amsterdam"

		// The function under test
		err := app.checkMunicipalityScope(c, &reportMunicipality)

		// ASSERTION: We EXPECT this to fail (return error)
		// If it returns nil, the vulnerability exists
		if err == nil {
			t.Fatal("Vulnerability confirmed: municipality_operator with nil municipality bypassed scope check!")
		}

		// We expect a 403 Forbidden error
		apiErr, ok := err.(*apiError)
		if !ok {
			t.Fatalf("Expected apiError, got %T: %v", err, err)
		}
		if apiErr.Status != http.StatusForbidden {
			t.Errorf("Expected status 403, got %d", apiErr.Status)
		}
	})

	// Case 2: Happy path - Admin should bypass
	t.Run("Admin_ShouldBypass", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		session := OperatorSession{
			Email:        "admin@zwerffiets.org",
			Role:         "admin",
			Municipality: nil, // Admins don't need a municipality
		}
		c.Set("operatorSession", session)

		reportMunicipality := "Amsterdam"
		err := app.checkMunicipalityScope(c, &reportMunicipality)

		if err != nil {
			t.Errorf("Admin should be allowed to access any municipality, got error: %v", err)
		}
	})

	// Case 3: Happy path - Matching Municipality
	t.Run("MatchingMunicipality_ShouldAllow", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		muni := "Amsterdam"
		session := OperatorSession{
			Email:        "amsterdam@operator.com",
			Role:         "municipality_operator",
			Municipality: &muni,
		}
		c.Set("operatorSession", session)

		reportMunicipality := "Amsterdam"
		err := app.checkMunicipalityScope(c, &reportMunicipality)
		if err != nil {
			t.Errorf("Matching municipality should be allowed, got error: %v", err)
		}
	})

	// Case 4: Mismatching Municipality
	t.Run("MismatchingMunicipality_ShouldDeny", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		muni := "Rotterdam"
		session := OperatorSession{
			Email:        "rotterdam@operator.com",
			Role:         "municipality_operator",
			Municipality: &muni,
		}
		c.Set("operatorSession", session)

		reportMunicipality := "Amsterdam"
		err := app.checkMunicipalityScope(c, &reportMunicipality)

		if err == nil {
			t.Error("Mismatching municipality should be denied, got nil error")
		} else {
			apiErr, ok := err.(*apiError)
			if !ok {
				t.Errorf("Expected apiError, got %T: %v", err, err)
			} else if apiErr.Status != http.StatusForbidden {
				t.Errorf("Expected status 403, got %d", apiErr.Status)
			}
		}
	})
}
