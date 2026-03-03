package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	distassets "bbbench/dist"
	"bbbench/internal/blockdev"
	"bbbench/internal/config"
	"bbbench/internal/dmi"
	"bbbench/internal/scsi"
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

	for _, disk := range devs {
		var wcePtr *bool
		if disk.ParentSubsystem == "scsi" && disk.Rotational {
			wce, err := scsi.GetWCE(disk.Path)
			if err == nil {
				wcePtr = &wce
				if cfg.BBBench.WCE != wce {
					fmt.Printf("\twce: %s current:%v target:%v\n", disk.Path, wce, cfg.BBBench.WCE)
				}
			}
		}

		workloadType := disk.WorkloadType()
		workloads := cfg.BBBench.Workloads[workloadType]

		logger.Info("disk workloads", "device", disk.Path, "type", workloadType, "workloads_count", len(workloads))
		if len(workloads) == 0 {
			logger.Warn("no workloads defined for device type", "type", workloadType, "device", disk.Path)
		}

		diskMap := map[string]any{
			"path":        disk.Path,
			"model":       util.SanitizeFilename(disk.Model, "_"),
			"serial":      util.SanitizeFilename(disk.Serial, "_"),
			"rotational":  disk.Rotational,
			"capacity":    disk.CapacityBytes,
			"capacity_gb": disk.CapacityGB(),
			"block_count": disk.BlockCount,
			"block_size":  disk.BlockSize,
			"vendor":      strings.TrimSpace(disk.Vendor),
		}

		manufacturer := util.SanitizeFilename(strings.TrimSpace(disk.Vendor), "_")
		if manufacturer == "" {
			manufacturer = "unknown"
		}

		path := filepath.Join(outBase, host)
		deviceName := filepath.Base(disk.Path)
		filename := util.SanitizeFilename(fmt.Sprintf("%s_%s_%s_%s_%s_config.fio", host, manufacturer, disk.Model, disk.Serial, disk.ParentSubsystem), "_")
		filenameFioplot := util.SanitizeFilename(fmt.Sprintf("%s_%s_%s_%s_%s_fioplot.yml", host, manufacturer, disk.Model, disk.Serial, disk.ParentSubsystem), "_")

		if err := os.MkdirAll(path, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", path, err)
		}

		ctx := map[string]any{
			"Manufacturer": manufacturer,
			"WorkloadsDB":  cfg.BBBench.WorkloadsDB,
			"Workloads":    workloads,
			"Disk":         diskMap,
			"Device":       deviceName,
			"Serial":       util.SanitizeFilename(disk.Serial, "_"),
			"Host":         host,
			"WCE":          wcePtr,
		}

		fioOut, err := eng.Render("disk.fio", ctx)
		if err != nil {
			return fmt.Errorf("render fio: %w", err)
		}
		fioPath := filepath.Join(path, filename)
		if err := os.WriteFile(fioPath, []byte(fioOut), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", fioPath, err)
		}
		fmt.Printf("\t%s\n", fioPath)

		plotOut, err := eng.Render("output.yml", ctx)
		if err != nil {
			return fmt.Errorf("render fioplot: %w", err)
		}
		plotPath := filepath.Join(path, filenameFioplot)
		if err := os.WriteFile(plotPath, []byte(plotOut), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", plotPath, err)
		}
		fmt.Printf("\t%s\n", plotPath)
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
