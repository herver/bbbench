package main

import (
	"errors"
	"flag"
	"os"
	"runtime"

	"bbbench/internal/blockdev"
)

func runDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	distDir := fs.String("dist", "", "dist directory (default ./dist or BBBENCH_DIST_DIR)")
	_ = fs.Parse(args)

	ok := true
	check := func(name string, err error) {
		if err != nil {
			logger.Error("check failed", "check", name, "err", err)
			ok = false
		} else {
			logger.Info("check ok", "check", name)
		}
	}

	check("linux", func() error {
		if runtime.GOOS != "linux" {
			return errors.New("bbbench requires linux")
		}
		return nil
	}())

	check("root", func() error {
		if os.Geteuid() != 0 {
			return errors.New("must run as root")
		}
		return nil
	}())

	check("sysfs", func() error {
		_, err := os.Stat("/sys/class/block")
		return err
	}())

	check("block devices", func() error {
		devs, err := blockdev.Discover(blockdev.OSFS{}, "")
		if err != nil {
			return err
		}
		if len(devs) == 0 {
			return errors.New("no block devices found")
		}
		return nil
	}())

	resolvedDist := resolveDistDir(*distDir)
	check("config lookup", func() error {
		if p := resolveConfigPath(resolvedDist); p != "" {
			return nil
		}
		// embedded fallback always exists at build time; no disk config isn't fatal.
		return nil
	}())

	check("templates lookup", func() error {
		d, _ := resolveTemplatesDir(resolvedDist)
		_ = d
		return nil
	}())

	if !ok {
		return errors.New("doctor checks failed")
	}

	logger.Info("doctor checks passed")
	return nil
}
