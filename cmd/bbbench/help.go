package main

import "fmt"

func printHelp() {
	fmt.Print(`bbbench - fio job generator

Usage:
  bbbench <command> [flags]

Commands:
  generate             generate fio job files
  orchestrator         run benchmarks on multiple drives
  validate-templates   validate template syntax
  doctor               check host readiness
  completion           generate shell completions
  help                 show this help

Environment:
  BBBENCH_LOG_LEVEL    debug|info|warn|error
  BBBENCH_DIST_DIR     overrides dist search path
`)
}
