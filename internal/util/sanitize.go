package util

import (
	"path/filepath"
	"regexp"
	"strings"
)

var weirdChars = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

// SanitizeFilename replaces characters outside [a-zA-Z0-9._-] with replacement.
// The extension is preserved.
func SanitizeFilename(filename, replacement string) string {
	if replacement == "" {
		replacement = "_"
	}
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)
	clean := weirdChars.ReplaceAllString(name, replacement)
	return clean + ext
}
