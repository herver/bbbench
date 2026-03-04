package main

import (
	"os"
	"testing"
)

func TestAcquireOrchestratorLock(t *testing.T) {
	// Clean up any existing lock
	defer releaseOrchestratorLock()

	err := acquireOrchestratorLock()
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// Verify lock file exists
	if !isOrchestratorRunning() {
		t.Error("isOrchestratorRunning should return true after acquiring lock")
	}

	// Try to acquire again - should fail
	err = acquireOrchestratorLock()
	if err == nil {
		t.Error("Should not be able to acquire lock twice")
	}

	// Release lock
	releaseOrchestratorLock()

	// Should be able to acquire again
	err = acquireOrchestratorLock()
	if err != nil {
		t.Fatalf("Should be able to acquire lock after release: %v", err)
	}
}

func TestReleaseOrchestratorLock(t *testing.T) {
	defer releaseOrchestratorLock()

	// Acquire lock
	err := acquireOrchestratorLock()
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// Release it
	releaseOrchestratorLock()

	// Should not be running anymore
	if isOrchestratorRunning() {
		t.Error("isOrchestratorRunning should return false after release")
	}

	// Lock file should be gone
	lockPath := getLockFilePath()
	if _, err := os.Stat(lockPath); err == nil {
		t.Error("Lock file should be removed after release")
	}
}

func TestIsOrchestratorRunning(t *testing.T) {
	defer releaseOrchestratorLock()

	// Should be false initially
	if isOrchestratorRunning() {
		t.Error("isOrchestratorRunning should be false initially")
	}

	// Acquire lock
	err := acquireOrchestratorLock()
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// Should be true now
	if !isOrchestratorRunning() {
		t.Error("isOrchestratorRunning should be true after acquiring lock")
	}

	// Release lock
	releaseOrchestratorLock()

	// Should be false again
	if isOrchestratorRunning() {
		t.Error("isOrchestratorRunning should be false after release")
	}
}

func TestStaleLockFile(t *testing.T) {
	defer releaseOrchestratorLock()

	lockPath := getLockFilePath()

	// Create a stale lock file with an invalid PID
	err := os.WriteFile(lockPath, []byte("999999"), 0644)
	if err != nil {
		t.Fatalf("Failed to create stale lock file: %v", err)
	}

	// Should be able to acquire lock (stale lock is removed)
	err = acquireOrchestratorLock()
	if err != nil {
		t.Fatalf("Should be able to acquire lock with stale lock file: %v", err)
	}

	// Verify we now own the lock
	if !isOrchestratorRunning() {
		t.Error("isOrchestratorRunning should return true after acquiring stale lock")
	}
}

func TestGetLockFilePath(t *testing.T) {
	path := getLockFilePath()

	if path == "" {
		t.Error("Lock file path should not be empty")
	}

	if !contains(path, ".bbbench.orchestrator.lock") {
		t.Errorf("Lock file path should contain lock file name, got: %s", path)
	}
}
