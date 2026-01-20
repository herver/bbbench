package templating

import (
	"bytes"
	"fmt"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

// Engine wraps a parsed template set.
//
// baseDir is either "templates" (when the FS root contains templates/ and templates/includes/)
// or "." (when the FS root IS the templates directory containing *.gotmpl and includes/).
type Engine struct {
	tpl      *template.Template
	includes map[string]bool
	baseDir  string
}

// NewFromFS creates a templating engine from an fs.FS.
// baseDir must be "templates" or ".".
func NewFromFS(tfs fs.FS, baseDir string) (*Engine, error) {
	files, includes, err := listTemplates(tfs, baseDir)
	if err != nil {
		return nil, err
	}

	funcs := sprig.TxtFuncMap()
	// include renders a template by name (which can be dynamic) and returns the result.
	// Go's built-in {{template "name" .}} action does not accept a variable name.
	var root *template.Template
	funcs["include"] = func(name string, data any) (string, error) {
		if root == nil {
			return "", fmt.Errorf("include: templates not initialized")
		}
		var b bytes.Buffer
		if err := root.ExecuteTemplate(&b, name, data); err != nil {
			return "", err
		}
		return b.String(), nil
	}

	// deref returns the value a pointer points to (or the value itself if not a pointer).
	// Useful when templates receive optional pointer fields (e.g. *bool) and need a concrete value.
	funcs["deref"] = func(v any) any {
		if v == nil {
			return nil
		}
		rv := reflect.ValueOf(v)
		for rv.IsValid() && rv.Kind() == reflect.Pointer {
			if rv.IsNil() {
				return nil
			}
			rv = rv.Elem()
		}
		if !rv.IsValid() {
			return nil
		}
		return rv.Interface()
	}
	funcs["includeName"] = func(processType string) string {
		// ParseFS names templates by *base filename*.
		// So "includes/trim.gotmpl" becomes template name "trim.gotmpl".
		name := processType + ".gotmpl"
		if includes[name] {
			return name
		}
		return "default.gotmpl"
	}
	funcs["includeOutputName"] = func() string {
		return "default_output.gotmpl"
	}

	tpl, err := template.New("bbbench").Funcs(funcs).ParseFS(tfs, files...)
	if err != nil {
		return nil, err
	}
	root = tpl
	return &Engine{tpl: tpl, includes: includes, baseDir: baseDir}, nil
}

// Render renders a named template (without extension).
func (e *Engine) Render(templateName string, data any) (string, error) {
	// ParseFS names templates by base filename, regardless of subdirectories.
	name := templateName + ".gotmpl"
	var b strings.Builder
	if err := e.tpl.ExecuteTemplate(&b, name, data); err != nil {
		return "", err
	}
	return b.String(), nil
}

func listTemplates(tfs fs.FS, baseDir string) ([]string, map[string]bool, error) {
	root := baseDir
	includesRoot := filepath.Join(baseDir, "includes")
	if baseDir == "." {
		root = "."
		includesRoot = "includes"
	}

	var out []string
	includes := map[string]bool{}

	walk := func(dir string, markIncludes bool) error {
		ents, err := fs.ReadDir(tfs, dir)
		if err != nil {
			return err
		}
		for _, e := range ents {
			if e.IsDir() {
				continue
			}
			if strings.HasSuffix(e.Name(), ".gotmpl") {
				p := filepath.ToSlash(filepath.Join(dir, e.Name()))
				out = append(out, p)
				if markIncludes {
					// ParseFS registers templates by base filename.
					// Store both the relative path and the base name for lookups.
					includes[p] = true
					includes[filepath.Base(p)] = true
				}
			}
		}
		return nil
	}

	// Root templates
	if err := walk(root, false); err != nil {
		return nil, nil, fmt.Errorf("list templates: %w", err)
	}
	// Includes
	if err := walk(includesRoot, true); err != nil {
		return nil, nil, fmt.Errorf("list includes: %w", err)
	}

	return out, includes, nil
}
