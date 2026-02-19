package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveExistingPhotoStoragePath_ExactPath(t *testing.T) {
	dataRoot := t.TempDir()
	app := &App{cfg: &Config{DataRoot: dataRoot}}

	relPath := filepath.ToSlash(filepath.Join("uploads", "reports", "10001", "10001.jpg"))
	fullPath := filepath.Join(dataRoot, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	got, err := app.resolveExistingPhotoStoragePath(relPath)
	if err != nil {
		t.Fatalf("resolveExistingPhotoStoragePath returned error: %v", err)
	}
	if got != relPath {
		t.Fatalf("expected %q, got %q", relPath, got)
	}
}

func TestResolveExistingPhotoStoragePath_FallbackToAnyExtension(t *testing.T) {
	dataRoot := t.TempDir()
	app := &App{cfg: &Config{DataRoot: dataRoot}}

	relPathNoExt := filepath.ToSlash(filepath.Join("uploads", "reports", "10004", "10007"))
	fullPathWithExt := filepath.Join(dataRoot, filepath.FromSlash(relPathNoExt+".webp"))
	if err := os.MkdirAll(filepath.Dir(fullPathWithExt), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(fullPathWithExt, []byte("x"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	got, err := app.resolveExistingPhotoStoragePath(relPathNoExt)
	if err != nil {
		t.Fatalf("resolveExistingPhotoStoragePath returned error: %v", err)
	}
	expected := relPathNoExt + ".webp"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestBuildOperatorMediaInternalPath(t *testing.T) {
	got := buildOperatorMediaInternalPath(filepath.Join("uploads", "reports", "10014", "10028.jpg"))
	want := "/_protected_media/uploads/reports/10014/10028.jpg"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestShouldUseInternalMediaRedirect(t *testing.T) {
	productionApp := &App{cfg: &Config{Env: "production"}}
	if !productionApp.shouldUseInternalMediaRedirect() {
		t.Fatalf("expected production app to use internal media redirect")
	}

	developmentApp := &App{cfg: &Config{Env: "development"}}
	if developmentApp.shouldUseInternalMediaRedirect() {
		t.Fatalf("expected development app to stream media directly")
	}
}
