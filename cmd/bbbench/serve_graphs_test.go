package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bbbench/internal/blockdev"
)

func TestGenerateIOPSGraph(t *testing.T) {
	result := &BenchmarkResult{
		Phases: []PhaseResult{
			{
				PhaseName: "Phase 1",
				Jobs: []JobResult{
					{
						JobName: "read-job",
						ReadStats: &IOStats{
							IOPS: 50000,
						},
					},
				},
			},
			{
				PhaseName: "Phase 2",
				Jobs: []JobResult{
					{
						JobName: "write-job",
						WriteStats: &IOStats{
							IOPS: 40000,
						},
					},
				},
			},
		},
	}

	graph := generateIOPSGraph(result)

	if graph == nil {
		t.Fatal("Graph should not be nil")
	}

	if graph.Title != "IOPS (I/O Operations Per Second)" {
		t.Errorf("Title = %q, want IOPS", graph.Title)
	}

	if graph.Type != "line" {
		t.Errorf("Type = %q, want line", graph.Type)
	}

	if len(graph.Labels) != 2 {
		t.Fatalf("Labels length = %d, want 2", len(graph.Labels))
	}

	if graph.Labels[0] != "Phase 1" {
		t.Errorf("Labels[0] = %q, want Phase 1", graph.Labels[0])
	}

	// Should have read and write datasets
	if len(graph.Datasets) != 2 {
		t.Fatalf("Datasets length = %d, want 2", len(graph.Datasets))
	}

	// Check read dataset
	readDataset := graph.Datasets[0]
	if readDataset.Label != "Read IOPS" {
		t.Errorf("Read dataset label = %q, want Read IOPS", readDataset.Label)
	}

	if len(readDataset.Data) != 2 {
		t.Fatalf("Read data length = %d, want 2", len(readDataset.Data))
	}

	if readDataset.Data[0] != 50000 {
		t.Errorf("Read data[0] = %f, want 50000", readDataset.Data[0])
	}

	if readDataset.Data[1] != 0 {
		t.Errorf("Read data[1] = %f, want 0", readDataset.Data[1])
	}
}

func TestGenerateBandwidthGraph(t *testing.T) {
	result := &BenchmarkResult{
		Phases: []PhaseResult{
			{
				PhaseName: "Phase 1",
				Jobs: []JobResult{
					{
						JobName: "read-job",
						ReadStats: &IOStats{
							Bandwidth: 204800, // 200 MB/s in KB/s
						},
					},
				},
			},
		},
	}

	graph := generateBandwidthGraph(result)

	if graph == nil {
		t.Fatal("Graph should not be nil")
	}

	if graph.Title != "Bandwidth (MB/s)" {
		t.Errorf("Title = %q, want Bandwidth", graph.Title)
	}

	if len(graph.Datasets) == 0 {
		t.Fatal("Should have at least one dataset")
	}

	readDataset := graph.Datasets[0]
	if readDataset.Label != "Read Bandwidth (MB/s)" {
		t.Errorf("Dataset label = %q, want Read Bandwidth (MB/s)", readDataset.Label)
	}

	// Should convert KB/s to MB/s
	expectedMBps := 204800.0 / 1024
	if readDataset.Data[0] != expectedMBps {
		t.Errorf("Bandwidth = %f MB/s, want %f", readDataset.Data[0], expectedMBps)
	}
}

func TestGenerateLatencyGraph(t *testing.T) {
	result := &BenchmarkResult{
		Phases: []PhaseResult{
			{
				PhaseName: "Phase 1",
				Jobs: []JobResult{
					{
						JobName: "test-job",
						Latency: &LatencyStats{
							Mean: 10000, // 10 μs in ns
							P95:  15000,
							P99:  20000,
						},
					},
				},
			},
		},
	}

	graph := generateLatencyGraph(result)

	if graph == nil {
		t.Fatal("Graph should not be nil")
	}

	if graph.Title != "Latency (microseconds)" {
		t.Errorf("Title = %q, want Latency", graph.Title)
	}

	// Should have 3 datasets: avg, p95, p99
	if len(graph.Datasets) != 3 {
		t.Fatalf("Datasets length = %d, want 3", len(graph.Datasets))
	}

	// Check average latency (should convert ns to μs)
	avgDataset := graph.Datasets[0]
	if avgDataset.Label != "Average Latency (μs)" {
		t.Errorf("Avg dataset label = %q", avgDataset.Label)
	}

	expectedAvg := 10000.0 / 1000 // ns to μs
	if avgDataset.Data[0] != expectedAvg {
		t.Errorf("Avg latency = %f μs, want %f", avgDataset.Data[0], expectedAvg)
	}
}

func TestGenerateGraphsForResult_NoPhases(t *testing.T) {
	result := &BenchmarkResult{
		Phases: []PhaseResult{},
	}

	graphs := generateGraphsForResult(result)

	if len(graphs) != 0 {
		t.Errorf("Should return empty graphs for result with no phases")
	}
}

func TestGenerateGraphsForResult_AllGraphs(t *testing.T) {
	result := &BenchmarkResult{
		Phases: []PhaseResult{
			{
				PhaseName: "Test Phase",
				Jobs: []JobResult{
					{
						JobName: "job1",
						ReadStats: &IOStats{
							IOPS:      10000,
							Bandwidth: 40960, // 40 MB/s
						},
						WriteStats: &IOStats{
							IOPS:      8000,
							Bandwidth: 32768,
						},
						Latency: &LatencyStats{
							Mean: 5000,
							P95:  8000,
							P99:  10000,
						},
					},
				},
			},
		},
	}

	graphs := generateGraphsForResult(result)

	// Should generate 3 graphs: IOPS, Bandwidth, Latency
	if len(graphs) != 3 {
		t.Errorf("Graphs length = %d, want 3", len(graphs))
	}
}

func TestHandleResultGraphs(t *testing.T) {
	// Add test result to store
	result := &BenchmarkResult{
		ID:        "test-123",
		Timestamp: time.Now(),
		Phases: []PhaseResult{
			{
				PhaseName: "Test",
				Jobs: []JobResult{
					{
						ReadStats: &IOStats{IOPS: 1000},
					},
				},
			},
		},
	}
	globalResultsStore.AddResult(result)
	defer globalResultsStore.Clear()

	req := httptest.NewRequest("GET", "/api/results/graphs?id=test-123", nil)
	w := httptest.NewRecorder()

	handleResultGraphs(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var graphs []GraphData
	if err := json.NewDecoder(resp.Body).Decode(&graphs); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if len(graphs) == 0 {
		t.Error("Should return at least one graph")
	}
}

func TestHandleResultGraphs_MissingID(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/results/graphs", nil)
	w := httptest.NewRecorder()

	handleResultGraphs(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestHandleResultGraphs_NotFound(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/results/graphs?id=nonexistent", nil)
	w := httptest.NewRecorder()

	handleResultGraphs(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestHandleResultsList(t *testing.T) {
	globalResultsStore.Clear()

	// Add test results
	now := time.Now()
	results := []*BenchmarkResult{
		{
			ID:        "1",
			Timestamp: now,
			Mode:      "parallel-sync",
			Drives:    []DriveInfo{{Device: blockdev.Device{Name: "sda"}}},
			Phases:    []PhaseResult{{}, {}},
		},
		{
			ID:        "2",
			Timestamp: now.Add(-1 * time.Hour),
			Mode:      "sequential",
			Drives:    []DriveInfo{{Device: blockdev.Device{Name: "nvme0n1"}}},
			Phases:    []PhaseResult{{}},
		},
	}

	for _, r := range results {
		globalResultsStore.AddResult(r)
	}
	defer globalResultsStore.Clear()

	req := httptest.NewRequest("GET", "/api/results", nil)
	w := httptest.NewRecorder()

	handleResultsList(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var summaries []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&summaries); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if len(summaries) != 2 {
		t.Errorf("Summaries length = %d, want 2", len(summaries))
	}

	// Should be sorted newest first
	if summaries[0]["id"] != "1" {
		t.Errorf("First result ID = %v, want 1", summaries[0]["id"])
	}

	if summaries[0]["mode"] != "parallel-sync" {
		t.Errorf("First result mode = %v", summaries[0]["mode"])
	}
}

func TestHandleGraphsPage(t *testing.T) {
	// Add test result
	result := &BenchmarkResult{
		ID:        "test-456",
		Timestamp: time.Now(),
		Phases:    []PhaseResult{{}},
	}
	globalResultsStore.AddResult(result)
	defer globalResultsStore.Clear()

	req := httptest.NewRequest("GET", "/graphs?id=test-456", nil)
	w := httptest.NewRecorder()

	handleGraphsPage(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q", contentType)
	}

	body := w.Body.String()
	if !contains(body, "chart.js") {
		t.Error("Page should include Chart.js library")
	}

	if !contains(body, "test-456") {
		t.Error("Page should include result ID")
	}
}

func TestHandleGraphsPage_MissingID(t *testing.T) {
	req := httptest.NewRequest("GET", "/graphs", nil)
	w := httptest.NewRecorder()

	handleGraphsPage(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}
