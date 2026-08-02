package compare_test

import (
	"bytes"
	"math"
	"testing"

	"github.com/google/pprof/profile"

	"github.com/Better-Go-Labs/pgoctl/internal/compare"
)

// buildTwoFnProfile creates a pprof with fn1Count samples in fn1 and fn2Count in fn2.
func buildTwoFnProfile(t *testing.T, fn1, fn2 string, fn1Count, fn2Count int) []byte {
	t.Helper()
	f1 := &profile.Function{ID: 1, Name: fn1, SystemName: fn1}
	f2 := &profile.Function{ID: 2, Name: fn2, SystemName: fn2}
	loc1 := &profile.Location{ID: 1, Line: []profile.Line{{Function: f1, Line: 1}}}
	loc2 := &profile.Location{ID: 2, Line: []profile.Line{{Function: f2, Line: 1}}}
	p := &profile.Profile{
		SampleType:    []*profile.ValueType{{Type: "cpu", Unit: "nanoseconds"}},
		PeriodType:    &profile.ValueType{Type: "cpu", Unit: "nanoseconds"},
		Period:        10000000,
		DurationNanos: 30 * 1e9,
		Function:      []*profile.Function{f1, f2},
		Location:      []*profile.Location{loc1, loc2},
	}
	for i := 0; i < fn1Count; i++ {
		p.Sample = append(p.Sample, &profile.Sample{
			Location: []*profile.Location{loc1},
			Value:    []int64{10000000},
		})
	}
	for i := 0; i < fn2Count; i++ {
		p.Sample = append(p.Sample, &profile.Sample{
			Location: []*profile.Location{loc2},
			Value:    []int64{10000000},
		})
	}
	var buf bytes.Buffer
	if err := p.Write(&buf); err != nil {
		t.Fatalf("buildTwoFnProfile: %v", err)
	}
	return buf.Bytes()
}

func TestCompare_Improvement(t *testing.T) {
	// Baseline: hotFn monopolises CPU (1000 samples, 0 in other)
	// Candidate: PGO made hotFn faster — fewer samples, rest moved to otherFn
	// Expected: hotFn base=100%, cand=70% → delta=+30% → Promote
	base := buildTwoFnProfile(t, "main.hotFn", "main.otherFn", 1000, 0)
	cand := buildTwoFnProfile(t, "main.hotFn", "main.otherFn", 700, 300)

	rpt, err := compare.Profiles(base, cand, compare.DefaultGateConfig())
	if err != nil {
		t.Fatalf("compare: %v", err)
	}
	if rpt.SummaryCPUDelta <= 0 {
		t.Errorf("expected positive CPU delta (improvement), got %.2f", rpt.SummaryCPUDelta)
	}
	if rpt.Verdict != compare.Promote {
		t.Errorf("expected Promote, got %s (delta=%.2f)", rpt.Verdict, rpt.SummaryCPUDelta)
	}
}

func TestCompare_Regression(t *testing.T) {
	// Candidate is worse: hotFn grew from 70% to 100%
	base := buildTwoFnProfile(t, "main.hotFn", "main.otherFn", 700, 300)
	cand := buildTwoFnProfile(t, "main.hotFn", "main.otherFn", 1000, 0)

	rpt, err := compare.Profiles(base, cand, compare.DefaultGateConfig())
	if err != nil {
		t.Fatalf("compare: %v", err)
	}
	if rpt.Verdict != compare.Rollback {
		t.Errorf("expected Rollback, got %s (delta=%.2f)", rpt.Verdict, rpt.SummaryCPUDelta)
	}
}

func TestCompare_Neutral(t *testing.T) {
	// Identical profiles — delta is 0, no change
	data := buildTwoFnProfile(t, "main.hotFn", "main.otherFn", 500, 500)
	rpt, err := compare.Profiles(data, data, compare.DefaultGateConfig())
	if err != nil {
		t.Fatalf("compare: %v", err)
	}
	if rpt.Verdict != compare.Neutral {
		t.Errorf("expected Neutral, got %s (delta=%.2f)", rpt.Verdict, rpt.SummaryCPUDelta)
	}
}

// buildFnProfile creates a pprof with one function per entry in fnCounts,
// each sampled fnCounts[name] times.
func buildFnProfile(t *testing.T, fnCounts map[string]int) []byte {
	t.Helper()
	var funcs []*profile.Function
	var locs []*profile.Location
	var samples []*profile.Sample
	id := uint64(1)
	for name, count := range fnCounts {
		f := &profile.Function{ID: id, Name: name, SystemName: name}
		loc := &profile.Location{ID: id, Line: []profile.Line{{Function: f, Line: 1}}}
		funcs = append(funcs, f)
		locs = append(locs, loc)
		for i := 0; i < count; i++ {
			samples = append(samples, &profile.Sample{
				Location: []*profile.Location{loc},
				Value:    []int64{10000000},
			})
		}
		id++
	}
	p := &profile.Profile{
		SampleType:    []*profile.ValueType{{Type: "cpu", Unit: "nanoseconds"}},
		PeriodType:    &profile.ValueType{Type: "cpu", Unit: "nanoseconds"},
		Period:        10000000,
		DurationNanos: 30 * 1e9,
		Function:      funcs,
		Location:      locs,
		Sample:        samples,
	}
	var buf bytes.Buffer
	if err := p.Write(&buf); err != nil {
		t.Fatalf("buildFnProfile: %v", err)
	}
	return buf.Bytes()
}

func TestCompare_MinCPUPercentFilter(t *testing.T) {
	// hotFn 60%, warmFn 30%, coldFn 10% in baseline.
	// Candidate shifts hotFn→warmFn and coldFn grows 10%→15%.
	// With MinCPUPercent=20, coldFn is below the threshold in BOTH profiles
	// and must be excluded from the comparison.
	base := buildFnProfile(t, map[string]int{"main.hotFn": 600, "main.warmFn": 300, "main.coldFn": 100})
	cand := buildFnProfile(t, map[string]int{"main.hotFn": 510, "main.warmFn": 340, "main.coldFn": 150})

	gate := compare.DefaultGateConfig()
	gate.MinCPUPercent = 20
	rpt, err := compare.Profiles(base, cand, gate)
	if err != nil {
		t.Fatalf("compare: %v", err)
	}
	if rpt.FilteredFunctions != 1 {
		t.Errorf("expected 1 filtered function, got %d", rpt.FilteredFunctions)
	}
	for _, d := range rpt.TopDeltas {
		if d.Function == "main.coldFn" {
			t.Errorf("coldFn should have been filtered out, got delta %+v", d)
		}
	}
	// 60*(60-51)/100 + 30*(30-34)/100 = 5.4 - 1.2 = 4.2
	if math.Abs(rpt.SummaryCPUDelta-4.2) > 0.01 {
		t.Errorf("expected summary delta 4.2 with filter, got %.2f", rpt.SummaryCPUDelta)
	}
	if rpt.Verdict != compare.Promote {
		t.Errorf("expected Promote, got %s (delta=%.2f)", rpt.Verdict, rpt.SummaryCPUDelta)
	}
}

func TestCompare_NoFilterByDefault(t *testing.T) {
	// Default GateConfig has MinCPUPercent=0 → no filtering: coldFn stays in
	// the comparison and drags the summary to 3.7 (incl. its -0.5 regression).
	base := buildFnProfile(t, map[string]int{"main.hotFn": 600, "main.warmFn": 300, "main.coldFn": 100})
	cand := buildFnProfile(t, map[string]int{"main.hotFn": 510, "main.warmFn": 340, "main.coldFn": 150})

	rpt, err := compare.Profiles(base, cand, compare.DefaultGateConfig())
	if err != nil {
		t.Fatalf("compare: %v", err)
	}
	if rpt.FilteredFunctions != 0 {
		t.Errorf("default behaviour must not filter, got %d filtered", rpt.FilteredFunctions)
	}
	found := false
	for _, d := range rpt.TopDeltas {
		if d.Function == "main.coldFn" {
			found = true
		}
	}
	if !found {
		t.Error("coldFn should be present when filtering is disabled")
	}
	// 60*0.09 + 30*(-0.04) + 10*(-0.05) = 3.7
	if math.Abs(rpt.SummaryCPUDelta-3.7) > 0.01 {
		t.Errorf("expected summary delta 3.7 without filter, got %.2f", rpt.SummaryCPUDelta)
	}
}
