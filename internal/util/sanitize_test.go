package util

import "testing"

func TestSanitizeFilenamePreservesExtension(t *testing.T) {
	got := SanitizeFilename("host#1_model?.fio", "_")
	want := "host_1_model_.fio"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSanitizeFilenameDefaultReplacement(t *testing.T) {
	got := SanitizeFilename("a b", "")
	want := "a_b"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
