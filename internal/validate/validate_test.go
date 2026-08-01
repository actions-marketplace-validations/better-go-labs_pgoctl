package validate_test

import (
	"os"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/Better-Go-Labs/pgoctl/internal/validate"
)

func generateCPUProfile(durationMs int) ([]byte, error) {
	f, err := os.CreateTemp("", "cpu*.pprof")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	defer f.Close()

	if err := pprof.StartCPUProfile(f); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(time.Duration(durationMs) * time.Millisecond)
	for time.Now().Before(deadline) {
		_ = fib(30)
	}
	pprof.StopCPUProfile()
	f.Seek(0, 0)
	return os.ReadFile(f.Name())
}

func fib(n int) int {
	if n <= 1 {
		return n
	}
	return fib(n-1) + fib(n-2)
}

func TestValidate_ValidProfile(t *testing.T) {
	data, err := generateCPUProfile(500)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.CreateTemp("", "valid*.pprof")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.Write(data)
	f.Close()

	opts := validate.DefaultOptions()
	opts.MinSamples = 1
	opts.MinScore = 0.1
	report, err := validate.ValidateFile(f.Name(), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report == nil {
		t.Fatal("nil report")
	}
	if report.Samples == 0 {
		t.Error("expected samples > 0")
	}
}

func TestValidate_NonexistentFile(t *testing.T) {
	_, err := validate.ValidateFile("/nonexistent/path.pprof", validate.DefaultOptions())
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestValidate_InvalidData(t *testing.T) {
	f, err := os.CreateTemp("", "bad*.pprof")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString("not a pprof file")
	f.Close()

	_, err = validate.ValidateFile(f.Name(), validate.DefaultOptions())
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestValidate_ScoreFormula(t *testing.T) {
	data, err := generateCPUProfile(200)
	if err != nil {
		t.Fatal(err)
	}
	f, _ := os.CreateTemp("", "score*.pprof")
	defer os.Remove(f.Name())
	f.Write(data)
	f.Close()

	opts := validate.DefaultOptions()
	opts.MinSamples = 1
	report, err := validate.ValidateFile(f.Name(), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.QualityScore < 0 || report.QualityScore > 1 {
		t.Errorf("score %f out of [0,1]", report.QualityScore)
	}
}
