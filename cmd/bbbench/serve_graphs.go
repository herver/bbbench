package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// GraphData represents data for a single graph.
type GraphData struct {
	Title    string    `json:"title"`
	Type     string    `json:"type"` // line, bar
	Labels   []string  `json:"labels"`
	Datasets []Dataset `json:"datasets"`
}

// Dataset represents a single data series in a graph.
type Dataset struct {
	Label           string    `json:"label"`
	Data            []float64 `json:"data"`
	BackgroundColor string    `json:"backgroundColor,omitempty"`
	BorderColor     string    `json:"borderColor,omitempty"`
	Fill            bool      `json:"fill"`
}

// generateGraphsForResult creates graph data for a benchmark result.
func generateGraphsForResult(result *BenchmarkResult) []GraphData {
	var graphs []GraphData

	if len(result.Phases) == 0 {
		return graphs
	}

	// Generate IOPS graph
	if iopsGraph := generateIOPSGraph(result); iopsGraph != nil {
		graphs = append(graphs, *iopsGraph)
	}

	// Generate bandwidth graph
	if bwGraph := generateBandwidthGraph(result); bwGraph != nil {
		graphs = append(graphs, *bwGraph)
	}

	// Generate latency graph
	if latGraph := generateLatencyGraph(result); latGraph != nil {
		graphs = append(graphs, *latGraph)
	}

	return graphs
}

// generateIOPSGraph creates an IOPS graph from benchmark results.
func generateIOPSGraph(result *BenchmarkResult) *GraphData {
	labels := make([]string, 0, len(result.Phases))
	readData := make([]float64, 0, len(result.Phases))
	writeData := make([]float64, 0, len(result.Phases))
	trimData := make([]float64, 0, len(result.Phases))

	hasRead := false
	hasWrite := false
	hasTrim := false

	for _, phase := range result.Phases {
		labels = append(labels, phase.PhaseName)

		// Aggregate IOPS for all jobs in phase
		var phaseReadIOPS, phaseWriteIOPS, phaseTrimIOPS float64
		for _, job := range phase.Jobs {
			if job.ReadStats != nil {
				phaseReadIOPS += job.ReadStats.IOPS
				hasRead = true
			}
			if job.WriteStats != nil {
				phaseWriteIOPS += job.WriteStats.IOPS
				hasWrite = true
			}
			if job.TrimStats != nil {
				phaseTrimIOPS += job.TrimStats.IOPS
				hasTrim = true
			}
		}

		readData = append(readData, phaseReadIOPS)
		writeData = append(writeData, phaseWriteIOPS)
		trimData = append(trimData, phaseTrimIOPS)
	}

	datasets := []Dataset{}
	if hasRead {
		datasets = append(datasets, Dataset{
			Label:       "Read IOPS",
			Data:        readData,
			BorderColor: "rgb(75, 192, 192)",
			Fill:        false,
		})
	}
	if hasWrite {
		datasets = append(datasets, Dataset{
			Label:       "Write IOPS",
			Data:        writeData,
			BorderColor: "rgb(255, 99, 132)",
			Fill:        false,
		})
	}
	if hasTrim {
		datasets = append(datasets, Dataset{
			Label:       "Trim IOPS",
			Data:        trimData,
			BorderColor: "rgb(255, 205, 86)",
			Fill:        false,
		})
	}

	if len(datasets) == 0 {
		return nil
	}

	return &GraphData{
		Title:    "IOPS (I/O Operations Per Second)",
		Type:     "line",
		Labels:   labels,
		Datasets: datasets,
	}
}

// generateBandwidthGraph creates a bandwidth graph from benchmark results.
func generateBandwidthGraph(result *BenchmarkResult) *GraphData {
	labels := make([]string, 0, len(result.Phases))
	readData := make([]float64, 0, len(result.Phases))
	writeData := make([]float64, 0, len(result.Phases))

	hasRead := false
	hasWrite := false

	for _, phase := range result.Phases {
		labels = append(labels, phase.PhaseName)

		// Aggregate bandwidth for all jobs in phase (convert KB/s to MB/s)
		var phaseReadBW, phaseWriteBW float64
		for _, job := range phase.Jobs {
			if job.ReadStats != nil {
				phaseReadBW += job.ReadStats.Bandwidth / 1024 // KB/s to MB/s
				hasRead = true
			}
			if job.WriteStats != nil {
				phaseWriteBW += job.WriteStats.Bandwidth / 1024
				hasWrite = true
			}
		}

		readData = append(readData, phaseReadBW)
		writeData = append(writeData, phaseWriteBW)
	}

	datasets := []Dataset{}
	if hasRead {
		datasets = append(datasets, Dataset{
			Label:       "Read Bandwidth (MB/s)",
			Data:        readData,
			BorderColor: "rgb(54, 162, 235)",
			Fill:        false,
		})
	}
	if hasWrite {
		datasets = append(datasets, Dataset{
			Label:       "Write Bandwidth (MB/s)",
			Data:        writeData,
			BorderColor: "rgb(255, 159, 64)",
			Fill:        false,
		})
	}

	if len(datasets) == 0 {
		return nil
	}

	return &GraphData{
		Title:    "Bandwidth (MB/s)",
		Type:     "line",
		Labels:   labels,
		Datasets: datasets,
	}
}

// generateLatencyGraph creates a latency graph from benchmark results.
func generateLatencyGraph(result *BenchmarkResult) *GraphData {
	labels := make([]string, 0, len(result.Phases))
	avgData := make([]float64, 0, len(result.Phases))
	p95Data := make([]float64, 0, len(result.Phases))
	p99Data := make([]float64, 0, len(result.Phases))

	hasLatency := false

	for _, phase := range result.Phases {
		labels = append(labels, phase.PhaseName)

		// Use first job with latency stats (convert ns to μs)
		var avgLat, p95Lat, p99Lat float64
		for _, job := range phase.Jobs {
			if job.Latency != nil {
				avgLat = job.Latency.Mean / 1000 // ns to μs
				p95Lat = job.Latency.P95 / 1000
				p99Lat = job.Latency.P99 / 1000
				hasLatency = true
				break
			}
		}

		avgData = append(avgData, avgLat)
		p95Data = append(p95Data, p95Lat)
		p99Data = append(p99Data, p99Lat)
	}

	if !hasLatency {
		return nil
	}

	datasets := []Dataset{
		{
			Label:       "Average Latency (μs)",
			Data:        avgData,
			BorderColor: "rgb(153, 102, 255)",
			Fill:        false,
		},
		{
			Label:       "P95 Latency (μs)",
			Data:        p95Data,
			BorderColor: "rgb(255, 159, 64)",
			Fill:        false,
		},
		{
			Label:       "P99 Latency (μs)",
			Data:        p99Data,
			BorderColor: "rgb(255, 99, 132)",
			Fill:        false,
		},
	}

	return &GraphData{
		Title:    "Latency (microseconds)",
		Type:     "line",
		Labels:   labels,
		Datasets: datasets,
	}
}

// handleResultGraphs serves graph data for a specific result.
func handleResultGraphs(w http.ResponseWriter, r *http.Request) {
	// Extract result ID from query parameter
	resultID := r.URL.Query().Get("id")
	if resultID == "" {
		http.Error(w, "Missing result ID", http.StatusBadRequest)
		return
	}

	// Get result from store
	result, ok := globalResultsStore.GetResult(resultID)
	if !ok {
		http.Error(w, "Result not found", http.StatusNotFound)
		return
	}

	// Generate graphs
	graphs := generateGraphsForResult(result)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(graphs); err != nil {
		if logger != nil {
			logger.Error("encode graphs", "err", err)
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleResultsList serves a list of all benchmark results.
func handleResultsList(w http.ResponseWriter, r *http.Request) {
	results := globalResultsStore.ListResults()

	// Create summary list (don't include full phase data)
	summaries := make([]map[string]interface{}, len(results))
	for i, result := range results {
		summaries[i] = map[string]interface{}{
			"id":          result.ID,
			"timestamp":   result.Timestamp,
			"mode":        result.Mode,
			"drive_count": len(result.Drives),
			"phase_count": len(result.Phases),
			"output_dir":  result.OutputDir,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(summaries); err != nil {
		if logger != nil {
			logger.Error("encode results list", "err", err)
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleGraphsPage serves the graphs visualization page.
func handleGraphsPage(w http.ResponseWriter, r *http.Request) {
	resultID := r.URL.Query().Get("id")
	if resultID == "" {
		http.Error(w, "Missing result ID", http.StatusBadRequest)
		return
	}

	// Verify result exists
	_, ok := globalResultsStore.GetResult(resultID)
	if !ok {
		http.Error(w, "Result not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>bbbench - Benchmark Graphs</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.0/dist/chart.umd.min.js"></script>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            max-width: 1400px;
            margin: 0 auto;
            padding: 20px;
            background: #f5f5f5;
        }
        .header {
            background: #fff;
            padding: 20px;
            border-radius: 8px;
            margin-bottom: 20px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        h1 {
            margin: 0;
            color: #333;
        }
        .nav {
            margin-top: 10px;
        }
        .nav a {
            color: #2196f3;
            text-decoration: none;
            margin-right: 15px;
        }
        .graph-container {
            background: #fff;
            padding: 20px;
            border-radius: 8px;
            margin-bottom: 20px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .graph-title {
            font-size: 18px;
            font-weight: 600;
            margin-bottom: 15px;
            color: #333;
        }
        canvas {
            max-height: 400px;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>Benchmark Graphs</h1>
        <div class="nav">
            <a href="/">Home</a>
            <a href="/status">Status</a>
            <a href="/results">Results</a>
        </div>
    </div>

    <div id="graphs-container">
        <p>Loading graphs...</p>
    </div>

    <script>
        const resultID = '%s';

        fetch('/api/results/graphs?id=' + resultID)
            .then(response => response.json())
            .then(graphs => {
                const container = document.getElementById('graphs-container');
                container.innerHTML = '';

                if (graphs.length === 0) {
                    container.innerHTML = '<p>No graph data available</p>';
                    return;
                }

                graphs.forEach((graphData, index) => {
                    const div = document.createElement('div');
                    div.className = 'graph-container';

                    const title = document.createElement('div');
                    title.className = 'graph-title';
                    title.textContent = graphData.title;
                    div.appendChild(title);

                    const canvas = document.createElement('canvas');
                    canvas.id = 'chart-' + index;
                    div.appendChild(canvas);

                    container.appendChild(div);

                    // Create chart
                    new Chart(canvas, {
                        type: graphData.type,
                        data: {
                            labels: graphData.labels,
                            datasets: graphData.datasets
                        },
                        options: {
                            responsive: true,
                            maintainAspectRatio: true,
                            plugins: {
                                legend: {
                                    position: 'top',
                                },
                            },
                            scales: {
                                y: {
                                    beginAtZero: true
                                }
                            }
                        }
                    });
                });
            })
            .catch(error => {
                console.error('Error loading graphs:', error);
                document.getElementById('graphs-container').innerHTML =
                    '<p>Error loading graphs: ' + error.message + '</p>';
            });
    </script>
</body>
</html>`, resultID)

	w.Write([]byte(html))
}
