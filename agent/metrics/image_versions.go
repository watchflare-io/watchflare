package metrics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	dockerHubAPIBaseURL = "https://hub.docker.com"
	githubAPIBaseURL    = "https://api.github.com"
	// maxTagPages bounds how far back tag lists are searched (Docker Hub: 100 tags
	// per page sorted by last update; other registries: up to 1000 names per page).
	maxTagPages = 5
	// maxTagHeads bounds HEAD requests per lookup on registries without digests in
	// their tag list. Candidates are tried newest version first.
	maxTagHeads = 40
)

// versionInfo is what the UI shows for a container's image versions.
type versionInfo struct {
	Local         string
	Remote        string
	CommitsBehind int
}

// tagInfo is what the registry's tags reveal about one digest.
type tagInfo struct {
	Version  string // most specific version tag, family suffix stripped ("2.30.1")
	Revision string // commit from a "sha-<hex>" tag
	score    []int  // version components of Version, for picking the best tag
}

var (
	numericTagRe = regexp.MustCompile(`^v?(\d+(?:\.\d+)*)(-.+)?$`)
	commitTagRe  = regexp.MustCompile(`^sha-([0-9a-f]{7,40})$`)
	// rollingTags name moving builds; "latest-pg16" belongs to the "-pg16" family.
	rollingTags = map[string]bool{"latest": true, "stable": true, "release": true, "main": true, "master": true, "edge": true, "nightly": true}
)

// tagFamily describes which version tags can stand for a given tag:
// "latest" → "1.2.3", "latest-pg16" → "1.2.3-pg16", "pg18" → "1.2.3-pg18", "1.2" → "1.2.x".
type tagFamily struct {
	prefix string // numeric prefix the version must start with ("" = any)
	suffix string // suffix every version tag of the family carries ("-pg16")
	re     *regexp.Regexp
}

func parseTagFamily(tag string) tagFamily {
	var f tagFamily
	switch m := numericTagRe.FindStringSubmatch(tag); {
	case m != nil:
		f.prefix, f.suffix = m[1], m[2]
	case rollingTags[tag]:
	case strings.Contains(tag, "-") && rollingTags[tag[:strings.Index(tag, "-")]]:
		f.suffix = tag[strings.Index(tag, "-"):]
	default:
		f.suffix = "-" + tag
	}
	f.re = regexp.MustCompile(`^(v?)(\d+(?:\.\d+)*)` + regexp.QuoteMeta(f.suffix) + `$`)
	return f
}

// match returns the displayed version of a tag in this family (suffix stripped)
// and its numeric components.
func (f tagFamily) match(tag string) (string, []int, bool) {
	m := f.re.FindStringSubmatch(tag)
	if m == nil {
		return "", nil, false
	}
	numeric := m[2]
	if f.prefix != "" && numeric != f.prefix && !strings.HasPrefix(numeric, f.prefix+".") {
		return "", nil, false
	}
	parts := strings.Split(numeric, ".")
	score := make([]int, len(parts))
	for i, p := range parts {
		score[i], _ = strconv.Atoi(p)
	}
	return m[1] + numeric, score, true
}

// betterVersion reports whether a should be shown instead of b: more specific
// ("2.16.0" over "2") first, then higher.
func betterVersion(a, b []int) bool {
	if len(a) != len(b) {
		return len(a) > len(b)
	}
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

func (t *tagInfo) offer(tag string, f tagFamily) {
	if version, score, ok := f.match(tag); ok && (t.Version == "" || betterVersion(score, t.score)) {
		t.Version, t.score = version, score
	}
	if m := commitTagRe.FindStringSubmatch(tag); m != nil && t.Revision == "" {
		t.Revision = m[1]
	}
}

// findVersionTags looks up which tags of ref's repository point at the given digests.
func (c *ImageUpdateChecker) findVersionTags(ctx context.Context, ref imageReference, digests []string) (map[string]*tagInfo, error) {
	found := make(map[string]*tagInfo, len(digests))
	for _, d := range digests {
		found[d] = &tagInfo{}
	}
	family := parseTagFamily(ref.Tag)
	if ref.Registry == dockerHubRegistry {
		return found, c.scanDockerHubTags(ctx, ref, family, found)
	}
	return found, c.scanRegistryTags(ctx, ref, family, found)
}

func allVersioned(found map[string]*tagInfo) bool {
	for _, t := range found {
		if t.Version == "" {
			return false
		}
	}
	return true
}

// scanDockerHubTags uses the Docker Hub API, which lists tags with their digests and
// doesn't count against pull limits.
func (c *ImageUpdateChecker) scanDockerHubTags(ctx context.Context, ref imageReference, family tagFamily, found map[string]*tagInfo) error {
	q := url.Values{"page_size": {"100"}, "ordering": {"last_updated"}}
	if family.suffix != "" {
		q.Set("name", strings.TrimPrefix(family.suffix, "-"))
	}
	next := c.dockerHubBaseURL + "/v2/repositories/" + ref.Repo + "/tags?" + q.Encode()

	for page := 0; next != "" && page < maxTagPages; page++ {
		var body struct {
			Next    string `json:"next"`
			Results []struct {
				Name   string `json:"name"`
				Digest string `json:"digest"`
			} `json:"results"`
		}
		if err := c.getJSON(ctx, next, nil, &body); err != nil {
			return err
		}
		for _, t := range body.Results {
			if info, ok := found[t.Digest]; ok {
				info.offer(t.Name, family)
			}
		}
		if allVersioned(found) {
			return nil
		}
		next = body.Next
	}
	return nil
}

// scanRegistryTags uses the standard registry API: list tag names, then resolve the
// newest family candidates with (free) HEAD requests until every digest is matched.
func (c *ImageUpdateChecker) scanRegistryTags(ctx context.Context, ref imageReference, family tagFamily, found map[string]*tagInfo) error {
	base := c.registryBaseURL(ref.Registry)
	next := base + "/v2/" + ref.Repo + "/tags/list?n=1000"
	token := ""

	type candidate struct {
		tag   string
		score []int
	}
	var candidates []candidate
	for page := 0; next != "" && page < maxTagPages; page++ {
		resp, err := c.registryRequest(ctx, http.MethodGet, next, "application/json", ref, &token)
		if err != nil {
			return err
		}
		var body struct {
			Tags []string `json:"tags"`
		}
		err = json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&body)
		link := resp.Header.Get("Link")
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("tag list returned status %d", resp.StatusCode)
		}
		if err != nil {
			return err
		}
		for _, tag := range body.Tags {
			if _, score, ok := family.match(tag); ok {
				candidates = append(candidates, candidate{tag, score})
			}
		}
		next = nextLink(base, link)
	}

	sort.Slice(candidates, func(i, j int) bool { return betterVersion(candidates[i].score, candidates[j].score) })
	for i, cand := range candidates {
		if i >= maxTagHeads || allVersioned(found) {
			break
		}
		resp, err := c.registryRequest(ctx, http.MethodHead, base+"/v2/"+ref.Repo+"/manifests/"+url.PathEscape(cand.tag), manifestAcceptTypes, ref, &token)
		if err != nil {
			return err
		}
		resp.Body.Close()
		if info, ok := found[resp.Header.Get("Docker-Content-Digest")]; ok {
			info.offer(cand.tag, family)
		}
	}
	return nil
}

// nextLink extracts the next page URL from a registry Link header
// (`</v2/repo/tags/list?last=x&n=1000>; rel="next"`).
func nextLink(base, header string) string {
	start, end := strings.Index(header, "<"), strings.Index(header, ">")
	if start < 0 || end < start || !strings.Contains(header, `rel="next"`) {
		return ""
	}
	link := header[start+1 : end]
	if strings.HasPrefix(link, "/") {
		return base + link
	}
	return link
}

// githubRepoRe extracts owner/repo from an image source label.
var githubRepoRe = regexp.MustCompile(`^(?:https?://)?github\.com/([^/]+)/([^/#?]+?)(?:\.git)?/?$`)

// githubVersions describes rolling builds (no version tags) relative to the latest
// GitHub release ("2025-08-05+277") and counts the commits between the running and
// the registry image.
func (c *ImageUpdateChecker) githubVersions(ctx context.Context, source, localRev, remoteRev string) (versionInfo, error) {
	m := githubRepoRe.FindStringSubmatch(source)
	if m == nil {
		return versionInfo{}, errors.New("source is not a GitHub repository")
	}
	repo := m[1] + "/" + m[2]

	var info versionInfo
	behind, _, err := c.githubCompare(ctx, repo, localRev, remoteRev)
	if err != nil {
		return info, err
	}
	info.CommitsBehind = behind

	release, err := c.githubLatestRelease(ctx, repo)
	if err != nil || release == "" {
		return info, err
	}
	if ahead, behindRelease, err := c.githubCompare(ctx, repo, release, localRev); err == nil && behindRelease == 0 {
		info.Local = fmt.Sprintf("%s+%d", release, ahead)
	}
	if ahead, behindRelease, err := c.githubCompare(ctx, repo, release, remoteRev); err == nil && behindRelease == 0 {
		info.Remote = fmt.Sprintf("%s+%d", release, ahead)
	}
	return info, nil
}

// githubCompare returns how many commits head is ahead of and behind base (cached:
// commits never change).
func (c *ImageUpdateChecker) githubCompare(ctx context.Context, repo, base, head string) (ahead, behind int, err error) {
	key := repo + ":" + base + "..." + head
	if r, ok := c.githubCache[key]; ok {
		return r[0], r[1], nil
	}
	var body struct {
		AheadBy  int `json:"ahead_by"`
		BehindBy int `json:"behind_by"`
	}
	u := c.githubBaseURL + "/repos/" + repo + "/compare/" + url.PathEscape(base) + "..." + url.PathEscape(head)
	if err := c.getJSON(ctx, u, githubHeaders, &body); err != nil {
		return 0, 0, err
	}
	c.githubCache[key] = [2]int{body.AheadBy, body.BehindBy}
	return body.AheadBy, body.BehindBy, nil
}

// githubLatestRelease returns the tag of the latest release ("" if there is none).
// Releases change, so this is cached per check only.
func (c *ImageUpdateChecker) githubLatestRelease(ctx context.Context, repo string) (string, error) {
	if tag, ok := c.githubReleases[repo]; ok {
		return tag, nil
	}
	var body struct {
		TagName string `json:"tag_name"`
	}
	err := c.getJSON(ctx, c.githubBaseURL+"/repos/"+repo+"/releases/latest", githubHeaders, &body)
	var statusErr httpStatusError
	if errors.As(err, &statusErr) && statusErr == http.StatusNotFound {
		err = nil
	}
	if err != nil {
		return "", err
	}
	c.githubReleases[repo] = body.TagName
	return body.TagName, nil
}

var githubHeaders = map[string]string{
	"Accept":     "application/vnd.github+json",
	"User-Agent": "watchflare-agent",
}

type httpStatusError int

func (e httpStatusError) Error() string { return fmt.Sprintf("HTTP status %d", int(e)) }

// getJSON fetches a public JSON API (Docker Hub, GitHub).
func (c *ImageUpdateChecker) getJSON(ctx context.Context, u string, headers map[string]string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.registryClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return httpStatusError(resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(out)
}

// resolveVersions determines the versions shown for a checked container. Results
// are cached per digest, so registries and GitHub are only asked about new images.
func (c *ImageUpdateChecker) resolveVersions(ctx context.Context, res imageResult, seen map[string]bool) versionInfo {
	repoKey := res.Ref.Registry + "/" + res.Ref.Repo + "@"
	digests := []string{res.LocalDigest}
	if res.Status == ImageStatusUpdateAvailable {
		digests = append(digests, res.RemoteDigest)
	}

	var missing []string
	for _, d := range digests {
		seen[repoKey+d] = true
		if _, ok := c.tagCache[repoKey+d]; !ok {
			missing = append(missing, d)
		}
	}
	if len(missing) > 0 {
		found, err := c.findVersionTags(ctx, res.Ref, missing)
		if err != nil {
			slog.Debug("version tag lookup failed", "image", res.Ref.Repo, "error", err)
		} else {
			for d, t := range found {
				c.tagCache[repoKey+d] = *t
			}
		}
	}

	local := c.tagCache[repoKey+res.LocalDigest]
	info := versionInfo{Local: local.Version}
	if info.Local == "" {
		info.Local = res.Local.Version
	}
	if res.Status != ImageStatusUpdateAvailable {
		return info
	}

	remote := c.tagCache[repoKey+res.RemoteDigest]
	info.Remote = remote.Version
	if info.Remote != "" && info.Local != "" {
		return info
	}

	// Without version tags, fall back to the registry image's labels, then to the
	// commit distance on GitHub for rolling builds.
	var remoteMeta imageMeta
	if info.Remote == "" || remote.Revision == "" {
		remoteMeta = c.remoteImageMeta(ctx, repoKey+res.RemoteDigest, res)
		seen[repoKey+res.RemoteDigest] = true
		if info.Remote == "" {
			info.Remote = remoteMeta.Version
		}
	}
	if info.Remote != "" && info.Local != "" {
		return info
	}

	localRev, remoteRev := firstNonEmpty(local.Revision, res.Local.Revision), firstNonEmpty(remote.Revision, remoteMeta.Revision)
	source := firstNonEmpty(res.Local.Source, remoteMeta.Source)
	if localRev == "" || remoteRev == "" || source == "" {
		return info
	}
	gh, err := c.githubVersions(ctx, source, localRev, remoteRev)
	if err != nil {
		slog.Debug("github version lookup failed", "source", source, "error", err)
	}
	if info.Local == "" {
		info.Local = gh.Local
	}
	if info.Remote == "" {
		info.Remote = gh.Remote
	}
	info.CommitsBehind = gh.CommitsBehind
	return info
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
