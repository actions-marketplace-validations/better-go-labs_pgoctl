#!/usr/bin/env bash
# demo.sh — pgoctl happy-path demo
#
# Runs the full collect → validate → merge → build(PGO) → explain pipeline
# against a real service. Environment variables control every step so this
# script works both interactively and in CI.
#
# Usage:
#   ./demo.sh                        # collect from default Parca URL
#   PARCA_URL=http://parca:7070 ./demo.sh
#   PROFILE_FILE=my.pprof ./demo.sh  # skip collect; use an existing pprof
#   APP_PACKAGE=./cmd/myapp ./demo.sh
#
# Requirements:
#   - pgoctl binary on PATH  (or run: go build -o bin/pgoctl ./cmd/pgoctl)
#   - go on PATH (for build step)

set -euo pipefail

# ── configuration ──────────────────────────────────────────────────────────
PARCA_URL="${PARCA_URL:-http://localhost:7070}"
DURATION="${DURATION:-30}"
APP_PACKAGE="${APP_PACKAGE:-./cmd/pgoctl}"
WORK_DIR="${WORK_DIR:-/tmp/pgoctl-demo-$$}"
PROFILE_FILE="${PROFILE_FILE:-}"
PGOCTL="${PGOCTL:-$(command -v pgoctl 2>/dev/null || echo bin/pgoctl)}"

# ── helpers ────────────────────────────────────────────────────────────────
step() { echo; echo "==> $*"; }
fail() { echo "FAIL: $*" >&2; exit 1; }

command -v "$PGOCTL" >/dev/null 2>&1 || fail "pgoctl not found — run: go build -o bin/pgoctl ./cmd/pgoctl"
command -v go >/dev/null 2>&1 || fail "go not found on PATH"

mkdir -p "$WORK_DIR"
trap 'rm -rf "$WORK_DIR"' EXIT

PPROF="$WORK_DIR/cpu.pprof"
MERGED="$WORK_DIR/default.pgo"

# ── step 1: collect (or use existing file) ─────────────────────────────────
if [[ -n "$PROFILE_FILE" ]]; then
    step "Using existing profile: $PROFILE_FILE"
    cp "$PROFILE_FILE" "$PPROF"
else
    step "Collecting ${DURATION}s CPU profile from Parca at $PARCA_URL"
    "$PGOCTL" collect \
        --source=parca \
        --url="$PARCA_URL" \
        --duration="$DURATION" \
        --out="$PPROF"
fi

# ── step 2: validate ───────────────────────────────────────────────────────
step "Validating profile quality"
"$PGOCTL" validate "$PPROF" || {
    echo "WARN: profile did not pass the quality gate — continuing with lower-quality data"
}

# ── step 3: merge → default.pgo ───────────────────────────────────────────
step "Merging profile → $MERGED"
"$PGOCTL" merge "$PPROF" --out="$MERGED"

# ── step 4: build with PGO ────────────────────────────────────────────────
step "Building $APP_PACKAGE with PGO"
go build -pgo="$MERGED" -o "$WORK_DIR/pgo-binary" "$APP_PACKAGE"
echo "PGO binary: $(du -sh "$WORK_DIR/pgo-binary" | cut -f1)"

# ── step 5: explain the profile ───────────────────────────────────────────
step "Explaining profile"
"$PGOCTL" explain "$PPROF"

step "Done — PGO artifact at $MERGED"
echo "To use this artifact permanently:"
echo "  cp $MERGED default.pgo && git add default.pgo"
