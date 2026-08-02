package merge_test

import (
	"bytes"
	"math"
	"testing"
	"time"

	"github.com/google/pprof/profile"

	"github.com/Better-Go-Labs/pgoctl/internal/merge"
)

func buildProfile(t *testing.T, sampleCount int, durationNanos, sampleValue, timeNanos int64) []byte {
	t.Helper()
	fn := &profile.Function{ID: 1, Name: "main.hotPath", SystemName: "main.hotPath"}
	p := &profile.Profile{
		SampleType:    []*profile.ValueType{{Type: "cpu", Unit: "nanoseconds"}},
		PeriodType:    &profile.ValueType{Type: "cpu", Unit: "nanoseconds"},
		Period:        10000000,
		DurationNanos: durationNanos,
		TimeNanos:     timeNanos,
		Function:      []*profile.Function{fn},
	}
	for i := 0; i < sampleCount; i++ {
		loc := &profile.Location{
			ID:   uint64(i + 1),
			Line: []profile.Line{{Function: fn, Line: int64(i + 1)}},
		}
		p.Location = append(p.Location, loc)
		p.Sample = append(p.Sample, &profile.Sample{
			Location: []*profile.Location{loc},
			Value:    []int64{sampleValue},
		})
	}
	var buf bytes.Buffer
	if err := p.Write(&buf); err != nil {
		t.Fatalf("buildProfile: %v", err)
	}
	return buf.Bytes()
}

func TestMerge_WeightedTwo(t *testing.T) {
	now := time.Now()
	inputs := []merge.Input{
		{Data: buildProfile(t, 500, 30*1e9, 10000000, 0), CapturedAt: now.Add(-48 * time.Hour)},
		{Data: buildProfile(t, 500, 30*1e9, 10000000, 0), CapturedAt: now},
	}
	var out bytes.Buffer
	if err := merge.Profiles(inputs, merge.DefaultOptions(), &out); err != nil {
		t.Fatalf("merge: %v", err)
	}
	if out.Len() == 0 {
		t.Fatal("expected non-empty merged output")
	}
	p, err := profile.ParseData(out.Bytes())
	if err != nil {
		t.Fatalf("parse merged output: %v", err)
	}
	if len(p.Sample) == 0 {
		t.Error("merged profile has no samples")
	}
}

func TestMerge_Latest(t *testing.T) {
	now := time.Now()
	inputs := []merge.Input{
		{Data: buildProfile(t, 100, 10*1e9, 10000000, 0), CapturedAt: now.Add(-72 * time.Hour)},
		{Data: buildProfile(t, 200, 20*1e9, 10000000, 0), CapturedAt: now},
	}
	opts := merge.DefaultOptions()
	opts.Strategy = merge.Latest
	var out bytes.Buffer
	if err := merge.Profiles(inputs, opts, &out); err != nil {
		t.Fatalf("merge: %v", err)
	}
	p, err := profile.ParseData(out.Bytes())
	if err != nil {
		t.Fatalf("parse merged output: %v", err)
	}
	if int64(len(p.Sample)) != 200 {
		t.Errorf("Latest strategy: expected 200 samples from newest profile, got %d", len(p.Sample))
	}
}

func TestMerge_Empty(t *testing.T) {
	if err := merge.Profiles(nil, merge.DefaultOptions(), &bytes.Buffer{}); err == nil {
		t.Fatal("expected error for empty input")
	}
}

func totalValue(t *testing.T, p *profile.Profile) int64 {
	t.Helper()
	var total int64
	for _, s := range p.Sample {
		total += s.Value[0]
	}
	return total
}

// TestMerge_StrategiesDifferWeighting verifies that the weighted and union
// strategies assign different weights when profile capture times differ.
func TestMerge_StrategiesDifferWeighting(t *testing.T) {
	now := time.Now()
	inputs := []merge.Input{
		{Data: buildProfile(t, 500, 30*1e9, 10000000, 0), CapturedAt: now.Add(-48 * time.Hour)},
		{Data: buildProfile(t, 500, 30*1e9, 10000000, 0), CapturedAt: now},
	}

	union := merge.DefaultOptions()
	union.Strategy = merge.Union
	var unionOut bytes.Buffer
	if err := merge.Profiles(inputs, union, &unionOut); err != nil {
		t.Fatalf("union merge: %v", err)
	}
	up, err := profile.ParseData(unionOut.Bytes())
	if err != nil {
		t.Fatalf("parse union output: %v", err)
	}
	unionTotal := totalValue(t, up)

	weighted := merge.DefaultOptions() // Strategy: weighted
	var weightedOut bytes.Buffer
	if err := merge.Profiles(inputs, weighted, &weightedOut); err != nil {
		t.Fatalf("weighted merge: %v", err)
	}
	wp, err := profile.ParseData(weightedOut.Bytes())
	if err != nil {
		t.Fatalf("parse weighted output: %v", err)
	}
	weightedTotal := totalValue(t, wp)

	// Union keeps both profiles at equal weight: 500*(10M+10M) = 10B.
	wantUnion := int64(500) * (10000000 + 10000000)
	if unionTotal != wantUnion {
		t.Errorf("union: expected total %d, got %d", wantUnion, unionTotal)
	}
	// Weighted scales newest by RecencyWeight (2.0) and the 48h-old profile
	// by 2.0*0.5^(48/24) = 0.5: 500*(10M*2.0 + 10M*0.5) = 12.5B.
	wantWeighted := int64(500) * (10000000*2 + int64(math.Round(10000000*0.5)))
	if weightedTotal != wantWeighted {
		t.Errorf("weighted: expected total %d, got %d", wantWeighted, weightedTotal)
	}
	if weightedTotal == unionTotal {
		t.Error("weighted and union strategies must produce different totals")
	}
}

// TestMerge_CaptureTimeFromProfile verifies that capture time is read from the
// profile's TimeNanos rather than the caller-supplied fallback.
func TestMerge_CaptureTimeFromProfile(t *testing.T) {
	now := time.Now().UTC()
	inputs := []merge.Input{
		// Newer TimeNanos but older CapturedAt: TimeNanos must win.
		{Data: buildProfile(t, 100, 10*1e9, 10000000, now.UnixNano()), CapturedAt: now.Add(-24 * time.Hour)},
		{Data: buildProfile(t, 200, 20*1e9, 10000000, now.Add(-48*time.Hour).UnixNano()), CapturedAt: now},
	}
	opts := merge.DefaultOptions()
	opts.Strategy = merge.Latest
	var out bytes.Buffer
	if err := merge.Profiles(inputs, opts, &out); err != nil {
		t.Fatalf("merge: %v", err)
	}
	p, err := profile.ParseData(out.Bytes())
	if err != nil {
		t.Fatalf("parse merged output: %v", err)
	}
	// Latest must pick by TimeNanos (input 0), not by the CapturedAt fallback.
	if int64(len(p.Sample)) != 100 {
		t.Errorf("expected 100 samples from TimeNanos-newest profile, got %d", len(p.Sample))
	}
}
