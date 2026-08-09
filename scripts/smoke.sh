#!/usr/bin/env bash
# smoke.sh — e2e smoke test for pgoctl
#
# Generates a synthetic CPU profile, then exercises the full pipeline:
#   validate (relaxed) → merge → build(PGO) → explain → compare(self)
#
# Designed to run in CI without any external services. Exits 0 on success.
#
# Usage:
#   ./scripts/smoke.sh
#   PGOCTL=./bin/pgoctl ./scripts/smoke.sh

set -euo pipefail

PGOCTL="${PGOCTL:-$(command -v pgoctl 2>/dev/null || echo bin/pgoctl)}"
WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT

pass() { echo "  PASS: $*"; }
fail() { echo "  FAIL: $*" >&2; exit 1; }
step() { echo; echo "── $* ──"; }

echo "pgoctl smoke test"
echo "binary: $PGOCTL"
echo "workdir: $WORK_DIR"

# Resolve binary — build if not found.
if ! command -v "$PGOCTL" >/dev/null 2>&1; then
    echo "pgoctl not found — building..."
    go build -o bin/pgoctl ./cmd/pgoctl
    PGOCTL=bin/pgoctl
fi

# ── generate a synthetic CPU profile ─────────────────────────────────────
step "Generate synthetic profile"
cat > "$WORK_DIR/gen.go" << 'GOEOF'
package main

import (
    "math"
    "os"
    "runtime/pprof"
)

func hotA() { x := 1.0; for i := 0; i < 5e6; i++ { x = math.Sin(x) }; _ = x }
func hotB() { x := 1.0; for i := 0; i < 3e6; i++ { x = math.Cos(x) }; _ = x }
func hotC() { x := 1.0; for i := 0; i < 2e6; i++ { x = math.Sqrt(x + 1) }; _ = x }

func main() {
    f, _ := os.Create(os.Args[1])
    pprof.StartCPUProfile(f)
    for i := 0; i < 8; i++ { hotA(); hotB(); hotC() }
    pprof.StopCPUProfile()
    f.Close()
}
GOEOF
go run "$WORK_DIR/gen.go" "$WORK_DIR/cpu.pprof"
pass "profile generated ($(wc -c < "$WORK_DIR/cpu.pprof") bytes)"

# ── validate (relaxed — synthetic profile won't hit prod thresholds) ───────
step "Validate"
# Use --min-samples=1 so a short synthetic profile passes the gate.
validate_out=$("$PGOCTL" validate \
    --min-samples=1 \
    --min-score=0.0 \
    --min-stack-depth=1 \
    "$WORK_DIR/cpu.pprof" 2>&1)
echo "$validate_out"
echo "$validate_out" | grep -q "^valid" || fail "validate output missing 'valid' field"
pass "validate completed"

# ── merge ──────────────────────────────────────────────────────────────────
step "Merge"
"$PGOCTL" merge "$WORK_DIR/cpu.pprof" --out="$WORK_DIR/default.pgo" 2>&1
[[ -s "$WORK_DIR/default.pgo" ]] || fail "default.pgo is empty after merge"
pass "merge produced $(wc -c < "$WORK_DIR/default.pgo") byte artifact"

# ── build with PGO ─────────────────────────────────────────────────────────
step "Build with PGO"
go build -pgo="$WORK_DIR/default.pgo" -o "$WORK_DIR/pgoctl-pgo" ./cmd/pgoctl
[[ -x "$WORK_DIR/pgoctl-pgo" ]] || fail "PGO binary not executable"
pass "PGO binary built ($(du -sh "$WORK_DIR/pgoctl-pgo" | cut -f1))"

# ── explain ────────────────────────────────────────────────────────────────
step "Explain"
explain_out=$("$WORK_DIR/pgoctl-pgo" explain "$WORK_DIR/cpu.pprof" 2>&1)
echo "$explain_out"
echo "$explain_out" | grep -q "^verdict" || fail "explain output missing 'verdict' field"
echo "$explain_out" | grep -q "Top functions" || fail "explain output missing function table"
pass "explain output looks correct"

# ── compare (self-compare baseline: all deltas zero) ───────────────────────
step "Compare (self)"
compare_out=$("$WORK_DIR/pgoctl-pgo" compare \
    "$WORK_DIR/cpu.pprof" "$WORK_DIR/cpu.pprof" 2>&1)
echo "$compare_out"
echo "$compare_out" | grep -qE "^verdict\s+neutral" || fail "self-compare should be neutral"
pass "compare self → neutral (expected)"

# ── JSON output check ──────────────────────────────────────────────────────
step "JSON output"
json_out=$("$WORK_DIR/pgoctl-pgo" explain --format json "$WORK_DIR/cpu.pprof" 2>&1)
echo "$json_out" | python3 -c "import sys,json; d=json.load(sys.stdin); assert 'verdict' in d" \
    || fail "explain --format json produced invalid JSON"
pass "JSON output valid"

echo
echo "smoke test PASSED"
