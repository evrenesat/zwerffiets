package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func TestCORSMiddleware_AllowsConfiguredOrigins(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app := &App{cfg: &Config{Env: "development", PublicBaseURL: "https://zwerffiets.org"}}

	router := gin.New()
	router.Use(app.corsMiddleware())
	router.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	tests := []string{
		"https://zwerffiets.org",
		devCORSOriginLocalhost,
		devCORSOriginLoopback,
	}

	for _, origin := range tests {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.Header.Set("Origin", origin)
		router.ServeHTTP(rec, req)

		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != origin {
			t.Fatalf("expected allow origin %q, got %q", origin, got)
		}
		if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
			t.Fatalf("expected credentials header true, got %q", got)
		}
	}
}

func TestCORSMiddleware_BlocksUnlistedOrigins(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app := &App{cfg: &Config{Env: "production", PublicBaseURL: "https://zwerffiets.org"}}

	router := gin.New()
	router.Use(app.corsMiddleware())
	router.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("Origin", "https://evil.example")
	router.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no allow-origin header, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("expected no credentials header, got %q", got)
	}
}

func TestPruneRateLimiterState_RemovesExpiredBuckets(t *testing.T) {
	now := time.Now().UTC()
	stale := now.Add(-reportRateLimitWindow)
	recent := now.Add(-time.Minute)

	app := &App{
		rateBuckets: map[string]rateBucket{
			"stale":  {start: stale, count: 8},
			"recent": {start: recent, count: 2},
		},
		fingerprints: map[string]fingerprintBucket{
			"stale":  {start: stale, count: 4},
			"recent": {start: recent, count: 1},
		},
	}

	app.pruneRateLimiterState(now)

	if _, ok := app.rateBuckets["stale"]; ok {
		t.Fatal("expected stale rate bucket to be pruned")
	}
	if _, ok := app.fingerprints["stale"]; ok {
		t.Fatal("expected stale fingerprint bucket to be pruned")
	}
	if _, ok := app.rateBuckets["recent"]; !ok {
		t.Fatal("expected recent rate bucket to remain")
	}
	if _, ok := app.fingerprints["recent"]; !ok {
		t.Fatal("expected recent fingerprint bucket to remain")
	}
}

func TestVerifyUserSessionToken_InvalidUserIDClaimRejected(t *testing.T) {
	app := &App{cfg: &Config{AppSigningSecret: "0123456789abcdef"}}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": map[string]any{"bad": "shape"},
		"email":   "user@example.com",
		"iat":     time.Now().Unix(),
		"exp":     time.Now().Add(time.Hour).Unix(),
	})
	tokenString, err := token.SignedString([]byte(app.cfg.AppSigningSecret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	_, verifyErr := app.verifyUserSessionToken(tokenString)
	if verifyErr == nil {
		t.Fatal("expected invalid user_id claim to be rejected")
	}
}
