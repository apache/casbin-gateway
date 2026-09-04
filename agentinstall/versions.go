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

package agentinstall

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/apache/casbin-gateway/agent"
	"github.com/apache/casbin-gateway/proxy"
)

const (
	// catalogTTL is how long a registry answer is trusted. Every card on the
	// agents page asks for one, and the registries publish a few times a day.
	catalogTTL     = 30 * time.Minute
	catalogTimeout = 20 * time.Second
	// maxVersions bounds the list handed to the page: npm keeps every release
	// an agent ever published, and a picker cannot use a thousand of them.
	maxVersions = 60
)

// Catalog is what one agent's package manager publishes: the release it calls
// current, and the ones a downgrade can go back to.
type Catalog struct {
	AgentId string `json:"agentId"`
	Manager string `json:"manager,omitempty"`
	Package string `json:"package,omitempty"`
	Latest  string `json:"latest,omitempty"`
	// Versions are the published releases, newest first. Empty for a manager
	// that only ever names the current one.
	Versions []string `json:"versions"`
	// CommandTemplate is the command a version change would run, with the
	// version left as "{version}". Empty for a manager that installs only the
	// release its own index names, which is every Homebrew cask.
	CommandTemplate string `json:"commandTemplate,omitempty"`
	// Detail says why there is nothing to list, when there is nothing.
	Detail    string `json:"detail,omitempty"`
	CheckedAt string `json:"checkedAt,omitempty"`
}

// versionPlaceholder stands in for the release in a command template.
const versionPlaceholder = "{version}"

// Update is one installation measured against what its manager publishes.
type Update struct {
	AgentId string `json:"agentId"`
	Path    string `json:"path"`
	Owner   string `json:"owner"`
	Manager string `json:"manager,omitempty"`
	Current string `json:"current,omitempty"`
	Latest  string `json:"latest,omitempty"`
	// Available is a newer release than the one on disk, which is the whole
	// point of asking. False while either version is unknown: an installation
	// whose version could not be read is not called out of date on a guess.
	Available bool   `json:"available"`
	Detail    string `json:"detail,omitempty"`
	CheckedAt string `json:"checkedAt,omitempty"`
}

var catalogs = struct {
	sync.Mutex
	byKey map[string]catalogEntry
	// pending keeps one lookup per package: a page listing a dozen agents asks
	// for each of them at once, and they would otherwise all leave the host.
	pending map[string]*sync.WaitGroup
}{byKey: map[string]catalogEntry{}, pending: map[string]*sync.WaitGroup{}}

type catalogEntry struct {
	catalog Catalog
	// full marks an answer that carries the version list, not only the release
	// the manager calls current.
	full bool
	at   time.Time
}

var catalogClient = &http.Client{Timeout: catalogTimeout, Transport: proxy.Transport()}

// VersionsOf lists what the manager that owns an installation publishes for it.
// installMethod picks the manager, since an agent published on two of them is
// upgraded through the one that put it on this host; an empty method means the
// agent is not installed, and the manager an install would use answers instead.
func VersionsOf(agentId string, installMethod string, force bool) Catalog {
	return lookupCatalog(agentId, installMethod, true, force)
}

// LatestOf is only the release the manager calls current, which is one small
// request where the version list is a document of many megabytes. It is what a
// listing of installations needs, and the picker asks for the rest.
func LatestOf(agentId string, installMethod string, force bool) Catalog {
	return lookupCatalog(agentId, installMethod, false, force)
}

func lookupCatalog(agentId string, installMethod string, full bool, force bool) Catalog {
	manager, packageId := managerPackage(agentId, installMethod)
	result := Catalog{AgentId: agentId, Manager: manager, Package: packageId, Versions: []string{}}
	if manager == "" {
		result.Detail = catalogUnavailableDetail(agentId, installMethod)
		return result
	}

	key := manager + "|" + packageId
	for {
		catalogs.Lock()
		cached, ok := catalogs.byKey[key]
		// A cached list answers a question about the current release too; the
		// other way round it does not.
		if ok && !force && time.Since(cached.at) < catalogTTL && (cached.full || !full) {
			catalogs.Unlock()
			return withAgent(cached.catalog, agentId, installMethod)
		}
		waiting, running := catalogs.pending[key]
		if !running {
			break
		}
		catalogs.Unlock()
		// Another caller is already asking; take its answer rather than a
		// second round trip, and stop forcing so the wait cannot repeat.
		waiting.Wait()
		force = false
	}

	group := &sync.WaitGroup{}
	group.Add(1)
	catalogs.pending[key] = group
	catalogs.Unlock()

	fetched := fetchCatalog(manager, packageId, full)
	fetched.Manager, fetched.Package = manager, packageId

	catalogs.Lock()
	catalogs.byKey[key] = catalogEntry{catalog: fetched, full: full, at: time.Now()}
	delete(catalogs.pending, key)
	catalogs.Unlock()
	group.Done()

	return withAgent(fetched, agentId, installMethod)
}

// UpdatesFor measures every installation against its manager, looking each
// package up once however many installations share it.
func UpdatesFor(installations []agent.Installation, force bool) []Update {
	warmCatalogs(installations, force)

	result := make([]Update, 0, len(installations))
	for _, installation := range installations {
		// The lookups are already done and cached, so this asks for none.
		catalog := LatestOf(installation.AgentId, installation.InstallMethod, false)
		update := Update{
			AgentId:   installation.AgentId,
			Path:      installation.Path,
			Owner:     installation.Owner,
			Manager:   catalog.Manager,
			Current:   installation.Version,
			Latest:    catalog.Latest,
			Detail:    catalog.Detail,
			CheckedAt: catalog.CheckedAt,
		}
		update.Available = installation.Version != "" && catalog.Latest != "" &&
			CompareVersions(catalog.Latest, installation.Version) > 0
		result = append(result, update)
	}
	return result
}

// warmCatalogs fetches what a listing needs, several at a time: a host with a
// dozen agents would otherwise hold the request open for a dozen round trips.
func warmCatalogs(installations []agent.Installation, force bool) {
	const parallel = 6

	slots := make(chan struct{}, parallel)
	group := sync.WaitGroup{}
	asked := map[string]bool{}
	for _, installation := range installations {
		key := installation.AgentId + "|" + installation.InstallMethod
		if asked[key] {
			continue
		}
		asked[key] = true

		group.Add(1)
		go func(agentId string, installMethod string) {
			defer group.Done()

			slots <- struct{}{}
			defer func() { <-slots }()
			LatestOf(agentId, installMethod, force)
		}(installation.AgentId, installation.InstallMethod)
	}
	group.Wait()
}

// UpdatesForMissing reports the release an install would land on for every
// agent this host has none of, so one listing can cover all of them. The rows
// carry no path or owner, which is what marks them as installations that are
// not there.
func UpdatesForMissing(installed []agent.Installation, force bool) []Update {
	here := map[string]bool{}
	for _, installation := range installed {
		here[installation.AgentId] = true
	}

	missing := []agent.Installation{}
	for _, known := range agent.KnownAgents() {
		if !here[known.AgentId] {
			missing = append(missing, agent.Installation{AgentId: known.AgentId})
		}
	}
	return UpdatesFor(missing, force)
}

func withAgent(catalog Catalog, agentId string, installMethod string) Catalog {
	catalog.AgentId = agentId
	if catalog.Versions == nil {
		catalog.Versions = []string{}
	}
	// The template is built here rather than cached: it names the program the
	// PATH search resolved, which the cache outlives.
	plan := managerPlan(agentId, installMethod, ActionDowngrade, versionPlaceholder)
	if installMethod == "" {
		// Nothing installed, so there is no manager that owns a tree: the one
		// an install would reach for answers instead.
		plan = installPlan(agentId, versionPlaceholder)
	}
	if plan.Available {
		catalog.CommandTemplate = plan.Command
	}
	return catalog
}

// managerPackage is the manager that owns an installation and the id it knows
// the agent by. An installMethod no manager matches leaves both empty, and so
// does one whose manager does not publish this agent.
func managerPackage(agentId string, installMethod string) (string, string) {
	packages := agent.PackagesOf(agentId)
	if installMethod == "" {
		// Not installed: the manager an install would reach for is the one
		// whose releases are worth listing.
		for _, candidate := range installOrder() {
			if manager, packageId := managerPackage(agentId, candidate); manager != "" {
				return manager, packageId
			}
		}
		return "", ""
	}

	switch installMethod {
	case ManagerNpm:
		return withPackage(ManagerNpm, packages.Npm)
	case ManagerHomebrew:
		return withPackage(ManagerHomebrew, packages.HomebrewCask)
	case ManagerWinget:
		if runtime.GOOS != "windows" {
			return "", ""
		}
		return withPackage(ManagerWinget, packages.Winget)
	}
	return "", ""
}

func withPackage(manager string, packageId string) (string, string) {
	if packageId == "" {
		return "", ""
	}
	return manager, packageId
}

func catalogUnavailableDetail(agentId string, installMethod string) string {
	if installMethod != "" && installMethod != ManagerNpm &&
		installMethod != ManagerHomebrew && installMethod != ManagerWinget {
		return "this agent was installed as \"" + installMethod + "\", which publishes no version list Gateway can read"
	}
	if agent.PackagesOf(agentId).Desktop {
		return "this is a desktop app; its versions come from the vendor's own downloads"
	}
	return "no package manager on this platform publishes this agent"
}

func fetchCatalog(manager string, packageId string, full bool) Catalog {
	var catalog Catalog
	var err error
	switch manager {
	case ManagerNpm:
		if full {
			catalog, err = npmCatalog(packageId)
		} else {
			catalog, err = npmLatest(packageId)
		}
	case ManagerHomebrew:
		catalog, err = homebrewCatalog(packageId)
	case ManagerWinget:
		catalog, err = wingetCatalog(packageId)
	}
	if err != nil {
		catalog.Detail = err.Error()
	}
	if catalog.Versions == nil {
		catalog.Versions = []string{}
	}
	catalog.CheckedAt = time.Now().Format(time.RFC3339)
	return catalog
}

// npmLatest reads the tags alone, which is a few hundred bytes where the
// package document of a busy agent is fifteen megabytes.
func npmLatest(packageId string) (Catalog, error) {
	var payload map[string]string
	if err := npmGet("/-/package/"+url.PathEscape(packageId)+"/dist-tags", "", &payload); err != nil {
		return Catalog{}, err
	}
	return Catalog{Latest: payload["latest"]}, nil
}

// npmCatalog reads the registry rather than running npm: the abbreviated
// document carries the tags and every published version, and asking for it
// costs one request instead of starting a package manager.
func npmCatalog(packageId string) (Catalog, error) {
	var payload struct {
		DistTags map[string]string          `json:"dist-tags"`
		Versions map[string]json.RawMessage `json:"versions"`
	}
	if err := npmGet("/"+url.PathEscape(packageId), "application/vnd.npm.install-v1+json", &payload); err != nil {
		return Catalog{}, err
	}

	published := make([]string, 0, len(payload.Versions))
	for version := range payload.Versions {
		published = append(published, version)
	}
	return Catalog{Latest: payload.DistTags["latest"], Versions: newestFirst(published)}, nil
}

func npmGet(path string, accept string, into any) error {
	req, err := http.NewRequest("GET", "https://registry.npmjs.org"+path, nil)
	if err != nil {
		return err
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	req.Header.Set("User-Agent", "casbin-gateway")

	resp, err := catalogClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// The abbreviated document of an agent that publishes several times a day
	// runs to tens of megabytes, so the cap is generous rather than tight.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("the npm registry answered HTTP %d for %s", resp.StatusCode, path)
	}
	if err := json.Unmarshal(body, into); err != nil {
		return fmt.Errorf("the npm registry answered something that is not a package: %w", err)
	}
	return nil
}

// homebrewCatalog reads the cask, which names the one version it installs.
func homebrewCatalog(cask string) (Catalog, error) {
	address := "https://formulae.brew.sh/api/cask/" + url.PathEscape(cask) + ".json"
	req, err := http.NewRequest("GET", address, nil)
	if err != nil {
		return Catalog{}, err
	}
	req.Header.Set("User-Agent", "casbin-gateway")

	resp, err := catalogClient.Do(req)
	if err != nil {
		return Catalog{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Catalog{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return Catalog{}, fmt.Errorf("Homebrew answered HTTP %d for the %s cask", resp.StatusCode, cask)
	}

	var payload struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return Catalog{}, fmt.Errorf("Homebrew answered something that is not a cask: %w", err)
	}

	latest := strings.TrimSpace(payload.Version)
	catalog := Catalog{Latest: latest}
	if latest != "" {
		catalog.Versions = []string{latest}
	}
	catalog.Detail = "Homebrew publishes only the current version of a cask"
	return catalog, nil
}

// wingetCatalog asks winget itself: the community repository has no API a
// client is meant to read, and the program that installed the agent is here.
func wingetCatalog(packageId string) (Catalog, error) {
	program := lookup("winget")
	if program == "" {
		return Catalog{}, fmt.Errorf("winget is not on Gateway's PATH")
	}

	ctx, cancel := context.WithTimeout(context.Background(), catalogTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, program, "show", "--id", packageId, "--exact", "--versions",
		"--disable-interactivity", "--accept-source-agreements")
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return Catalog{}, fmt.Errorf("winget could not list the versions of %s", packageId)
	}

	published := []string{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		// The table is preceded by a heading and a rule of dashes, and every
		// row after them is one version.
		if line == "" || strings.HasPrefix(line, "-") || !IsValidVersion(line) {
			continue
		}
		published = append(published, line)
	}
	if len(published) == 0 {
		return Catalog{}, fmt.Errorf("winget listed no versions for %s", packageId)
	}

	sorted := newestFirst(published)
	return Catalog{Latest: sorted[0], Versions: sorted}, nil
}

// newestFirst sorts releases and keeps the newest maxVersions of them. The
// prereleases go: an agent that publishes a nightly every day would fill the
// picker with builds nobody is choosing to run, and the versions worth going
// back to are the ones the vendor released. An agent that has only ever
// published prereleases keeps them, since otherwise it lists nothing.
func newestFirst(versions []string) []string {
	stable := make([]string, 0, len(versions))
	for _, version := range versions {
		if _, prerelease := splitPrerelease(version); prerelease == "" {
			stable = append(stable, version)
		}
	}
	if len(stable) > 0 {
		versions = stable
	}

	sort.SliceStable(versions, func(left, right int) bool {
		return CompareVersions(versions[left], versions[right]) > 0
	})
	if len(versions) > maxVersions {
		versions = versions[:maxVersions]
	}
	return versions
}

// CompareVersions orders two releases the way their publishers do: numbers
// compare as numbers, a missing part counts as zero, and a prerelease sorts
// below the release it leads to. An unreadable version sorts below a readable
// one, which keeps a build string out of the "newer" answer.
func CompareVersions(left string, right string) int {
	leftCore, leftPre := splitPrerelease(left)
	rightCore, rightPre := splitPrerelease(right)

	if order := compareParts(leftCore, rightCore); order != 0 {
		return order
	}
	switch {
	case leftPre == rightPre:
		return 0
	case leftPre == "":
		return 1
	case rightPre == "":
		return -1
	}
	return strings.Compare(leftPre, rightPre)
}

func splitPrerelease(version string) ([]string, string) {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "v")
	if plus := strings.IndexByte(version, '+'); plus >= 0 {
		version = version[:plus]
	}
	core, prerelease, _ := strings.Cut(version, "-")
	return strings.Split(core, "."), prerelease
}

func compareParts(left []string, right []string) int {
	for i := 0; i < len(left) || i < len(right); i++ {
		leftPart, rightPart := numberAt(left, i), numberAt(right, i)
		if leftPart != rightPart {
			if leftPart < rightPart {
				return -1
			}
			return 1
		}
	}
	return 0
}

func numberAt(parts []string, index int) int {
	if index >= len(parts) {
		return 0
	}
	number := 0
	for i := 0; i < len(parts[index]); i++ {
		digit := parts[index][i]
		if digit < '0' || digit > '9' {
			return number
		}
		number = number*10 + int(digit-'0')
	}
	return number
}
