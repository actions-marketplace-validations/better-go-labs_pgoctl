# pgoctl

> Continuous PGO and profile-guided optimization for Go workloads on Kubernetes.

`pgoctl` is the CLI backbone of GoOpt — a Kubernetes-native control plane that turns production Go profiles into optimized builds with measurable CPU and latency gains.

## What it does

Production profiles from your Go services → validated PGO artifacts → optimized builds → safe canary rollouts → cost reports.

```
pgoctl collect  --source=parca --url=http://parca:7070 --duration=30
pgoctl validate cpu.pprof
pgoctl merge    profiles/*.pprof --out default.pgo
pgoctl explain  default.pgo
pgoctl compare  baseline.pprof candidate.pprof
```

## Quick start

```bash
# Build pgoctl
go build -o bin/pgoctl ./cmd/pgoctl

# Run the full demo (collect → validate → merge → build → explain)
PARCA_URL=http://localhost:7070 ./demo.sh

# Run the e2e smoke test (no external service required)
make smoke
```

## CLI reference

### `pgoctl collect`

Fetch a CPU profile from a running service.

```
pgoctl collect --source=parca --url=<base-url> [--duration=30] [--out=cpu.pprof]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--source` | `parca` | Profiling backend. Currently: `parca` |
| `--url` | _(required)_ | Base URL of the service (e.g. `http://parca:7070`) |
| `--duration` | `30` | Collection duration in seconds |
| `--out` | `cpu.pprof` | Output path (`-` for stdout) |

Fetches `/debug/pprof/profile?seconds=<duration>` from the target URL and validates the response is a parseable pprof file before writing.

---

### `pgoctl validate`

Score a CPU pprof for quality before merging.

```
pgoctl validate [flags] <path>
```

| Flag | Default | Description |
|------|---------|-------------|
| `--min-samples` | `10000` | Minimum sample count |
| `--min-duration` | `10.0` | Minimum profile duration in seconds |
| `--min-score` | `0.6` | Minimum quality score (0–1) |
| `--min-package-share` | — | Minimum combined flat CPU % for a package prefix (e.g. `tsdb:5`) |
| `--json` | `false` | JSON output |

Exit codes: **0** = valid, **1** = below quality gate, **2** = input error.

Flags can also be set via env vars (`PGOCTL_MIN_SAMPLES=…`) or a `pgoctl.conf` YAML file. See [Configuration](#configuration).

---

### `pgoctl merge`

Merge validated CPU profiles into a `default.pgo` artifact.

```
pgoctl merge [flags] <profile...>
```

| Flag | Default | Description |
|------|---------|-------------|
| `--strategy` | `weighted` | Merge strategy: `weighted`, `latest`, `union` |
| `--recency-weight` | `2.0` | Multiplier for the most recent profile |
| `--half-life` | `24.0` | Recency decay half-life in hours |
| `--drop-invalid` | `false` | Skip unparseable profiles instead of failing |
| `--out` | `default.pgo` | Output path (`-` for stdout) |

---

### `pgoctl explain`

Analyse a pprof file in human-readable form.

```
pgoctl explain [--top N] [--format text|json] <path>
```

| Flag | Default | Description |
|------|---------|-------------|
| `--top` | `20` | Number of top functions to show |
| `--format` | `text` | Output format: `text` or `json` |

Prints the top hot functions by flat CPU share, groups them by package, and gives a plain-English PGO readiness verdict:

- **ready** — ≥ 50 000 samples across ≥ 20 functions: good PGO baseline
- **borderline** — 10 000–49 999 samples: will work, denser profile improves inlining
- **not-ready** — < 10 000 samples or < 20 functions: collect a richer profile first

---

### `pgoctl compare`

Compare two CPU profiles and emit a gate verdict.

```
pgoctl compare [flags] <baseline.pprof> <candidate.pprof>
```

| Flag | Default | Description |
|------|---------|-------------|
| `--min-improvement` | `3.0` | Min CPU delta % to promote |
| `--min-regression` | `3.0` | Min CPU regression % to rollback |
| `--min-cpu-percent` | `0.0` | Drop functions below this CPU % in both profiles |
| `--top` | `10` | Number of function deltas to show |
| `--json` | `false` | JSON output |

Verdict: **promote** (improvement ≥ threshold), **rollback** (regression ≥ threshold), or **neutral**.
Exit codes: **0** = promote or neutral, **1** = rollback, **2** = input error.

---

## Configuration

All `pgoctl validate` flags can be set from a config file or environment variable.

| Source | Example |
|--------|---------|
| CLI flag | `pgoctl validate --min-score 0.9 cpu.pprof` |
| Env var | `PGOCTL_MIN_SCORE=0.9 pgoctl validate cpu.pprof` |
| Config file | `pgoctl.conf` (YAML) — see [pgoctl.conf.example](pgoctl.conf.example) |

Precedence: **CLI flag > env var > config file > built-in default**.

Config file is discovered as `pgoctl.conf` in `./`, `~/.config/pgoctl/`, then `/etc/pgoctl/`. Missing file is not an error.

```yaml
# pgoctl.conf (example)
min-samples: 1000
min-score: 0.3
min-package-share:
  - github.com/prometheus/prometheus/tsdb:5.0
  - github.com/prometheus/prometheus/promql:1.5
```

## Demo service

We benchmark against **Prometheus** — pure Go, pprof-enabled by default, widely deployed on Kubernetes.

## Requirements

- Go 1.23+
- [kind](https://kind.sigs.k8s.io/) + [kubectl](https://kubernetes.io/docs/tasks/tools/) + [helm](https://helm.sh/) (for local dev cluster)
- [hey](https://github.com/rakyll/hey) (load generator, optional)

## Project layout

```
cmd/
  pgoctl/     — CLI entry point (validate/merge/compare/explain/collect)
  baseline/   — standalone pprof collector for dev/baseline capture
internal/
  collect/    — Parca HTTP adapter and source interface
  compare/    — profile comparison and gate logic
  explain/    — flat CPU attribution, package grouping, PGO verdict
  merge/      — weighted profile merge strategies
  validate/   — quality scoring, package-share gates
scripts/
  kind-prometheus.sh  — provision kind cluster + kube-prometheus-stack
  smoke.sh            — e2e smoke test (no external service required)
testdata/             — captured .pprof files (LFS)
demo.sh               — interactive happy-path demo
BENCHMARKS.md         — before/after PGO numbers
```

## Roadmap

| Phase | Focus | Target |
|-------|-------|--------|
| Week 1 (D1–D5) | Signal: prove PGO before/after on Prometheus | Aug 2 |
| Week 2 (D6–D10) | pgoctl CLI: all 5 subcommands | Aug 7 |
| Week 3 (D11–D15) | GitHub Action + Parca adapter + HN launch | Aug 12 |

## License

Apache 2.0 — see [LICENSE](LICENSE).
