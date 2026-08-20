package collect

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCollectFromPprof_Success(t *testing.T) {
	fakeData := []byte("FAKE_PPROF_DATA")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(fakeData)
	}))
	defer srv.Close()

	opts := Options{
		Source:  SourcePprof,
		URL:     srv.URL + "/debug/pprof/profile",
		Timeout: 5 * time.Second,
	}
	result, err := CollectFromPprof(opts)
	require.NoError(t, err)
	require.Equal(t, fakeData, result.Bytes)
	require.Equal(t, len(result.Bytes), result.SizeBytes)
	require.Equal(t, SourcePprof, result.Source)
}

func TestCollectFromPprof_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer srv.Close()

	opts := Options{
		Source:  SourcePprof,
		URL:     srv.URL + "/debug/pprof/profile",
		Timeout: 5 * time.Second,
	}
	_, err := CollectFromPprof(opts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "500")
}

func TestCollectFromPprof_EmptyProfile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	opts := Options{
		Source:  SourcePprof,
		URL:     srv.URL + "/debug/pprof/profile",
		Timeout: 5 * time.Second,
	}
	_, err := CollectFromPprof(opts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty profile")
}
