package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMinimalConfig(t *testing.T) {
	yaml := `fio:
  generated:
    path: "~/.bbbench"

bbbench:
  wce: false
  default:
    template:
      path: "./templates"
      filename: "disk.fio.tmpl"
    template_fioplot:
      filename: "output.yml.tmpl"
  workloadsdb:
    hdd_seqread:
      blocksize: "4k"
      duration: 600
      write_log: true
      process:
        read: 1
        write: 0
  workloads:
    hdd: [hdd_seqread]
`
	dir := t.TempDir()
	p := filepath.Join(dir, "cfg.yml")
	if err := os.WriteFile(p, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Fio.Generated.Path != "~/.bbbench" {
		t.Fatalf("unexpected fio path: %q", cfg.Fio.Generated.Path)
	}
	if cfg.BBBench.WorkloadsDB["hdd_seqread"].Process["read"] != 1 {
		t.Fatalf("unexpected process count")
	}
}
