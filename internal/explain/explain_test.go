package explain

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/pprof/profile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeProfile(t *testing.T, samples []struct {
	fn    string
	count int64
}) *profile.Profile {
	t.Helper()
	p := &profile.Profile{
		SampleType: []*profile.ValueType{
			{Type: "cpu", Unit: "nanoseconds"},
		},
		DurationNanos: 30 * 1e9,
	}
	fnMap := map[string]*profile.Function{}
	locMap := map[string]*profile.Location{}
	for _, s := range samples {
		if _, ok := fnMap[s.fn]; !ok {
			id := uint64(len(fnMap) + 1)
			fn := &profile.Function{ID: id, Name: s.fn}
			loc := &profile.Location{
				ID:   id,
				Line: []profile.Line{{Function: fn}},
			}
			fnMap[s.fn] = fn
			locMap[s.fn] = loc
			p.Function = append(p.Function, fn)
			p.Location = append(p.Location, loc)
		}
		p.Sample = append(p.Sample, &profile.Sample{
			Location: []*profile.Location{locMap[s.fn]},
			Value:    []int64{s.count},
		})
	}
	return p
}

func writeTmpProfile(t *testing.T, p *profile.Profile) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "cpu*.pprof")
	require.NoError(t, err)
	require.NoError(t, p.Write(f))
	require.NoError(t, f.Close())
	return f.Name()
}

func TestAnalyzeFile_TopN(t *testing.T) {
	funcs := []struct {
		fn    string
		count int64
	}{
		{"github.com/prometheus/prometheus/tsdb.(*Head).Append", 5000},
		{"github.com/prometheus/prometheus/tsdb.(*Head).gc", 3000},
		{"github.com/prometheus/prometheus/promql.(*Engine).exec", 2000},
		{"runtime.mallocgc", 1000},
		{"runtime.memmove", 500},
	}
	// Pad to pass minSamples=10000: repeat each sample to reach 11500.
	expanded := make([]struct {
		fn    string
		count int64
	}, 0, 115)
	for _, f := range funcs {
		for i := int64(0); i < f.count/100; i++ {
			expanded = append(expanded, struct {
				fn    string
				count int64
			}{f.fn, 100})
		}
	}
	p := makeProfile(t, expanded)
	path := writeTmpProfile(t, p)

	rpt, err := AnalyzeFile(path, 3)
	require.NoError(t, err)

	assert.Equal(t, 3, len(rpt.TopFunctions))
	assert.Equal(t, "github.com/prometheus/prometheus/tsdb.(*Head).Append", rpt.TopFunctions[0].Function)
	assert.Greater(t, rpt.TopFunctions[0].FlatPct, rpt.TopFunctions[1].FlatPct)

	// Package groups: tsdb should be the top group.
	require.NotEmpty(t, rpt.PackageGroups)
	assert.Equal(t, "github.com/prometheus/prometheus/tsdb", rpt.PackageGroups[0].Package)
}

func TestAnalyzeFile_Verdict(t *testing.T) {
	tests := []struct {
		name          string
		sampleCount   int
		funcCount     int
		wantVerdictIn []Verdict
	}{
		{
			name:          "not_ready_too_few_samples",
			sampleCount:   100,
			funcCount:     5,
			wantVerdictIn: []Verdict{VerdictNotReady},
		},
		{
			name:          "borderline",
			sampleCount:   15000,
			funcCount:     30,
			wantVerdictIn: []Verdict{VerdictBorderline},
		},
		{
			name:          "ready",
			sampleCount:   60000,
			funcCount:     50,
			wantVerdictIn: []Verdict{VerdictReady},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &profile.Profile{
				SampleType:    []*profile.ValueType{{Type: "cpu", Unit: "nanoseconds"}},
				DurationNanos: 30 * 1e9,
			}
			for i := 0; i < tt.funcCount; i++ {
				fn := &profile.Function{ID: uint64(i + 1), Name: "pkg.Func" + string(rune('A'+i%26))}
				loc := &profile.Location{ID: uint64(i + 1), Line: []profile.Line{{Function: fn}}}
				p.Function = append(p.Function, fn)
				p.Location = append(p.Location, loc)
			}
			for i := 0; i < tt.sampleCount; i++ {
				loc := p.Location[i%tt.funcCount]
				p.Sample = append(p.Sample, &profile.Sample{
					Location: []*profile.Location{loc},
					Value:    []int64{1000},
				})
			}

			path := writeTmpProfile(t, p)
			rpt, err := AnalyzeFile(path, 20)
			require.NoError(t, err)
			assert.Contains(t, tt.wantVerdictIn, rpt.Verdict, "verdict=%s reason=%s", rpt.Verdict, rpt.VerdictReason)
		})
	}
}

func TestAnalyzeFile_NotFound(t *testing.T) {
	_, err := AnalyzeFile(filepath.Join(t.TempDir(), "nonexistent.pprof"), 10)
	require.Error(t, err)
}

func TestPackageFromFunction(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"github.com/prometheus/prometheus/tsdb.(*Head).Append", "github.com/prometheus/prometheus/tsdb"},
		{"github.com/prometheus/prometheus/tsdb/wlog.(*Watcher).run", "github.com/prometheus/prometheus/tsdb/wlog"},
		{"runtime.mallocgc", "runtime"},
		{"main.main", "main"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, packageFromFunction(tt.input), tt.input)
	}
}
