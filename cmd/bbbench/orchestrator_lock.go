package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

const lockFileName = ".bbbench.orchestrator.lock"

// getLockFilePath returns the path to the orchestrator lock file.
func getLockFilePath() string {
	tmpDir := os.TempDir()
	return filepath.Join(tmpDir, lockFileName)
}

// acquireOrchestratorLock creates a lock file to indicate orchestrator is running.
// Returns an error if another orchestrator instance is already running.
func acquireOrchestratorLock() error {
	lockPath := getLockFilePath()

	// Check if lock file exists
	if data, err := os.ReadFile(lockPath); err == nil {
		// Lock file exists, check if process is still running
		if pid, err := strconv.Atoi(string(data)); err == nil {
			// Check if process exists
			process, err := os.FindProcess(pid)
			if err == nil {
				// Send signal 0 to check if process is alive
				err = process.Signal(syscall.Signal(0))
				if err == nil {
					return fmt.Errorf("orchestrator is already running (PID %d)", pid)
				}
			}
		}
		// Stale lock file, remove it
		os.Remove(lockPath)
	}

	// Create new lock file with current PID
	pid := os.Getpid()
	if err := os.WriteFile(lockPath, []byte(strconv.Itoa(pid)), 0644); err != nil {
		return fmt.Errorf("create lock file: %w", err)
	}

	return nil
}

// releaseOrchestratorLock removes the lock file.
func releaseOrchestratorLock() {
	lockPath := getLockFilePath()
	os.Remove(lockPath)
}

// isOrchestratorRunning checks if an orchestrator instance is currently running.
func isOrchestratorRunning() bool {
	lockPath := getLockFilePath()

	data, err := os.ReadFile(lockPath)
	if err != nil {
		return false
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return false
	}

	// Check if process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Send signal 0 to check if process is alive
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
