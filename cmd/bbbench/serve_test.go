package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewWebServer(t *testing.T) {
	srv := newWebServer("[::1]:0")

	if srv == nil {
		t.Fatal("newWebServer returned nil")
	}

	if srv.ReadTimeout != 15*time.Second {
		t.Errorf("ReadTimeout = %v, want 15s", srv.ReadTimeout)
	}

	if srv.WriteTimeout != 15*time.Second {
		t.Errorf("WriteTimeout = %v, want 15s", srv.WriteTimeout)
	}

	if srv.IdleTimeout != 60*time.Second {
		t.Errorf("IdleTimeout = %v, want 60s", srv.IdleTimeout)
	}
}

func TestHandleHealth(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handleHealth(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", contentType)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	expected := `{"status":"ok","service":"bbbench"}`
	if string(body) != expected {
		t.Errorf("Body = %q, want %q", string(body), expected)
	}
}

func TestHandleIndex(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handleIndex(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", contentType)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	// Check for expected content
	html := string(body)
	if !strings.Contains(html, "bbbench") {
		t.Error("Index page missing 'bbbench' title")
	}

	if !strings.Contains(html, "Block Device Benchmark") {
		t.Error("Index page missing description")
	}

	// Check for navigation links
	if !strings.Contains(html, "/status") {
		t.Error("Index page missing status link")
	}

	if !strings.Contains(html, "/results") {
		t.Error("Index page missing results link")
	}

	if !strings.Contains(html, "/api/results") {
		t.Error("Index page missing API link")
	}
}

func TestHandle404(t *testing.T) {
	req := httptest.NewRequest("GET", "/nonexistent", nil)
	w := httptest.NewRecorder()

	handleIndex(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestIsServerListening(t *testing.T) {
	// Test with non-listening address
	if isServerListening("[::1]:99999") {
		t.Error("isServerListening returned true for non-listening address")
	}

	// Start a test server
	srv := newWebServer("[::1]:0")
	go srv.ListenAndServe()
	defer srv.Shutdown(context.Background())

	time.Sleep(100 * time.Millisecond)

	// Note: Can't easily test the positive case with :0 binding
	// as we don't know the actual port
}

func TestGetListenAddr(t *testing.T) {
	srv := newWebServer("[::1]:12345")

	addr, err := getListenAddr(srv)
	if err != nil {
		t.Fatalf("getListenAddr failed: %v", err)
	}

	if addr != "[::1]:12345" {
		t.Errorf("Address = %q, want [::1]:12345", addr)
	}

	// Test with empty address
	srv2 := &http.Server{}
	_, err = getListenAddr(srv2)
	if err == nil {
		t.Error("getListenAddr should fail with empty server address")
	}
}

func TestAllRoutesRegistered(t *testing.T) {
	srv := newWebServer("[::1]:12345")
	mux, ok := srv.Handler.(*http.ServeMux)
	if !ok {
		t.Fatal("Server handler is not *http.ServeMux")
	}

	// Test that all expected routes respond (not 404)
	routes := []struct {
		path           string
		expectedStatus int
	}{
		{"/health", http.StatusOK},
		{"/api/status", http.StatusOK},
		{"/api/results", http.StatusOK},
		{"/api/results/detail", http.StatusBadRequest}, // requires ID
		{"/api/results/graphs", http.StatusBadRequest}, // requires ID
		{"/status", http.StatusOK},
		{"/results", http.StatusOK},
		{"/result", http.StatusBadRequest}, // requires ID
		{"/graphs", http.StatusBadRequest}, // requires ID
		{"/export", http.StatusBadRequest}, // requires ID
		{"/", http.StatusOK},
		{"/nonexistent", http.StatusNotFound},
	}

	for _, route := range routes {
		req := httptest.NewRequest("GET", route.path, nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		if w.Code != route.expectedStatus {
			t.Errorf("Route %s: got status %d, want %d", route.path, w.Code, route.expectedStatus)
		}
	}
}

func TestServerConfiguration(t *testing.T) {
	srv := newWebServer("[::1]:8080")

	// Verify server configuration
	if srv.Addr != "[::1]:8080" {
		t.Errorf("Addr = %q, want [::1]:8080", srv.Addr)
	}

	if srv.ReadTimeout != 15*time.Second {
		t.Errorf("ReadTimeout = %v, want 15s", srv.ReadTimeout)
	}

	if srv.WriteTimeout != 15*time.Second {
		t.Errorf("WriteTimeout = %v, want 15s", srv.WriteTimeout)
	}

	if srv.IdleTimeout != 60*time.Second {
		t.Errorf("IdleTimeout = %v, want 60s", srv.IdleTimeout)
	}

	if srv.Handler == nil {
		t.Error("Handler should not be nil")
	}
}

func TestHandleIndexRejectsNonRoot(t *testing.T) {
	req := httptest.NewRequest("GET", "/some-path", nil)
	w := httptest.NewRecorder()

	handleIndex(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Non-root path should return 404, got %d", resp.StatusCode)
	}
}
