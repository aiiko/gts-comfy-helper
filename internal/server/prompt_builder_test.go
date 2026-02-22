package server

import "testing"

func TestBuildFinalPromptOrder(t *testing.T) {
	got := buildFinalPrompt(
		"best quality",
		"1girl, a giantess girl, 1 tiny",
		"1girl in city",
		"anime screencap",
		"upper body",
		"from above",
	)
	want := "best quality, 1girl, a giantess girl, 1 tiny, 1girl in city, anime screencap, upper body, from above"
	if got != want {
		t.Fatalf("buildFinalPrompt order mismatch:\nwant: %q\ngot:  %q", want, got)
	}
}

func TestBuildFinalPromptSkipsEmptyOptionals(t *testing.T) {
	got := buildFinalPrompt(
		"best quality",
		"1girl, a giantess girl, 1 tiny",
		"1girl in city",
		"",
		"",
		"",
	)
	want := "best quality, 1girl, a giantess girl, 1 tiny, 1girl in city"
	if got != want {
		t.Fatalf("buildFinalPrompt empty optionals mismatch:\nwant: %q\ngot:  %q", want, got)
	}
}

func TestBuildCharacterDefinitionCountMode(t *testing.T) {
	got, err := buildCharacterDefinition(1, "count", 2, "female", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "1girl, a giantess girl, 2 female tinies"
	if got != want {
		t.Fatalf("character definition mismatch:\nwant: %q\ngot:  %q", want, got)
	}
}

func TestBuildCharacterDefinitionCountModeWithDescriptor(t *testing.T) {
	got, err := buildCharacterDefinition(2, "count", 10, "", " white ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "2girls, two giantess girls, 10 white tinies"
	if got != want {
		t.Fatalf("character definition mismatch:\nwant: %q\ngot:  %q", want, got)
	}
}

func TestBuildCharacterDefinitionGroupMode(t *testing.T) {
	got, err := buildCharacterDefinition(1, "group", 1, "male", "white")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "1girl, a giantess girl, a group of white tinies"
	if got != want {
		t.Fatalf("character definition mismatch:\nwant: %q\ngot:  %q", want, got)
	}
}

func TestBuildCharacterDefinitionRejectsInvalidInputs(t *testing.T) {
	if _, err := buildCharacterDefinition(3, "count", 1, "", ""); err == nil {
		t.Fatalf("expected error for invalid giantess count")
	}
	if _, err := buildCharacterDefinition(1, "count", 0, "", ""); err == nil {
		t.Fatalf("expected error for invalid tiny count")
	}
	if _, err := buildCharacterDefinition(1, "invalid", 1, "", ""); err == nil {
		t.Fatalf("expected error for invalid tinies mode")
	}
}
