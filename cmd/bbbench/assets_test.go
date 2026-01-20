package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveConfigPathPrefersUserThenSystemThenDist(t *testing.T) {
	temp := t.TempDir()
	// Isolate HOME so ~/.bbbench/config resolves into temp.
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", temp)
	t.Cleanup(func() {
		os.Setenv("HOME", oldHome)
	})

	// Isolate /etc/bbbench by overriding systemConfigRoot.
	oldSystem := systemConfigRoot
	systemConfigRoot = filepath.Join(temp, "etc")
	t.Cleanup(func() { systemConfigRoot = oldSystem })

	dist := filepath.Join(temp, "dist")
	userCfgDir := filepath.Join(temp, ".bbbench", "config")
	sysCfgDir := systemConfigRoot

	// Create all three, with distinct content.
	if err := os.MkdirAll(userCfgDir, 0o755); err != nil {
		t.Fatalf("mkdir user: %v", err)
	}
	if err := os.MkdirAll(sysCfgDir, 0o755); err != nil {
		t.Fatalf("mkdir system: %v", err)
	}
	if err := os.MkdirAll(dist, 0o755); err != nil {
		t.Fatalf("mkdir dist: %v", err)
	}

	userCfg := filepath.Join(userCfgDir, "default.yml")
	sysCfg := filepath.Join(sysCfgDir, "default.yml")
	distCfg := filepath.Join(dist, "default.yml")

	os.WriteFile(distCfg, []byte("dist"), 0o644)
	os.WriteFile(sysCfg, []byte("system"), 0o644)
	os.WriteFile(userCfg, []byte("user"), 0o644)

	if got := resolveConfigPath(dist); got != userCfg {
		t.Fatalf("expected user config %q, got %q", userCfg, got)
	}

	// Remove user, system should win.
	os.Remove(userCfg)
	if got := resolveConfigPath(dist); got != sysCfg {
		t.Fatalf("expected system config %q, got %q", sysCfg, got)
	}

	// Remove system, dist should win.
	os.Remove(sysCfg)
	if got := resolveConfigPath(dist); got != distCfg {
		t.Fatalf("expected dist config %q, got %q", distCfg, got)
	}
}

func TestResolveTemplatesDirPrefersRootTemplatesFolder(t *testing.T) {
	temp := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", temp)
	t.Cleanup(func() { os.Setenv("HOME", oldHome) })

	oldSystem := systemConfigRoot
	systemConfigRoot = filepath.Join(temp, "etc")
	t.Cleanup(func() { systemConfigRoot = oldSystem })

	dist := filepath.Join(temp, "dist")
	userTpl := filepath.Join(temp, ".bbbench", "config", "templates")
	if err := os.MkdirAll(filepath.Join(userTpl, "includes"), 0o755); err != nil {
		t.Fatalf("mkdir user templates: %v", err)
	}
	if err := os.MkdirAll(dist, 0o755); err != nil {
		t.Fatalf("mkdir dist: %v", err)
	}
	// Touch a template so it looks like a templates dir.
	os.WriteFile(filepath.Join(userTpl, "x.gotmpl"), []byte("x"), 0o644)

	dir, base := resolveTemplatesDir(dist)
	if dir != userTpl || base != "." {
		t.Fatalf("expected %q base '.'; got dir=%q base=%q", userTpl, dir, base)
	}
}
