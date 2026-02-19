package main

import (
	"errors"
	"testing"
)

const (
	testDatabaseURL   = "postgres://zwerffiets:changeme@127.0.0.1:5432/zwerffiets?sslmode=disable"
	testSigningSecret = "0123456789abcdef"
)

func setupRequiredConfigEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", testDatabaseURL)
	t.Setenv("APP_SIGNING_SECRET", testSigningSecret)
	t.Setenv("BOOTSTRAP_OPERATOR_ROLE", "admin")
	t.Setenv("MAX_LOCATION_ACCURACY_M", "")
}

func TestLoadConfigUsesDefaultMaxLocationAccuracy(t *testing.T) {
	setupRequiredConfigEnv(t)

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("expected config to load: %v", err)
	}

	if cfg.MaxLocationAccuracyM != 3000 {
		t.Fatalf("expected default max accuracy 3000, got %v", cfg.MaxLocationAccuracyM)
	}
}

func TestLoadConfigUsesConfiguredMaxLocationAccuracy(t *testing.T) {
	setupRequiredConfigEnv(t)
	t.Setenv("MAX_LOCATION_ACCURACY_M", "1234.5")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("expected config to load: %v", err)
	}

	if cfg.MaxLocationAccuracyM != 1234.5 {
		t.Fatalf("expected max accuracy 1234.5, got %v", cfg.MaxLocationAccuracyM)
	}
}

func TestLoadConfigRejectsNegativeMaxLocationAccuracy(t *testing.T) {
	setupRequiredConfigEnv(t)
	t.Setenv("MAX_LOCATION_ACCURACY_M", "-1")

	_, err := loadConfig()
	if err == nil {
		t.Fatal("expected error for negative max accuracy")
	}
}

func TestValidateReportCreatePayloadRespectsConfiguredAccuracyThreshold(t *testing.T) {
	payload := ReportCreatePayload{
		Location: ReportLocation{
			Lat:       52.3676,
			Lng:       4.9041,
			AccuracyM: 2500,
		},
		Tags: []string{"flat_tires"},
	}

	if err := validateReportCreatePayload(payload, 3000); err != nil {
		t.Fatalf("expected payload to be accepted at configured threshold: %v", err)
	}

	err := validateReportCreatePayload(payload, 1000)
	if err == nil {
		t.Fatal("expected payload to be rejected above configured threshold")
	}

	var apiErr *apiError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected apiError, got %T", err)
	}
	if apiErr.Code != "invalid_location" {
		t.Fatalf("expected invalid_location error code, got %s", apiErr.Code)
	}
}
