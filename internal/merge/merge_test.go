package merge_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/google/pprof/profile"

	"github.com/Better-Go-Labs/pgoctl/internal/merge"
)

func buildProfile(t *testing.T, sampleCount int, durationNanos int64) []byte {
	t.Helper()
	fn := &profile.Function{ID: 1, Name: "main.hotPath", SystemName: "main.hotPath"}
	loc := &profile.Location{
		ID:   1,
		Line: []profile.Line{{Function: fn, Line: 1}},
	}
	p := &profile.Profile{
		SampleType:    []*profile.ValueType{{Type: "cpu", Unit: "nanoseconds"}},
		PeriodType:    &profile.ValueType{Type: "cpu", Unit: "nanoseconds"},
		Period:        10000000,
		DurationNanos: durationNanos,
		Function:      []*profile.Function{fn},
		Location:      []*profile.Location{loc},
	}
	for i := 0; i < sampleCount; i++ {
		p.Sample = append(p.Sample, &profile.Sample{
			Location: []*profile.Location{loc},
			Value:    []int64{10000000},
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
		{Data: buildProfile(t, 500, 30*1e9), CapturedAt: now.Add(-48 * time.Hour)},
		{Data: buildProfile(t, 500, 30*1e9), CapturedAt: now},
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
		{Data: buildProfile(t, 100, 10*1e9), CapturedAt: now.Add(-72 * time.Hour)},
		{Data: buildProfile(t, 200, 20*1e9), CapturedAt: now},
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
