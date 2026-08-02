# HANDOFF.md

## 2026-08-01 Dev ‒ D2: pgoctl validate command
Status: done
Output: https://github.com/Better-Go-Labs/pgoctl/pull/2
Notes:
- Implements `pgoctl validate <cpu.pprof>` with density/richness/coverage/depth scoring (spec §12.1a)
- `--format json` implemented as `--json` bool flag (functionally equivalent)
- Baseline capture half of D2 (BENCHMARKS.md / 3 prod profiles) is split out -- blocked on infra (kind+Prometheus); will land separately once infra is sorted

## 2026-08-02 Dev ‒ PR #3 remaining review comments (compare filter + gate clarity)
Status: needs-review
Output: https://github.com/Better-Go-Labs/pgoctl/pull/3 (commit 83d906b on feat/d3-d5-merge-compare-bench)
Notes:
- #1 "why - ?" — sign-convention comment added on the rollback gate; replied in thread
- #2 filter — GateConfig.MinCPUPercent + `--min-cpu-percent` flag; drops functions below threshold in BOTH profiles (hot-in-either kept); Report.FilteredFunctions added; replied in thread, awaiting sign-off on semantics
- #3 default no filter — MinCPUPercent defaults 0, guard `> 0`; TestCompare_NoFilterByDefault added; replied in thread
- Verify gate: gofmt clean, go build, go vet, go test -count=1 ./... green, end-to-end CLI smoke (filter moves summary 3.7→4.2, filtered_functions=1)
- Threads NOT resolved (filter semantics need Gyanesh sign-off first)
- Flag: testdata/cpu_valid.pprof is committed BASE64-ENCODED (base64 -d → valid 874-byte gzipped pprof); parse commands fail on it as-is — re-commit as binary if it's the shared merge/compare fixture
