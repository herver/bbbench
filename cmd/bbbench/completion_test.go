package main

import (
	"strings"
	"testing"
)

func TestCompletionScriptsContainCommands(t *testing.T) {
	cmds := []string{"generate", "validate-templates", "doctor", "completion", "help"}
	for _, c := range cmds {
		if !containsAll(bashCompletion, c) {
			t.Fatalf("bash completion missing %q", c)
		}
		if !containsAll(zshCompletion, c) {
			t.Fatalf("zsh completion missing %q", c)
		}
		if !containsAll(fishCompletion, c) {
			t.Fatalf("fish completion missing %q", c)
		}
	}
}

func containsAll(s, substr string) bool { return strings.Contains(s, substr) }
