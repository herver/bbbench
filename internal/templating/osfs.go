package templating

import (
	"io/fs"
	"os"
)

// osDirFS wraps a real directory as an fs.FS.
// We keep this helper to avoid exposing os.DirFS from this package API.
func osDirFS(dir string) fs.FS {
	return os.DirFS(dir)
}
