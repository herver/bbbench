package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"bbbench/internal/blockdev"
)

func TestHandleExportHTML(t *testing.T) {
	globalResultsStore.Clear()

	// Add test result with comprehensive data
	result := &BenchmarkResult{
		ID:        "export-test-123",
		Timestamp: time.Now(),
		Mode:      "parallel-sync",
		OutputDir: "/tmp/test",
		Drives: []DriveInfo{
			{
				Device: blockdev.Device{
					Name:   "nvme0n1",
					Vendor: "Samsung",
					Model:  "970 EVO",
				},
			},
			{
				Device: blockdev.Device{
					Name:   "sda",
					Vendor: "Seagate",
					Model:  "ST2000",
				},
			},
		},
		Phases: []PhaseResult{
			{
				PhaseName: "Sequential Read",
				Duration:  60.5,
				Jobs: []JobResult{
					{
						JobName: "seq-read-job",
						ReadStats: &IOStats{
							IOPS:      50000,
							Bandwidth: 204800, // 200 MB/s in KB/s
						},
						Latency: &LatencyStats{
							Mean: 10000,
							P95:  15000,
							P99:  20000,
						},
					},
				},
			},
			{
				PhaseName: "Random Write",
				Duration:  45.2,
				Jobs: []JobResult{
					{
						JobName: "rand-write-job",
						WriteStats: &IOStats{
							IOPS:      30000,
							Bandwidth: 122880,
						},
					},
				},
			},
		},
	}
	globalResultsStore.AddResult(result)
	defer globalResultsStore.Clear()

	req := httptest.NewRequest("GET", "/export?id=export-test-123", nil)
	w := httptest.NewRecorder()

	handleExportHTML(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/html; charset=utf-8", contentType)
	}

	disposition := resp.Header.Get("Content-Disposition")
	if !contains(disposition, "attachment") {
		t.Errorf("Content-Disposition should contain 'attachment', got: %q", disposition)
	}

	if !contains(disposition, "bbbench_export-test-123") {
		t.Errorf("Content-Disposition should contain result ID, got: %q", disposition)
	}

	body := w.Body.String()

	// Check for essential HTML structure
	if !contains(body, "<!DOCTYPE html>") {
		t.Error("Missing DOCTYPE declaration")
	}

	if !contains(body, "bbbench Benchmark Report") {
		t.Error("Missing report title")
	}

	// Check for result data
	if !contains(body, "export-test-123") {
		t.Error("Missing result ID")
	}

	if !contains(body, "parallel-sync") {
		t.Error("Missing mode")
	}

	// Check for drives
	if !contains(body, "nvme0n1") {
		t.Error("Missing first drive name")
	}

	if !contains(body, "Samsung") {
		t.Error("Missing first drive vendor")
	}

	if !contains(body, "sda") {
		t.Error("Missing second drive name")
	}

	// Check for phases
	if !contains(body, "Sequential Read") {
		t.Error("Missing first phase name")
	}

	if !contains(body, "Random Write") {
		t.Error("Missing second phase name")
	}

	if !contains(body, "60.5s") {
		t.Error("Missing first phase duration")
	}

	// Check for Chart.js inclusion
	if !contains(body, "chart.js") {
		t.Error("Missing Chart.js library")
	}

	// Check for embedded graphs data
	if !contains(body, "const graphsData") {
		t.Error("Missing graphs data variable")
	}

	// Check for graph rendering code
	if !contains(body, "new Chart(") {
		t.Error("Missing Chart instantiation code")
	}

	// Check for print-friendly styles
	if !contains(body, "@media print") {
		t.Error("Missing print styles")
	}
}

func TestHandleExportHTML_MissingID(t *testing.T) {
	req := httptest.NewRequest("GET", "/export", nil)
	w := httptest.NewRecorder()

	handleExportHTML(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestHandleExportHTML_NotFound(t *testing.T) {
	req := httptest.NewRequest("GET", "/export?id=nonexistent", nil)
	w := httptest.NewRecorder()

	handleExportHTML(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestGenerateExportHTML(t *testing.T) {
	result := &BenchmarkResult{
		ID:        "html-test-456",
		Timestamp: time.Date(2026, 3, 4, 12, 30, 0, 0, time.UTC),
		Mode:      "sequential",
		Drives: []DriveInfo{
			{
				Device: blockdev.Device{
					Name:   "sdb",
					Vendor: "WD",
					Model:  "Blue",
				},
			},
		},
		Phases: []PhaseResult{
			{
				PhaseName: "Test Phase",
				Duration:  30.0,
				Jobs: []JobResult{
					{JobName: "job1"},
					{JobName: "job2"},
				},
			},
		},
	}

	graphsJSON := `[{"title":"Test Graph","type":"line","labels":["A","B"],"datasets":[]}]`

	html := generateExportHTML(result, graphsJSON)

	// Check basic structure
	if !contains(html, "<!DOCTYPE html>") {
		t.Error("Missing DOCTYPE")
	}

	if !contains(html, "html-test-456") {
		t.Error("Missing result ID")
	}

	if !contains(html, "sequential") {
		t.Error("Missing mode")
	}

	if !contains(html, "Test Phase") {
		t.Error("Missing phase name")
	}

	if !contains(html, "30.0s") {
		t.Error("Missing phase duration")
	}

	if !contains(html, "Jobs: 2") {
		t.Error("Missing job count")
	}

	if !contains(html, "sdb") {
		t.Error("Missing drive name")
	}

	if !contains(html, "WD") {
		t.Error("Missing vendor")
	}

	if !contains(html, "Blue") {
		t.Error("Missing model")
	}

	// Check graphs data is embedded
	if !contains(html, "const graphsData = "+graphsJSON) {
		t.Error("Graphs data not properly embedded")
	}

	// Check styling
	if !contains(html, "background: linear-gradient") {
		t.Error("Missing header gradient")
	}

	if !contains(html, ".graph-container") {
		t.Error("Missing graph container styles")
	}
}

func TestGenerateExportHTML_EmptyPhases(t *testing.T) {
	result := &BenchmarkResult{
		ID:        "empty-test",
		Timestamp: time.Now(),
		Mode:      "parallel-sync",
		Drives:    []DriveInfo{},
		Phases:    []PhaseResult{},
	}

	graphsJSON := "[]"

	html := generateExportHTML(result, graphsJSON)

	if !contains(html, "empty-test") {
		t.Error("Should include result ID even with empty phases")
	}

	if !contains(html, "Total Phases") {
		t.Error("Should include phases section")
	}

	if !contains(html, "Total Drives") {
		t.Error("Should include drives section")
	}
}

func TestGenerateExportHTML_XSSProtection(t *testing.T) {
	result := &BenchmarkResult{
		ID:        "<script>alert('xss')</script>",
		Timestamp: time.Now(),
		Mode:      "<img src=x onerror=alert(1)>",
		Drives: []DriveInfo{
			{
				Device: blockdev.Device{
					Name:   "sda",
					Vendor: "<script>",
					Model:  "'>alert(2)",
				},
			},
		},
		Phases: []PhaseResult{
			{
				PhaseName: "<svg/onload=alert(3)>",
			},
		},
	}

	graphsJSON := "[]"

	html := generateExportHTML(result, graphsJSON)

	// Should not contain executable XSS (unescaped HTML tags with dangerous attributes)
	// These patterns would be dangerous if unescaped (note: without &lt; or &gt;)
	if contains(html, "<script>alert") {
		t.Error("XSS vulnerability: unescaped script tag")
	}

	// Check for unescaped img tag with event handler
	// The dangerous pattern is: <img ... onerror= (not &lt;img)
	if contains(html, "<img") && contains(html, "onerror=") {
		// Make sure it's not the escaped version
		if !contains(html, "&lt;img") {
			t.Error("XSS vulnerability: unescaped img tag with event handler")
		}
	}

	// Should contain escaped versions (safe rendering)
	if !contains(html, "&lt;script&gt;") {
		t.Error("Script tags should be escaped")
	}

	if !contains(html, "&lt;img") {
		t.Error("Image tags should be escaped")
	}

	// Verify the dangerous strings are not in executable form
	// Pattern: <img ... onerror= (without entity encoding)
	if strings.Contains(html, "<img") {
		imgIdx := strings.Index(html, "<img")
		closeIdx := strings.Index(html[imgIdx:], ">")
		if closeIdx > 0 {
			imgTag := html[imgIdx : imgIdx+closeIdx+1]
			if strings.Contains(imgTag, "onerror=") {
				t.Errorf("XSS vulnerability: found executable img tag with onerror: %s", imgTag)
			}
		}
	}
}

func TestExportIntegration(t *testing.T) {
	globalResultsStore.Clear()

	// Create realistic result
	result := &BenchmarkResult{
		ID:        "integration-789",
		Timestamp: time.Now(),
		Mode:      "parallel-sync",
		Drives: []DriveInfo{
			{
				Device: blockdev.Device{
					Name:   "nvme0n1",
					Vendor: "Intel",
					Model:  "P4510",
				},
			},
		},
		Phases: []PhaseResult{
			{
				PhaseName: "Warmup",
				Duration:  10.0,
				Jobs: []JobResult{
					{
						JobName: "warmup-job",
						ReadStats: &IOStats{
							IOPS:      1000,
							Bandwidth: 10240,
						},
					},
				},
			},
		},
	}
	globalResultsStore.AddResult(result)
	defer globalResultsStore.Clear()

	// Test export endpoint
	req := httptest.NewRequest("GET", "/export?id=integration-789", nil)
	w := httptest.NewRecorder()

	handleExportHTML(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Export failed with status %d", resp.StatusCode)
	}

	body := w.Body.String()

	// Verify complete export
	checks := []string{
		"integration-789",
		"parallel-sync",
		"nvme0n1",
		"Intel",
		"P4510",
		"Warmup",
		"10.0s",
		"Jobs: 1",
		"chart.js",
		"IOPS",
	}

	for _, check := range checks {
		if !contains(body, check) {
			t.Errorf("Export missing expected content: %q", check)
		}
	}
}
