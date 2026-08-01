package validate

import (
	"fmt"
	"math"
	"os"

	"github.com/google/pprof/profile"
	profiletypes "github.com/Better-Go-Labs/pgoctl/internal/profile"
)

const (
	targetSamples  = 50000
	targetDuration = 30.0 // seconds
)

type Options struct {
	MinSamples  int64
	MinDuration float64 // seconds
	MinScore    float64
}

func DefaultOptions() Options {
	return Options{MinSamples: 10000, MinDuration: 10.0, MinScore: 0.6}
}

func clamp01(v float64) float64 {
	return math.Max(0, math.Min(1, v))
}

// ValidateFile parses path and returns a QualityReport.
// error is non-nil only on I/O or parse failure (caller should exit 2).
// report.Valid==false with error==nil means below threshold (caller exits 1).
func ValidateFile(path string, opts Options) (*profiletypes.QualityReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	p, err := profile.ParseData(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	hasCPU := false
	for _, st := range p.SampleType {
		if st.Type == "cpu" && st.Unit == "nanoseconds" {
			hasCPU = true
			break
		}
		if st.Type == "samples" && st.Unit == "count" {
			hasCPU = true
			break
		}
	}

	report := &profiletypes.QualityReport{}

	if !hasCPU {
		report.Errors = append(report.Errors, "no cpu sample type")
		return report, nil
	}

	sampleCount := int64(len(p.Sample))

	stackSet := make(map[string]struct{})
	var totalDepth int64
	for _, s := range p.Sample {
		key := ""
		for _, loc := range s.Location {
			for _, line := range loc.Line {
				if line.Function != nil {
					key += line.Function.Name + ";"
				}
			}
		}
		stackSet[key] = struct{}{}
		totalDepth += int64(len(s.Location))
	}
	uniqueStacks := int64(len(stackSet))

	var avgStackDepth float64
	if sampleCount > 0 {
		avgStackDepth = float64(totalDepth) / float64(sampleCount)
	}

	durationSec := p.DurationNanos / 1e9

	report.Samples = sampleCount
	report.UniqueStacks = uniqueStacks

	if sampleCount < opts.MinSamples {
		report.Errors = append(report.Errors, fmt.Sprintf("insufficient samples: %d < %d", sampleCount, opts.MinSamples))
	}
	if durationSec < int64(opts.MinDuration) {
		report.Warnings = append(report.Warnings, fmt.Sprintf("profile too short: %ds < %.0fs", durationSec, opts.MinDuration))
	}
	if avgStackDepth < 2 {
		report.Errors = append(report.Errors, "flat/cold profile: avg stack depth < 2")
	}

	density := clamp01(float64(sampleCount) / targetSamples)
	richness := clamp01(float64(uniqueStacks) / (0.02*float64(sampleCount) + 1))
	coverage := clamp01(float64(durationSec) / targetDuration)
	var depthOK float64
	if avgStackDepth >= 2 {
		depthOK = 1.0
	}
	score := 0.40*density + 0.30*richness + 0.20*coverage + 0.10*depthOK

	report.QualityScore = math.Round(score*1000) / 1000
	report.Valid = score >= opts.MinScore && avgStackDepth >= 2 && sampleCount >= opts.MinSamples && len(report.Errors) == 0

	return report, nil
}
