package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// ExecutionCoordinator manages benchmark execution across multiple drives.
type ExecutionCoordinator struct {
	drives    []DriveInfo
	workloads map[string]*FioWorkload // keyed by device name
	mode      string
	outputDir string
	dryRun    bool
	results   map[string][]*FioResult
	mu        sync.Mutex
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

// newExecutionCoordinator creates a new coordinator and parses all fio files.
func newExecutionCoordinator(drives []DriveInfo, mode, outputDir string) (*ExecutionCoordinator, error) {
	ec := &ExecutionCoordinator{
		drives:    drives,
		workloads: make(map[string]*FioWorkload),
		mode:      mode,
		outputDir: outputDir,
		results:   make(map[string][]*FioResult),
	}

	// Parse all fio files
	for _, drive := range drives {
		workload, err := parseFioFile(drive.FioFilePath)
		if err != nil {
			return nil, fmt.Errorf("parse fio file for %s: %w", drive.Device.Name, err)
		}
		ec.workloads[drive.Device.Name] = workload
		logger.Info("parsed fio file",
			"device", drive.Device.Name,
			"jobs", len(workload.Jobs),
			"phases", len(workload.Phases))
	}

	return ec, nil
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

	// Execute each phase
	for phaseIdx := 0; phaseIdx < maxPhases; phaseIdx++ {
		fmt.Printf("Phase %d/%d: ", phaseIdx+1, maxPhases)

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

		if len(phaseErrors) > 0 {
			fmt.Printf("FAILED (%.1fs) - errors:\n", elapsed.Seconds())
			for _, err := range phaseErrors {
				fmt.Printf("  - %v\n", err)
			}
		} else {
			fmt.Printf("COMPLETE (%.1fs)\n", elapsed.Seconds())
		}
	}

	fmt.Println("\nAll phases complete!")
	return nil
}

// executeSequential runs benchmarks on drives one at a time.
func (ec *ExecutionCoordinator) executeSequential() error {
	logger.Info("sequential execution", "drives", len(ec.drives))
	fmt.Printf("Executing benchmarks sequentially on %d drive(s)\n\n", len(ec.drives))

	for i, drive := range ec.drives {
		fmt.Printf("Drive %d/%d: %s\n", i+1, len(ec.drives), drive.Device.Name)

		workload := ec.workloads[drive.Device.Name]
		startTime := time.Now()

		for phaseIdx, phase := range workload.Phases {
			fmt.Printf("  Phase %d/%d: ", phaseIdx+1, len(workload.Phases))
			phaseStart := time.Now()

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
		}

		totalElapsed := time.Since(startTime)
		fmt.Printf("  Total: %.1f minutes\n\n", totalElapsed.Minutes())
	}

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
	defer os.Remove(tmpPath)

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

	// Execute fio
	cmd := exec.Command("fio",
		"--output-format=json",
		"--output="+outputFile,
		tmpPath)

	logger.Debug("executing fio",
		"device", drive.Device.Name,
		"phase", phaseIdx,
		"output", outputFile)

	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Error = fmt.Errorf("fio execution failed: %w\nOutput: %s", err, string(output))
		result.EndTime = time.Now()
		return result
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
