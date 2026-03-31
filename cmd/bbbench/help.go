package main

import "fmt"

const version = "0.1.0"

func printHelp() {
	fmt.Printf(`bbbench %s - fio job generator

Usage:
  bbbench <command> [flags]

Commands:
  generate             generate fio job files
  orchestrator         run benchmarks on multiple drives
  serve                start web server GUI (default [::1]:12345)
  validate-templates   validate template syntax
  doctor               check host readiness
  completion           generate shell completions
  help                 show this help

Environment:
  BBBENCH_LOG_LEVEL    debug|info|warn|error
  BBBENCH_DIST_DIR     overrides dist search path
`, version)
}
