# bbbench (Go)

`bbbench` generates **fio job files** for block-device benchmarking using:
- a YAML config (`default.yml`)
- Go `text/template` templates (Sprig functions available)
- host introspection (sysfs + optional SCSI write-cache check)

This repository builds a **single CLI binary**: `bbbench`.

## Requirements

- Linux
- Root privileges for `bbbench generate` (reads sysfs and may perform SCSI SG_IO)
- Go >= 1.21 to build
- `fio` to run the generated jobs

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
