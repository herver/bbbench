package templating

import (
	"testing"
	"testing/fstest"
)

func TestIncludeRendersDynamicTemplateName(t *testing.T) {
	fs := fstest.MapFS{
		"main.gotmpl":             {Data: []byte("{{ $n := includeName \"trim\" }}{{ include $n . }}")},
		"includes/trim.gotmpl":    {Data: []byte("TRIM={{ .Val }}")},
		"includes/default.gotmpl": {Data: []byte("DEFAULT={{ .Val }}")},
	}
	eng, err := NewFromFS(fs, ".")
	if err != nil {
		t.Fatalf("NewFromFS: %v", err)
	}
	out, err := eng.Render("main", map[string]any{"Val": 42})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if out != "TRIM=42" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestIncludeNameFallsBackToDefault(t *testing.T) {
	fs := fstest.MapFS{
		"main.gotmpl":             {Data: []byte("{{ $n := includeName \"missing\" }}{{ include $n . }}")},
		"includes/default.gotmpl": {Data: []byte("DEFAULT")},
	}
	eng, err := NewFromFS(fs, ".")
	if err != nil {
		t.Fatalf("NewFromFS: %v", err)
	}
	out, err := eng.Render("main", map[string]any{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if out != "DEFAULT" {
		t.Fatalf("unexpected output: %q", out)
	}
}
