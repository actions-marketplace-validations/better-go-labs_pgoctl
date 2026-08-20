// Package merge implements profile merging strategies for pgoctl.
package merge

import (
	"fmt"
	"io"
	"math"
	"time"

	"github.com/google/pprof/profile"
)

// Strategy selects how profiles are merged.
type Strategy string

// Merge strategy constants.
const (
	Weighted Strategy = "weighted"
	Latest   Strategy = "latest"
	Union    Strategy = "union"
)

// Options configures the merge operation.
type Options struct {
	Strategy      Strategy
	RecencyWeight float64
	HalfLife      time.Duration
	DropInvalid   bool
}

// DefaultOptions returns Options with sensible defaults.
func DefaultOptions() Options {
	return Options{
		Strategy:      Weighted,
		RecencyWeight: 2.0,
		HalfLife:      24 * time.Hour,
	}
}

// Input is a single pprof profile. CapturedAt is only used as a fallback when
// the profile itself carries no timestamp (p.TimeNanos == 0).
type Input struct {
	Data       []byte
	CapturedAt time.Time
}

// captureTime returns the profile's own capture time when present, falling
// back to the caller-provided timestamp otherwise.
func captureTime(p *profile.Profile, fallback time.Time) time.Time {
	if p.TimeNanos != 0 {
		return time.Unix(0, p.TimeNanos)
	}
	return fallback
}

// Profiles merges the given pprof inputs according to opts and writes the
// resulting gzipped pprof to w. The output is a valid default.pgo input.
func Profiles(inputs []Input, opts Options, w io.Writer) error {
	if len(inputs) == 0 {
		return fmt.Errorf("at least one profile is required")
	}

	profiles := make([]*profile.Profile, 0, len(inputs))
	times := make([]time.Time, 0, len(inputs))
	for i, inp := range inputs {
		p, err := profile.ParseData(inp.Data)
		if err != nil {
			if opts.DropInvalid {
				continue
			}
			return fmt.Errorf("input %d: parse: %w", i, err)
		}
		profiles = append(profiles, p)
		times = append(times, captureTime(p, inp.CapturedAt))
	}
	if len(profiles) == 0 {
		return fmt.Errorf("no valid profiles after parsing")
	}

	switch opts.Strategy {
	case Latest:
		newest := 0
		for i := 1; i < len(times); i++ {
			if times[i].After(times[newest]) {
				newest = i
			}
		}
		profiles = []*profile.Profile{profiles[newest]}

	case Union:
		// Equal weight for every profile: merge all at their native scale.
		for _, p := range profiles {
			p.Scale(1.0)
		}

	case Weighted:
		// Exponential decay (weight = RecencyWeight * 0.5^(age/HalfLife)) so
		// that recent profiles dominate while older ones still contribute.
		applyWeights(profiles, times, opts)

	default:
		return fmt.Errorf("unknown strategy %q", opts.Strategy)
	}

	merged, err := profile.Merge(profiles)
	if err != nil {
		return fmt.Errorf("profile.Merge: %w", err)
	}
	return merged.Write(w)
}

// applyWeights scales each profile by an exponential recency weight before merge.
// weight = RecencyWeight * 0.5^(age/HalfLife), clamped to [0.25, RecencyWeight].
func applyWeights(profiles []*profile.Profile, times []time.Time, opts Options) {
	newest := times[0]
	for _, t := range times[1:] {
		if t.After(newest) {
			newest = t
		}
	}
	halfLifeH := opts.HalfLife.Hours()
	if halfLifeH <= 0 {
		halfLifeH = 24.0
	}
	for i, p := range profiles {
		ageH := newest.Sub(times[i]).Hours()
		w := opts.RecencyWeight * math.Pow(0.5, ageH/halfLifeH)
		w = math.Max(0.25, math.Min(opts.RecencyWeight, w))
		p.Scale(w)
	}
}
