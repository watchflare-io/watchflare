package metrics

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestTagFamilyMatch(t *testing.T) {
	tests := []struct {
		family string
		tag    string
		want   string // "" = no match
	}{
		{"latest", "2.16.0", "2.16.0"},
		{"latest", "2", "2"},
		{"latest", "v5.17.0", "v5.17.0"},
		{"latest", "0.8.7-pg18", ""},
		{"latest", "sha-cc8a645", ""},
		{"latest", "latest", ""},
		{"latest-pg16", "2.30.2-pg16", "2.30.2"},
		{"latest-pg16", "2.30.2", ""},
		{"latest-pg16", "2.30.2-pg16-oss", ""},
		{"pg18", "0.8.7-pg18", "0.8.7"},
		{"pg18", "0.8.7-pg18-bookworm", ""},
		{"1.27", "1.27.3", "1.27.3"},
		{"1.27", "1.27", "1.27"},
		{"1.27", "1.28.0", ""},
		{"1.27", "1.270.0", ""},
		{"16-alpine", "16.4-alpine", "16.4"},
		{"16-alpine", "17.0-alpine", ""},
		{"alpine", "1.27.3-alpine", "1.27.3"},
	}
	for _, tt := range tests {
		got, _, ok := parseTagFamily(tt.family).match(tt.tag)
		if !ok {
			got = ""
		}
		if got != tt.want {
			t.Errorf("family %q, tag %q: got %q, want %q", tt.family, tt.tag, got, tt.want)
		}
	}
}

func TestTagInfoPrefersMostSpecificVersion(t *testing.T) {
	f := parseTagFamily("latest")
	var info tagInfo
	for _, tag := range []string{"latest", "2", "2.16.0", "2.16", "sha-cc8a645"} {
		info.offer(tag, f)
	}
	if info.Version != "2.16.0" || info.Revision != "cc8a645" {
		t.Errorf("got %+v, want version 2.16.0 and revision cc8a645", info)
	}
}

func TestNextLink(t *testing.T) {
	if got := nextLink("https://ghcr.io", `</v2/org/app/tags/list?last=b&n=2>; rel="next"`); got != "https://ghcr.io/v2/org/app/tags/list?last=b&n=2" {
		t.Errorf("relative link = %q", got)
	}
	if got := nextLink("https://ghcr.io", ""); got != "" {
		t.Errorf("empty header = %q", got)
	}
}

// newFakeHub serves Docker Hub's tag API for one repository, two tags per page.
func newFakeHub(t *testing.T, repo string, tags [][2]string, queries *[]string) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/repositories/"+repo+"/tags" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if queries != nil {
			*queries = append(*queries, r.URL.RawQuery)
		}
		page := 0
		if p := r.URL.Query().Get("page"); p == "2" {
			page = 1
		}
		type result struct {
			Name   string `json:"name"`
			Digest string `json:"digest"`
		}
		var body struct {
			Next    string   `json:"next"`
			Results []result `json:"results"`
		}
		for i := page * 2; i < len(tags) && i < page*2+2; i++ {
			body.Results = append(body.Results, result{tags[i][0], tags[i][1]})
		}
		if page == 0 && len(tags) > 2 {
			body.Next = srv.URL + r.URL.Path + "?page=2"
		}
		json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFindVersionTags_DockerHub(t *testing.T) {
	var queries []string
	hub := newFakeHub(t, "jc21/nginx-proxy-manager", [][2]string{
		{"latest", "sha256:new"}, {"2.16.0", "sha256:new"},
		{"2", "sha256:new"}, {"2.15.1", "sha256:old"},
	}, &queries)

	c := NewImageUpdateChecker()
	c.dockerHubBaseURL = hub.URL
	ref, _ := parseImageReference("jc21/nginx-proxy-manager:latest")
	found, err := c.findVersionTags(context.Background(), ref, []string{"sha256:old", "sha256:new"})
	if err != nil {
		t.Fatalf("findVersionTags: %v", err)
	}
	if found["sha256:old"].Version != "2.15.1" || found["sha256:new"].Version != "2.16.0" {
		t.Errorf("versions = %q → %q", found["sha256:old"].Version, found["sha256:new"].Version)
	}
	if len(queries) != 2 || strings.Contains(queries[0], "name=") {
		t.Errorf("expected 2 unfiltered pages, got %v", queries)
	}
}

func TestFindVersionTags_DockerHubFiltersByFamily(t *testing.T) {
	var queries []string
	hub := newFakeHub(t, "timescale/timescaledb", [][2]string{
		{"latest-pg16", "sha256:new"}, {"2.30.2-pg16", "sha256:new"}, {"2.30.1-pg16", "sha256:old"},
	}, &queries)

	c := NewImageUpdateChecker()
	c.dockerHubBaseURL = hub.URL
	ref, _ := parseImageReference("timescale/timescaledb:latest-pg16")
	found, err := c.findVersionTags(context.Background(), ref, []string{"sha256:old", "sha256:new"})
	if err != nil {
		t.Fatalf("findVersionTags: %v", err)
	}
	if found["sha256:old"].Version != "2.30.1" || found["sha256:new"].Version != "2.30.2" {
		t.Errorf("versions = %q → %q, want suffix-free 2.30.1 → 2.30.2", found["sha256:old"].Version, found["sha256:new"].Version)
	}
	if !strings.Contains(queries[0], "name=pg16") {
		t.Errorf("expected name filter, got %q", queries[0])
	}
}

func TestFindVersionTags_Registry(t *testing.T) {
	reg := newFakeRegistry(t, map[string]string{
		"bookorbit/bookorbit:3.2.0":  "sha256:new",
		"bookorbit/bookorbit:3.1.0":  "sha256:old",
		"bookorbit/bookorbit:3.0.0":  "sha256:older",
		"bookorbit/bookorbit:2.10.0": "sha256:oldest",
	})
	reg.tags = []string{"2.10.0", "3.0.0", "3.1.0", "3.2.0", "latest", "pr-12", "sha-abcdef0"}

	c := NewImageUpdateChecker()
	c.registryClient = reg.server.Client()
	c.registryBaseURL = func(string) string { return reg.server.URL }
	ref, _ := parseImageReference("ghcr.io/bookorbit/bookorbit:latest")
	found, err := c.findVersionTags(context.Background(), ref, []string{"sha256:old", "sha256:new"})
	if err != nil {
		t.Fatalf("findVersionTags: %v", err)
	}
	if found["sha256:old"].Version != "3.1.0" || found["sha256:new"].Version != "3.2.0" {
		t.Errorf("versions = %q → %q", found["sha256:old"].Version, found["sha256:new"].Version)
	}
	// Newest candidates are checked first and the search stops once both are found.
	if n := reg.manifests.Load(); n != 2 {
		t.Errorf("HEAD requests = %d, want 2", n)
	}
}

func TestResolveVersions_RollingBuildUsesGitHubCommits(t *testing.T) {
	const localSHA = "f4aa9e0419d7aa000000000000000000000000aa"
	hub := newFakeHub(t, "rssbridge/rss-bridge", [][2]string{
		{"latest", "sha256:new"}, {"sha-cc8a645", "sha256:new"},
	}, nil)

	var githubCalls atomic.Int32
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		githubCalls.Add(1)
		if r.Header.Get("User-Agent") == "" {
			t.Error("GitHub requires a User-Agent")
		}
		compare := map[string][2]int{
			"2025-08-05..." + localSHA: {277, 0},
			"2025-08-05...cc8a645":     {279, 0},
			localSHA + "...cc8a645":    {2, 0},
		}
		switch {
		case r.URL.Path == "/repos/RSS-Bridge/rss-bridge/releases/latest":
			json.NewEncoder(w).Encode(map[string]string{"tag_name": "2025-08-05"})
		case strings.HasPrefix(r.URL.Path, "/repos/RSS-Bridge/rss-bridge/compare/"):
			res, ok := compare[strings.TrimPrefix(r.URL.Path, "/repos/RSS-Bridge/rss-bridge/compare/")]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			json.NewEncoder(w).Encode(map[string]int{"ahead_by": res[0], "behind_by": res[1]})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer github.Close()

	reg := newFakeRegistry(t, nil)
	now := time.Unix(1_800_000_000, 0)
	c := newTestChecker(reg, &now)
	c.dockerHubBaseURL = hub.URL
	c.githubBaseURL = github.URL

	ref, _ := parseImageReference("rssbridge/rss-bridge:latest")
	res := imageResult{
		Ref:          ref,
		Status:       ImageStatusUpdateAvailable,
		LocalDigest:  "sha256:old",
		RemoteDigest: "sha256:new",
		Local:        imageMeta{Revision: localSHA, Source: "https://github.com/RSS-Bridge/rss-bridge"},
	}
	info := c.resolveVersions(context.Background(), res, map[string]bool{})
	want := versionInfo{Local: "2025-08-05+277", Remote: "2025-08-05+279", CommitsBehind: 2}
	if info != want {
		t.Fatalf("got %+v, want %+v", info, want)
	}

	// Compare results are cached; only the release (reset per check) is fetched again.
	calls := githubCalls.Load()
	c.githubReleases = map[string]string{}
	if again := c.resolveVersions(context.Background(), res, map[string]bool{}); again != want {
		t.Errorf("second resolve = %+v", again)
	}
	if extra := githubCalls.Load() - calls; extra != 1 {
		t.Errorf("GitHub calls on second resolve = %d, want 1 (latest release)", extra)
	}
}

func TestResolveVersions_UpToDateShowsTagVersion(t *testing.T) {
	hub := newFakeHub(t, "freshrss/freshrss", [][2]string{{"latest", "sha256:cur"}, {"1.30.0", "sha256:cur"}}, nil)
	reg := newFakeRegistry(t, nil)
	now := time.Unix(1_800_000_000, 0)
	c := newTestChecker(reg, &now)
	c.dockerHubBaseURL = hub.URL

	ref, _ := parseImageReference("freshrss/freshrss:latest")
	info := c.resolveVersions(context.Background(), imageResult{Ref: ref, Status: ImageStatusUpToDate, LocalDigest: "sha256:cur"}, map[string]bool{})
	if info != (versionInfo{Local: "1.30.0"}) {
		t.Errorf("got %+v", info)
	}
}
