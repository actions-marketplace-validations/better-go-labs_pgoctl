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
		_, _ = w.Write(fakeData)
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
	if !result.Start.Before(result.End) {
		t.Error("expected Start to be before End")
	}
}

func TestCollectFromPprof_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal server error"))
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

func TestCollectFromPprof_WindowOverride(t *testing.T) {
	var receivedSeconds string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSeconds = r.URL.Query().Get("seconds")
		_, _ = w.Write([]byte("fakeprofiledata"))
	}))
	defer ts.Close()

	opts := Options{
		URL:    ts.URL + "/debug/pprof/profile",
		Window: 30 * time.Second,
	}
	result, err := CollectFromPprof(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedSeconds != "30" {
		t.Errorf("expected seconds=30, got %q", receivedSeconds)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
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
