package compare_test

import (
	"bytes"
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
