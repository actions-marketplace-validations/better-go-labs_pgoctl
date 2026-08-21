package collect

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// newH2CServer wraps a handler with H2C so tests match the production transport.
func newH2CServer(h http.Handler) *httptest.Server {
	return httptest.NewServer(h2c.NewHandler(h, &http2.Server{}))
}

func TestCollectFromParca_ConnectProtocolHeader(t *testing.T) {
	fakeData := []byte("FAKE_PPROF_DATA")
	var gotContentType, gotConnectVersion string
	srv := newH2CServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		gotConnectVersion = r.Header.Get("Connect-Protocol-Version")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(mergeResponse{
			Pprof: base64.StdEncoding.EncodeToString(fakeData),
		})
	}))
	defer srv.Close()

	opts := Options{
		Source:    SourceParca,
		ParcaAddr: srv.URL,
		Query:     "process_cpu:cpu:nanoseconds:cpu:nanoseconds",
		Window:    5 * time.Minute,
		End:       time.Now(),
		Timeout:   5 * time.Second,
	}
	_, err := CollectFromParca(opts)
	require.NoError(t, err)
	require.Equal(t, "application/json", gotContentType)
	require.Equal(t, "1", gotConnectVersion)
}

func TestCollectFromParca_Success(t *testing.T) {
	fakeData := []byte("FAKE_PPROF_DATA")
	srv := newH2CServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(mergeResponse{
			Pprof: base64.StdEncoding.EncodeToString(fakeData),
		})
	}))
	defer srv.Close()

	opts := Options{
		Source:    SourceParca,
		ParcaAddr: srv.URL,
		Query:     "process_cpu:cpu:nanoseconds:cpu:nanoseconds",
		Window:    5 * time.Minute,
		End:       time.Now(),
		Timeout:   5 * time.Second,
	}
	result, err := CollectFromParca(opts)
	require.NoError(t, err)
	require.Equal(t, fakeData, result.Bytes)
	require.Equal(t, len(result.Bytes), result.SizeBytes)
}

func TestCollectFromParca_ServerError(t *testing.T) {
	srv := newH2CServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("service unavailable"))
	}))
	defer srv.Close()

	opts := Options{
		Source:    SourceParca,
		ParcaAddr: srv.URL,
		Query:     "process_cpu:cpu:nanoseconds:cpu:nanoseconds",
		Window:    5 * time.Minute,
		End:       time.Now(),
		Timeout:   5 * time.Second,
	}
	_, err := CollectFromParca(opts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "503")
}

func TestCollectFromParca_EmptyProfile(t *testing.T) {
	srv := newH2CServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(mergeResponse{Pprof: ""})
	}))
	defer srv.Close()

	opts := Options{
		Source:    SourceParca,
		ParcaAddr: srv.URL,
		Query:     "process_cpu:cpu:nanoseconds:cpu:nanoseconds",
		Window:    5 * time.Minute,
		End:       time.Now(),
		Timeout:   5 * time.Second,
	}
	_, err := CollectFromParca(opts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty profile")
}
