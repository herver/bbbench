package templating

import (
	"io/fs"
	"os"

	distassets "bbbench/dist"
)

// DefaultEngine returns an Engine parsed from embedded dist templates.
func DefaultEngine() (*Engine, error) {
	return NewFromFS(distassets.FS, "templates")
}

// EngineFromTemplatesDir parses templates from a directory that contains *.gotmpl and includes/.
func EngineFromTemplatesDir(dir string) (*Engine, error) {
	return NewFromFS(os.DirFS(dir), ".")
}

// EngineFromRootDir parses templates from a root directory that contains templates/ and templates/includes/.
func EngineFromRootDir(root string) (*Engine, error) {
	sub := os.DirFS(root)
	// Here, baseDir is "templates" because templates are under templates/.
	return NewFromFS(sub, "templates")
}

// SubFS safely returns an fs.FS rooted under subdir.
func SubFS(fsys fs.FS, subdir string) (fs.FS, error) {
	return fs.Sub(fsys, subdir)
}
