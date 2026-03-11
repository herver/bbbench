package main

import (
	"testing"

	"bbbench/internal/config"
)

// ── countJobs ────────────────────────────────────────────────────────────────

func TestCountJobs_AllActive(t *testing.T) {
	wdef := config.WorkloadDef{
		Process: map[string]any{"read": 4, "write": 4},
	}
	if n := countJobs(wdef); n != 2 {
		t.Errorf("countJobs = %d, want 2", n)
	}
}

func TestCountJobs_ZeroExcluded(t *testing.T) {
	// Process entries with count=0 must not be counted.
	wdef := config.WorkloadDef{
		Process: map[string]any{"read": 4, "write": 0},
	}
	if n := countJobs(wdef); n != 1 {
		t.Errorf("countJobs = %d, want 1", n)
	}
}

func TestCountJobs_NonIntExcluded(t *testing.T) {
	// Non-integer values (e.g. string "auto") must not be counted.
	wdef := config.WorkloadDef{
		Process: map[string]any{"read": 4, "mode": "auto"},
	}
	if n := countJobs(wdef); n != 1 {
		t.Errorf("countJobs = %d, want 1", n)
	}
}

func TestCountJobs_Empty(t *testing.T) {
	wdef := config.WorkloadDef{}
	if n := countJobs(wdef); n != 0 {
		t.Errorf("countJobs = %d, want 0", n)
	}
}

// ── warnings ─────────────────────────────────────────────────────────────────

var sampleDB = map[string]config.WorkloadDef{
	"seq_write": {Process: map[string]any{"write": 4}},
	"rand_read": {Process: map[string]any{"read": 4}},
}

func TestWarnings_NoWorkloads(t *testing.T) {
	ws := warnings("hdd", nil, sampleDB)
	if len(ws) != 1 {
		t.Fatalf("want 1 warning, got %d: %v", len(ws), ws)
	}
	if ws[0] != "no workloads defined for hdd" {
		t.Errorf("unexpected warning: %q", ws[0])
	}
}

func TestWarnings_EmptyWorkloadSlice(t *testing.T) {
	ws := warnings("ssd", []string{}, sampleDB)
	if len(ws) != 1 {
		t.Fatalf("want 1 warning, got %d: %v", len(ws), ws)
	}
}

func TestWarnings_AllKnown(t *testing.T) {
	ws := warnings("hdd", []string{"seq_write", "rand_read"}, sampleDB)
	if len(ws) != 0 {
		t.Errorf("want no warnings, got %v", ws)
	}
}

func TestWarnings_UnknownWorkload(t *testing.T) {
	ws := warnings("ssd", []string{"seq_write", "missing_workload"}, sampleDB)
	if len(ws) != 1 {
		t.Fatalf("want 1 warning, got %d: %v", len(ws), ws)
	}
	want := `workload "missing_workload" not found in workloads database`
	if ws[0] != want {
		t.Errorf("got %q, want %q", ws[0], want)
	}
}

func TestWarnings_AllUnknown(t *testing.T) {
	ws := warnings("ssd", []string{"a", "b"}, sampleDB)
	if len(ws) != 2 {
		t.Errorf("want 2 warnings, got %d: %v", len(ws), ws)
	}
}

// ── config.LoadBytes (edge cases exercised by check-config) ──────────────────

func TestLoadBytes_MissingWorkloadsDB(t *testing.T) {
	yaml := `
bbbench:
  workloads:
    hdd: [seq_write]
`
	_, err := config.LoadBytes([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing workloadsdb, got nil")
	}
}

func TestLoadBytes_MissingWorkloads(t *testing.T) {
	yaml := `
bbbench:
  workloadsdb:
    seq_write:
      blocksize: "128k"
      duration: 600
      process:
        write: 4
`
	_, err := config.LoadBytes([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing workloads, got nil")
	}
}

func TestLoadBytes_InvalidYAML(t *testing.T) {
	_, err := config.LoadBytes([]byte(":\t:bad yaml"))
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestLoadBytes_WorkloadDefFields(t *testing.T) {
	yaml := `
bbbench:
  workloadsdb:
    seq_write:
      blocksize: "128k"
      fulldisk: true
      duration: 600
      write_log: true
      random_distribution: "zipf:1.2"
      process:
        write: 4
        read: 0
  workloads:
    hdd: [seq_write]
`
	cfg, err := config.LoadBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadBytes: %v", err)
	}
	wdef, ok := cfg.BBBench.WorkloadsDB["seq_write"]
	if !ok {
		t.Fatal("seq_write not found in workloadsdb")
	}
	if wdef.Blocksize != "128k" {
		t.Errorf("Blocksize = %q, want %q", wdef.Blocksize, "128k")
	}
	if !wdef.Fulldisk {
		t.Error("Fulldisk = false, want true")
	}
	if wdef.Duration != 600 {
		t.Errorf("Duration = %d, want 600", wdef.Duration)
	}
	if !wdef.WriteLog {
		t.Error("WriteLog = false, want true")
	}
	if wdef.RandomDistribution != "zipf:1.2" {
		t.Errorf("RandomDistribution = %q, want %q", wdef.RandomDistribution, "zipf:1.2")
	}
	if n := countJobs(wdef); n != 1 {
		t.Errorf("countJobs(seq_write) = %d, want 1 (write=4, read=0)", n)
	}
}

func TestLoadBytes_MultipleWorkloadTypes(t *testing.T) {
	yaml := `
bbbench:
  workloadsdb:
    seq_write: {blocksize: "128k", duration: 600, process: {write: 4}}
    rand_read:  {blocksize: "4k",   duration: 300, process: {read: 8}}
  workloads:
    hdd: [seq_write]
    ssd: [seq_write, rand_read]
`
	cfg, err := config.LoadBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadBytes: %v", err)
	}
	if len(cfg.BBBench.WorkloadsDB) != 2 {
		t.Errorf("WorkloadsDB len = %d, want 2", len(cfg.BBBench.WorkloadsDB))
	}
	if len(cfg.BBBench.Workloads["ssd"]) != 2 {
		t.Errorf("ssd workloads len = %d, want 2", len(cfg.BBBench.Workloads["ssd"]))
	}
	// No warnings: all referenced names are in the DB.
	for typ, wl := range cfg.BBBench.Workloads {
		if ws := warnings(typ, wl, cfg.BBBench.WorkloadsDB); len(ws) != 0 {
			t.Errorf("unexpected warnings for %s: %v", typ, ws)
		}
	}
}
