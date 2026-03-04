package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// handleExportHTML serves a standalone HTML export of a benchmark result.
func handleExportHTML(w http.ResponseWriter, r *http.Request) {
	resultID := r.URL.Query().Get("id")
	if resultID == "" {
		http.Error(w, "Missing result ID", http.StatusBadRequest)
		return
	}

	result, ok := globalResultsStore.GetResult(resultID)
	if !ok {
		http.Error(w, "Result not found", http.StatusNotFound)
		return
	}

	// Generate graphs data
	graphs := generateGraphsForResult(result)
	graphsJSON, err := json.Marshal(graphs)
	if err != nil {
		if logger != nil {
			logger.Error("marshal graphs for export", "err", err)
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Set headers for file download
	filename := fmt.Sprintf("bbbench_%s_%s.html", result.ID, time.Now().Format("20060102_150405"))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	html := generateExportHTML(result, string(graphsJSON))
	w.Write([]byte(html))
}

// generateExportHTML creates a standalone HTML document with embedded graphs.
func generateExportHTML(result *BenchmarkResult, graphsJSON string) string {
	// Build phases HTML
	phasesHTML := ""
	for i, phase := range result.Phases {
		jobCount := len(phase.Jobs)
		duration := "N/A"
		if phase.Duration > 0 {
			duration = fmt.Sprintf("%.1fs", phase.Duration)
		}

		phasesHTML += fmt.Sprintf(`
		<div class="phase-item">
			<div class="phase-header">
				<strong>Phase %d:</strong> %s
			</div>
			<div class="phase-details">
				Jobs: %d | Duration: %s
			</div>
		</div>
		`, i+1, escapeHTML(phase.PhaseName), jobCount, duration)
	}

	// Build drives HTML
	drivesHTML := ""
	for _, drive := range result.Drives {
		drivesHTML += fmt.Sprintf(`
		<div class="drive-item">
			<strong>%s</strong> - %s %s
		</div>
		`, escapeHTML(drive.Device.Name), escapeHTML(drive.Device.Vendor), escapeHTML(drive.Device.Model))
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>bbbench Report - %s</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.0/dist/chart.umd.min.js"></script>
    <style>
        * {
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            margin: 0;
            padding: 20px;
            background: #f5f5f5;
        }
        .container {
            max-width: 1400px;
            margin: 0 auto;
        }
        .header {
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            color: white;
            padding: 30px;
            border-radius: 8px;
            margin-bottom: 20px;
            box-shadow: 0 4px 6px rgba(0,0,0,0.1);
        }
        .header h1 {
            margin: 0 0 10px 0;
            font-size: 28px;
        }
        .header .subtitle {
            opacity: 0.9;
            font-size: 14px;
        }
        .card {
            background: white;
            padding: 20px;
            border-radius: 8px;
            margin-bottom: 20px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .card h2 {
            margin: 0 0 15px 0;
            font-size: 20px;
            color: #333;
            border-bottom: 2px solid #f0f0f0;
            padding-bottom: 10px;
        }
        .info-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 15px;
            margin-bottom: 20px;
        }
        .info-item {
            padding: 12px;
            background: #f9f9f9;
            border-radius: 6px;
        }
        .info-label {
            font-size: 12px;
            color: #666;
            text-transform: uppercase;
            font-weight: 600;
            margin-bottom: 5px;
        }
        .info-value {
            font-size: 16px;
            color: #333;
            font-weight: 500;
        }
        .phase-item, .drive-item {
            padding: 12px;
            background: #f9f9f9;
            border-left: 4px solid #667eea;
            margin-bottom: 10px;
            border-radius: 4px;
        }
        .phase-header {
            font-size: 15px;
            color: #333;
            margin-bottom: 5px;
        }
        .phase-details, .drive-item {
            font-size: 13px;
            color: #666;
        }
        .graph-container {
            background: white;
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
        .footer {
            text-align: center;
            padding: 20px;
            color: #999;
            font-size: 12px;
        }
        @media print {
            body {
                background: white;
            }
            .card, .graph-container {
                box-shadow: none;
                border: 1px solid #ddd;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>bbbench Benchmark Report</h1>
            <div class="subtitle">Result ID: %s | Generated: %s</div>
        </div>

        <div class="card">
            <h2>Overview</h2>
            <div class="info-grid">
                <div class="info-item">
                    <div class="info-label">Result ID</div>
                    <div class="info-value"><code>%s</code></div>
                </div>
                <div class="info-item">
                    <div class="info-label">Timestamp</div>
                    <div class="info-value">%s</div>
                </div>
                <div class="info-item">
                    <div class="info-label">Mode</div>
                    <div class="info-value">%s</div>
                </div>
                <div class="info-item">
                    <div class="info-label">Total Phases</div>
                    <div class="info-value">%d</div>
                </div>
                <div class="info-item">
                    <div class="info-label">Total Drives</div>
                    <div class="info-value">%d</div>
                </div>
            </div>
        </div>

        <div class="card">
            <h2>Drives</h2>
            %s
        </div>

        <div class="card">
            <h2>Phases</h2>
            %s
        </div>

        <div id="graphs-container"></div>

        <div class="footer">
            Generated by bbbench | Export created: %s
        </div>
    </div>

    <script>
        const graphsData = %s;

        if (graphsData && graphsData.length > 0) {
            const container = document.getElementById('graphs-container');

            graphsData.forEach((graphData, index) => {
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
        }
    </script>
</body>
</html>`,
		escapeHTML(result.ID),
		escapeHTML(result.ID),
		time.Now().Format("2006-01-02 15:04:05"),
		escapeHTML(result.ID),
		result.Timestamp.Format("2006-01-02 15:04:05"),
		escapeHTML(result.Mode),
		len(result.Phases),
		len(result.Drives),
		drivesHTML,
		phasesHTML,
		time.Now().Format("2006-01-02 15:04:05"),
		graphsJSON)
}
