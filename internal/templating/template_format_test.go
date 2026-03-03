package templating

import (
	"strings"
	"testing"

	"bbbench/internal/config"
)

// TestFioTemplateJobHeadersOnSeparateLines ensures that job section headers
// appear on their own lines and are not concatenated with comment lines.
// This is a regression test for a bug where {{- ... -}} stripped newlines
// causing [job-name] to be concatenated with ### comment lines.
func TestFioTemplateJobHeadersOnSeparateLines(t *testing.T) {
	eng, err := DefaultEngine()
	if err != nil {
		t.Fatalf("Failed to load templates: %v", err)
	}

	// Sample context similar to what generate.go provides
	workloadsDB := map[string]config.WorkloadDef{
		"test_workload": {
			Blocksize: "4k",
			Duration:  600,
			Fulldisk:  false,
			WriteLog:  true,
			Process: map[string]any{
				"read": 1,
			},
		},
	}

	ctx := map[string]any{
		"Manufacturer": "TestVendor",
		"WorkloadsDB":  workloadsDB,
		"Workloads":    []string{"test_workload"},
		"Disk": map[string]any{
			"path":        "/dev/sda",
			"model":       "TestModel",
			"serial":      "TestSerial",
			"rotational":  false,
			"capacity":    1000000000000,
			"capacity_gb": 931,
			"block_count": 1953525168,
			"block_size":  512,
			"vendor":      "TestVendor",
		},
		"Device": "sda",
		"Serial": "TestSerial",
		"Host":   "testhost",
		"WCE":    nil,
	}

	output, err := eng.Render("criteo_disk.fio", ctx)
	if err != nil {
		t.Fatalf("Failed to render template: %v", err)
	}

	lines := strings.Split(output, "\n")

	// Check that job headers are on their own lines
	foundJobHeader := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Look for job section headers
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") && trimmed != "[global]" {
			foundJobHeader = true

			// The line should ONLY contain the job header
			if trimmed != line {
				t.Errorf("Line %d: Job header has leading whitespace: %q", i+1, line)
			}

			// Check that it's not concatenated with previous content
			if strings.Contains(line, "#") {
				t.Errorf("Line %d: Job header concatenated with comment: %q", i+1, line)
			}

			// Job header should be the entire line
			if !strings.HasPrefix(line, "[") {
				t.Errorf("Line %d: Job header doesn't start line: %q", i+1, line)
			}
		}
	}

	if !foundJobHeader {
		t.Error("No job section headers found in output")
	}
}

// TestFioTemplateGeneratesValidJobSections ensures the template generates
// valid fio job sections that can be parsed.
func TestFioTemplateGeneratesValidJobSections(t *testing.T) {
	eng, err := DefaultEngine()
	if err != nil {
		t.Fatalf("Failed to load templates: %v", err)
	}

	workloadsDB := map[string]config.WorkloadDef{
		"workload1": {
			Blocksize: "4k",
			Duration:  600,
			Fulldisk:  false,
			WriteLog:  false,
			Process: map[string]any{
				"read":  1,
				"write": 0,
			},
		},
		"workload2": {
			Blocksize: "1M",
			Duration:  300,
			Fulldisk:  true,
			WriteLog:  false,
			Process: map[string]any{
				"randread":  5,
				"randwrite": 3,
			},
		},
	}

	ctx := map[string]any{
		"Manufacturer": "Vendor",
		"WorkloadsDB":  workloadsDB,
		"Workloads":    []string{"workload1", "workload2"},
		"Disk": map[string]any{
			"path":        "/dev/nvme0n1",
			"model":       "Model",
			"serial":      "Serial",
			"rotational":  false,
			"capacity":    1000000000000,
			"capacity_gb": 931,
			"block_count": 1953525168,
			"block_size":  512,
			"vendor":      "Vendor",
		},
		"Device": "nvme0n1",
		"Serial": "Serial",
		"Host":   "host",
		"WCE":    nil,
	}

	output, err := eng.Render("criteo_disk.fio", ctx)
	if err != nil {
		t.Fatalf("Failed to render template: %v", err)
	}

	// Count job sections
	jobCount := 0
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") && trimmed != "[global]" {
			jobCount++

			// Verify job name format
			if !strings.Contains(trimmed, "nvme0n1") {
				t.Errorf("Job header missing device name: %s", trimmed)
			}
		}
	}

	// Should have 3 jobs: workload1 has 1 process (read), workload2 has 2 processes (randread, randwrite)
	expectedJobs := 3
	if jobCount != expectedJobs {
		t.Errorf("Expected %d job sections, got %d", expectedJobs, jobCount)
	}

	// Verify global section exists
	if !strings.Contains(output, "[global]") {
		t.Error("Missing [global] section")
	}
}

// TestFioTemplateNoJobsWithZeroCount ensures jobs with count=0 are not generated.
func TestFioTemplateNoJobsWithZeroCount(t *testing.T) {
	eng, err := DefaultEngine()
	if err != nil {
		t.Fatalf("Failed to load templates: %v", err)
	}

	workloadsDB := map[string]config.WorkloadDef{
		"workload_zero": {
			Blocksize: "4k",
			Duration:  600,
			Fulldisk:  false,
			WriteLog:  false,
			Process: map[string]any{
				"read":  0, // Zero count - should not generate job
				"write": 0, // Zero count - should not generate job
			},
		},
	}

	ctx := map[string]any{
		"Manufacturer": "Vendor",
		"WorkloadsDB":  workloadsDB,
		"Workloads":    []string{"workload_zero"},
		"Disk": map[string]any{
			"path":   "/dev/sda",
			"model":  "Model",
			"serial": "Serial",
			"vendor": "Vendor",
		},
		"Device": "sda",
		"Serial": "Serial",
		"Host":   "host",
		"WCE":    nil,
	}

	output, err := eng.Render("criteo_disk.fio", ctx)
	if err != nil {
		t.Fatalf("Failed to render template: %v", err)
	}

	// Should only have [global] section, no job sections
	lines := strings.Split(output, "\n")
	jobCount := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") && trimmed != "[global]" {
			jobCount++
		}
	}

	if jobCount != 0 {
		t.Errorf("Expected 0 job sections with zero counts, got %d", jobCount)
	}
}
