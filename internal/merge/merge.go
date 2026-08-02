package merge

import (
	"fmt"
	"io"
	"math"
	"os"
	"time"

	"github.com/google/pprof/profile"
)

type Strategy string

const (
	Weighted Strategy = "weighted"
	Latest   Strategy = "latest"
	Union    Strategy = "union"
)

type Options struct {
	Strategy      Strategy
	RecencyWeight float64
	HalfLife      time.Duration
	DropInvalid   bool
}

func DefaultOptions() Options {
	return Options{
		Strategy:      Weighted,
		RecencyWeight: 2.0,
		HalfLife:      24 * time.Hour,
	}
}

// Input is a single pprof profile with its capture timestamp.
type Input struct {
	Data       []byte
	CapturedAt time.Time
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
		times = append(times, inp.CapturedAt)
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
		// merge with equal weights — no scaling

	case Weighted:
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

// Files is a convenience wrapper that reads pprof files from disk.
func Files(paths []string, capturedAts []time.Time, opts Options, w io.Writer) error {
	inputs := make([]Input, len(paths))
	for i, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		t := time.Now()
		if i < len(capturedAts) {
			t = capturedAts[i]
		}
		inputs[i] = Input{Data: data, CapturedAt: t}
	}
	return Profiles(inputs, opts, w)
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
