// Package main implements the orchestrator command for running fio benchmarks
// across multiple drives with synchronized or sequential execution.
//
// Usage:
//
//	sudo ./bbbench orchestrator [flags]
//
// The orchestrator command:
//  1. Discovers block devices and matches them with generated fio files
//  2. Presents an interactive TUI for drive selection
//  3. Executes benchmarks in the chosen mode (parallel-sync or sequential)
//  4. Saves JSON results for each phase
//  5. Displays a summary of results
//
// Execution modes:
//   - parallel-sync: All drives run phase 1, then all run phase 2, etc.
//   - sequential: Complete benchmark on drive 1, then drive 2, etc.
//
// Flags:
//
//	--mode string        Execution mode: parallel-sync or sequential (default "parallel-sync")
//	--output string      Output directory for results (default: from config)
//	--config string      Config file path
//	--dist string        Dist directory
//
// Example:
//
//	sudo ./bbbench generate                              # Generate fio files first
//	sudo ./bbbench orchestrator --mode=parallel-sync    # Run benchmarks
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bbbench/internal/blockdev"
	"bbbench/internal/config"
	"bbbench/internal/dmi"
	"bbbench/internal/util"
)

// DriveInfo holds information about a drive and its associated fio file.
type DriveInfo struct {
	Device      blockdev.Device
	FioFilePath string
	FioExists   bool
	Selected    bool
}

func runOrchestrator(args []string) error {
	fs := flag.NewFlagSet("orchestrator", flag.ExitOnError)
	configPath := fs.String("config", "", "config file (default: search ~/.bbbench/config, /etc/bbbench, dist)")
	distDir := fs.String("dist", "", "dist directory (default ./dist or BBBENCH_DIST_DIR)")
	mode := fs.String("mode", "parallel-sync", "execution mode: parallel-sync or sequential")
	outputDir := fs.String("output", "", "output directory for results (default: from config fioplot.output.path)")
	dmiPath := fs.String("dmi-path", "", "override /sys/class/dmi/id (tests)")
	sysBlock := fs.String("sys-block", "", "override /sys/block (tests)")
	_ = fs.Parse(args)

	if os.Geteuid() != 0 {
		return errors.New("bbbench orchestrator must run as root")
	}

	if *mode != "parallel-sync" && *mode != "sequential" {
		return fmt.Errorf("invalid mode: %s (must be parallel-sync or sequential)", *mode)
	}

	dir := resolveDistDir(*distDir)

	cfg, err := loadConfigWithSearch(*configPath, dir)
	if err != nil {
		return err
	}

	// Discover drives and match with fio files
	drives, err := discoverDrivesWithFioFiles(cfg, *dmiPath, *sysBlock)
	if err != nil {
		return err
	}

	if len(drives) == 0 {
		return errors.New("no drives with generated fio files found. Run 'bbbench generate' first")
	}

	// Launch TUI for drive selection
	selectedDrives, err := runDriveSelectionTUI(drives)
	if err != nil {
		return err
	}

	if len(selectedDrives) == 0 {
		return errors.New("no drives selected")
	}

	// Determine output directory
	outDir := *outputDir
	if outDir == "" {
		// Extract output path from fioplot map
		if fioplot, ok := cfg.Fioplot.(map[string]any); ok {
			if output, ok := fioplot["output"].(map[string]any); ok {
				if path, ok := output["path"].(string); ok {
					outDir = path
				}
			}
		}
		if outDir == "" {
			outDir = "~/.bbbench/output" // default fallback
		}
	}
	outDir = expandUser(outDir)

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	logger.Info("orchestrator start", "mode", *mode, "drives", len(selectedDrives), "output", outDir)
	fmt.Printf("Running benchmarks on %d drive(s) in %s mode\n", len(selectedDrives), *mode)
	fmt.Printf("Output directory: %s\n\n", outDir)

	// Parse fio files
	coordinator, err := newExecutionCoordinator(selectedDrives, *mode, outDir)
	if err != nil {
		return err
	}

	// Execute based on mode
	if *mode == "parallel-sync" {
		if err := coordinator.executeParallelSync(); err != nil {
			return err
		}
	} else {
		if err := coordinator.executeSequential(); err != nil {
			return err
		}
	}

	// Display results
	if err := displayResults(coordinator); err != nil {
		return err
	}

	logger.Info("orchestrator complete")
	return nil
}

// discoverDrivesWithFioFiles discovers drives and matches them with generated fio files.
func discoverDrivesWithFioFiles(cfg *config.Root, dmiPath, sysBlock string) ([]DriveInfo, error) {
	// Load DMI info for host identification
	dmiInfo, err := dmi.Load(dmi.OSReader{}, dmiPath)
	if err != nil {
		return nil, fmt.Errorf("read dmi: %w", err)
	}
	chassis, err := dmiInfo.ChassisSerial()
	if err != nil {
		return nil, fmt.Errorf("read chassis_serial: %w", err)
	}
	host := util.SanitizeFilename(chassis, "_")

	// Discover block devices
	devs, err := blockdev.Discover(blockdev.OSFS{}, sysBlock)
	if err != nil {
		return nil, fmt.Errorf("discover block devices: %w", err)
	}

	// Get the base output path from config
	outBase := expandUser(cfg.Fio.Generated.Path)
	hostDir := filepath.Join(outBase, host)

	// Match devices with fio files
	var drives []DriveInfo
	for _, disk := range devs {
		manufacturer := util.SanitizeFilename(strings.TrimSpace(disk.Vendor), "_")
		if manufacturer == "" {
			manufacturer = "unknown"
		}

		// Construct expected fio filename (matches generate.go pattern)
		filename := util.SanitizeFilename(fmt.Sprintf(
			"%s_%s_%s_%s_%s_config.fio",
			host, manufacturer, disk.Model, disk.Serial, disk.ParentSubsystem,
		), "_")

		fioPath := filepath.Join(hostDir, filename)
		fioExists := false
		if _, err := os.Stat(fioPath); err == nil {
			fioExists = true
		}

		drives = append(drives, DriveInfo{
			Device:      disk,
			FioFilePath: fioPath,
			FioExists:   fioExists,
			Selected:    false,
		})
	}

	return drives, nil
}
