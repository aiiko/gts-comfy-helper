package server

import "testing"

func TestBuildFinalPromptOrder(t *testing.T) {
	got := buildFinalPrompt(
		"best quality",
		"1girl in city",
		"anime screencap",
		"upper body",
		"from above",
	)
	want := "best quality, 1girl in city, anime screencap, upper body, from above"
	if got != want {
		t.Fatalf("buildFinalPrompt order mismatch:\nwant: %q\ngot:  %q", want, got)
	}
}

func TestBuildFinalPromptSkipsEmptyOptionals(t *testing.T) {
	got := buildFinalPrompt(
		"best quality",
		"1girl in city",
		"",
		"",
		"",
	)
	want := "best quality, 1girl in city"
	if got != want {
		t.Fatalf("buildFinalPrompt empty optionals mismatch:\nwant: %q\ngot:  %q", want, got)
	}
}
