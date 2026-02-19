package main

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestTrackingTokenFromRequest_PrefersQueryToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/reports/ABC123/status?token=query-token", nil)
	c.Request.Header.Set("Authorization", "Bearer header-token")

	token := trackingTokenFromRequest(c)
	if token != "query-token" {
		t.Fatalf("expected query token, got %q", token)
	}
}

func TestTrackingTokenFromRequest_UsesBearerHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/reports/ABC123/status", nil)
	c.Request.Header.Set("Authorization", "Bearer header-token")

	token := trackingTokenFromRequest(c)
	if token != "header-token" {
		t.Fatalf("expected bearer token, got %q", token)
	}
}
