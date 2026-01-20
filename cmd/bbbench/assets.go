package main

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultConfigName = "default.yml"
)

func expandUser(p string) string {
	if p == "" {
		return p
	}
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		if p == "~" {
			return home
		}
		if strings.HasPrefix(p, "~/") {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

func resolveDistDir(flagValue string) string {
	if flagValue != "" {
		return expandUser(flagValue)
	}
	if v := os.Getenv("BBBENCH_DIST_DIR"); v != "" {
		return expandUser(v)
	}
	return "./dist"
}

// systemConfigRoot is separated for testability.
// In production it should remain "/etc/bbbench".
var systemConfigRoot = "/etc/bbbench"

// Search order:
//  1. ~/.bbbench/config
//  2. /etc/bbbench
//  3. <distDir> (default ./dist)
func searchRoots(distDir string) []string {
	home, err := os.UserHomeDir()
	var userRoot string
	if err == nil {
		userRoot = filepath.Join(home, ".bbbench", "config")
	}
	roots := []string{}
	if userRoot != "" {
		roots = append(roots, userRoot)
	}
	roots = append(roots, systemConfigRoot)
	roots = append(roots, distDir)
	return roots
}

func firstExistingFile(candidates []string) string {
	for _, p := range candidates {
		st, err := os.Stat(p)
		if err == nil && st.Mode().IsRegular() {
			return p
		}
	}
	return ""
}

func firstExistingDir(candidates []string) string {
	for _, p := range candidates {
		st, err := os.Stat(p)
		if err == nil && st.IsDir() {
			return p
		}
	}
	return ""
}

func resolveConfigPath(distDir string) string {
	roots := searchRoots(distDir)
	cands := make([]string, 0, len(roots))
	for _, r := range roots {
		cands = append(cands, filepath.Join(r, defaultConfigName))
	}
	return firstExistingFile(cands)
}

// resolveTemplates chooses a templates directory on disk.
// It looks for either:
//   - <root>/templates (preferred)
//   - <root> itself if it appears to be a templates directory (has includes/ and *.gotmpl)
func resolveTemplatesDir(distDir string) (dir string, baseDir string) {
	roots := searchRoots(distDir)

	cands := make([]string, 0, len(roots))
	for _, r := range roots {
		cands = append(cands, filepath.Join(r, "templates"))
	}
	if d := firstExistingDir(cands); d != "" {
		return d, "." // FS root is templates dir
	}

	// If someone points distDir at a templates dir itself, accept it.
	if isTemplatesDir(distDir) {
		return distDir, "."
	}
	return "", ""
}

func isTemplatesDir(dir string) bool {
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		return false
	}
	inc, err := os.Stat(filepath.Join(dir, "includes"))
	if err != nil || !inc.IsDir() {
		return false
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".gotmpl") {
			return true
		}
	}
	return false
}
