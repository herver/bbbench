package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// runServe starts the web server for the bbbench GUI.
func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", "[::1]:12345", "address to bind web server (IPv6 localhost by default)")
	_ = fs.Parse(args)

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

	// Root endpoint
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
        <p>The web interface is running. Status page and benchmark results will appear here.</p>
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
