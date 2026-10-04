package metrics

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestParseImageReference(t *testing.T) {
	tests := []struct {
		ref  string
		want imageReference
	}{
		{"nginx", imageReference{Registry: "docker.io", Repo: "library/nginx", Tag: "latest"}},
		{"nginx:1.27", imageReference{Registry: "docker.io", Repo: "library/nginx", Tag: "1.27"}},
		{"freshrss/freshrss:latest", imageReference{Registry: "docker.io", Repo: "freshrss/freshrss", Tag: "latest"}},
		{"docker.io/library/redis:7", imageReference{Registry: "docker.io", Repo: "library/redis", Tag: "7"}},
		{"index.docker.io/jc21/nginx-proxy-manager", imageReference{Registry: "docker.io", Repo: "jc21/nginx-proxy-manager", Tag: "latest"}},
		{"ghcr.io/watchflare-io/watchflare:latest", imageReference{Registry: "ghcr.io", Repo: "watchflare-io/watchflare", Tag: "latest"}},
		{"localhost:5000/app", imageReference{Registry: "localhost:5000", Repo: "app", Tag: "latest"}},
		{"localhost/app:dev", imageReference{Registry: "localhost", Repo: "app", Tag: "dev"}},
		{"timescale/timescaledb:latest-pg16", imageReference{Registry: "docker.io", Repo: "timescale/timescaledb", Tag: "latest-pg16"}},
		{"nginx@sha256:abc", imageReference{Registry: "docker.io", Repo: "library/nginx", Digest: "sha256:abc"}},
		{"ghcr.io/org/app:1.0@sha256:abc", imageReference{Registry: "ghcr.io", Repo: "org/app", Tag: "1.0", Digest: "sha256:abc"}},
	}
	for _, tt := range tests {
		got, err := parseImageReference(tt.ref)
		if err != nil {
			t.Errorf("parseImageReference(%q) error: %v", tt.ref, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseImageReference(%q) = %+v, want %+v", tt.ref, got, tt.want)
		}
	}
}

func TestParseImageReference_Invalid(t *testing.T) {
	for _, ref := range []string{"", "   ", ":tag"} {
		if _, err := parseImageReference(ref); err == nil {
			t.Errorf("parseImageReference(%q) expected error", ref)
		}
	}
}

func TestMatchingRepoDigests(t *testing.T) {
	ref, _ := parseImageReference("nginx:latest")
	repoDigests := []string{
		"nginx@sha256:aaa",
		"docker.io/library/nginx@sha256:bbb",
		"ghcr.io/library/nginx@sha256:ccc",
		"other/nginx@sha256:ddd",
		"malformed",
	}
	got := matchingRepoDigests(repoDigests, ref)
	if len(got) != 2 || got[0] != "sha256:aaa" || got[1] != "sha256:bbb" {
		t.Errorf("matchingRepoDigests = %v, want [sha256:aaa sha256:bbb]", got)
	}
}

func TestParseBearerChallenge(t *testing.T) {
	params, ok := parseBearerChallenge(`Bearer realm="https://auth.docker.io/token",service="registry.docker.io",scope="repository:library/nginx:pull"`)
	if !ok {
		t.Fatal("expected Bearer challenge to parse")
	}
	if params["realm"] != "https://auth.docker.io/token" || params["service"] != "registry.docker.io" || params["scope"] != "repository:library/nginx:pull" {
		t.Errorf("unexpected params: %v", params)
	}

	if _, ok := parseBearerChallenge(`Basic realm="registry"`); ok {
		t.Error("Basic challenge should not parse as Bearer")
	}
}

// fakeRegistry serves manifests for repo:tag → digest and requires a Bearer token
// obtained from its /token endpoint, like Docker Hub and GHCR do for anonymous pulls.
type fakeRegistry struct {
	server    *httptest.Server
	digests   map[string]string // "repo:tag" → digest
	objects   map[string]string // digest → JSON body of a manifest, index or config blob
	tags      []string          // served by /tags/list, two per page
	manifests atomic.Int32      // HEAD manifest requests
	gets      atomic.Int32      // GET manifest requests (count against Docker Hub limits)
}

func newFakeRegistry(t *testing.T, digests map[string]string) *fakeRegistry {
	t.Helper()
	fr := &fakeRegistry{digests: digests, objects: map[string]string{}}
	fr.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			json.NewEncoder(w).Encode(map[string]string{"token": "tok-" + r.URL.Query().Get("scope")})
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/v2/")
		repo, ref, isManifest := strings.Cut(path, "/manifests/")
		isTagList := !isManifest && strings.HasSuffix(path, "/tags/list")
		switch {
		case isTagList:
			repo = strings.TrimSuffix(path, "/tags/list")
		case !isManifest:
			repo, ref, _ = strings.Cut(path, "/blobs/")
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer tok-repository:"+repo+":pull") {
			w.Header().Set("WWW-Authenticate", `Bearer realm="`+fr.server.URL+`/token",service="fake"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if isTagList {
			start := 0
			for i, tag := range fr.tags {
				if tag == r.URL.Query().Get("last") {
					start = i + 1
				}
			}
			end := min(start+2, len(fr.tags))
			if end < len(fr.tags) {
				w.Header().Set("Link", `</v2/`+repo+`/tags/list?last=`+fr.tags[end-1]+`&n=2>; rel="next"`)
			}
			json.NewEncoder(w).Encode(map[string]any{"name": repo, "tags": fr.tags[start:end]})
			return
		}
		if r.Method == http.MethodGet {
			if isManifest {
				fr.gets.Add(1)
			}
			body, ok := fr.objects[ref]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Write([]byte(body))
			return
		}
		tag := ref
		fr.manifests.Add(1)
		digest, ok := fr.digests[repo+":"+tag]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Docker-Content-Digest", digest)
	}))
	t.Cleanup(fr.server.Close)
	return fr
}

// newFakeDaemon serves the Docker Engine endpoints used by the image checker.
// images maps image ID → RepoDigests; meta optionally adds labels/platform.
func newFakeDaemon(t *testing.T, containers []daemonContainer, images map[string][]string, meta ...map[string]daemonImage) *http.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/"+dockerAPIVersion)
		switch {
		case path == "/containers/json":
			json.NewEncoder(w).Encode(containers)
		case strings.HasPrefix(path, "/images/"):
			id, _ := url.PathUnescape(strings.TrimSuffix(strings.TrimPrefix(path, "/images/"), "/json"))
			digests, ok := images[id]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			image := daemonImage{Os: "linux", Architecture: "amd64"}
			if len(meta) > 0 {
				if m, ok := meta[0][id]; ok {
					image = m
				}
			}
			image.RepoDigests = digests
			json.NewEncoder(w).Encode(image)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	target, _ := url.Parse(srv.URL)
	return &http.Client{Transport: rewriteHostTransport{target: target}}
}

// rewriteHostTransport sends requests for http://localhost/... to a test server.
type rewriteHostTransport struct{ target *url.URL }

func (rt rewriteHostTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.URL.Scheme = rt.target.Scheme
	r.URL.Host = rt.target.Host
	return http.DefaultTransport.RoundTrip(r)
}

func newTestChecker(reg *fakeRegistry, now *time.Time) *ImageUpdateChecker {
	c := NewImageUpdateChecker()
	c.registryClient = reg.server.Client()
	c.registryBaseURL = func(string) string { return reg.server.URL }
	c.now = func() time.Time { return *now }
	// Never reach the real Docker Hub API or GitHub from tests.
	c.dockerHubBaseURL = reg.server.URL + "/hub"
	c.githubBaseURL = reg.server.URL + "/github"
	c.githubReleases = map[string]string{}
	return c
}

func TestImageUpdateChecker_Statuses(t *testing.T) {
	reg := newFakeRegistry(t, map[string]string{
		"library/nginx:latest": "sha256:new",
		"org/app:1.0":          "sha256:same",
	})
	daemon := newFakeDaemon(t,
		[]daemonContainer{
			{ID: "aaaaaaaaaaaaaaaa", Names: []string{"/web"}, Image: "nginx:latest", ImageID: "sha256:img1", State: "running"},
			{ID: "bbbbbbbbbbbbbbbb", Names: []string{"/app"}, Image: "ghcr.io/org/app:1.0", ImageID: "sha256:img2", State: "running"},
			{ID: "cccccccccccccccc", Names: []string{"/built"}, Image: "mybuild:dev", ImageID: "sha256:img3", State: "running"},
			{ID: "dddddddddddddddd", Names: []string{"/pinned"}, Image: "redis@sha256:pin", ImageID: "sha256:img4", State: "running"},
			{ID: "eeeeeeeeeeeeeeee", Names: []string{"/gone"}, Image: "org/missing:1", ImageID: "sha256:img5", State: "running"},
			{ID: "ffffffffffffffff", Names: []string{"/stopped"}, Image: "nginx:latest", ImageID: "sha256:img1", State: "exited"},
		},
		map[string][]string{
			"sha256:img1": {"nginx@sha256:old"},
			"sha256:img2": {"ghcr.io/org/app@sha256:same"},
			"sha256:img3": {},
			"sha256:img4": {"redis@sha256:pin"},
			"sha256:img5": {"org/missing@sha256:x"},
		},
	)

	now := time.Unix(1_700_000_000, 0)
	c := newTestChecker(reg, &now)
	updates, err := c.checkSocket(context.Background(), daemon, map[string]bool{})
	if err != nil {
		t.Fatalf("checkSocket: %v", err)
	}

	byName := make(map[string]ImageUpdate)
	for _, u := range updates {
		byName[u.ContainerName] = u
	}
	if len(byName) != 5 {
		t.Fatalf("expected 5 running containers, got %d: %+v", len(byName), updates)
	}

	want := map[string]string{
		"web":    ImageStatusUpdateAvailable,
		"app":    ImageStatusUpToDate,
		"built":  ImageStatusLocal,
		"pinned": ImageStatusPinned,
		"gone":   ImageStatusLocal,
	}
	for name, status := range want {
		if got := byName[name].Status; got != status {
			t.Errorf("%s: status = %q, want %q (error: %s)", name, got, status, byName[name].Error)
		}
	}

	web := byName["web"]
	if web.ContainerID != "aaaaaaaaaaaa" || web.LocalDigest != "sha256:old" || web.RemoteDigest != "sha256:new" || !web.CheckedAt.Equal(now) {
		t.Errorf("unexpected web result: %+v", web)
	}
}

func TestImageUpdateChecker_CachesUntilMaxAge(t *testing.T) {
	reg := newFakeRegistry(t, map[string]string{"library/nginx:latest": "sha256:old"})
	daemon := newFakeDaemon(t,
		[]daemonContainer{{ID: "aaaaaaaaaaaaaaaa", Names: []string{"/web"}, Image: "nginx:latest", ImageID: "sha256:img1", State: "running"}},
		map[string][]string{"sha256:img1": {"nginx@sha256:old"}},
	)

	now := time.Unix(1_700_000_000, 0)
	c := newTestChecker(reg, &now)
	check := func() ImageUpdate {
		t.Helper()
		updates, err := c.checkSocket(context.Background(), daemon, map[string]bool{})
		if err != nil || len(updates) != 1 {
			t.Fatalf("checkSocket: %v, %d updates", err, len(updates))
		}
		return updates[0]
	}

	if got := check().Status; got != ImageStatusUpToDate {
		t.Fatalf("initial status = %q", got)
	}

	// The registry moves the tag, but the cached result is still fresh.
	reg.digests["library/nginx:latest"] = "sha256:new"
	now = now.Add(5 * time.Hour)
	if got := check().Status; got != ImageStatusUpToDate {
		t.Errorf("status within max age = %q, want cached up_to_date", got)
	}
	if n := reg.manifests.Load(); n != 1 {
		t.Errorf("registry manifest requests = %d, want 1", n)
	}

	now = now.Add(time.Hour)
	if got := check().Status; got != ImageStatusUpdateAvailable {
		t.Errorf("status after max age = %q, want update_available", got)
	}
}

func TestFetchRemoteDigest_DeniedAfterToken(t *testing.T) {
	// Docker Hub hands out anonymous tokens for any repo but then rejects
	// manifests of repos that don't exist (or are private) with 401.
	reg := newFakeRegistry(t, nil)
	c := NewImageUpdateChecker()
	c.registryClient = reg.server.Client()
	c.registryBaseURL = func(string) string { return reg.server.URL }
	ref, _ := parseImageReference("watchflare-local:image-updates")
	reg.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			json.NewEncoder(w).Encode(map[string]string{"token": "anon"})
			return
		}
		w.Header().Set("WWW-Authenticate", `Bearer realm="`+reg.server.URL+`/token",service="fake",error="insufficient_scope"`)
		w.WriteHeader(http.StatusUnauthorized)
	})
	if _, err := c.fetchRemoteDigest(context.Background(), ref); !errors.Is(err, errNotPublic) {
		t.Errorf("expected errNotPublic, got %v", err)
	}
}

func TestFetchRemoteDigest_ServerErrorIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	c := NewImageUpdateChecker()
	c.registryClient = srv.Client()
	c.registryBaseURL = func(string) string { return srv.URL }
	ref, _ := parseImageReference("registry.example.com/app:1")
	_, err := c.fetchRemoteDigest(context.Background(), ref)
	if err == nil || errors.Is(err, errNotPublic) {
		t.Errorf("expected a real error for 502, got %v", err)
	}
}

func TestFetchRemoteDigest_UnauthorizedWithoutChallenge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := NewImageUpdateChecker()
	c.registryClient = srv.Client()
	c.registryBaseURL = func(string) string { return srv.URL }
	ref, _ := parseImageReference("registry.example.com/private/app:1")
	if _, err := c.fetchRemoteDigest(context.Background(), ref); !errors.Is(err, errNotPublic) {
		t.Errorf("expected errNotPublic, got %v", err)
	}
}

func TestFetchRemoteDigest_FallsBackToBodyHash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Write([]byte("manifest"))
		}
	}))
	defer srv.Close()

	c := NewImageUpdateChecker()
	c.registryClient = srv.Client()
	c.registryBaseURL = func(string) string { return srv.URL }
	ref, _ := parseImageReference("registry.example.com/app:1")
	got, err := c.fetchRemoteDigest(context.Background(), ref)
	if err != nil {
		t.Fatalf("fetchRemoteDigest: %v", err)
	}
	// sha256("manifest")
	if want := "sha256:05b3abf2579a5eb66403cd78be557fd860633a1fe2103c7642030defe32c657f"; got != want {
		t.Errorf("digest = %q, want %q", got, want)
	}
}

func TestMetaFromLabels(t *testing.T) {
	m := metaFromLabels(map[string]string{
		"org.opencontainers.image.version":  "latest",
		"org.opencontainers.image.revision": "f4aa9e0419d7abc",
		"org.opencontainers.image.source":   "https://github.com/RSS-Bridge/rss-bridge",
	})
	if m.Version != "" || m.Revision != "f4aa9e0419d7abc" || m.Source != "https://github.com/RSS-Bridge/rss-bridge" {
		t.Errorf("unexpected meta: %+v", m)
	}
	if v := metaFromLabels(map[string]string{"org.label-schema.version": "2.0"}).Version; v != "2.0" {
		t.Errorf("label-schema version = %q", v)
	}
	if m := metaFromLabels(nil); m != (imageMeta{}) {
		t.Errorf("nil labels = %+v", m)
	}
}

// addRemoteImage registers a multi-arch image in the fake registry and returns its index digest.
func (fr *fakeRegistry) addRemoteImage(name, version string) string {
	config := `{"config":{"Labels":{"org.opencontainers.image.version":"` + version + `"}}}`
	fr.objects["sha256:cfg-"+name] = config
	fr.objects["sha256:amd64-"+name] = `{"config":{"digest":"sha256:cfg-` + name + `"}}`
	fr.objects["sha256:arm64-"+name] = `{"config":{"digest":"sha256:wrong"}}`
	index := "sha256:index-" + name
	fr.objects[index] = `{"manifests":[
		{"digest":"sha256:arm64-` + name + `","platform":{"os":"linux","architecture":"arm64","variant":"v8"}},
		{"digest":"sha256:att-` + name + `","platform":{"os":"unknown","architecture":"unknown"}},
		{"digest":"sha256:amd64-` + name + `","platform":{"os":"linux","architecture":"amd64"}}
	]}`
	return index
}

func TestImageUpdateChecker_VersionsFromLabels(t *testing.T) {
	reg := newFakeRegistry(t, map[string]string{})
	reg.digests["library/nginx:latest"] = reg.addRemoteImage("nginx", "1.27.3")

	localImage := daemonImage{Os: "linux", Architecture: "amd64"}
	localImage.Config.Labels = map[string]string{"org.opencontainers.image.version": "1.27.0"}
	daemon := newFakeDaemon(t,
		[]daemonContainer{{ID: "aaaaaaaaaaaaaaaa", Names: []string{"/web"}, Image: "nginx:latest", ImageID: "sha256:img1", State: "running"}},
		map[string][]string{"sha256:img1": {"nginx@sha256:old"}},
		map[string]daemonImage{"sha256:img1": localImage},
	)

	now := time.Unix(1_800_000_000, 0)
	c := newTestChecker(reg, &now)
	c.metaBudget = c.metaFetchesPerCheck
	updates, err := c.checkSocket(context.Background(), daemon, map[string]bool{})
	if err != nil || len(updates) != 1 {
		t.Fatalf("checkSocket: %v, %d updates", err, len(updates))
	}
	u := updates[0]
	if u.Status != ImageStatusUpdateAvailable {
		t.Fatalf("status = %q", u.Status)
	}
	// No version tags in the (fake) registry: both sides fall back to image labels.
	if u.LocalVersion != "1.27.0" || u.RemoteVersion != "1.27.3" {
		t.Errorf("versions = %q → %q, want 1.27.0 → 1.27.3", u.LocalVersion, u.RemoteVersion)
	}
}

func TestImageUpdateChecker_MetaBudgetAndCache(t *testing.T) {
	reg := newFakeRegistry(t, map[string]string{})
	reg.digests["library/nginx:latest"] = reg.addRemoteImage("nginx", "1.27.3")
	reg.digests["library/redis:7"] = reg.addRemoteImage("redis", "7.4.1")
	daemon := newFakeDaemon(t,
		[]daemonContainer{
			{ID: "aaaaaaaaaaaaaaaa", Names: []string{"/web"}, Image: "nginx:latest", ImageID: "sha256:img1", State: "running"},
			{ID: "bbbbbbbbbbbbbbbb", Names: []string{"/cache"}, Image: "redis:7", ImageID: "sha256:img2", State: "running"},
		},
		map[string][]string{"sha256:img1": {"nginx@sha256:old"}, "sha256:img2": {"redis@sha256:old"}},
	)

	now := time.Unix(1_800_000_000, 0)
	c := newTestChecker(reg, &now)
	c.metaFetchesPerCheck = 1
	check := func() map[string]ImageUpdate {
		t.Helper()
		c.metaBudget = c.metaFetchesPerCheck
		updates, err := c.checkSocket(context.Background(), daemon, map[string]bool{})
		if err != nil {
			t.Fatalf("checkSocket: %v", err)
		}
		out := map[string]ImageUpdate{}
		for _, u := range updates {
			out[u.ContainerName] = u
		}
		return out
	}

	first := check()
	if (first["web"].RemoteVersion == "") == (first["cache"].RemoteVersion == "") {
		t.Fatalf("budget of 1 should fill exactly one container: %+v", first)
	}
	second := check()
	if second["web"].RemoteVersion != "1.27.3" || second["cache"].RemoteVersion != "7.4.1" {
		t.Fatalf("second check should fill the rest: %+v", second)
	}
	gets := reg.gets.Load()
	check()
	if reg.gets.Load() != gets {
		t.Errorf("cached metadata must not be fetched again (%d → %d GETs)", gets, reg.gets.Load())
	}
}
