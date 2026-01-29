package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFioFile(t *testing.T) {
	// Create a temporary fio file for testing
	content := `### Generated file do not edit

[global]
ioengine=libaio
invalidate=1
direct=1

[001-sda-workload1-read]
stonewall
rw=read
bs=4k
numjobs=1

[002-sda-workload1-write]
rw=write
bs=4k
numjobs=1
wait_for=001-sda-workload1-read

[003-sda-workload2-read]
stonewall
rw=randread
bs=64k
numjobs=2

[004-sda-workload2-write]
stonewall
rw=randwrite
bs=64k
numjobs=2
`

	tmpFile, err := os.CreateTemp("", "test-*.fio")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	tmpFile.Close()

	// Parse the file
	workload, err := parseFioFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("parse fio file: %v", err)
	}

	// Verify global section
	if workload.GlobalSection == "" {
		t.Error("expected global section to be non-empty")
	}

	// Verify jobs
	expectedJobs := 4
	if len(workload.Jobs) != expectedJobs {
		t.Errorf("expected %d jobs, got %d", expectedJobs, len(workload.Jobs))
	}

	// Verify job names
	expectedNames := []string{
		"001-sda-workload1-read",
		"002-sda-workload1-write",
		"003-sda-workload2-read",
		"004-sda-workload2-write",
	}
	for i, expected := range expectedNames {
		if i >= len(workload.Jobs) {
			break
		}
		if workload.Jobs[i].Name != expected {
			t.Errorf("job %d: expected name %q, got %q", i, expected, workload.Jobs[i].Name)
		}
	}

	// Verify stonewall flags
	expectedStonewalls := []bool{true, false, true, true}
	for i, expected := range expectedStonewalls {
		if i >= len(workload.Jobs) {
			break
		}
		if workload.Jobs[i].IsStonewall != expected {
			t.Errorf("job %d: expected stonewall=%v, got %v", i, expected, workload.Jobs[i].IsStonewall)
		}
	}

	// Verify wait_for
	if workload.Jobs[1].WaitFor != "001-sda-workload1-read" {
		t.Errorf("job 1: expected wait_for=%q, got %q", "001-sda-workload1-read", workload.Jobs[1].WaitFor)
	}

	// Verify phases
	// Expected phases:
	// Phase 0: jobs 0-1 (first stonewall group)
	// Phase 1: job 2 (second stonewall)
	// Phase 2: job 3 (third stonewall)
	expectedPhases := 3
	if len(workload.Phases) != expectedPhases {
		t.Errorf("expected %d phases, got %d", expectedPhases, len(workload.Phases))
	}
}

func TestBuildPhaseFioFile(t *testing.T) {
	workload := &FioWorkload{
		GlobalSection: "[global]\nioengine=libaio\n",
		Jobs: []FioJob{
			{
				Name:    "job1",
				Section: "[job1]\nrw=read\n",
				Index:   0,
			},
			{
				Name:    "job2",
				Section: "[job2]\nrw=write\n",
				Index:   1,
			},
		},
	}

	phase := []FioJob{workload.Jobs[0]}
	content := buildPhaseFioFile(workload, phase)

	// Verify global section is included
	if content == "" {
		t.Error("expected non-empty fio file content")
	}

	// Just basic smoke test - verify it contains key parts
	if !containsString(content, "[global]") {
		t.Error("expected global section in output")
	}
	if !containsString(content, "[job1]") {
		t.Error("expected job1 section in output")
	}
	if containsString(content, "[job2]") {
		t.Error("did not expect job2 section in output")
	}
}

func containsString(s, substr string) bool {
	return filepath.Base(s) != s || len(s) >= len(substr) && (s == substr || len(s) > 0 && hasSubstring(s, substr))
}

func hasSubstring(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
