# pgoctl

> Continuous PGO and profile-guided optimization for Go workloads on Kubernetes.

`pgoctl` is the CLI backbone of GoOpt — a Kubernetes-native control plane that turns production Go profiles into optimized builds with measurable CPU and latency gains.

## What it does

Production profiles from your Go services → validated PGO artifacts → optimized builds → safe canary rollouts → cost reports.

```
pgoctl collect  --source=parca --parca-addr=http://parca:7070 --query='process_cpu:cpu:nanoseconds:cpu:nanoseconds{job="myapp"}' --window=5m
pgoctl validate cpu.pprof
pgoctl merge    profiles/*.pprof --out default.pgo
pgoctl explain  default.pgo
pgoctl compare  baseline.pprof candidate.pprof
```

## Contents

- [Quick start](#quick-start)
- [CLI reference](#cli-reference)
  - [collect — via Parca](#via-parca-continuous-profiling-server)
  - [collect — via pprof endpoint](#via-go-pprof-http-endpoint)
  - [validate](#pgoctl-validate)
  - [merge](#pgoctl-merge)
  - [explain](#pgoctl-explain)
  - [compare](#pgoctl-compare)
- [GitHub Action](#github-action)
- [Docker](#docker)
- [Configuration](#configuration)
- [Requirements](#requirements)
- [Project layout](#project-layout)
- [Status](#status)
- [License](#license)

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

Fetch a CPU profile from a running service. Two backends are supported.

#### Via Parca (continuous profiling server)

```
pgoctl collect --source=parca \
  --parca-addr=<base-url> \
  --query=<selector> \
  [--window=5m] \
  [--out=cpu.pprof]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--source` | `parca` | Profiling backend (`parca`) |
| `--parca-addr` | _(required)_ | Base URL of the Parca server (e.g. `http://localhost:7070`) |
| `--query` | _(required)_ | Parca label selector (e.g. `process_cpu:cpu:nanoseconds:cpu:nanoseconds{job="myapp"}`) |
| `--window` | `5m` | Time window for the merged profile (e.g. `5m`, `1h`) |
| `--out` | `cpu.pprof` | Output path (`-` for stdout) |

Calls `POST /parca.query.v1alpha1.QueryService/MergeProfile` with body `{"start": "…", "end": "…", "query": "…", "reportType": "REPORT_TYPE_PPROF"}`, decodes the base64 `pprof` field from the response, and validates it is a parseable pprof file before writing.

#### Via Go pprof HTTP endpoint

Any Go service with pprof enabled (`import _ "net/http/pprof"`) exposes a raw CPU profile endpoint. Collect directly with curl and feed it into the pipeline:

```bash
# Capture a 30s CPU profile from any pprof-enabled service
curl -o cpu.pprof "http://localhost:6060/debug/pprof/profile?seconds=30"

# Validate and merge as normal
pgoctl validate cpu.pprof
pgoctl merge cpu.pprof --out default.pgo
```

The endpoint is `GET /debug/pprof/profile?seconds=<N>` — standard on any Go binary that imports `net/http/pprof`. The standalone `cmd/baseline` collector wraps this for Prometheus (`make collect-baseline`).

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

## GitHub Action

`.github/actions/pgo-action` is a composite Action that runs the full pgoctl pipeline in CI — collect (or reuse an existing profile), validate, and compare against a baseline — then optionally uploads the artifact and posts a verdict comment on the PR.

```yaml
- uses: better-go-labs/pgoctl/.github/actions/pgo-action@main
  with:
    parca-url: http://parca:7070
    github-token: ${{ secrets.GITHUB_TOKEN }}
```

### Inputs

| Input | Description |
|-------|-------------|
| `parca-url` | Parca server URL |
| `profile-file` | Use an existing pprof (skips collect) |
| `baseline-profile` | Baseline pprof for the compare step |
| `duration` | Collection duration |
| `min-improvement` | Promote threshold (CPU delta %) |
| `min-regression` | Rollback threshold (CPU regression %) |
| `validate-flags` | Extra flags passed to `pgoctl validate` |
| `artifact-name` | Name for the uploaded artifact |
| `upload-artifact` | Whether to upload the artifact (default: `true`) |
| `comment-on-pr` | Whether to post a verdict comment (default: `true`) |
| `github-token` | Token used to post the PR comment |

### Outputs

| Output | Description |
|--------|-------------|
| `verdict` | `promote`, `neutral`, or `rollback` |
| `profile-path` | Path to the collected/used profile |
| `artifact-path` | Path to the produced artifact |
| `validate-score` | Quality score from `pgoctl validate` |

## Docker

A multi-stage `Dockerfile` builds a static, non-root image (~10MB).

```bash
docker build -t pgoctl .
docker run --rm pgoctl --help
```

Builder stage: `golang:1.23-alpine`, `CGO_ENABLED=0`, `-trimpath -ldflags="-s -w"`. Runtime stage: `gcr.io/distroless/static-debian12:nonroot`.

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

`demo.sh` runs the interactive happy-path pipeline. Set `PARCA_URL` to point it at your Parca server (it controls the Parca address used by the collect step).

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

## Status

v0.1.0 sprint complete — D1 through D15 shipped. See [BENCHMARKS.md](BENCHMARKS.md) for before/after PGO numbers on Prometheus.

## License

Apache 2.0 — see [LICENSE](LICENSE).
