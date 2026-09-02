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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// How an installed skill relates to the source it came from.
const (
	// InstallCopy writes the skill into the agent's own skills directory. The
	// agent then owns it, and a later fetch of the source is an update to take
	// or leave.
	InstallCopy = "copy"
	// InstallLink points the agent at Gateway's copy of the source, so every
	// agent linked to it moves together when the source is fetched again.
	InstallLink = "link"
)

// MaxUploadBytes bounds an archive uploaded through the browser, which is the
// same bound a downloaded one is held to.
const MaxUploadBytes = maxArchiveBytes

// SkillCatalog is what one source holds, as the page lists it.
type SkillCatalog struct {
	Source *SkillSource `json:"source"`
	// Root is the folder the skills below were found in, which is the store for
	// a downloaded source and the folder itself for a local one.
	Root   string          `json:"root"`
	Skills []*CatalogSkill `json:"skills"`
}

// CatalogSkill is one skill of a source, before it is installed anywhere.
type CatalogSkill struct {
	// Name is the folder the skill would be installed under, and Path is where
	// it is in the source.
	Name        string `json:"name"`
	Path        string `json:"path"`
	Description string `json:"description,omitempty"`

	Files    int    `json:"files,omitempty"`
	Bytes    int64  `json:"bytes,omitempty"`
	Digest   string `json:"digest,omitempty"`
	Modified int64  `json:"modified,omitempty"`
}

// InstallRequest is one install: some of a source's skills, into one or more
// agents belonging to the same account.
type InstallRequest struct {
	Owner    string   `json:"owner"`
	SourceId string   `json:"sourceId"`
	To       []string `json:"to"`
	Names    []string `json:"names"`
	// Mode is InstallCopy or InstallLink.
	Mode      string `json:"mode"`
	Overwrite bool   `json:"overwrite"`
}

// ReadCatalog lists what one source holds, fetching it first when the store has
// nothing or when refresh asks for it again.
func ReadCatalog(owner string, id string, refresh bool) (*SkillCatalog, error) {
	home, err := homeOf(owner)
	if err != nil {
		return nil, err
	}
	source, err := findSource(home, id)
	if err != nil {
		return nil, err
	}

	tree := filepath.Join(sourceStore(home, id), storeTree)
	if source.Kind != SourceLocal && (refresh || !exists(tree)) {
		if err := fetchSource(home, source); err != nil {
			return nil, err
		}
	}

	root, err := catalogRoot(home, source)
	if err != nil {
		return nil, err
	}

	catalog := &SkillCatalog{Source: source, Root: root, Skills: []*CatalogSkill{}}
	for _, relative := range findSkillFolders(root) {
		catalog.Skills = append(catalog.Skills, readCatalogSkill(root, relative))
	}

	if source.Kind != SourceLocal {
		writeFetchRecord(home, id, len(catalog.Skills))
	}
	attachFetch(home, source)
	return catalog, nil
}

// UploadSource takes an archive the operator picked in the browser, unpacks it
// into the store and records it as a source. An archive that holds no skill is
// refused here rather than added as a source with nothing in it.
func UploadSource(owner string, name string, data []byte) (*SkillSource, error) {
	home, err := homeOf(owner)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, errors.New("the uploaded file is empty")
	}

	source := &SkillSource{Kind: SourceUpload, Name: strings.TrimSpace(name)}
	if err := normalizeSource(source); err != nil {
		return nil, err
	}
	if err := storeArchive(home, source.Id, data); err != nil {
		return nil, err
	}

	root, err := catalogRoot(home, source)
	if err != nil {
		os.RemoveAll(sourceStore(home, source.Id))
		return nil, err
	}
	if len(findSkillFolders(root)) == 0 {
		os.RemoveAll(sourceStore(home, source.Id))
		return nil, fmt.Errorf("%s holds no folder with a %s in it", source.Name, skillFile)
	}

	added, err := AddSource(owner, source)
	if err != nil {
		os.RemoveAll(sourceStore(home, source.Id))
		return nil, err
	}
	return added, nil
}

// catalogRoot is the folder a source's skills are read from: the folder itself
// for a local source, the unwrapped archive for the rest, narrowed by the
// subfolder the source names.
func catalogRoot(home string, source *SkillSource) (string, error) {
	root := source.Url
	if source.Kind != SourceLocal {
		root = archiveRoot(filepath.Join(sourceStore(home, source.Id), storeTree))
	}

	if source.Subdir != "" {
		narrowed, err := safeJoin(root, source.Subdir)
		if err != nil {
			return "", err
		}
		root = narrowed
	}
	if !exists(root) {
		return "", fmt.Errorf("%s is not there", root)
	}
	return root, nil
}

func readCatalogSkill(root string, relative string) *CatalogSkill {
	path := filepath.Join(root, filepath.FromSlash(relative))
	skill := &CatalogSkill{Name: relative, Path: path}

	if manifest, ok := manifestPath(path); ok {
		if raw, err := os.ReadFile(manifest); err == nil {
			_, skill.Description = parseFrontMatter(string(raw))
		}
	}
	stat := measure(path)
	skill.Files, skill.Bytes, skill.Digest, skill.Modified = stat.files, stat.bytes, stat.digest, stat.modified
	return skill
}

// InstallSkills writes the chosen skills into every target agent. One skill's
// failure is recorded against it and the rest still run, the way a copy between
// agents is reported item by item.
func InstallSkills(request InstallRequest) ([]*PlanItem, error) {
	switch {
	case request.SourceId == "":
		return nil, errors.New("no skill source was chosen")
	case len(request.To) == 0:
		return nil, errors.New("no target agent was selected")
	case len(request.Names) == 0:
		return nil, errors.New("nothing was selected to install")
	case request.Mode != InstallCopy && request.Mode != InstallLink:
		return nil, fmt.Errorf("unknown install mode: %s", request.Mode)
	}

	home, err := homeOf(request.Owner)
	if err != nil {
		return nil, err
	}
	catalog, err := ReadCatalog(request.Owner, request.SourceId, false)
	if err != nil {
		return nil, err
	}

	available := map[string]*CatalogSkill{}
	for _, skill := range catalog.Skills {
		available[skill.Name] = skill
	}

	planned := []*PlanItem{}
	for _, agentId := range request.To {
		for _, name := range request.Names {
			planned = append(planned, installOne(request, home, catalog, available[name], agentId, name))
		}
	}
	return planned, nil
}

// installOne puts one skill of the source into one agent, deciding first what
// it would do to what is already there.
func installOne(
	request InstallRequest, home string, catalog *SkillCatalog,
	skill *CatalogSkill, agentId string, name string,
) *PlanItem {
	item := &PlanItem{AgentId: agentId, Name: name}
	if skill == nil {
		item.Action, item.Reason = ActionSkip, "this skill is no longer in the source"
		return item
	}

	found, _, err := resolve(agentId, request.Owner, KindSkill)
	if err != nil {
		item.Action, item.Reason = ActionSkip, err.Error()
		return item
	}
	dir := found.skills.dir(home)
	if dir == "" {
		item.Action, item.Reason = ActionSkip, "Gateway does not know where this agent keeps its own skills"
		return item
	}

	folder := targetName(KindSkill, name)
	if err := checkName(folder); err != nil {
		item.Action, item.Reason = ActionSkip, err.Error()
		return item
	}
	target := filepath.Join(dir, folder)
	item.Path = target

	// A copy holding the source's content is done; the same content behind a
	// link is not, when a copy is what was asked for.
	sameMode := isLink(target) == (request.Mode == InstallLink)

	switch {
	case !exists(target):
		item.Action = ActionCreate
	case sameMode && installedDigest(target) == skill.Digest:
		item.Action, item.Reason = ActionSkip, "already installed"
		return item
	case !request.Overwrite:
		item.Action, item.Reason = ActionSkip, "a different version is already there"
		return item
	default:
		item.Action, item.Reason = ActionOverwrite, "replaces what was installed"
		if err := trashInstalled(home, agentId, request.Owner, name, target); err != nil {
			item.Action, item.Reason = ActionFailed, err.Error()
			return item
		}
	}

	if err := placeSkill(skill.Path, target, request.Mode); err != nil {
		item.Action, item.Reason = ActionFailed, err.Error()
		return item
	}

	// The store keeps what was installed, so the source of an installed skill
	// is a folder on this machine like any other and the update badge, the
	// diff against it and the Update button all work unchanged.
	recordSkillOrigin(home, target, catalog.Source.Id, sourceItemName(catalog.Source, skill), skill.Path)
	return item
}

// sourceItemName is what the update badge calls this skill's source: the skill
// inside the source it was installed from.
func sourceItemName(source *SkillSource, skill *CatalogSkill) string {
	return source.Name + ":" + skill.Name
}

// placeSkill puts one skill folder at target, as a copy of the source or as a
// link to it.
func placeSkill(from string, target string, mode string) error {
	if _, ok := manifestPath(from); !ok {
		return fmt.Errorf("%s is not a skill folder", from)
	}
	if mode == InstallCopy {
		_, err := copySkill(from, filepath.Dir(target), filepath.Base(target))
		return err
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if err := os.RemoveAll(target); err != nil {
		return err
	}
	return makeLink(from, target)
}

// trashInstalled recycles what an overwrite replaces, so an install over the
// wrong skill is one restore away like a delete.
func trashInstalled(home string, agentId string, owner string, name string, target string) error {
	stat := measure(target)
	return trashSkill(home, &Item{
		AgentId: agentId,
		Owner:   owner,
		Kind:    KindSkill,
		Name:    targetName(KindSkill, name),
		Path:    target,
		Files:   stat.files,
		Bytes:   stat.bytes,
	})
}

// installedDigest identifies what is at a path already, so installing the same
// skill twice is reported as nothing to do rather than as a replacement.
func installedDigest(path string) string {
	if _, ok := manifestPath(path); !ok {
		return ""
	}
	return measure(path).digest
}

// isLink reports a skill installed as a symbolic link rather than as a copy.
func isLink(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSymlink != 0
}

// linkTarget is what a linked skill points at, for the listing to show. It is
// empty for a skill that is a folder of its own.
func linkTarget(path string) string {
	if !isLink(path) {
		return ""
	}
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		// A link whose target has been removed still says what it aimed at,
		// which is what explains a skill that has stopped working.
		if raw, readErr := os.Readlink(path); readErr == nil {
			return raw
		}
		return ""
	}
	return target
}

// resolvedDir looks through a symbolic link to the folder it names, because
// walking a link visits the link and not what is inside it.
func resolvedDir(dir string) string {
	if !isLink(dir) {
		return dir
	}
	if target, err := filepath.EvalSymlinks(dir); err == nil {
		return target
	}
	return dir
}

// isDirEntry reports a directory, following a symbolic link. A skill installed
// as a link is a link in its agent's skills directory, and os.ReadDir does not
// look through one.
func isDirEntry(path string, isDir bool, symlink bool) bool {
	if isDir {
		return true
	}
	if !symlink {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
