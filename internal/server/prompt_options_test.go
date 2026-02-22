package server

import "testing"

func TestCanonicalOptionArtStyle(t *testing.T) {
	got, ok := canonicalOption("  Anime Screencap  ", artStyleOptionsMap)
	if !ok {
		t.Fatalf("expected valid art style")
	}
	if got != "anime screencap" {
		t.Fatalf("unexpected art style canonical value: %q", got)
	}
}

func TestCanonicalOptionBodyFraming(t *testing.T) {
	got, ok := canonicalOption("LOWER BODY", bodyFramingOptionsMap)
	if !ok {
		t.Fatalf("expected valid body framing")
	}
	if got != "lower body" {
		t.Fatalf("unexpected body framing canonical value: %q", got)
	}
}

func TestCanonicalOptionEmptyAllowed(t *testing.T) {
	got, ok := canonicalOption("", artStyleOptionsMap)
	if !ok {
		t.Fatalf("expected empty value to be allowed")
	}
	if got != "" {
		t.Fatalf("expected empty canonical value, got: %q", got)
	}
}

func TestCanonicalOptionRejectsUnknown(t *testing.T) {
	_, ok := canonicalOption("oil painting", artStyleOptionsMap)
	if ok {
		t.Fatalf("expected unknown option to be rejected")
	}
}

func TestCanonicalOptionCameraSelector(t *testing.T) {
	got, ok := canonicalOption("  FROM BELOW ", cameraSelectorOptionsMap)
	if !ok {
		t.Fatalf("expected valid camera selector")
	}
	if got != "from below" {
		t.Fatalf("unexpected camera selector canonical value: %q", got)
	}
}

func TestCanonicalOptionTiniesMode(t *testing.T) {
	got, ok := canonicalOption(" GROUP ", tiniesModeOptionsMap)
	if !ok {
		t.Fatalf("expected valid tinies mode")
	}
	if got != "group" {
		t.Fatalf("unexpected tinies mode canonical value: %q", got)
	}
}

func TestCanonicalOptionTinyGender(t *testing.T) {
	got, ok := canonicalOption(" Female ", tinyGenderOptionsMap)
	if !ok {
		t.Fatalf("expected valid tiny gender")
	}
	if got != "female" {
		t.Fatalf("unexpected tiny gender canonical value: %q", got)
	}
}
