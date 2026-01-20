package templating

import (
	"strings"
	"testing"

	"bbbench/internal/config"
)

func TestRenderFioTemplate(t *testing.T) {
	eng, err := DefaultEngine()
	if err != nil {
		t.Fatalf("default engine: %v", err)
	}
	wdb := map[string]config.WorkloadDef{
		"hdd_seqread": {Blocksize: "4k", Duration: 60, WriteLog: true, Process: map[string]any{"read": 1, "write": 0}},
	}
	ctx := map[string]any{
		"Manufacturer": "ACME",
		"WorkloadsDB":  wdb,
		"Workloads":    []string{"hdd_seqread"},
		"Disk":         map[string]any{"path": "/dev/sda", "model": "X", "capacity": 1000},
		"Device":       "sda",
		"Serial":       "S1",
		"Host":         "host1",
	}
	out, err := eng.Render("criteo_disk.fio", ctx)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out, "[001-sda-hdd_seqread-read]") {
		t.Fatalf("missing job section:\n%s", out)
	}
	if !strings.Contains(out, "filename=/dev/sda") {
		t.Fatalf("missing filename")
	}
}
