package collect

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

type mergeResponse struct {
	Pprof   string `json:"pprof"`   // base64-encoded bytes
	Code    string `json:"code"`    // present on error
	Message string `json:"message"` // present on error
}

// CollectFromParca fetches a merged CPU pprof from a Parca server using the
// grpc-gateway REST GET /profiles/query endpoint.
func CollectFromParca(opts Options) (*Result, error) {
	end := opts.End
	if end.IsZero() {
		end = time.Now()
	}
	start := end.Add(-opts.Window)

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// build GET /profiles/query with merge-mode params
	u, err := url.Parse(opts.ParcaAddr + "/api/profiles/query")
	if err != nil {
		return nil, fmt.Errorf("collect parca: parse addr: %w", err)
	}
	q := u.Query()
	q.Set("mode", "MODE_MERGE")
	q.Set("merge.query", opts.Query)
	q.Set("merge.start", start.UTC().Format(time.RFC3339Nano))
	q.Set("merge.end", end.UTC().Format(time.RFC3339Nano))
	q.Set("report_type", "REPORT_TYPE_PPROF")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("collect parca: create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	httpClient := &http.Client{Timeout: timeout}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("collect parca: get %s: %w", u.String(), err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("collect parca: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		snippet := respBytes
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return nil, fmt.Errorf("collect parca: server returned %d: %s", resp.StatusCode, snippet)
	}

	fmt.Fprintf(os.Stderr, "[parca-diag] MergeProfile raw response (%d bytes): %.500s\n", len(respBytes), respBytes)

	var mergeResp mergeResponse
	if err := json.Unmarshal(respBytes, &mergeResp); err != nil {
		ct := resp.Header.Get("Content-Type")
		snippet := respBytes
		if len(snippet) > 500 {
			snippet = snippet[:500]
		}
		return nil, fmt.Errorf("collect parca: decode response (HTTP %d, Content-Type: %q): %w\nResponse body: %s", resp.StatusCode, ct, err, snippet)
	}

	if mergeResp.Code != "" {
		return nil, fmt.Errorf("collect parca: server error %s: %s", mergeResp.Code, mergeResp.Message)
	}

	profileBytes, err := base64.StdEncoding.DecodeString(mergeResp.Pprof)
	if err != nil {
		return nil, fmt.Errorf("collect parca: decode pprof base64: %w", err)
	}

	if len(profileBytes) == 0 {
		return nil, fmt.Errorf("collect parca: parca returned empty profile")
	}

	return &Result{
		Bytes:     profileBytes,
		Source:    SourceParca,
		Start:     start,
		End:       end,
		SizeBytes: len(profileBytes),
	}, nil
}
