# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0-alpha] - 2026-08-19

### Added

#### CLI Subcommands

- `pgoctl collect` — fetch CPU profiles from pprof (`/debug/pprof/profile`) or Parca endpoints; configurable service name, time window, and output path
- `pgoctl validate` — validate a `.pprof` file against quality gates: minimum sample count, aggregate score, and per-package CPU coverage thresholds; supports `--format json` for machine-readable output and all flags settable via env var or config file
- `pgoctl merge` — merge one or more `.pprof` files into a single `default.pgo` artifact ready for `go build -pgo`
- `pgoctl explain` — analyse a `.pgo` file and display the top-N hottest CPU functions grouped by package, with a PGO readiness verdict; supports `--format json`
- `pgoctl compare` — diff two benchmark result files (JSON) and report per-function CPU deltas, surfacing regressions and improvements
- `--version` flag on all subcommands with consistent exit codes: `0` success, `1` validation failure, `2` usage error

#### Parca Adapter

- Native Parca OSS profile source for `pgoctl collect`: authenticate, query by service label and time window, and write the resulting `.pprof` to disk

#### Configuration System

- All `pgoctl validate` flags configurable via CLI flag, environment variable (`PGOCTL_<FLAG>`), or `pgoctl.conf` / `pgoctl.yaml` config file; precedence is CLI > env > file > built-in default
- Config file discovery: current directory → `~/.config/pgoctl/` → `/etc/pgoctl/`; first match wins, missing file is not an error

#### CI & Benchmarking Workflows

- `ci.yml` — GitHub Actions workflow: build, vet, and test on every push and pull request
- `pgo-bench.yml` — 10-round PGO benchmark workflow: pre-compiles baseline and PGO binaries once, runs compute-bound micro-benchmarks across `tsdb`, `promql`, `storage`, and `tsdb/chunkenc`, and posts a benchstat summary as a sticky PR comment
- `profile-collect.yml` — end-to-end profile-collection workflow: provisions a kind cluster with Prometheus, drives load via the built-in load generator, captures a CPU profile, validates coverage gates, and merges into `default.pgo`

[Unreleased]: https://github.com/better-go-labs/pgoctl/compare/v0.1.0-alpha...HEAD
[0.1.0-alpha]: https://github.com/better-go-labs/pgoctl/releases/tag/v0.1.0-alpha
