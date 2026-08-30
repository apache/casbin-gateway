// Copyright 2026 The casbin Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package version

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/apache/casbin-gateway/proxy"
)

const (
	// Updates come from the same place scripts/install.sh takes its binaries
	// from: the rolling "nightly" pre-release. Official releases are source
	// packages and carry no executable to install.
	repository  = "apache/casbin-gateway"
	releaseTag  = "nightly"
	assetPrefix = "casbin-gateway-nightly"

	// GitHub allows 60 unauthenticated calls an hour per address, and the web
	// UI asks on every page load, so the answer is kept for a while.
	checkInterval = 10 * time.Minute
	checkTimeout  = 20 * time.Second
)

// Release is the published build this Gateway can move to.
type Release struct {
	Tag         string `json:"tag"`
	Name        string `json:"name"`
	Commit      string `json:"commit"`
	ShortCommit string `json:"shortCommit"`
	PublishedAt string `json:"publishedAt"`
	PageUrl     string `json:"pageUrl"`
	AssetName   string `json:"assetName"`
	AssetUrl    string `json:"assetUrl"`
	AssetSize   int64  `json:"assetSize"`
}

func (r *Release) publishedTime() time.Time {
	moment, err := time.Parse(time.RFC3339, r.PublishedAt)
	if err != nil {
		return time.Time{}
	}

	return moment
}

type githubRelease struct {
	TagName         string `json:"tag_name"`
	Name            string `json:"name"`
	Body            string `json:"body"`
	TargetCommitish string `json:"target_commitish"`
	PublishedAt     string `json:"published_at"`
	HtmlUrl         string `json:"html_url"`
	Assets          []struct {
		Name string `json:"name"`
		Url  string `json:"browser_download_url"`
		Size int64  `json:"size"`
	} `json:"assets"`
}

var (
	cacheLock   sync.Mutex
	cached      *Release
	cachedErr   error
	cachedAt    time.Time
	httpClient  = &http.Client{Timeout: checkTimeout, Transport: proxy.Transport()}
	reCommitSha = regexp.MustCompile(`\b[0-9a-f]{40}\b`)
)

// LatestRelease returns the published build, from the cache unless force says
// to ask GitHub again. A failed lookup is cached too, so a machine with no
// internet does not spend twenty seconds on every page load.
func LatestRelease(force bool) (*Release, error) {
	cacheLock.Lock()
	defer cacheLock.Unlock()

	if !force && !cachedAt.IsZero() && time.Since(cachedAt) < checkInterval {
		return cached, cachedErr
	}

	cached, cachedErr = fetchLatestRelease()
	cachedAt = time.Now()
	return cached, cachedErr
}

func fetchLatestRelease() (*Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s", repository, releaseTag)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	// GitHub rejects a request without one.
	req.Header.Set("User-Agent", "casbin-gateway/"+Current().Version)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub answered HTTP %d for %s", resp.StatusCode, url)
	}

	var payload githubRelease
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("GitHub answered something that is not a release: %w", err)
	}

	release := &Release{
		Tag:         payload.TagName,
		Name:        payload.Name,
		Commit:      releaseCommit(&payload),
		PublishedAt: payload.PublishedAt,
		PageUrl:     payload.HtmlUrl,
	}
	release.ShortCommit = ShortCommit(release.Commit)

	wanted := AssetName(runtime.GOOS, runtime.GOARCH)
	for _, asset := range payload.Assets {
		if asset.Name != wanted {
			continue
		}
		release.AssetName = asset.Name
		release.AssetUrl = asset.Url
		release.AssetSize = asset.Size
		break
	}

	return release, nil
}

// IsNetworkError reports whether err is this machine failing to reach GitHub
// rather than the update itself going wrong. Where GitHub is unreachable a
// proxy is what fixes it, so the web UI says so instead of only quoting the
// error.
func IsNetworkError(err error) bool {
	if err == nil {
		return false
	}

	var netErr net.Error
	var urlErr *url.Error
	if errors.As(err, &netErr) || errors.As(err, &urlErr) {
		return true
	}

	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, io.ErrUnexpectedEOF)
}

// releaseCommit finds the commit the nightly was built from. The release is
// created with --target <sha>, so target_commitish normally holds it; the notes
// repeat it, which covers the case where GitHub hands back a branch name.
func releaseCommit(payload *githubRelease) string {
	if reCommitSha.MatchString(payload.TargetCommitish) {
		return payload.TargetCommitish
	}

	return reCommitSha.FindString(payload.Body)
}

// AssetName is the archive published for a platform, empty where none is.
// Only x86_64 archives are built; macOS runs them under Rosetta 2, so an Apple
// Silicon machine is served by the same file.
func AssetName(goos string, goarch string) string {
	switch goos {
	case "linux":
		if goarch != "amd64" {
			return ""
		}
		return assetPrefix + "-linux-x86_64.tar.gz"
	case "darwin":
		if goarch != "amd64" && goarch != "arm64" {
			return ""
		}
		return assetPrefix + "-darwin-x86_64.tar.gz"
	case "windows":
		if goarch != "amd64" {
			return ""
		}
		return assetPrefix + "-windows-x86_64.zip"
	default:
		return ""
	}
}

// IsNewer reports whether the release is a build this one is not already on.
// Commits settle it when both are known; otherwise the dates do, which also
// keeps a locally built Gateway that is ahead of master from being told to
// "update" backwards onto the published nightly.
func IsNewer(release *Release, build Build) bool {
	if release == nil || release.AssetUrl == "" {
		return false
	}

	// A build carrying the released commit is that release, uncommitted local
	// changes included: replacing it would throw those away.
	if release.Commit != "" && build.Commit != "" && strings.EqualFold(release.Commit, build.Commit) {
		return false
	}

	return !buildIsAhead(release, build)
}

func buildIsAhead(release *Release, build Build) bool {
	builtAt, publishedAt := build.Time(), release.publishedTime()
	if builtAt.IsZero() || publishedAt.IsZero() {
		return false
	}

	return builtAt.After(publishedAt)
}
