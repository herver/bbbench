package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	distassets "bbbench/dist"
	"bbbench/internal/blockdev"
	"bbbench/internal/config"
	"bbbench/internal/dmi"
	"bbbench/internal/templating"
	"bbbench/internal/util"
)

func runGenerate(args []string) error {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)
	configPath := fs.String("config", "", "config file (default: search ~/.bbbench/config, /etc/bbbench, dist)")
	templatesDir := fs.String("templates", "", "templates dir (directory containing *.gotmpl and includes/)")
	distDir := fs.String("dist", "", "dist directory (default ./dist or BBBENCH_DIST_DIR)")
	outDirOverride := fs.String("out", "", "override fio.generated.path")
	dmiPath := fs.String("dmi-path", "", "override /sys/class/dmi/id (tests)")
	sysBlock := fs.String("sys-block", "", "override /sys/block (tests)")
	_ = fs.Parse(args)

	if os.Geteuid() != 0 {
		return errors.New("bbbench generate must run as root")
	}

	dir := resolveDistDir(*distDir)

	cfg, err := loadConfigWithSearch(*configPath, dir)
	if err != nil {
		return err
	}

	outBase := cfg.Fio.Generated.Path
	if *outDirOverride != "" {
		outBase = *outDirOverride
	}
	outBase = expandUser(outBase)

	eng, err := loadTemplatesWithSearch(*templatesDir, dir)
	if err != nil {
		return err
	}

	dmiInfo, err := dmi.Load(dmi.OSReader{}, *dmiPath)
	if err != nil {
		return fmt.Errorf("read dmi: %w", err)
	}
	chassis, err := dmiInfo.ChassisSerial()
	if err != nil {
		return fmt.Errorf("read chassis_serial: %w", err)
	}
	host := util.SanitizeFilename(chassis, "_")

	devs, err := blockdev.Discover(blockdev.OSFS{}, *sysBlock)
	if err != nil {
		return fmt.Errorf("discover block devices: %w", err)
	}

	logger.Info("generation start", "host", chassis, "out_base", outBase, "disks", len(devs))
	fmt.Printf("Write cache on scsi drives should be %v\n", cfg.BBBench.WCE)
	fmt.Printf("%s:\n", chassis)

	// Convert map to slice for shared generation logic
	var devSlice []blockdev.Device
	for _, dev := range devs {
		devSlice = append(devSlice, dev)
	}

	// Use shared generation logic
	if err := generateFioFilesForDrives(cfg, eng, devSlice, host, outBase); err != nil {
		return err
	}

	logger.Info("generation complete")
	return nil
}

func loadConfigWithSearch(overridePath string, distDir string) (*config.Root, error) {
	if overridePath != "" {
		return config.Load(expandUser(overridePath))
	}
	if p := resolveConfigPath(distDir); p != "" {
		logger.Info("using config", "path", p)
		return config.Load(p)
	}
	logger.Info("using embedded config")
	return config.LoadFS(distassets.FS, "default.yml")
}

func loadTemplatesWithSearch(overrideTemplatesDir string, distDir string) (*templating.Engine, error) {
	if overrideTemplatesDir != "" {
		dir := expandUser(overrideTemplatesDir)
		logger.Info("using templates", "dir", dir)
		return templating.EngineFromTemplatesDir(dir)
	}
	if dir, _ := resolveTemplatesDir(distDir); dir != "" {
		logger.Info("using templates", "dir", dir)
		return templating.EngineFromTemplatesDir(dir)
	}
	logger.Info("using embedded templates")
	return templating.DefaultEngine()
}
