package metrics

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Image update statuses reported to the Hub.
const (
	ImageStatusUpToDate        = "up_to_date"
	ImageStatusUpdateAvailable = "update_available"
	ImageStatusPinned          = "pinned" // referenced by digest, the tag can't move
	ImageStatusLocal           = "local"  // not in a public registry: built locally, loaded from a tarball, or private
	ImageStatusError           = "error"
)

const (
	defaultImageCheckMaxAge = 6 * time.Hour
	// imageCheckErrorMaxAge is shorter so transient registry failures recover sooner.
	imageCheckErrorMaxAge = 1 * time.Hour
	// defaultMetaFetchesPerCheck caps registry manifest GETs per check. Unlike HEAD
	// requests they count against Docker Hub's anonymous pull limit, so version
	// lookups for many new images are spread over several checks.
	defaultMetaFetchesPerCheck = 3
	dockerHubRegistry          = "docker.io"
	dockerHubAPIHost           = "registry-1.docker.io"
)

// errNotPublic means the registry doesn't serve the image anonymously. Docker Hub
// answers "unauthorized" for repositories that don't exist, so a locally built
// image and a private one are indistinguishable; neither can be checked.
var errNotPublic = errors.New("not found in a public registry")

// manifestAcceptTypes asks for the multi-arch index first so the digest matches
// the one Docker records in RepoDigests on pull.
var manifestAcceptTypes = strings.Join([]string{
	"application/vnd.oci.image.index.v1+json",
	"application/vnd.docker.distribution.manifest.list.v2+json",
	"application/vnd.docker.distribution.manifest.v2+json",
	"application/vnd.oci.image.manifest.v1+json",
}, ", ")

// ImageUpdate is the registry update state of the image a running container uses.
// Versions come from version tags, image labels or, for rolling builds, the latest
// GitHub release plus commits ("2025-08-05+277"); empty if unknown.
type ImageUpdate struct {
	ContainerID   string
	ContainerName string
	Image         string
	LocalDigest   string
	RemoteDigest  string
	Status        string
	Error         string
	CheckedAt     time.Time
	LocalVersion  string
	RemoteVersion string
	CommitsBehind int // commits between running and registry image (rolling builds only)
}

// imageResult is the cached outcome of a registry lookup for one (image ref, image ID) pair.
type imageResult struct {
	LocalDigest  string
	RemoteDigest string
	Status       string
	Error        string
	CheckedAt    time.Time
	Local        imageMeta
	Platform     imagePlatform // platform of the local image, used to pick the remote manifest
	Ref          imageReference
}

// imageMeta is the version information an image carries in its labels.
type imageMeta struct {
	Version  string // version label ("latest" ignored)
	Revision string // VCS revision label
	Source   string // source repository URL label
}

type imagePlatform struct {
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
	Variant      string `json:"variant"`
}

// imageReference is a parsed image reference such as "ghcr.io/org/app:1.2".
type imageReference struct {
	Registry string // "docker.io", "ghcr.io", ...
	Repo     string // "library/nginx", "org/app"
	Tag      string // "latest" when omitted
	Digest   string // "sha256:..." when referenced by digest
}

// ImageUpdateChecker compares the images of running containers against their registries.
// Lookups are cached per (image ref, image ID) so the registry is only queried for
// images that are new or whose result is older than MaxAge.
type ImageUpdateChecker struct {
	MaxAge time.Duration

	registryClient *http.Client
	// registryBaseURL maps a registry name to its API base URL (overridable in tests).
	registryBaseURL func(registry string) string
	now             func() time.Time
	cache           map[string]imageResult
	// remoteMeta caches registry image metadata by "registry/repo@digest"; a digest's
	// metadata never changes, so entries live as long as a container needs them.
	remoteMeta          map[string]imageMeta
	metaFetchesPerCheck int
	metaBudget          int

	dockerHubBaseURL string
	githubBaseURL    string
	tagCache         map[string]tagInfo // "registry/repo@digest" → tags pointing at it
	githubCache      map[string][2]int  // compare results (ahead, behind)
	githubReleases   map[string]string  // latest release per repo, reset every check
}

// NewImageUpdateChecker returns a checker with the default 6h re-check interval.
func NewImageUpdateChecker() *ImageUpdateChecker {
	return &ImageUpdateChecker{
		MaxAge:          defaultImageCheckMaxAge,
		registryClient:  &http.Client{Timeout: 15 * time.Second},
		registryBaseURL: defaultRegistryBaseURL,
		now:             time.Now,
		cache:           make(map[string]imageResult),

		remoteMeta:          make(map[string]imageMeta),
		metaFetchesPerCheck: defaultMetaFetchesPerCheck,

		dockerHubBaseURL: dockerHubAPIBaseURL,
		githubBaseURL:    githubAPIBaseURL,
		tagCache:         make(map[string]tagInfo),
		githubCache:      make(map[string][2]int),
	}
}

func defaultRegistryBaseURL(registry string) string {
	if registry == dockerHubRegistry {
		return "https://" + dockerHubAPIHost
	}
	return "https://" + registry
}

// Check returns the update state of every running container across all accessible
// container runtime sockets, sorted by container name.
func (c *ImageUpdateChecker) Check(ctx context.Context) ([]ImageUpdate, error) {
	var all []ImageUpdate
	var lastErr error
	collected := make(map[string]bool)
	seen := make(map[string]bool)
	c.metaBudget = c.metaFetchesPerCheck
	c.githubReleases = make(map[string]string)

	for _, sock := range knownContainerSockets() {
		if collected[sock.runtime] {
			continue
		}
		updates, err := c.checkSocket(ctx, newSocketClient(sock.path), seen)
		if err != nil {
			slog.Debug("container runtime unavailable for image check", "runtime", sock.runtime, "path", sock.path, "error", err)
			lastErr = err
			continue
		}
		all = append(all, updates...)
		collected[sock.runtime] = true
	}

	if len(collected) == 0 && lastErr != nil {
		return nil, lastErr
	}

	// Drop cache entries for images no running container uses anymore.
	for key := range c.cache {
		if !seen[key] {
			delete(c.cache, key)
		}
	}
	for key := range c.remoteMeta {
		if !seen[key] {
			delete(c.remoteMeta, key)
		}
	}
	for key := range c.tagCache {
		if !seen[key] {
			delete(c.tagCache, key)
		}
	}

	sort.Slice(all, func(i, j int) bool { return all[i].ContainerName < all[j].ContainerName })
	return all, nil
}

// checkSocket checks all running containers reachable through one daemon socket.
// Every cache key used is recorded in seen.
func (c *ImageUpdateChecker) checkSocket(ctx context.Context, daemon *http.Client, seen map[string]bool) ([]ImageUpdate, error) {
	containers, err := listDaemonContainers(ctx, daemon)
	if err != nil {
		return nil, err
	}

	var updates []ImageUpdate
	for _, ct := range containers {
		if ct.State != "running" {
			continue
		}

		ref := ct.Image
		// Docker reports the image ID instead of the name when the tag was moved
		// to another image after the container was created; recover the original name.
		if strings.HasPrefix(ref, "sha256:") {
			if name, err := containerConfigImage(ctx, daemon, ct.ID); err == nil && name != "" {
				ref = name
			}
		}

		key := ref + "|" + ct.ImageID
		seen[key] = true
		res, ok := c.cache[key]
		if !ok || c.expired(res) {
			res = c.resolve(ctx, daemon, ref, ct.ImageID)
			c.cache[key] = res
		}

		name := truncateID(ct.ID)
		if len(ct.Names) > 0 {
			name = strings.TrimPrefix(ct.Names[0], "/")
		}

		versions := versionInfo{Local: res.Local.Version}
		if res.LocalDigest != "" && (res.Status == ImageStatusUpToDate || res.Status == ImageStatusUpdateAvailable) {
			versions = c.resolveVersions(ctx, res, seen)
		}

		updates = append(updates, ImageUpdate{
			ContainerID:   truncateID(ct.ID),
			ContainerName: name,
			Image:         ref,
			LocalDigest:   res.LocalDigest,
			RemoteDigest:  res.RemoteDigest,
			Status:        res.Status,
			Error:         res.Error,
			CheckedAt:     res.CheckedAt,
			LocalVersion:  versions.Local,
			RemoteVersion: versions.Remote,
			CommitsBehind: versions.CommitsBehind,
		})
	}
	return updates, nil
}

// remoteImageMeta returns the cached label metadata of a registry image, fetching
// it while this check's budget lasts. Failures are not cached, so the
// lookup is retried on the next check.
func (c *ImageUpdateChecker) remoteImageMeta(ctx context.Context, key string, res imageResult) imageMeta {
	if meta, ok := c.remoteMeta[key]; ok {
		return meta
	}
	if c.metaBudget <= 0 {
		return imageMeta{}
	}
	c.metaBudget--
	meta, err := c.fetchRemoteMeta(ctx, res.Ref, res.RemoteDigest, res.Platform)
	if err != nil {
		slog.Debug("registry metadata lookup failed", "image", key, "error", err)
		return imageMeta{}
	}
	c.remoteMeta[key] = meta
	return meta
}

func (c *ImageUpdateChecker) expired(res imageResult) bool {
	maxAge := c.MaxAge
	if res.Status == ImageStatusError {
		maxAge = imageCheckErrorMaxAge
	}
	return c.now().Sub(res.CheckedAt) >= maxAge
}

// resolve determines the update state of one image by comparing the repo digest of
// the image the container runs with the digest the tag resolves to in the registry.
func (c *ImageUpdateChecker) resolve(ctx context.Context, daemon *http.Client, ref, imageID string) imageResult {
	res := imageResult{CheckedAt: c.now()}

	parsed, err := parseImageReference(ref)
	if err != nil {
		res.Status, res.Error = ImageStatusError, err.Error()
		return res
	}
	res.Ref = parsed

	image, err := inspectDaemonImage(ctx, daemon, imageID)
	if err != nil {
		res.Status, res.Error = ImageStatusError, fmt.Sprintf("inspect image: %v", err)
		return res
	}
	res.Local = metaFromLabels(image.Config.Labels)
	res.Platform = imagePlatform{OS: image.Os, Architecture: image.Architecture, Variant: image.Variant}

	if parsed.Digest != "" {
		res.Status, res.LocalDigest = ImageStatusPinned, parsed.Digest
		return res
	}

	localDigests := matchingRepoDigests(image.RepoDigests, parsed)
	if len(localDigests) == 0 {
		res.Status = ImageStatusLocal
		return res
	}
	res.LocalDigest = localDigests[0]

	remote, err := c.fetchRemoteDigest(ctx, parsed)
	if errors.Is(err, errNotPublic) {
		res.Status = ImageStatusLocal
		return res
	}
	if err != nil {
		slog.Debug("registry digest lookup failed", "image", ref, "error", err)
		res.Status, res.Error = ImageStatusError, err.Error()
		return res
	}
	res.RemoteDigest = remote

	res.Status = ImageStatusUpdateAvailable
	for _, d := range localDigests {
		if d == remote {
			res.Status, res.LocalDigest = ImageStatusUpToDate, d
			break
		}
	}
	return res
}

// parseImageReference parses a Docker image reference, applying the same defaults
// as the Docker CLI (docker.io registry, "library/" namespace, "latest" tag).
func parseImageReference(ref string) (imageReference, error) {
	var r imageReference
	name := strings.TrimSpace(ref)
	if name == "" {
		return r, errors.New("empty image reference")
	}

	if i := strings.Index(name, "@"); i >= 0 {
		r.Digest = name[i+1:]
		name = name[:i]
	}

	// The first path component is a registry host if it contains a dot or a port,
	// or is "localhost"; otherwise the image lives on Docker Hub.
	if i := strings.Index(name, "/"); i >= 0 && (strings.ContainsAny(name[:i], ".:") || name[:i] == "localhost") {
		r.Registry = name[:i]
		name = name[i+1:]
	} else {
		r.Registry = dockerHubRegistry
	}
	if r.Registry == "index.docker.io" || r.Registry == dockerHubAPIHost {
		r.Registry = dockerHubRegistry
	}

	// A tag follows the last colon, but only if that colon is after the last slash.
	if i := strings.LastIndex(name, ":"); i > strings.LastIndex(name, "/") {
		r.Tag = name[i+1:]
		name = name[:i]
	}
	if r.Tag == "" && r.Digest == "" {
		r.Tag = "latest"
	}

	if name == "" {
		return r, fmt.Errorf("invalid image reference %q", ref)
	}
	if r.Registry == dockerHubRegistry && !strings.Contains(name, "/") {
		name = "library/" + name
	}
	r.Repo = name
	return r, nil
}

// matchingRepoDigests returns the digests from an image's RepoDigests
// ("repo@sha256:...") that belong to the same registry and repository as ref.
func matchingRepoDigests(repoDigests []string, ref imageReference) []string {
	var out []string
	for _, rd := range repoDigests {
		parsed, err := parseImageReference(rd)
		if err != nil || parsed.Digest == "" {
			continue
		}
		if parsed.Registry == ref.Registry && parsed.Repo == ref.Repo {
			out = append(out, parsed.Digest)
		}
	}
	return out
}

// fetchRemoteDigest resolves the tag of ref to a manifest digest using the registry
// v2 API. HEAD requests are used so Docker Hub doesn't count them against pull limits.
func (c *ImageUpdateChecker) fetchRemoteDigest(ctx context.Context, ref imageReference) (string, error) {
	manifestURL := c.registryBaseURL(ref.Registry) + "/v2/" + ref.Repo + "/manifests/" + url.PathEscape(ref.Tag)

	token := ""
	resp, err := c.registryRequest(ctx, http.MethodHead, manifestURL, manifestAcceptTypes, ref, &token)
	if err != nil {
		return "", err
	}
	resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		if d := resp.Header.Get("Docker-Content-Digest"); d != "" {
			return d, nil
		}
		return c.digestFromBody(ctx, manifestURL, ref, &token)
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
		return "", errNotPublic
	default:
		return "", fmt.Errorf("registry returned status %d", resp.StatusCode)
	}
}

// digestFromBody computes the digest from the manifest body for registries that
// don't send the Docker-Content-Digest header.
func (c *ImageUpdateChecker) digestFromBody(ctx context.Context, manifestURL string, ref imageReference, token *string) (string, error) {
	resp, err := c.registryRequest(ctx, http.MethodGet, manifestURL, manifestAcceptTypes, ref, token)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("registry returned status %d", resp.StatusCode)
	}
	if d := resp.Header.Get("Docker-Content-Digest"); d != "" {
		return d, nil
	}
	h := sha256.New()
	if _, err := io.Copy(h, io.LimitReader(resp.Body, 4<<20)); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// registryManifest covers both image indexes (manifests) and image manifests (config).
type registryManifest struct {
	Manifests []struct {
		Digest   string        `json:"digest"`
		Platform imagePlatform `json:"platform"`
	} `json:"manifests"`
	Config struct {
		Digest string `json:"digest"`
	} `json:"config"`
}

// fetchRemoteMeta reads the labels of the registry image with the given digest:
// index → manifest for the local platform → config blob.
func (c *ImageUpdateChecker) fetchRemoteMeta(ctx context.Context, ref imageReference, digest string, platform imagePlatform) (imageMeta, error) {
	base := c.registryBaseURL(ref.Registry) + "/v2/" + ref.Repo
	token := ""

	var manifest registryManifest
	if err := c.registryGetJSON(ctx, base+"/manifests/"+digest, manifestAcceptTypes, ref, &token, &manifest); err != nil {
		return imageMeta{}, err
	}
	if len(manifest.Manifests) > 0 {
		platformDigest := selectPlatformManifest(manifest, platform)
		if platformDigest == "" {
			return imageMeta{}, fmt.Errorf("no manifest for platform %s/%s", platform.OS, platform.Architecture)
		}
		manifest = registryManifest{}
		if err := c.registryGetJSON(ctx, base+"/manifests/"+platformDigest, manifestAcceptTypes, ref, &token, &manifest); err != nil {
			return imageMeta{}, err
		}
	}
	if manifest.Config.Digest == "" {
		return imageMeta{}, errors.New("manifest has no config")
	}

	var config struct {
		Config struct {
			Labels map[string]string `json:"Labels"`
		} `json:"config"`
	}
	if err := c.registryGetJSON(ctx, base+"/blobs/"+manifest.Config.Digest, "*/*", ref, &token, &config); err != nil {
		return imageMeta{}, err
	}
	return metaFromLabels(config.Config.Labels), nil
}

// selectPlatformManifest picks the index entry matching the local image's platform.
// Attestation entries ("unknown/unknown") never match.
func selectPlatformManifest(index registryManifest, platform imagePlatform) string {
	var fallback string
	for _, m := range index.Manifests {
		if m.Platform.OS != platform.OS || m.Platform.Architecture != platform.Architecture {
			continue
		}
		if platform.Variant == "" || m.Platform.Variant == platform.Variant {
			return m.Digest
		}
		if fallback == "" {
			fallback = m.Digest
		}
	}
	return fallback
}

func (c *ImageUpdateChecker) registryGetJSON(ctx context.Context, u, accept string, ref imageReference, token *string, out any) error {
	resp, err := c.registryRequest(ctx, http.MethodGet, u, accept, ref, token)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("registry returned status %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(out)
}

// registryRequest sends a registry request, answering a Bearer challenge once with an
// anonymous token. The token is stored in *token for follow-up requests.
func (c *ImageUpdateChecker) registryRequest(ctx context.Context, method, u, accept string, ref imageReference, token *string) (*http.Response, error) {
	resp, err := c.doRegistryRequest(ctx, method, u, accept, *token)
	if err != nil || resp.StatusCode != http.StatusUnauthorized {
		return resp, err
	}
	challenge := resp.Header.Get("WWW-Authenticate")
	resp.Body.Close()
	if _, ok := parseBearerChallenge(challenge); !ok {
		return nil, errNotPublic
	}
	t, err := c.fetchToken(ctx, challenge, ref.Repo)
	if err != nil {
		return nil, fmt.Errorf("registry auth: %w", err)
	}
	*token = t
	return c.doRegistryRequest(ctx, method, u, accept, t)
}

func (c *ImageUpdateChecker) doRegistryRequest(ctx context.Context, method, u, accept, token string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return c.registryClient.Do(req)
}

// fetchToken obtains an anonymous pull token from the auth server named in a
// "WWW-Authenticate: Bearer realm=...,service=...,scope=..." challenge.
func (c *ImageUpdateChecker) fetchToken(ctx context.Context, challenge, repo string) (string, error) {
	params, ok := parseBearerChallenge(challenge)
	if !ok || params["realm"] == "" {
		return "", errors.New("registry requires authentication")
	}

	tokenURL, err := url.Parse(params["realm"])
	if err != nil {
		return "", fmt.Errorf("invalid auth realm: %w", err)
	}
	q := tokenURL.Query()
	if s := params["service"]; s != "" {
		q.Set("service", s)
	}
	scope := params["scope"]
	if scope == "" {
		scope = "repository:" + repo + ":pull"
	}
	q.Set("scope", scope)
	tokenURL.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL.String(), nil)
	if err != nil {
		return "", err
	}
	resp, err := c.registryClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint returned status %d", resp.StatusCode)
	}

	var body struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return "", err
	}
	if body.Token != "" {
		return body.Token, nil
	}
	if body.AccessToken != "" {
		return body.AccessToken, nil
	}
	return "", errors.New("token endpoint returned no token")
}

// parseBearerChallenge parses the key="value" pairs of a Bearer WWW-Authenticate header.
func parseBearerChallenge(header string) (map[string]string, bool) {
	const prefix = "bearer "
	if len(header) < len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return nil, false
	}
	params := make(map[string]string)
	rest := header[len(prefix):]
	for rest != "" {
		rest = strings.TrimLeft(rest, " ,")
		eq := strings.Index(rest, "=")
		if eq < 0 {
			break
		}
		key := strings.ToLower(strings.TrimSpace(rest[:eq]))
		rest = rest[eq+1:]
		var val string
		if strings.HasPrefix(rest, `"`) {
			end := strings.Index(rest[1:], `"`)
			if end < 0 {
				val, rest = rest[1:], ""
			} else {
				val, rest = rest[1:end+1], rest[end+2:]
			}
		} else if comma := strings.Index(rest, ","); comma >= 0 {
			val, rest = rest[:comma], rest[comma:]
		} else {
			val, rest = rest, ""
		}
		params[key] = val
	}
	return params, true
}

// daemonContainer is the subset of /containers/json used by the image checker.
type daemonContainer struct {
	ID      string   `json:"Id"`
	Names   []string `json:"Names"`
	Image   string   `json:"Image"`
	ImageID string   `json:"ImageID"`
	State   string   `json:"State"`
}

func listDaemonContainers(ctx context.Context, daemon *http.Client) ([]daemonContainer, error) {
	var containers []daemonContainer
	if err := daemonGetJSON(ctx, daemon, "/containers/json", &containers); err != nil {
		return nil, err
	}
	return containers, nil
}

// daemonImage is the subset of /images/{id}/json used by the image checker.
type daemonImage struct {
	RepoDigests  []string `json:"RepoDigests"`
	Os           string   `json:"Os"`
	Architecture string   `json:"Architecture"`
	Variant      string   `json:"Variant"`
	Config       struct {
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
}

func inspectDaemonImage(ctx context.Context, daemon *http.Client, imageID string) (daemonImage, error) {
	var image daemonImage
	err := daemonGetJSON(ctx, daemon, "/images/"+url.PathEscape(imageID)+"/json", &image)
	return image, err
}

// versionLabels are checked in order for a human-readable image version.
var versionLabels = []string{"org.opencontainers.image.version", "org.label-schema.version", "version"}

// metaFromLabels extracts version, revision and source from image labels.
// Moving versions like "latest" are ignored.
func metaFromLabels(labels map[string]string) imageMeta {
	var m imageMeta
	for _, key := range versionLabels {
		if v := strings.TrimSpace(labels[key]); v != "" && v != "latest" {
			m.Version = v
			break
		}
	}
	m.Revision = firstNonEmpty(strings.TrimSpace(labels["org.opencontainers.image.revision"]), strings.TrimSpace(labels["org.label-schema.vcs-ref"]))
	m.Source = firstNonEmpty(strings.TrimSpace(labels["org.opencontainers.image.source"]), strings.TrimSpace(labels["org.label-schema.vcs-url"]))
	return m
}

func containerConfigImage(ctx context.Context, daemon *http.Client, containerID string) (string, error) {
	var ct struct {
		Config struct {
			Image string `json:"Image"`
		} `json:"Config"`
	}
	if err := daemonGetJSON(ctx, daemon, "/containers/"+url.PathEscape(containerID)+"/json", &ct); err != nil {
		return "", err
	}
	return ct.Config.Image, nil
}

func daemonGetJSON(ctx context.Context, daemon *http.Client, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost/"+dockerAPIVersion+path, nil)
	if err != nil {
		return err
	}
	resp, err := daemon.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d for %s", resp.StatusCode, path)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out)
}
