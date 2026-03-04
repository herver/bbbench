package main

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// BenchmarkStatus represents the current state of benchmark execution.
type BenchmarkStatus struct {
	// Running indicates if benchmarks are currently executing
	Running bool `json:"running"`

	// StartTime is when the current benchmark run started
	StartTime time.Time `json:"start_time,omitempty"`

	// Mode is the execution mode (parallel-sync or sequential)
	Mode string `json:"mode,omitempty"`

	// Drives is the list of drives being benchmarked
	Drives []DriveStatus `json:"drives,omitempty"`

	// CurrentPhase is the current phase number (0-indexed)
	CurrentPhase int `json:"current_phase"`

	// TotalPhases is the total number of phases
	TotalPhases int `json:"total_phases"`

	// CompletedPhases is the number of completed phases
	CompletedPhases int `json:"completed_phases"`

	// LastUpdate is the last time the status was updated
	LastUpdate time.Time `json:"last_update"`
}

// DriveStatus represents the status of a single drive.
type DriveStatus struct {
	Name          string `json:"name"`
	Device        string `json:"device"`
	Vendor        string `json:"vendor"`
	Model         string `json:"model"`
	Serial        string `json:"serial"`
	Capacity      uint64 `json:"capacity_gb"`
	Type          string `json:"type"` // "SSD" or "HDD"
	CurrentPhase  int    `json:"current_phase"`
	PhaseName     string `json:"phase_name,omitempty"`
	Status        string `json:"status"` // "running", "complete", "error", "pending"
	ErrorMessage  string `json:"error,omitempty"`
	LastCompleted string `json:"last_completed,omitempty"`
}

// globalStatus is the singleton status tracker
var (
	globalStatus   = &BenchmarkStatus{LastUpdate: time.Now()}
	globalStatusMu sync.RWMutex
)

// UpdateStatus updates the global benchmark status.
func UpdateStatus(fn func(*BenchmarkStatus)) {
	globalStatusMu.Lock()
	defer globalStatusMu.Unlock()

	fn(globalStatus)
	globalStatus.LastUpdate = time.Now()
}

// GetStatus returns a copy of the current status.
func GetStatus() BenchmarkStatus {
	globalStatusMu.RLock()
	defer globalStatusMu.RUnlock()

	// Deep copy to avoid race conditions
	status := *globalStatus
	if globalStatus.Drives != nil {
		status.Drives = make([]DriveStatus, len(globalStatus.Drives))
		copy(status.Drives, globalStatus.Drives)
	}

	return status
}

// ResetStatus resets the global status to initial state.
func ResetStatus() {
	UpdateStatus(func(s *BenchmarkStatus) {
		s.Running = false
		s.StartTime = time.Time{}
		s.Mode = ""
		s.Drives = nil
		s.CurrentPhase = 0
		s.TotalPhases = 0
		s.CompletedPhases = 0
	})
}

// handleStatus serves the current benchmark status as JSON.
func handleStatus(w http.ResponseWriter, r *http.Request) {
	status := GetStatus()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	if err := json.NewEncoder(w).Encode(status); err != nil {
		logger.Error("encode status", "err", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleStatusPage serves the auto-refreshing HTML status page.
func handleStatusPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	html := `<!DOCTYPE html>
<html>
<head>
    <title>bbbench - Status</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            max-width: 1200px;
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
        .subtitle {
            color: #666;
            margin-top: 5px;
        }
        .status-card {
            background: #fff;
            padding: 20px;
            border-radius: 8px;
            margin-bottom: 20px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .status-badge {
            display: inline-block;
            padding: 4px 12px;
            border-radius: 4px;
            font-size: 14px;
            font-weight: 500;
        }
        .status-idle {
            background: #e0e0e0;
            color: #666;
        }
        .status-running {
            background: #4caf50;
            color: white;
            animation: pulse 2s infinite;
        }
        @keyframes pulse {
            0%, 100% { opacity: 1; }
            50% { opacity: 0.7; }
        }
        .progress-bar {
            width: 100%;
            height: 24px;
            background: #e0e0e0;
            border-radius: 4px;
            overflow: hidden;
            margin: 10px 0;
        }
        .progress-fill {
            height: 100%;
            background: linear-gradient(90deg, #4caf50, #45a049);
            transition: width 0.3s ease;
            display: flex;
            align-items: center;
            justify-content: center;
            color: white;
            font-size: 12px;
            font-weight: 500;
        }
        .drives-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
            gap: 15px;
            margin-top: 20px;
        }
        .drive-card {
            background: #f9f9f9;
            padding: 15px;
            border-radius: 6px;
            border-left: 4px solid #2196f3;
        }
        .drive-name {
            font-weight: 600;
            font-size: 16px;
            color: #333;
        }
        .drive-detail {
            font-size: 14px;
            color: #666;
            margin-top: 5px;
        }
        .last-update {
            text-align: right;
            color: #999;
            font-size: 12px;
            margin-top: 10px;
        }
        .no-data {
            text-align: center;
            padding: 40px;
            color: #999;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>bbbench</h1>
        <div class="subtitle">Benchmark Status</div>
    </div>

    <div class="status-card">
        <h2>Current Status</h2>
        <div id="status-info">
            <span class="status-badge status-idle">Idle</span>
        </div>
        <div id="progress-container" style="display: none;">
            <div class="progress-bar">
                <div class="progress-fill" id="progress-fill" style="width: 0%">
                    <span id="progress-text">0%</span>
                </div>
            </div>
        </div>
        <div id="benchmark-details"></div>
    </div>

    <div class="status-card">
        <h2>Drives</h2>
        <div id="drives-list" class="no-data">No active benchmarks</div>
    </div>

    <div class="last-update" id="last-update">Last updated: Never</div>

    <script>
        function updateStatus() {
            fetch('/api/status')
                .then(response => response.json())
                .then(data => {
                    updateStatusDisplay(data);
                })
                .catch(error => {
                    console.error('Error fetching status:', error);
                });
        }

        function updateStatusDisplay(status) {
            const statusInfo = document.getElementById('status-info');
            const progressContainer = document.getElementById('progress-container');
            const progressFill = document.getElementById('progress-fill');
            const progressText = document.getElementById('progress-text');
            const benchmarkDetails = document.getElementById('benchmark-details');
            const drivesList = document.getElementById('drives-list');
            const lastUpdate = document.getElementById('last-update');

            // Update status badge
            if (status.running) {
                statusInfo.innerHTML = '<span class="status-badge status-running">Running</span>';
                progressContainer.style.display = 'block';

                // Update progress
                const progress = status.total_phases > 0
                    ? Math.round((status.completed_phases / status.total_phases) * 100)
                    : 0;
                progressFill.style.width = progress + '%';
                progressText.textContent = progress + '%';

                // Show benchmark details
                benchmarkDetails.innerHTML =
                    '<div class="drive-detail">Mode: ' + status.mode + '</div>' +
                    '<div class="drive-detail">Phase: ' + (status.current_phase + 1) + ' / ' + status.total_phases + '</div>' +
                    '<div class="drive-detail">Completed: ' + status.completed_phases + ' / ' + status.total_phases + '</div>';
            } else {
                statusInfo.innerHTML = '<span class="status-badge status-idle">Idle</span>';
                progressContainer.style.display = 'none';
                benchmarkDetails.innerHTML = '';
            }

            // Update drives list
            if (status.drives && status.drives.length > 0) {
                // Sort drives alphabetically by name
                const sortedDrives = status.drives.slice().sort((a, b) => a.name.localeCompare(b.name));

                let html = '<div class="drives-grid">';
                sortedDrives.forEach(drive => {
                    // Determine status display
                    let statusText = drive.status || 'pending';
                    let statusColor = '#666';
                    if (statusText === 'running') {
                        statusColor = '#4caf50';
                        statusText = 'Running';
                    } else if (statusText === 'complete') {
                        statusColor = '#2196f3';
                        statusText = 'Complete';
                    } else if (statusText === 'error') {
                        statusColor = '#f44336';
                        statusText = 'Error';
                    } else {
                        statusText = 'Pending';
                    }

                    html += '<div class="drive-card">' +
                        '<div class="drive-name">' + drive.name + ' (' + (drive.type || 'Unknown') + ')</div>' +
                        '<div class="drive-detail">' + drive.vendor + ' ' + drive.model + '</div>';
                    if (drive.serial) {
                        html += '<div class="drive-detail">S/N: ' + drive.serial + '</div>';
                    }
                    if (drive.capacity_gb) {
                        html += '<div class="drive-detail">Capacity: ' + drive.capacity_gb + ' GB</div>';
                    }
                    html += '<div class="drive-detail" style="color: ' + statusColor + '; font-weight: 600;">Status: ' + statusText + '</div>';
                    if (drive.phase_name && drive.status === 'running') {
                        html += '<div class="drive-detail">Phase: ' + drive.phase_name + '</div>';
                    }
                    if (drive.error) {
                        html += '<div class="drive-detail" style="color: #f44336;">Error: ' + drive.error + '</div>';
                    }
                    html += '</div>';
                });
                html += '</div>';
                drivesList.innerHTML = html;
            } else {
                drivesList.innerHTML = '<div class="no-data">No active benchmarks</div>';
            }

            // Update last update time
            if (status.last_update) {
                const date = new Date(status.last_update);
                lastUpdate.textContent = 'Last updated: ' + date.toLocaleTimeString();
            }
        }

        // Update every 2 seconds
        updateStatus();
        setInterval(updateStatus, 2000);
    </script>
</body>
</html>`

	w.Write([]byte(html))
}
