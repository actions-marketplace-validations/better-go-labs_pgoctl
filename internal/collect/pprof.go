package collect

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

// CollectFromPprof fetches a CPU profile from a standard Go pprof HTTP endpoint.
// opts.URL must be the full URL including query params (e.g. /debug/pprof/profile?seconds=30).
func CollectFromPprof(opts Options) (*Result, error) {
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 120 * time.Second
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(opts.URL)
	if err != nil {
		return nil, fmt.Errorf("collect pprof: get %s: %w", opts.URL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("collect pprof: server returned %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("collect pprof: read response: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("collect pprof: endpoint returned empty profile")
	}
	now := time.Now()
	return &Result{
		Bytes:     data,
		Source:    SourcePprof,
		Start:     now,
		End:       now,
		SizeBytes: len(data),
	}, nil
}
