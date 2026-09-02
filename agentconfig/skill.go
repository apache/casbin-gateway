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
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	// skillFile is the manifest that makes a directory a skill rather than
	// whatever else the operator keeps next to their skills.
	skillFile = "SKILL.md"
	// maxSkillFileBytes bounds what the detail view loads: a skill is meant to
	// be read by a model, but nothing stops one from carrying a large file.
	maxSkillFileBytes = 256 * 1024
	// maxSkillFiles bounds the file list shown beside a skill's manifest.
	maxSkillFiles = 200
	// maxDigestFileBytes bounds what the digest reads; a larger file is
	// identified by its size and modification time instead of its content.
	maxDigestFileBytes = 1 << 20
	// A plugin tree is somebody else's checkout, so the search for the skills
	// folders in it is bounded in both depth and result count.
	maxPluginDepth    = 6
	maxPluginSkillDir = 200
)

// pluginNoise is what a plugin checkout carries besides its own files. The
// agents' own plugin cache is not on this list: an installed plugin lives under
// it, and skipping it hid every skill of an agent that caches its plugins that
// way.
var pluginNoise = map[string]bool{
	"node_modules": true, "dist": true, "build": true,
	"vendor": true, "__pycache__": true, ".venv": true, "venv": true,
}

// skillDir is one directory of skill folders, and what the skills read from it
// are called.
type skillDir struct {
	path  string
	scope string
	// owner is the plugin shipping these skills, empty for the agent's own
	// skills directory. It qualifies their names the way the agents do.
	owner string
}

// skillScan is one agent's skills and the directories they were read from.
type skillScan struct {
	items    []*Item
	dirs     []string
	problems []string
}

// readSkills lists the skills of one agent, across every place it keeps them. A
// directory that does not exist is an agent with no skills there, not an error.
func readSkills(agentId string, owner string, found *skillLayout, home string) skillScan {
	scan := skillScan{items: []*Item{}, dirs: []string{}, problems: []string{}}
	seen := map[string]bool{}

	for _, source := range found.sources {
		for _, root := range source.roots(home) {
			dirs := []skillDir{{path: root, scope: source.scope}}
			switch {
			case source.scan:
				dirs = pluginSkillDirs(root, source.scope)
			case source.scope == ScopeProject:
				// A project's skills are named after the checkout they are in,
				// so two projects can hold a skill of the same name and the
				// listing still says which is which.
				if !exists(root) {
					continue
				}
				dirs[0].owner = filepath.Base(filepath.Dir(filepath.Dir(root)))
				scan.dirs = append(scan.dirs, root)
			default:
				scan.dirs = append(scan.dirs, root)
			}

			for _, dir := range dirs {
				items, err := readSkillDir(agentId, owner, dir)
				if err != nil {
					scan.problems = append(scan.problems, err.Error())
					continue
				}
				if source.scan && len(items) > 0 {
					scan.dirs = append(scan.dirs, dir.path)
				}
				for _, item := range items {
					if seen[item.Name] {
						continue
					}
					seen[item.Name] = true
					scan.items = append(scan.items, item)
				}
			}
		}
	}
	sortItems(scan.items)
	return scan
}

// readSkillDir reads one directory of skill folders. A folder without a
// manifest is looked into one level further: a skills directory may group its
// skills, and a grouped skill is a skill like any other.
func readSkillDir(agentId string, owner string, dir skillDir) ([]*Item, error) {
	entries, err := os.ReadDir(dir.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	items := []*Item{}
	for _, entry := range entries {
		path := filepath.Join(dir.path, entry.Name())
		// A dot directory is the agent's own bookkeeping, such as the manifest
		// Cursor writes beside the skills it manages.
		if !isDirEntry(path, entry.IsDir(), entry.Type()&fs.ModeSymlink != 0) ||
			strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		if _, ok := manifestPath(path); ok {
			items = append(items, newSkillItem(agentId, owner, dir, entry.Name(), path))
			continue
		}

		nested, err := os.ReadDir(path)
		if err != nil {
			continue
		}
		for _, child := range nested {
			childPath := filepath.Join(path, child.Name())
			if !isDirEntry(childPath, child.IsDir(), child.Type()&fs.ModeSymlink != 0) ||
				strings.HasPrefix(child.Name(), ".") {
				continue
			}
			if _, ok := manifestPath(childPath); ok {
				items = append(items, newSkillItem(agentId, owner, dir, entry.Name()+"/"+child.Name(), childPath))
			}
		}
	}
	return items, nil
}

func newSkillItem(agentId string, owner string, dir skillDir, name string, path string) *Item {
	item := &Item{
		AgentId: agentId,
		Owner:   owner,
		Kind:    KindSkill,
		Name:    name,
		Path:    path,
		Scope:   dir.scope,
		Origin:  dir.owner,
	}
	if dir.owner != "" {
		item.Name = dir.owner + ":" + name
	}
	if dir.scope == ScopePlugin {
		item.ReadOnly = "this skill belongs to a plugin; uninstall the plugin to remove it"
	}
	if dir.scope == ScopeProject {
		item.Project = filepath.Dir(filepath.Dir(dir.path))
	}

	if manifest, ok := manifestPath(path); ok {
		if raw, err := os.ReadFile(manifest); err == nil {
			_, item.Description = parseFrontMatter(string(raw))
		}
	}
	item.Link = linkTarget(path)
	stat := measure(path)
	item.Files, item.Bytes, item.Digest, item.Modified = stat.files, stat.bytes, stat.digest, stat.modified
	return item
}

// pluginSkillDirs finds the skills folders inside a plugin tree. Agents read
// the skills their plugins ship, so those are on the machine exactly like the
// hand-written ones and a listing that leaves them out is wrong.
func pluginSkillDirs(root string, scope string) []skillDir {
	dirs := []skillDir{}
	filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.IsDir() || path == root {
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".") || pluginNoise[strings.ToLower(entry.Name())] {
			return fs.SkipDir
		}
		if len(dirs) >= maxPluginSkillDir {
			return filepath.SkipAll
		}
		if strings.EqualFold(entry.Name(), "skills") {
			dirs = append(dirs, skillDir{path: path, scope: scope, owner: filepath.Base(filepath.Dir(path))})
			return fs.SkipDir
		}
		if depthUnder(root, path) >= maxPluginDepth {
			return fs.SkipDir
		}
		return nil
	})
	return dirs
}

func depthUnder(root string, path string) int {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return maxPluginDepth
	}
	return len(strings.Split(filepath.ToSlash(relative), "/"))
}

// skillDetail loads one skill's manifest and the names of the files shipped
// with it.
func skillDetail(item *Item) (*Detail, error) {
	manifest, ok := manifestPath(item.Path)
	if !ok {
		return nil, fmt.Errorf("no %s in %s", skillFile, item.Path)
	}

	raw, err := os.ReadFile(manifest)
	if err != nil {
		return nil, err
	}
	content := string(raw)
	if len(raw) > maxSkillFileBytes {
		content = string(raw[:maxSkillFileBytes]) + "\n\n... truncated by Gateway ..."
	}
	return &Detail{Item: item, Content: content, Files: listFiles(item.Path)}, nil
}

// copySkill copies one skill folder into another agent's skills directory,
// replacing what is there. The two agents Gateway supports keep skills in the
// same format, so the copy is the folder itself and nothing is converted.
func copySkill(from string, dir string, name string) (string, error) {
	if err := checkName(name); err != nil {
		return "", err
	}
	if _, ok := manifestPath(from); !ok {
		return "", fmt.Errorf("%s is not a skill folder", from)
	}

	to := filepath.Join(dir, name)
	same, err := samePath(from, to)
	if err != nil {
		return "", err
	}
	if same {
		return "", fmt.Errorf("%s is already the source of this copy", to)
	}

	staged := to + ".gateway-copy"
	if err := os.RemoveAll(staged); err != nil {
		return "", err
	}
	if err := copyTree(from, staged); err != nil {
		os.RemoveAll(staged)
		return "", err
	}
	if err := os.RemoveAll(to); err != nil {
		os.RemoveAll(staged)
		return "", err
	}
	if err := os.Rename(staged, to); err != nil {
		os.RemoveAll(staged)
		return "", err
	}
	return to, nil
}

// manifestPath finds the skill manifest inside a folder. The name is matched
// case-insensitively because the agents that write it do not agree on the case
// on the file systems that care.
func manifestPath(dir string) (string, bool) {
	path := filepath.Join(dir, skillFile)
	if _, err := os.Stat(path); err == nil {
		return path, true
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(entry.Name(), skillFile) {
			return filepath.Join(dir, entry.Name()), true
		}
	}
	return "", false
}

// treeStat summarizes a skill folder. digest identifies its content and
// modified is its newest file, which is what lets two agents' copies of one
// skill be told apart and put in order.
type treeStat struct {
	files    int
	bytes    int64
	digest   string
	modified int64
}

func measure(dir string) treeStat {
	// A skill installed as a link is a link, and walking one visits the link
	// rather than the folder it names.
	dir = resolvedDir(dir)

	stat := treeStat{}
	hash := sha256.New()
	// WalkDir visits in lexical order, so one folder digests the same wherever
	// it is read from.
	filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		stat.files++
		stat.bytes += info.Size()
		if modified := info.ModTime().Unix(); modified > stat.modified {
			stat.modified = modified
		}

		relative, err := filepath.Rel(dir, path)
		if err != nil {
			return nil
		}
		fmt.Fprintf(hash, "%s\x00%d\x00", filepath.ToSlash(relative), info.Size())
		if !entry.Type().IsRegular() || info.Size() > maxDigestFileBytes {
			fmt.Fprintf(hash, "%d\x00", info.ModTime().UnixNano())
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		io.Copy(hash, file)
		file.Close()
		return nil
	})
	stat.digest = shortDigest(hash.Sum(nil))
	return stat
}

func shortDigest(sum []byte) string {
	return hex.EncodeToString(sum)[:16]
}

// listFiles names what a skill ships besides its manifest, in slash form so the
// list reads the same on every platform.
func listFiles(dir string) []string {
	dir = resolvedDir(dir)

	files := []string{}
	filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || len(files) >= maxSkillFiles {
			return nil
		}
		relative, err := filepath.Rel(dir, path)
		if err != nil || strings.EqualFold(relative, skillFile) {
			return nil
		}
		files = append(files, filepath.ToSlash(relative))
		return nil
	})
	sort.Strings(files)
	return files
}

func copyTree(from string, to string) error {
	return filepath.WalkDir(from, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, relative)

		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		// A symbolic link in a skill folder would point at the source agent's
		// directory, so it is left behind rather than copied as a broken link.
		if !entry.Type().IsRegular() {
			return nil
		}
		return copyOneFile(path, target, entry)
	})
}

func copyOneFile(path string, target string, entry fs.DirEntry) error {
	info, err := entry.Info()
	if err != nil {
		return err
	}

	source, err := os.Open(path)
	if err != nil {
		return err
	}
	defer source.Close()

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	destination, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(destination, source); err != nil {
		destination.Close()
		return err
	}
	return destination.Close()
}

// samePath compares two paths by what they resolve to, so a copy cannot be
// asked to overwrite its own source through a link or a differently spelled
// path to the same directory.
func samePath(left string, right string) (bool, error) {
	leftInfo, err := os.Stat(left)
	if err != nil {
		return false, err
	}
	rightInfo, err := os.Stat(right)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return os.SameFile(leftInfo, rightInfo), nil
}
