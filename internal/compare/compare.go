package compare

import (
	"fmt"
	"math"
	"os"
	"sort"

	"github.com/google/pprof/profile"
)

// Verdict is the gate decision.
type Verdict string

const (
	Promote  Verdict = "promote"
	Rollback Verdict = "rollback"
	Neutral  Verdict = "neutral"
)

// FunctionDelta captures the per-function CPU change between two profiles.
type FunctionDelta struct {
	Function string  `json:"function"`
	BasePct  float64 `json:"base_pct"`
	CandPct  float64 `json:"cand_pct"`
	DeltaPct float64 `json:"delta_pct"` // positive = candidate uses LESS CPU
}

// Report is the full output of pgoctl compare.
type Report struct {
	BaselineSamples  int64           `json:"baseline_samples"`
	CandidateSamples int64           `json:"candidate_samples"`
	TopDeltas        []FunctionDelta `json:"top_deltas"`
	SummaryCPUDelta  float64         `json:"summary_cpu_delta_pct"` // positive = improvement
	Verdict          Verdict         `json:"verdict"`
}

// GateConfig holds the thresholds used to decide the verdict.
type GateConfig struct {
	MinCPUImprovement float64 // percentage points required for Promote
	TopN              int     // number of function deltas to include in output
}

func DefaultGateConfig() GateConfig {
	return GateConfig{MinCPUImprovement: 3.0, TopN: 10}
}

// ProfileFiles loads two pprof files and compares CPU function attribution.
func ProfileFiles(basePath, candPath string, gate GateConfig) (*Report, error) {
	baseData, err := os.ReadFile(basePath)
	if err != nil {
		return nil, fmt.Errorf("read baseline %s: %w", basePath, err)
	}
	candData, err := os.ReadFile(candPath)
	if err != nil {
		return nil, fmt.Errorf("read candidate %s: %w", candPath, err)
	}
	return Profiles(baseData, candData, gate)
}

// Profiles compares two raw pprof byte slices.
func Profiles(baseData, candData []byte, gate GateConfig) (*Report, error) {
	base, err := profile.ParseData(baseData)
	if err != nil {
		return nil, fmt.Errorf("parse baseline: %w", err)
	}
	cand, err := profile.ParseData(candData)
	if err != nil {
		return nil, fmt.Errorf("parse candidate: %w", err)
	}
	return compareProfiles(base, cand, gate), nil
}

func compareProfiles(base, cand *profile.Profile, gate GateConfig) *Report {
	basePct := flatPercents(base)
	candPct := flatPercents(cand)

	// union of all function names across both profiles
	seen := make(map[string]struct{}, len(basePct)+len(candPct))
	for k := range basePct {
		seen[k] = struct{}{}
	}
	for k := range candPct {
		seen[k] = struct{}{}
	}

	deltas := make([]FunctionDelta, 0, len(seen))
	summary := 0.0
	for fn := range seen {
		b := basePct[fn]
		c := candPct[fn]
		d := b - c // positive = candidate spent less CPU here
		deltas = append(deltas, FunctionDelta{
			Function: fn,
			BasePct:  math.Round(b*100) / 100,
			CandPct:  math.Round(c*100) / 100,
			DeltaPct: math.Round(d*100) / 100,
		})
		// weighted by baseline share: how much of total CPU did this save?
		summary += b * (d / 100.0)
	}
	sort.Slice(deltas, func(i, j int) bool {
		return math.Abs(deltas[i].DeltaPct) > math.Abs(deltas[j].DeltaPct)
	})

	top := gate.TopN
	if top <= 0 || top > len(deltas) {
		top = len(deltas)
	}

	v := Neutral
	switch {
	case summary >= gate.MinCPUImprovement:
		v = Promote
	case summary <= -gate.MinCPUImprovement:
		v = Rollback
	}

	return &Report{
		BaselineSamples:  int64(len(base.Sample)),
		CandidateSamples: int64(len(cand.Sample)),
		TopDeltas:        deltas[:top],
		SummaryCPUDelta:  math.Round(summary*100) / 100,
		Verdict:          v,
	}
}

// flatPercents returns per-function CPU percentage attribution (top frame only).
func flatPercents(p *profile.Profile) map[string]float64 {
	total := 0.0
	counts := make(map[string]float64)
	for _, s := range p.Sample {
		if len(s.Location) == 0 || len(s.Value) == 0 {
			continue
		}
		v := float64(s.Value[0])
		total += v
		loc := s.Location[0]
		if len(loc.Line) > 0 && loc.Line[0].Function != nil {
			counts[loc.Line[0].Function.Name] += v
		}
	}
	if total == 0 {
		return counts
	}
	result := make(map[string]float64, len(counts))
	for fn, c := range counts {
		result[fn] = 100.0 * c / total
	}
	return result
}
