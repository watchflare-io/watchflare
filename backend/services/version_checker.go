package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	githubReleaseURL     = "https://api.github.com/repos/watchflare-io/watchflare/releases/latest"
	versionCheckInterval = 6 * time.Hour
	versionCheckTimeout  = 10 * time.Second
)

var (
	cachedLatestVersion string
	cachedVersionMu     sync.RWMutex
)

type githubReleaseResponse struct {
	TagName string `json:"tag_name"`
}

// GetCachedLatestAgentVersion returns the latest agent version cached from GitHub.
// Returns an empty string if not yet fetched or if the last fetch failed.
func GetCachedLatestAgentVersion() string {
	cachedVersionMu.RLock()
	defer cachedVersionMu.RUnlock()
	return cachedLatestVersion
}

// StartVersionChecker starts a background goroutine that fetches the latest
// agent version from GitHub every 6 hours and caches it in memory.
func StartVersionChecker(ctx context.Context) {
	go func() {
		fetchAndCacheLatestVersion(githubReleaseURL)

		ticker := time.NewTicker(versionCheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				fetchAndCacheLatestVersion(githubReleaseURL)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func fetchAndCacheLatestVersion(url string) {
	version, err := fetchLatestVersionFromGitHub(url)
	if err != nil {
		slog.Warn("failed to fetch latest agent version from GitHub", "error", err)
		return
	}

	cachedVersionMu.Lock()
	cachedLatestVersion = version
	cachedVersionMu.Unlock()

	slog.Debug("latest agent version cached", "version", version)
}

func fetchLatestVersionFromGitHub(url string) (string, error) {
	client := &http.Client{Timeout: versionCheckTimeout}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "watchflare-hub")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status from GitHub API: %s", resp.Status)
	}

	var release githubReleaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return strings.TrimPrefix(release.TagName, "v"), nil
}
