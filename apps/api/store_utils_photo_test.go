package main

import (
	"strings"
	"testing"
)

func TestGeneratePhotoStorageFileName(t *testing.T) {
	first, err := generatePhotoStorageFileName(".jpg")
	if err != nil {
		t.Fatalf("generatePhotoStorageFileName returned error: %v", err)
	}
	second, err := generatePhotoStorageFileName(".jpg")
	if err != nil {
		t.Fatalf("generatePhotoStorageFileName returned error: %v", err)
	}

	if !strings.HasSuffix(first, ".jpg") {
		t.Fatalf("expected .jpg suffix in %q", first)
	}
	if first == second {
		t.Fatalf("expected unique random filenames, got duplicate %q", first)
	}
}
