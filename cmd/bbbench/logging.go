package main

import (
	"log/slog"
	"os"
	"strings"
)

var logger *slog.Logger

func initLoggerFromEnv() {
	level := strings.ToLower(os.Getenv("BBBENCH_LOG_LEVEL"))
	if level == "" {
		level = "info"
	}

	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl}))
}
