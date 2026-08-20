package validate_test

import (
	"os"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	profiletypes "github.com/Better-Go-Labs/pgoctl/internal/profile"
	"github.com/Better-Go-Labs/pgoctl/internal/validate"
)

func generateCPUProfile(t *testing.T, durationMs int) []byte {
	f, err := os.CreateTemp("", "cpu*.pprof")
	require.NoError(t, err)
	defer func() { _ = os.Remove(f.Name()) }()
	defer func() { _ = f.Close() }()

	err = pprof.StartCPUProfile(f)
	require.NoError(t, err)

	deadline := time.Now().Add(time.Duration(durationMs) * time.Millisecond)
	for time.Now().Before(deadline) {
		_ = fib(30)
	}
	pprof.StopCPUProfile()

	_, err = f.Seek(0, 0)
	require.NoError(t, err)

	data, err := os.ReadFile(f.Name())
	require.NoError(t, err)
	return data
}

func fib(n int) int {
	if n <= 1 {
		return n
	}
	return fib(n-1) + fib(n-2)
}

func TestValidate_TableDriven(t *testing.T) {
	// Generate valid pprof data for test setup
	validData := generateCPUProfile(t, 200)

	tests := []struct {
		name        string
		setup       func(t *testing.T) string
		teardown    func(t *testing.T, path string)
		opts        validate.Options
		expectValid bool
		expectError bool
		checkResult func(t *testing.T, r *profiletypes.QualityReport)
	}{
		{
			name: "Valid Profile",
			setup: func(t *testing.T) string {
				f, err := os.CreateTemp("", "valid*.pprof")
				require.NoError(t, err)
				_, err = f.Write(validData)
				require.NoError(t, err)
				_ = f.Close()
				return f.Name()
			},
			teardown: func(_ *testing.T, path string) {
				_ = os.Remove(path)
			},
			opts: func() validate.Options {
				o := validate.DefaultOptions()
				o.MinSamples = 1
				o.MinScore = 0.1
				return o
			}(),
			expectValid: true,
			expectError: false,
			checkResult: func(t *testing.T, r *profiletypes.QualityReport) {
				assert.NotEmpty(t, r.Samples, "Samples count should be non-zero")
				assert.NotEmpty(t, r.UniqueStacks, "Unique stacks count should be non-zero")
			},
		},
		{
			name: "Nonexistent File",
			setup: func(_ *testing.T) string {
				return "/nonexistent/path/does/not/exist.pprof"
			},
			teardown:    func(_ *testing.T, _ string) {},
			opts:        validate.DefaultOptions(),
			expectValid: false,
			expectError: true,
		},
		{
			name: "Invalid Data",
			setup: func(t *testing.T) string {
				f, err := os.CreateTemp("", "bad*.pprof")
				require.NoError(t, err)
				_, err = f.WriteString("not a pprof file")
				require.NoError(t, err)
				_ = f.Close()
				return f.Name()
			},
			teardown: func(_ *testing.T, path string) {
				_ = os.Remove(path)
			},
			opts:        validate.DefaultOptions(),
			expectValid: false,
			expectError: true,
		},
		{
			name: "Score Formula Validation",
			setup: func(t *testing.T) string {
				f, err := os.CreateTemp("", "score*.pprof")
				require.NoError(t, err)
				_, err = f.Write(validData)
				require.NoError(t, err)
				_ = f.Close()
				return f.Name()
			},
			teardown: func(_ *testing.T, path string) {
				_ = os.Remove(path)
			},
			opts: func() validate.Options {
				o := validate.DefaultOptions()
				o.MinSamples = 1
				return o
			}(),
			expectValid: false,
			expectError: false,
			checkResult: func(t *testing.T, r *profiletypes.QualityReport) {
				assert.GreaterOrEqual(t, r.QualityScore, 0.0, "Score should be >= 0")
				assert.LessOrEqual(t, r.QualityScore, 1.0, "Score should be <= 1")
			},
		},
		{
			name: "Custom Density-Only Weight",
			setup: func(t *testing.T) string {
				f, err := os.CreateTemp("", "density-only*.pprof")
				require.NoError(t, err)
				_, err = f.Write(validData)
				require.NoError(t, err)
				_ = f.Close()
				return f.Name()
			},
			teardown: func(_ *testing.T, path string) { _ = os.Remove(path) },
			opts: func() validate.Options {
				o := validate.DefaultOptions()
				o.MinSamples = 1
				o.WeightDensity = 1
				o.WeightRichness = 0
				o.WeightCoverage = 0
				o.WeightDepth = 0
				return o
			}(),
			expectValid: false,
			expectError: false,
			checkResult: func(t *testing.T, r *profiletypes.QualityReport) {
				expected := float64(r.Samples) / 50000
				assert.InDelta(t, expected, r.QualityScore, 0.001, "score should equal density term")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t)
			defer tt.teardown(t, path)

			report, err := validate.ValidateFile(path, tt.opts)
			if tt.expectError {
				assert.Error(t, err, "Expected an error")
				assert.Nil(t, report, "Report should be nil on error")
			} else {
				assert.NoError(t, err, "Unexpected error")
				assert.NotNil(t, report, "Report should not be nil")
				if tt.expectValid {
					assert.True(t, report.Valid, "Report should be valid")
				}
				if tt.checkResult != nil {
					tt.checkResult(t, report)
				}
			}
		})
	}
}
