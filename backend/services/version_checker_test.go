package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetCachedLatestAgentVersion_EmptyByDefault(t *testing.T) {
	// Reset state between tests.
	cachedVersionMu.Lock()
	cachedLatestVersion = ""
	cachedVersionMu.Unlock()

	if got := GetCachedLatestAgentVersion(); got != "" {
		t.Errorf("expected empty string before any fetch, got %q", got)
	}
}

func TestGetCachedLatestAgentVersion_ReturnsStoredValue(t *testing.T) {
	cachedVersionMu.Lock()
	cachedLatestVersion = "1.2.3"
	cachedVersionMu.Unlock()
	defer func() {
		cachedVersionMu.Lock()
		cachedLatestVersion = ""
		cachedVersionMu.Unlock()
	}()

	if got := GetCachedLatestAgentVersion(); got != "1.2.3" {
		t.Errorf("got %q, want 1.2.3", got)
	}
}

func TestFetchLatestVersionFromGitHub_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(githubReleaseResponse{TagName: "v1.5.0"})
	}))
	defer srv.Close()

	version, err := fetchLatestVersionFromGitHub(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version != "1.5.0" {
		t.Errorf("got %q, want 1.5.0", version)
	}
}

func TestFetchLatestVersionFromGitHub_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	_, err := fetchLatestVersionFromGitHub(srv.URL)
	if err == nil {
		t.Error("expected error for non-200 status")
	}
}

func TestFetchLatestVersionFromGitHub_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	_, err := fetchLatestVersionFromGitHub(srv.URL)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestFetchAndCacheLatestVersion_UpdatesCache(t *testing.T) {
	cachedVersionMu.Lock()
	cachedLatestVersion = ""
	cachedVersionMu.Unlock()
	defer func() {
		cachedVersionMu.Lock()
		cachedLatestVersion = ""
		cachedVersionMu.Unlock()
	}()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(githubReleaseResponse{TagName: "v2.0.0"})
	}))
	defer srv.Close()

	fetchAndCacheLatestVersion(srv.URL)

	if got := GetCachedLatestAgentVersion(); got != "2.0.0" {
		t.Errorf("got %q, want 2.0.0", got)
	}
}

func TestFetchAndCacheLatestVersion_ErrorLeavesCache(t *testing.T) {
	cachedVersionMu.Lock()
	cachedLatestVersion = "1.0.0"
	cachedVersionMu.Unlock()
	defer func() {
		cachedVersionMu.Lock()
		cachedLatestVersion = ""
		cachedVersionMu.Unlock()
	}()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	fetchAndCacheLatestVersion(srv.URL)

	// On error, the cached value must be unchanged.
	if got := GetCachedLatestAgentVersion(); got != "1.0.0" {
		t.Errorf("got %q, want 1.0.0 (cache must not be cleared on error)", got)
	}
}
