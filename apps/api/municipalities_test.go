package main

import (
	"sort"
	"testing"
)

func TestLookupMunicipalityKnownPlace(t *testing.T) {
	got := lookupMunicipality("IJmuiden")
	if got != "Velsen" {
		t.Fatalf("expected Velsen for IJmuiden, got %s", got)
	}
}

func TestLookupMunicipalityIsMunicipality(t *testing.T) {
	got := lookupMunicipality("Amsterdam")
	if got != "Amsterdam" {
		t.Fatalf("expected Amsterdam for Amsterdam, got %s", got)
	}
}

func TestLookupMunicipalityUnknown(t *testing.T) {
	got := lookupMunicipality("SomeRandomPlace")
	if got != "SomeRandomPlace" {
		t.Fatalf("expected fallback SomeRandomPlace, got %s", got)
	}
}

func TestLookupMunicipalityEmpty(t *testing.T) {
	got := lookupMunicipality("")
	if got != "" {
		t.Fatalf("expected empty string for empty input, got %s", got)
	}
}

func TestIsValidMunicipalityKnown(t *testing.T) {
	if !isValidMunicipality("Amsterdam") {
		t.Fatal("expected Amsterdam to be a valid municipality")
	}
}

func TestIsValidMunicipalityPlace(t *testing.T) {
	if isValidMunicipality("IJmuiden") {
		t.Fatal("expected IJmuiden NOT to be a valid municipality")
	}
}

func TestIsValidMunicipalityEmpty(t *testing.T) {
	if isValidMunicipality("") {
		t.Fatal("expected empty string NOT to be a valid municipality")
	}
}

func TestIsValidMunicipalityCaseInsensitive(t *testing.T) {
	if !isValidMunicipality("amsterdam") {
		t.Fatal("expected lowercase amsterdam to be valid")
	}
	if !isValidMunicipality("AMSTERDAM") {
		t.Fatal("expected uppercase AMSTERDAM to be valid")
	}
}

func TestMunicipalityListSorted(t *testing.T) {
	list := municipalityList()
	if !sort.StringsAreSorted(list) {
		t.Fatal("expected municipality list to be sorted alphabetically")
	}
}

func TestMunicipalityListIsCopy(t *testing.T) {
	list1 := municipalityList()
	list2 := municipalityList()
	if len(list1) == 0 {
		t.Fatal("expected non-empty municipality list")
	}
	list1[0] = "modified"
	if list2[0] == "modified" {
		t.Fatal("expected municipalityList to return independent copies")
	}
}

func TestLookupMunicipalitySantpoortNoord(t *testing.T) {
	got := lookupMunicipality("Santpoort-Noord")
	if got != "Velsen" {
		t.Fatalf("expected Velsen for Santpoort-Noord, got %s", got)
	}
}
