package dmi

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type Reader interface {
	ReadDir(name string) ([]fs.DirEntry, error)
	ReadFile(name string) ([]byte, error)
}

type OSReader struct{}

func (OSReader) ReadDir(name string) ([]fs.DirEntry, error) { return os.ReadDir(name) }
func (OSReader) ReadFile(name string) ([]byte, error)       { return os.ReadFile(name) }

type Info struct {
	BasePath string
	data     map[string]string
}

func Load(r Reader, basePath string) (*Info, error) {
	if basePath == "" {
		basePath = "/sys/class/dmi/id"
	}
	ents, err := r.ReadDir(basePath)
	if err != nil {
		return nil, err
	}
	data := map[string]string{}
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		p := filepath.Join(basePath, e.Name())
		b, err := r.ReadFile(p)
		if err != nil {
			// best-effort: some entries are root-only
			continue
		}
		data[e.Name()] = strings.TrimSpace(string(b))
	}
	return &Info{BasePath: basePath, data: data}, nil
}

func (i *Info) Get(key string) (string, error) {
	v, ok := i.data[key]
	if !ok {
		return "", fmt.Errorf("dmi key %q not found", key)
	}
	return v, nil
}

func (i *Info) ChassisSerial() (string, error) {
	return i.Get("chassis_serial")
}
