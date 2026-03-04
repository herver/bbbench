package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ExecutionCoordinator manages benchmark execution across multiple drives.
type ExecutionCoordinator struct {
	drives      []DriveInfo
	workloads   map[string]*FioWorkload // keyed by device name
	mode        string
	outputDir   string
	dryRun      bool
	verbose     bool
	results     map[string][]*FioResult
	mu          sync.Mutex
	ctx         context.Context
	cancel      context.CancelFunc
	interrupted bool
	tempFiles   []string
	tempFilesMu sync.Mutex
	stateFile   string
	resumeState *ExecutionState
	startTime   time.Time
}

// FioResult represents the result of a single fio execution.
type FioResult struct {
	DeviceName string
	JobName    string
	Phase      int
	OutputPath string
	JsonOutput []byte
	Error      error
	StartTime  time.Time
	EndTime    time.Time
}

// checkFioInstalled verifies that the fio binary is available in PATH.
func checkFioInstalled() error {
	path, err := exec.LookPath("fio")
	if err != nil {
		return fmt.Errorf("fio is not installed or not in PATH. Please install fio before running benchmarks")
	}
	logger.Debug("found fio binary", "path", path)
	return nil
}

// validateFioFile validates a fio file using fio --parse-only.
func validateFioFile(path string) error {
	cmd := exec.Command("fio", "--parse-only", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Include fio's error output in the error message
		return fmt.Errorf("fio validation failed: %w\nfio output:\n%s", err, string(output))
	}
	logger.Debug("validated fio file", "path", path)
	return nil
}

// newExecutionCoordinator creates a new coordinator and parses all fio files.
func newExecutionCoordinator(drives []DriveInfo, mode, outputDir string) (*ExecutionCoordinator, error) {
	// Check if fio is installed
	if err := checkFioInstalled(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	ec := &ExecutionCoordinator{
		drives:    drives,
		workloads: make(map[string]*FioWorkload),
		mode:      mode,
		outputDir: outputDir,
		results:   make(map[string][]*FioResult),
		ctx:       ctx,
		cancel:    cancel,
		startTime: time.Now(),
	}

	// Set up signal handling for graceful shutdown
	go ec.handleSignals()

	// Parse and validate all fio files
	for _, drive := range drives {
		workload, err := parseFioFile(drive.FioFilePath)
		if err != nil {
			return nil, fmt.Errorf("parse fio file for %s: %w", drive.Device.Name, err)
		}

		// Validate fio file syntax using fio --parse-only
		if err := validateFioFile(drive.FioFilePath); err != nil {
			return nil, fmt.Errorf("validate fio file for %s (%s): %w", drive.Device.Name, drive.FioFilePath, err)
		}

		ec.workloads[drive.Device.Name] = workload
		logger.Info("parsed fio file",
			"device", drive.Device.Name,
			"jobs", len(workload.Jobs),
			"phases", len(workload.Phases))
	}

	return ec, nil
}

// exitCodeSIGINT is the conventional exit code for processes terminated by SIGINT (128 + signal number).
const exitCodeSIGINT = 128 + int(syscall.SIGINT)

// handleSignals sets up signal handling for graceful shutdown.
func (ec *ExecutionCoordinator) handleSignals() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	fmt.Println("\n\nReceived interrupt signal. Gracefully shutting down...")
	fmt.Println("Waiting for current phase to complete...")
	fmt.Println("Press Ctrl+C again to force quit (may leave temporary files)")

	ec.mu.Lock()
	ec.interrupted = true
	ec.mu.Unlock()

	ec.cancel()

	// Set up second signal handler for force quit
	go func() {
		<-sigChan
		fmt.Println("\nForce quitting...")
		ec.cleanup()
		os.Exit(exitCodeSIGINT)
	}()
}

// cleanup removes temporary files.
func (ec *ExecutionCoordinator) cleanup() {
	ec.tempFilesMu.Lock()
	defer ec.tempFilesMu.Unlock()

	for _, tmpFile := range ec.tempFiles {
		if err := os.Remove(tmpFile); err != nil {
			logger.Debug("cleanup temp file", "path", tmpFile, "err", err)
		}
	}
}

// registerTempFile adds a temporary file to the cleanup list.
func (ec *ExecutionCoordinator) registerTempFile(path string) {
	ec.tempFilesMu.Lock()
	defer ec.tempFilesMu.Unlock()
	ec.tempFiles = append(ec.tempFiles, path)
}

// unregisterTempFile removes a temporary file from the cleanup list.
func (ec *ExecutionCoordinator) unregisterTempFile(path string) {
	ec.tempFilesMu.Lock()
	defer ec.tempFilesMu.Unlock()
	for i, f := range ec.tempFiles {
		if f == path {
			ec.tempFiles = append(ec.tempFiles[:i], ec.tempFiles[i+1:]...)
			break
		}
	}
}

// executeParallelSync runs all drives phase-by-phase with synchronization.
func (ec *ExecutionCoordinator) executeParallelSync() error {
	// Find maximum number of phases across all drives
	maxPhases := 0
	for _, workload := range ec.workloads {
		if len(workload.Phases) > maxPhases {
			maxPhases = len(workload.Phases)
		}
	}

	logger.Info("parallel-sync execution", "phases", maxPhases, "drives", len(ec.drives))
	fmt.Printf("Executing %d phase(s) across %d drive(s)\n\n", maxPhases, len(ec.drives))

	// Initialize web status
	UpdateStatus(func(s *BenchmarkStatus) {
		s.Running = true
		s.StartTime = ec.startTime
		s.Mode = ec.mode
		s.TotalPhases = maxPhases
		s.CompletedPhases = 0
		s.CurrentPhase = 0
		// Build drive status list
		s.Drives = make([]DriveStatus, len(ec.drives))
		for i, drive := range ec.drives {
			s.Drives[i] = DriveStatus{
				Name:   drive.Device.Name,
				Device: drive.Device.Name,
				Vendor: drive.Device.Vendor,
				Model:  drive.Device.Model,
			}
		}
	})

	// Execute each phase
	for phaseIdx := 0; phaseIdx < maxPhases; phaseIdx++ {
		// Check for interruption
		select {
		case <-ec.ctx.Done():
			fmt.Println("\nExecution interrupted. Cleaning up...")
			ec.cleanup()
			ec.saveState()
			return fmt.Errorf("execution interrupted by user")
		default:
		}

		// Check if all drives completed this phase (for resume)
		allCompleted := true
		for _, drive := range ec.drives {
			workload := ec.workloads[drive.Device.Name]
			if phaseIdx < len(workload.Phases) && !ec.isPhaseCompleted(drive.Device.Name, phaseIdx) {
				allCompleted = false
				break
			}
		}

		if allCompleted {
			fmt.Printf("Phase %d/%d: SKIPPED (already completed)\n", phaseIdx+1, maxPhases)
			continue
		}

		// Get phase name from first drive's workload for display
		phaseName := ""
		for _, drive := range ec.drives {
			workload := ec.workloads[drive.Device.Name]
			if phaseIdx < len(workload.Phases) && len(workload.Phases[phaseIdx]) > 0 {
				// Use first job name from the phase
				jobNames := make([]string, len(workload.Phases[phaseIdx]))
				for i, job := range workload.Phases[phaseIdx] {
					jobNames[i] = job.Name
				}
				phaseName = strings.Join(jobNames, ", ")
				break
			}
		}

		if phaseName != "" {
			fmt.Printf("Phase %d/%d (%s): ", phaseIdx+1, maxPhases, phaseName)
		} else {
			fmt.Printf("Phase %d/%d: ", phaseIdx+1, maxPhases)
		}

		// Update web status for current phase
		UpdateStatus(func(s *BenchmarkStatus) {
			s.CurrentPhase = phaseIdx + 1
		})

		var wg sync.WaitGroup
		var phaseErrors []error
		var mu sync.Mutex

		startTime := time.Now()

		for _, drive := range ec.drives {
			workload := ec.workloads[drive.Device.Name]

			// Skip if this drive has fewer phases
			if phaseIdx >= len(workload.Phases) {
				continue
			}

			// Skip if already completed in previous run
			if ec.isPhaseCompleted(drive.Device.Name, phaseIdx) {
				continue
			}

			wg.Add(1)
			go func(drv DriveInfo, wl *FioWorkload, idx int) {
				defer wg.Done()

				phase := wl.Phases[idx]
				result := ec.executePhase(drv, phase, idx)

				ec.mu.Lock()
				ec.results[drv.Device.Name] = append(ec.results[drv.Device.Name], result)
				ec.mu.Unlock()

				if result.Error != nil {
					mu.Lock()
					phaseErrors = append(phaseErrors, fmt.Errorf("%s: %w", drv.Device.Name, result.Error))
					mu.Unlock()
				}
			}(drive, workload, phaseIdx)
		}

		wg.Wait()
		elapsed := time.Since(startTime)

		// Update completed phases count
		UpdateStatus(func(s *BenchmarkStatus) {
			s.CompletedPhases = phaseIdx + 1
		})

		if len(phaseErrors) > 0 {
			fmt.Printf("FAILED (%.1fs) - errors:\n", elapsed.Seconds())
			for _, err := range phaseErrors {
				fmt.Printf("  - %v\n", err)
			}
		} else {
			fmt.Printf("COMPLETE (%.1fs)\n", elapsed.Seconds())
		}

		// Save state after each phase
		if err := ec.saveState(); err != nil {
			logger.Error("save state", "err", err)
		} else if ec.verbose {
			fmt.Printf("  [Saved state]\n")
		}
	}

	ec.cleanup()
	ec.cleanupStateFile()
	fmt.Println("\nAll phases complete!")
	return nil
}

// executeSequential runs benchmarks on drives one at a time.
func (ec *ExecutionCoordinator) executeSequential() error {
	logger.Info("sequential execution", "drives", len(ec.drives))
	fmt.Printf("Executing benchmarks sequentially on %d drive(s)\n\n", len(ec.drives))

	// Count total phases across all drives
	totalPhases := 0
	for _, workload := range ec.workloads {
		totalPhases += len(workload.Phases)
	}

	// Initialize web status
	UpdateStatus(func(s *BenchmarkStatus) {
		s.Running = true
		s.StartTime = ec.startTime
		s.Mode = ec.mode
		s.TotalPhases = totalPhases
		s.CompletedPhases = 0
		s.CurrentPhase = 0
		// Build drive status list
		s.Drives = make([]DriveStatus, len(ec.drives))
		for i, drive := range ec.drives {
			s.Drives[i] = DriveStatus{
				Name:   drive.Device.Name,
				Device: drive.Device.Name,
				Vendor: drive.Device.Vendor,
				Model:  drive.Device.Model,
			}
		}
	})

	completedPhases := 0

	for i, drive := range ec.drives {
		// Check for interruption
		select {
		case <-ec.ctx.Done():
			fmt.Println("\nExecution interrupted. Cleaning up...")
			ec.cleanup()
			return fmt.Errorf("execution interrupted by user")
		default:
		}

		fmt.Printf("Drive %d/%d: %s\n", i+1, len(ec.drives), drive.Device.Name)

		workload := ec.workloads[drive.Device.Name]
		startTime := time.Now()

		for phaseIdx, phase := range workload.Phases {
			// Check for interruption before each phase
			select {
			case <-ec.ctx.Done():
				fmt.Println("\n  Interrupted during execution. Cleaning up...")
				ec.cleanup()
				ec.saveState()
				return fmt.Errorf("execution interrupted by user")
			default:
			}

			// Skip if already completed in previous run
			if ec.isPhaseCompleted(drive.Device.Name, phaseIdx) {
				fmt.Printf("  Phase %d/%d: SKIPPED (already completed)\n", phaseIdx+1, len(workload.Phases))
				continue
			}

			// Get phase name from job names
			jobNames := make([]string, len(phase))
			for i, job := range phase {
				jobNames[i] = job.Name
			}
			phaseName := strings.Join(jobNames, ", ")

			fmt.Printf("  Phase %d/%d (%s): ", phaseIdx+1, len(workload.Phases), phaseName)
			phaseStart := time.Now()

			// Update web status
			UpdateStatus(func(s *BenchmarkStatus) {
				s.CurrentPhase = completedPhases + 1
			})

			result := ec.executePhase(drive, phase, phaseIdx)

			ec.mu.Lock()
			ec.results[drive.Device.Name] = append(ec.results[drive.Device.Name], result)
			ec.mu.Unlock()

			elapsed := time.Since(phaseStart)
			if result.Error != nil {
				fmt.Printf("FAILED (%.1fs) - %v\n", elapsed.Seconds(), result.Error)
			} else {
				fmt.Printf("COMPLETE (%.1fs)\n", elapsed.Seconds())
			}

			// Update completed phases count
			completedPhases++
			UpdateStatus(func(s *BenchmarkStatus) {
				s.CompletedPhases = completedPhases
			})

			// Save state after each phase
			if err := ec.saveState(); err != nil {
				logger.Error("save state", "err", err)
			} else if ec.verbose {
				fmt.Printf("    [Saved state]\n")
			}
		}

		totalElapsed := time.Since(startTime)
		fmt.Printf("  Total: %.1f minutes\n\n", totalElapsed.Minutes())
	}

	ec.cleanup()
	ec.cleanupStateFile()
	fmt.Println("All drives complete!")
	return nil
}

// executePhase executes a single phase for a single drive.
func (ec *ExecutionCoordinator) executePhase(drive DriveInfo, phase []FioJob, phaseIdx int) *FioResult {
	result := &FioResult{
		DeviceName: drive.Device.Name,
		Phase:      phaseIdx,
		StartTime:  time.Now(),
	}

	// Get first job name for logging
	if len(phase) > 0 {
		result.JobName = phase[0].Name
	}

	workload := ec.workloads[drive.Device.Name]

	// Build temporary fio file for this phase
	phaseFioContent := buildPhaseFioFile(workload, phase)
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("bbbench-phase-%s-*.fio", drive.Device.Name))
	if err != nil {
		result.Error = fmt.Errorf("create temp fio file: %w", err)
		result.EndTime = time.Now()
		return result
	}
	tmpPath := tmpFile.Name()
	ec.registerTempFile(tmpPath)
	defer func() {
		os.Remove(tmpPath)
		ec.unregisterTempFile(tmpPath)
	}()

	if _, err := tmpFile.WriteString(phaseFioContent); err != nil {
		tmpFile.Close()
		result.Error = fmt.Errorf("write temp fio file: %w", err)
		result.EndTime = time.Now()
		return result
	}
	tmpFile.Close()

	// Construct output path
	timestamp := time.Now().Unix()
	outputFile := filepath.Join(ec.outputDir, fmt.Sprintf("%s_phase%d_%d.json",
		drive.Device.Name, phaseIdx, timestamp))
	result.OutputPath = outputFile

	// Execute fio with context for cancellation support
	cmd := exec.CommandContext(ec.ctx, "fio",
		"--output-format=json",
		"--output="+outputFile,
		tmpPath)

	if ec.verbose {
		fmt.Printf("\n[%s Phase %d] Command: fio --output-format=json --output=%s %s\n",
			drive.Device.Name, phaseIdx, outputFile, tmpPath)
		fmt.Printf("[%s Phase %d] Jobs: ", drive.Device.Name, phaseIdx)
		jobNames := make([]string, len(phase))
		for i, job := range phase {
			jobNames[i] = job.Name
		}
		fmt.Printf("%s\n", strings.Join(jobNames, ", "))
	}

	logger.Debug("executing fio",
		"device", drive.Device.Name,
		"phase", phaseIdx,
		"output", outputFile)

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if it was cancelled
		if ec.ctx.Err() != nil {
			result.Error = fmt.Errorf("fio execution cancelled")
		} else {
			result.Error = fmt.Errorf("fio execution failed: %w\nOutput: %s", err, string(output))
			if ec.verbose {
				fmt.Printf("[%s Phase %d] Error output:\n%s\n", drive.Device.Name, phaseIdx, string(output))
			}
		}
		result.EndTime = time.Now()
		return result
	}

	if ec.verbose && len(output) > 0 {
		fmt.Printf("[%s Phase %d] fio stderr/stdout:\n%s\n", drive.Device.Name, phaseIdx, string(output))
	}

	// Read the JSON output
	jsonData, err := os.ReadFile(outputFile)
	if err != nil {
		result.Error = fmt.Errorf("read fio output: %w", err)
		result.EndTime = time.Now()
		return result
	}

	result.JsonOutput = jsonData
	result.EndTime = time.Now()
	return result
}
