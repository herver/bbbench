package main

import (
	"os"
)

func exitWithError(err error) {
	if err == nil {
		os.Exit(0)
	}
	logger.Error("fatal", "err", err)
	// Keep a simple human-friendly marker for CLI users.
	os.Stderr.WriteString("✗ " + err.Error() + "\n")
	os.Exit(1)
}
