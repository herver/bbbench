# bbbench (Go)

`bbbench` generates **fio job files** for block-device benchmarking using:
- a YAML config (`default.yml`)
- Go `text/template` templates (Sprig functions available)
- host introspection (sysfs + optional SCSI write-cache check)

This repository builds a **single CLI binary**: `bbbench`.

## Requirements

- Linux
- Root privileges for `bbbench generate`, `bbbench doctor`, and `bbbench orchestrator`
- Go >= 1.21 to build
- `fio` to run the generated jobs (required for `bbbench orchestrator`)

## Build

```bash
go build ./cmd/bbbench
```

Binary: `./bbbench`

## Commands

### `bbbench generate`

Generate fio job files.

```bash
sudo ./bbbench generate
sudo ./bbbench generate --config /path/to/my.yml --templates /path/to/templates --out ./jobs
```

Flags:
- `--config` path to config file. If omitted, bbbench uses the **search order** described below.
- `--templates` path to a templates directory (a directory containing `*.gotmpl` and an `includes/` subdir). If omitted, bbbench uses the **search order** described below.
- `--out` overrides `fio.generated.path` from the config.
- `--dist` overrides the dist path used by the search order (default `./dist`, or `BBBENCH_DIST_DIR`).

### `bbbench validate-templates`

Validate template syntax without generating anything.

```bash
./bbbench validate-templates
./bbbench validate-templates --templates ./dist/templates
```

### `bbbench doctor`

Check host readiness (Linux, root, sysfs, block devices).

```bash
sudo ./bbbench doctor
```

### `bbbench orchestrator`

Run fio benchmarks on multiple drives with interactive drive selection.

```bash
sudo ./bbbench orchestrator
sudo ./bbbench orchestrator --mode sequential --output ~/benchmark-results
```

Flags:
- `--mode` execution mode: `parallel-sync` (default) or `sequential`
  - `parallel-sync`: All drives run phase 1, then all run phase 2, etc.
  - `sequential`: Complete benchmark on drive 1, then drive 2, etc.
- `--dry-run` show what would be executed without running fio
- `--resume` resume from previous interrupted run (skips completed phases)
- `--verbose` show detailed execution information (commands, stderr, debug info)
- `--filter-type` filter drives by type: `hdd` or `ssd`
- `--filter-vendor` filter drives by vendor (case-insensitive substring match)
- `--filter-model` filter drives by model (case-insensitive substring match)
- `--filter-min-capacity` filter drives by minimum capacity in GB
- `--filter-max-capacity` filter drives by maximum capacity in GB (0 = no limit)
- `--output` output directory for JSON results (default: from config `fioplot.output.path`)
- `--config` path to config file (uses search order if omitted)
- `--dist` overrides dist path

Filtering examples:
```bash
# Only SSD drives
sudo ./bbbench orchestrator --filter-type ssd

# Only Samsung drives
sudo ./bbbench orchestrator --filter-vendor samsung

# Drives between 1TB and 2TB
sudo ./bbbench orchestrator --filter-min-capacity 1000 --filter-max-capacity 2000

# Combine filters: Samsung SSDs over 500GB
sudo ./bbbench orchestrator --filter-type ssd --filter-vendor samsung --filter-min-capacity 500
```

The orchestrator command:
1. Discovers block devices and matches them with generated fio files
2. Presents an interactive TUI for drive selection (Space=toggle, Enter=confirm, q=quit)
3. Executes benchmarks in the chosen mode
4. Saves JSON results for each phase to the output directory
5. Displays a summary of IOPS and bandwidth results

**Note**: You must run `bbbench generate` first to create the fio job files.

### `bbbench completion`

Generate shell completion scripts.

```bash
./bbbench completion bash
./bbbench completion zsh
./bbbench completion fish
```

Install examples:

```bash
# Bash
./bbbench completion bash | sudo tee /etc/bash_completion.d/bbbench

# Zsh
./bbbench completion zsh > ~/.zsh/completions/_bbbench

# Fish
./bbbench completion fish > ~/.config/fish/completions/bbbench.fish
```

## Config and template discovery

If `--config` is not provided, bbbench searches for `default.yml` in this order:

1. `~/.bbbench/config/default.yml`
2. `/etc/bbbench/default.yml`
3. `<dist>/default.yml` where `<dist>` defaults to `./dist` (or overridden via `--dist` or `BBBENCH_DIST_DIR`)
4. Embedded fallback bundled in the binary

If `--templates` is not provided, bbbench searches for a templates directory in this order:

1. `~/.bbbench/config/templates/`
2. `/etc/bbbench/templates/`
3. `<dist>/templates/` (same `<dist>` rules)
4. Embedded fallback bundled in the binary

## Logging

bbbench uses Go `log/slog`.

Set log level with:

```bash
BBBENCH_LOG_LEVEL=debug ./bbbench doctor
```

Supported: `debug`, `info`, `warn`, `error`.

## dist/

The `dist/` folder is the "distribution" tree containing:

- `dist/default.yml`
- `dist/templates/*.gotmpl`
- `dist/templates/includes/*.gotmpl`

These assets are also embedded into the binary as a fallback.

## Testing

```bash
go test ./...
```

This includes a unit test that parses all embedded templates to ensure they are always valid.
