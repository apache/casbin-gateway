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

package agentconfig

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// sourceFile lists the places skills are installed from, beside the trash
	// and the origin records under the same home.
	sourceFile = "skill-sources.json"
	// storeFolder holds one folder per source: the repository or archive as it
	// was last downloaded, which is what an install copies or links from.
	storeFolder = "skill-store"
	// storeTree is the downloaded content and storeFetch the record of when it
	// was taken, kept beside it so wiping the tree cannot lose the record.
	storeTree  = "tree"
	storeFetch = "fetch.json"
)

// Where a source's skills come from.
const (
	// SourceGithub is a repository, given as owner/repo or as a github.com URL.
	SourceGithub = "github"
	// SourceArchive is a .zip or .tar.gz at a URL.
	SourceArchive = "archive"
	// SourceUpload is an archive uploaded through the browser, which lives in
	// the store and has nowhere to be fetched from again.
	SourceUpload = "upload"
	// SourceLocal is a folder on this machine, read where it is.
	SourceLocal = "local"
)

// SkillSource is one place skills can be installed from. The list is the
// operator's: the built-in entries are seeded once and can be removed like any
// other.
type SkillSource struct {
	Id   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`

	// Url is owner/repo for a repository, the download URL for an archive, or
	// the folder for a local source. An upload has none: it is already here.
	Url string `json:"url,omitempty"`
	// Ref is the branch, tag or commit of a repository. Empty follows its
	// default branch.
	Ref string `json:"ref,omitempty"`
	// Subdir narrows the search to one folder of the source, for a repository
	// that keeps its skills under one.
	Subdir string `json:"subdir,omitempty"`

	Builtin bool  `json:"builtin,omitempty"`
	AddedAt int64 `json:"addedAt,omitempty"`

	// FetchedAt is when the store last took a copy, and Skills how many it held
	// then. Both are read from the store rather than stored in the list.
	FetchedAt int64 `json:"fetchedAt,omitempty"`
	Skills    int   `json:"skills,omitempty"`
}

// storeFetchRecord is what one download left in the store.
type storeFetchRecord struct {
	FetchedAt int64 `json:"fetchedAt"`
	Skills    int   `json:"skills"`
}

// builtinSources are seeded into an empty list, so the page has somewhere to
// install from before the operator has added anything.
var builtinSources = []*SkillSource{
	{
		Name:    "Anthropic skills",
		Kind:    SourceGithub,
		Url:     "anthropics/skills",
		Builtin: true,
	},
}

// sourceLock serializes the read-modify-write of the source list, the way the
// origin records are serialized.
var sourceLock sync.Mutex

// ListSources returns the places skills can be installed from, with what the
// store holds for each. The built-in entries are written on the first read, so
// what the page shows and what the file holds never differ.
func ListSources(owner string) ([]*SkillSource, error) {
	home, err := homeOf(owner)
	if err != nil {
		return nil, err
	}

	sourceLock.Lock()
	sources, seeded := loadSources(home)
	if seeded {
		saveSources(home, sources)
	}
	sourceLock.Unlock()

	for _, source := range sources {
		attachFetch(home, source)
	}
	return sources, nil
}

// AddSource records one new place to install from. A source already on the list
// is returned as it stands rather than duplicated: the id is derived from what
// the source points at, so pasting the same URL twice is the same source.
func AddSource(owner string, source *SkillSource) (*SkillSource, error) {
	home, err := homeOf(owner)
	if err != nil {
		return nil, err
	}
	if err := normalizeSource(source); err != nil {
		return nil, err
	}

	sourceLock.Lock()
	defer sourceLock.Unlock()

	sources, _ := loadSources(home)
	for _, existing := range sources {
		if existing.Id == source.Id {
			attachFetch(home, existing)
			return existing, nil
		}
	}

	source.AddedAt = time.Now().Unix()
	sources = append(sources, source)
	if err := saveSources(home, sources); err != nil {
		return nil, err
	}
	return source, nil
}

// DeleteSource takes one source off the list and drops what the store holds for
// it. Skills already installed from it are untouched: they are the agent's now.
func DeleteSource(owner string, id string) error {
	home, err := homeOf(owner)
	if err != nil {
		return err
	}
	if err := checkName(id); err != nil {
		return err
	}

	sourceLock.Lock()
	defer sourceLock.Unlock()

	sources, _ := loadSources(home)
	kept := []*SkillSource{}
	found := false
	for _, source := range sources {
		if source.Id == id {
			found = true
			continue
		}
		kept = append(kept, source)
	}
	if !found {
		return fmt.Errorf("no skill source with id %q", id)
	}

	if err := saveSources(home, kept); err != nil {
		return err
	}
	os.RemoveAll(sourceStore(home, id))
	return nil
}

// findSource resolves one source by id, for the catalog and the install.
func findSource(home string, id string) (*SkillSource, error) {
	if err := checkName(id); err != nil {
		return nil, err
	}

	sourceLock.Lock()
	sources, seeded := loadSources(home)
	if seeded {
		saveSources(home, sources)
	}
	sourceLock.Unlock()

	for _, source := range sources {
		if source.Id == id {
			return source, nil
		}
	}
	return nil, fmt.Errorf("no skill source with id %q", id)
}

// normalizeSource turns what the browser sent into a source this package can
// fetch, and gives it the id it is known by.
func normalizeSource(source *SkillSource) error {
	source.Url = strings.TrimSpace(source.Url)
	source.Ref = strings.TrimSpace(source.Ref)
	source.Name = strings.TrimSpace(source.Name)
	source.Subdir = strings.Trim(strings.TrimSpace(filepath.ToSlash(source.Subdir)), "/")
	source.Builtin = false

	switch source.Kind {
	case SourceGithub:
		repo, ref, subdir, err := parseRepoUrl(source.Url)
		if err != nil {
			return err
		}
		source.Url = repo
		if source.Ref == "" {
			source.Ref = ref
		}
		if source.Subdir == "" {
			source.Subdir = subdir
		}
		if source.Name == "" {
			source.Name = repo
		}
	case SourceArchive:
		parsed, err := url.Parse(source.Url)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return fmt.Errorf("%q is not an http or https URL", source.Url)
		}
		if source.Name == "" {
			source.Name = strings.TrimSuffix(filepath.Base(parsed.Path), ".zip")
		}
	case SourceLocal:
		if source.Url == "" {
			return fmt.Errorf("the folder is empty")
		}
		absolute, err := filepath.Abs(source.Url)
		if err != nil {
			return err
		}
		info, err := os.Stat(absolute)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("%s is not a folder", absolute)
		}
		source.Url = absolute
		source.Ref = ""
		if source.Name == "" {
			source.Name = filepath.Base(absolute)
		}
	case SourceUpload:
		if source.Name == "" {
			source.Name = "uploaded archive"
		}
	default:
		return fmt.Errorf("unknown skill source kind: %s", source.Kind)
	}

	if source.Id == "" {
		source.Id = sourceId(source)
	}
	return nil
}

// parseRepoUrl accepts a repository the several ways one gets written down:
// owner/repo, the browser URL of the repository, and the browser URL of a
// branch or of one folder inside it, which carries the ref and the subfolder.
func parseRepoUrl(raw string) (repo string, ref string, subdir string, err error) {
	text := strings.TrimSpace(raw)
	text = strings.TrimSuffix(text, ".git")
	text = strings.TrimSuffix(text, "/")

	if strings.Contains(text, "://") || strings.HasPrefix(text, "git@") {
		text = strings.TrimPrefix(text, "git@github.com:")
		if parsed, parseErr := url.Parse(text); parseErr == nil && parsed.Host != "" {
			if !strings.EqualFold(parsed.Host, "github.com") && !strings.EqualFold(parsed.Host, "www.github.com") {
				return "", "", "", fmt.Errorf("%q is not a github.com repository", raw)
			}
			text = strings.Trim(parsed.Path, "/")
		}
	}

	parts := strings.Split(text, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", fmt.Errorf("%q is not a repository; write it as owner/repo", raw)
	}
	repo = parts[0] + "/" + parts[1]

	// .../tree/<ref>/<subdir> and .../blob/<ref>/<subdir> are what the browser
	// puts in the address bar when one is looking at the folder to install.
	if len(parts) > 3 && (parts[2] == "tree" || parts[2] == "blob") {
		ref = parts[3]
		subdir = strings.Join(parts[4:], "/")
	}
	return repo, ref, subdir, nil
}

// sourceId names a source by what it points at, so the same repository added
// twice is one source and the store folder of one source is never another's.
func sourceId(source *SkillSource) string {
	key := strings.Join([]string{source.Kind, strings.ToLower(source.Url), source.Ref, source.Subdir}, "\x00")
	if source.Kind == SourceUpload {
		// An upload has no address to be named after, and two uploads of the
		// same file are still two of them.
		key = fmt.Sprintf("%s\x00%d", source.Name, time.Now().UnixNano())
	}
	sum := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%s-%s", slug(sourceLabel(source)), shortDigest(sum[:])[:8])
}

func sourceLabel(source *SkillSource) string {
	if source.Kind == SourceGithub || source.Name == "" {
		return strings.ReplaceAll(source.Url, "/", "-")
	}
	return source.Name
}

// sourceStore is where one source's downloaded content is kept.
func sourceStore(home string, id string) string {
	return filepath.Join(home, trashRoot, storeFolder, id)
}

func loadSources(home string) ([]*SkillSource, bool) {
	sources := []*SkillSource{}
	if raw, err := os.ReadFile(filepath.Join(home, trashRoot, sourceFile)); err == nil {
		json.Unmarshal(raw, &sources)
		return sources, false
	}

	for _, builtin := range builtinSources {
		seeded := *builtin
		seeded.AddedAt = time.Now().Unix()
		if err := normalizeSource(&seeded); err != nil {
			continue
		}
		seeded.Builtin = true
		sources = append(sources, &seeded)
	}
	return sources, true
}

func saveSources(home string, sources []*SkillSource) error {
	sort.SliceStable(sources, func(i, j int) bool { return sources[i].AddedAt < sources[j].AddedAt })
	raw, err := json.MarshalIndent(sources, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(home, trashRoot, sourceFile), raw, defaultConfigMode)
}

// attachFetch tells a source what the store holds for it, which is what the
// page shows as "fetched 2 hours ago, 34 skills".
func attachFetch(home string, source *SkillSource) {
	source.FetchedAt, source.Skills = 0, 0
	raw, err := os.ReadFile(filepath.Join(sourceStore(home, source.Id), storeFetch))
	if err != nil {
		return
	}
	record := storeFetchRecord{}
	if err := json.Unmarshal(raw, &record); err != nil {
		return
	}
	source.FetchedAt, source.Skills = record.FetchedAt, record.Skills
}

func writeFetchRecord(home string, id string, skills int) {
	raw, err := json.Marshal(storeFetchRecord{FetchedAt: time.Now().Unix(), Skills: skills})
	if err != nil {
		return
	}
	writeFile(filepath.Join(sourceStore(home, id), storeFetch), raw, defaultConfigMode)
}
