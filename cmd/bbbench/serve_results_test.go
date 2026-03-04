package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestResultsStore_AddAndGetResult(t *testing.T) {
	store := &ResultsStore{
		results: make(map[string]*BenchmarkResult),
	}

	result := &BenchmarkResult{
		ID:        "test-123",
		Timestamp: time.Now(),
		Mode:      "parallel-sync",
		OutputDir: "/tmp/test",
	}

	store.AddResult(result)

	retrieved, ok := store.GetResult("test-123")
	if !ok {
		t.Fatal("Result not found")
	}

	if retrieved.ID != "test-123" {
		t.Errorf("ID = %q, want test-123", retrieved.ID)
	}

	if retrieved.Mode != "parallel-sync" {
		t.Errorf("Mode = %q, want parallel-sync", retrieved.Mode)
	}
}

func TestResultsStore_GetNonExistent(t *testing.T) {
	store := &ResultsStore{
		results: make(map[string]*BenchmarkResult),
	}

	_, ok := store.GetResult("nonexistent")
	if ok {
		t.Error("Should not find non-existent result")
	}
}

func TestResultsStore_ListResults(t *testing.T) {
	store := &ResultsStore{
		results: make(map[string]*BenchmarkResult),
	}

	// Add results with different timestamps
	now := time.Now()
	results := []*BenchmarkResult{
		{ID: "1", Timestamp: now.Add(-2 * time.Hour)},
		{ID: "2", Timestamp: now.Add(-1 * time.Hour)},
		{ID: "3", Timestamp: now},
	}

	for _, r := range results {
		store.AddResult(r)
	}

	list := store.ListResults()

	if len(list) != 3 {
		t.Fatalf("List length = %d, want 3", len(list))
	}

	// Should be sorted newest first
	if list[0].ID != "3" {
		t.Errorf("First result ID = %q, want 3", list[0].ID)
	}

	if list[1].ID != "2" {
		t.Errorf("Second result ID = %q, want 2", list[1].ID)
	}

	if list[2].ID != "1" {
		t.Errorf("Third result ID = %q, want 1", list[2].ID)
	}
}

func TestResultsStore_Clear(t *testing.T) {
	store := &ResultsStore{
		results: make(map[string]*BenchmarkResult),
	}

	store.AddResult(&BenchmarkResult{ID: "1"})
	store.AddResult(&BenchmarkResult{ID: "2"})

	if len(store.results) != 2 {
		t.Fatalf("Initial length = %d, want 2", len(store.results))
	}

	store.Clear()

	if len(store.results) != 0 {
		t.Errorf("Length after clear = %d, want 0", len(store.results))
	}
}

func TestParseFioJSON(t *testing.T) {
	// Sample fio JSON output
	fioJSON := []byte(`{
		"jobs": [
			{
				"jobname": "test-read",
				"read": {
					"iops": 50000.5,
					"bw_bytes": 204800000,
					"runtime": 10000,
					"lat_ns": {
						"mean": 20000.5,
						"stddev": 5000.2,
						"min": 10000,
						"max": 50000
					},
					"clat_ns": {
						"percentile": {
							"50.000000": 19000,
							"95.000000": 30000,
							"99.000000": 45000
						}
					}
				},
				"write": {
					"iops": 0
				},
				"trim": {
					"iops": 0
				}
			},
			{
				"jobname": "test-write",
				"read": {
					"iops": 0
				},
				"write": {
					"iops": 40000.0,
					"bw_bytes": 163840000,
					"runtime": 10000,
					"lat_ns": {
						"mean": 25000.0,
						"stddev": 6000.0,
						"min": 15000,
						"max": 60000
					},
					"clat_ns": {
						"percentile": {
							"50.000000": 24000,
							"95.000000": 35000,
							"99.000000": 50000
						}
					}
				},
				"trim": {
					"iops": 0
				}
			}
		]
	}`)

	results, err := parseFioJSON(fioJSON)
	if err != nil {
		t.Fatalf("parseFioJSON failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Results length = %d, want 2", len(results))
	}

	// Check read job
	readJob := results[0]
	if readJob.JobName != "test-read" {
		t.Errorf("Read job name = %q, want test-read", readJob.JobName)
	}

	if readJob.ReadStats == nil {
		t.Fatal("Read stats should not be nil")
	}

	if readJob.ReadStats.IOPS != 50000.5 {
		t.Errorf("Read IOPS = %f, want 50000.5", readJob.ReadStats.IOPS)
	}

	expectedBW := float64(204800000) / 1024 // Convert to KB/s
	if readJob.ReadStats.Bandwidth != expectedBW {
		t.Errorf("Read bandwidth = %f, want %f", readJob.ReadStats.Bandwidth, expectedBW)
	}

	if readJob.ReadStats.AvgLatNS != 20000.5 {
		t.Errorf("Read avg latency = %f, want 20000.5", readJob.ReadStats.AvgLatNS)
	}

	if readJob.Latency == nil {
		t.Fatal("Latency stats should not be nil")
	}

	if readJob.Latency.P50 != 19000 {
		t.Errorf("P50 latency = %f, want 19000", readJob.Latency.P50)
	}

	if readJob.Latency.P95 != 30000 {
		t.Errorf("P95 latency = %f, want 30000", readJob.Latency.P95)
	}

	// Check write job
	writeJob := results[1]
	if writeJob.JobName != "test-write" {
		t.Errorf("Write job name = %q, want test-write", writeJob.JobName)
	}

	if writeJob.WriteStats == nil {
		t.Fatal("Write stats should not be nil")
	}

	if writeJob.WriteStats.IOPS != 40000.0 {
		t.Errorf("Write IOPS = %f, want 40000.0", writeJob.WriteStats.IOPS)
	}
}

func TestParseFioJSON_InvalidJSON(t *testing.T) {
	invalidJSON := []byte(`{invalid json}`)

	_, err := parseFioJSON(invalidJSON)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestParseFioJSON_EmptyJobs(t *testing.T) {
	emptyJSON := []byte(`{"jobs": []}`)

	results, err := parseFioJSON(emptyJSON)
	if err != nil {
		t.Fatalf("parseFioJSON failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Results length = %d, want 0", len(results))
	}
}

func TestLoadResultsFromDir_NonExistent(t *testing.T) {
	// Should not error on non-existent directory
	err := LoadResultsFromDir("/tmp/nonexistent-dir-12345")
	if err != nil {
		t.Errorf("LoadResultsFromDir should not error on non-existent dir: %v", err)
	}
}

func TestLoadResultsFromDir(t *testing.T) {
	// Create temporary directory with test JSON file
	tmpDir := t.TempDir()

	testJSON := `{
		"jobs": [
			{
				"jobname": "test-job",
				"read": {
					"iops": 1000,
					"bw_bytes": 4096000,
					"runtime": 5000,
					"lat_ns": {"mean": 10000, "stddev": 2000, "min": 5000, "max": 20000},
					"clat_ns": {"percentile": {"50.000000": 9500}}
				},
				"write": {"iops": 0},
				"trim": {"iops": 0}
			}
		]
	}`

	testFile := filepath.Join(tmpDir, "test_result.json")
	if err := os.WriteFile(testFile, []byte(testJSON), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Load results
	err := LoadResultsFromDir(tmpDir)
	if err != nil {
		t.Errorf("LoadResultsFromDir failed: %v", err)
	}

	// Should have logged the file but not stored it yet (TODO implement storage)
}

func TestResultsStore_Concurrent(t *testing.T) {
	store := &ResultsStore{
		results: make(map[string]*BenchmarkResult),
	}

	// Run concurrent operations
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			id := string(rune('A' + n))
			store.AddResult(&BenchmarkResult{
				ID:        id,
				Timestamp: time.Now(),
			})
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have all 10 results
	list := store.ListResults()
	if len(list) != 10 {
		t.Errorf("List length = %d, want 10", len(list))
	}
}
