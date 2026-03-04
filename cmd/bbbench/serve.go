package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

// runServe starts the web server for the bbbench GUI.
func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", "0.0.0.0:12345", "address to bind web server (all interfaces by default)")
	outputDir := fs.String("output", "~/.bbbench/output", "output directory containing benchmark results")
	_ = fs.Parse(args)

	// Check if orchestrator is running
	if isOrchestratorRunning() {
		return errors.New("orchestrator is already running with web interface enabled\nConnect to http://0.0.0.0:12345 to view status")
	}

	// Load existing benchmark results from output directory
	if err := loadBenchmarkSummaries(expandUser(*outputDir)); err != nil {
		logger.Warn("failed to load benchmark summaries", "err", err)
		fmt.Printf("Warning: failed to load existing results: %v\n", err)
	}

	srv := newWebServer(*addr)

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		logger.Info("starting web server", "addr", *addr)
		fmt.Printf("Web server starting on http://%s\n", *addr)
		fmt.Printf("Press Ctrl+C to stop\n\n")

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errChan:
		return fmt.Errorf("server error: %w", err)
	case sig := <-sigChan:
		logger.Info("received signal, shutting down", "signal", sig)
		fmt.Println("\nShutting down gracefully...")

		// Graceful shutdown with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			return fmt.Errorf("server shutdown error: %w", err)
		}

		logger.Info("server stopped")
		return nil
	}
}

// newWebServer creates and configures the HTTP server.
func newWebServer(addr string) *http.Server {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", handleHealth)

	// API endpoints
	mux.HandleFunc("/api/status", handleStatus)
	mux.HandleFunc("/api/results", handleResultsList)
	mux.HandleFunc("/api/results/detail", handleResultDetail)
	mux.HandleFunc("/api/results/graphs", handleResultGraphs)

	// HTML pages
	mux.HandleFunc("/status", handleStatusPage)
	mux.HandleFunc("/results", handleBrowsePage)
	mux.HandleFunc("/result", handleResultDetailPage)
	mux.HandleFunc("/graphs", handleGraphsPage)
	mux.HandleFunc("/export", handleExportHTML)
	mux.HandleFunc("/", handleIndex)

	return &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

// handleHealth responds to health check requests.
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok","service":"bbbench"}`)
}

// handleIndex serves the main page.
func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>bbbench - Block Device Benchmark</title>
    <meta charset="utf-8">
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
        .content {
            background: #fff;
            padding: 20px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>bbbench</h1>
        <div class="subtitle">Block Device Benchmark Tool</div>
    </div>
    <div class="content">
        <h2>Welcome</h2>
        <p>The web interface is running.</p>
        <h3>Available Pages:</h3>
        <ul>
            <li><a href="/status">Status</a> - View current benchmark execution status</li>
            <li><a href="/results">Browse Results</a> - Browse and search benchmark results</li>
            <li><a href="/api/results">Results API</a> - List all benchmark results (JSON)</li>
            <li><a href="/health">Health</a> - Server health check</li>
        </ul>
        <p>Use the CLI to start benchmarks: <code>bbbench orchestrator</code></p>
    </div>
</body>
</html>`)
}

// getListenAddr extracts the actual listening address from the server.
// This is useful for testing when binding to :0 to get a random port.
func getListenAddr(srv *http.Server) (string, error) {
	// Try to extract from listener if server is already running
	if srv.Addr == "" {
		return "", fmt.Errorf("server address not set")
	}
	return srv.Addr, nil
}

// isServerListening checks if a server is listening on the given address.
func isServerListening(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// loadBenchmarkSummaries loads all benchmark summary files from the output directory.
func loadBenchmarkSummaries(outputDir string) error {
	// Check if directory exists
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		logger.Info("output directory does not exist", "dir", outputDir)
		return nil
	}

	// Find all summary files
	pattern := filepath.Join(outputDir, "run_*_summary.json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("glob summary files: %w", err)
	}

	logger.Info("loading benchmark summaries", "dir", outputDir, "files", len(files))
	fmt.Printf("Loading %d benchmark result(s) from %s\n", len(files), outputDir)

	loadedCount := 0
	for _, file := range files {
		if err := loadBenchmarkSummary(file); err != nil {
			logger.Error("failed to load summary", "file", file, "err", err)
			// Continue loading other files
		} else {
			loadedCount++
		}
	}

	fmt.Printf("Successfully loaded %d benchmark result(s)\n\n", loadedCount)
	return nil
}

// loadBenchmarkSummary loads a single benchmark summary file.
func loadBenchmarkSummary(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	var summary BenchmarkResult
	if err := json.Unmarshal(data, &summary); err != nil {
		return fmt.Errorf("unmarshal json: %w", err)
	}

	// Add to global results store
	globalResultsStore.AddResult(&summary)

	logger.Debug("loaded benchmark summary", "id", summary.ID, "timestamp", summary.Timestamp)
	return nil
}
