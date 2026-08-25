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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// A deleted item waits under the same home as the agent it came from, so a
	// skill folder is moved there rather than copied.
	trashRoot   = ".casbin-gateway"
	trashFolder = "trash"
	// trashRecord names the metadata of one deleted item, and trashPayload the
	// folder or file beside it.
	trashRecord  = "gateway-trash.json"
	trashPayload = "payload"
	// trashKeepDays is how long a deleted item can be restored. After that a
	// listing drops it, so the trash cannot grow without bound.
	trashKeepDays = 30
)

// TrashEntry is one deleted skill, MCP server or instruction file, kept so a
// delete can be taken back.
type TrashEntry struct {
	Id          string `json:"id"`
	AgentId     string `json:"agentId"`
	Owner       string `json:"owner"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Path is where the item was, and where a restore puts it back.
	Path      string `json:"path"`
	Files     int    `json:"files,omitempty"`
	Bytes     int64  `json:"bytes,omitempty"`
	DeletedAt int64  `json:"deletedAt"`

	// Entry is the MCP server as it was written, credentials included. It is
	// stripped before a listing leaves this package.
	Entry map[string]any `json:"entry,omitempty"`
}

// trashSkill moves a skill folder out of the agent's reach instead of deleting
// it, so an accidental delete is one restore away.
func trashSkill(home string, item *Item) error {
	if err := trashPath(home, item); err != nil {
		return err
	}

	// Nothing is at that path now, so the next skill written there is not the
	// one this record was about.
	forgetSkillOrigin(home, item.Path)
	return nil
}

// trashItem recycles one item by its kind, so replacing a skill keeps its
// origin bookkeeping in step.
func trashItem(home string, item *Item) error {
	if item.Kind == KindSkill {
		return trashSkill(home, item)
	}
	return trashPath(home, item)
}

// trashPath moves whatever is at the item's path into the recycle bin: a
// skill folder, or one agent's instruction file.
func trashPath(home string, item *Item) error {
	entry := newTrashEntry(item)
	dir, err := makeTrashDir(home, entry.Id)
	if err != nil {
		return err
	}

	if err := os.Rename(item.Path, filepath.Join(dir, trashPayload)); err != nil {
		os.RemoveAll(dir)
		return err
	}
	if err := writeTrashRecord(dir, entry); err != nil {
		// The folder is already out of the agent's skills directory, so put it
		// back rather than leave it in a trash entry nothing can find.
		os.Rename(filepath.Join(dir, trashPayload), item.Path)
		os.RemoveAll(dir)
		return err
	}
	return nil
}

// trashMcp records one MCP server before it is removed from its config file.
func trashMcp(home string, item *Item, entry map[string]any) error {
	record := newTrashEntry(item)
	record.Entry = entry
	dir, err := makeTrashDir(home, record.Id)
	if err != nil {
		return err
	}
	if err := writeTrashRecord(dir, record); err != nil {
		os.RemoveAll(dir)
		return err
	}
	return nil
}

// ListTrash returns what can still be restored, newest first, and drops what
// has waited longer than trashKeepDays.
func ListTrash(owner string) ([]*TrashEntry, error) {
	home, err := homeOf(owner)
	if err != nil {
		return nil, err
	}

	root := trashDir(home)
	dirs, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return []*TrashEntry{}, nil
	}
	if err != nil {
		return nil, err
	}

	entries := []*TrashEntry{}
	deadline := time.Now().AddDate(0, 0, -trashKeepDays).Unix()
	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		entry, err := readTrashRecord(filepath.Join(root, dir.Name()))
		if err != nil {
			continue
		}
		if entry.DeletedAt < deadline {
			os.RemoveAll(filepath.Join(root, dir.Name()))
			continue
		}
		// The stored MCP entry carries the credentials of the server it was;
		// what the page needs is that it existed.
		entry.Entry = nil
		entry.Owner = owner
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].DeletedAt > entries[j].DeletedAt })
	return entries, nil
}

// RestoreTrash puts one deleted item back where it came from. It refuses when
// something is there again, rather than overwriting whatever replaced it;
// replace says to go ahead, and what is there is moved into the trash first so
// that nothing is lost either way.
func RestoreTrash(owner string, id string, replace bool) (*TrashEntry, error) {
	home, err := homeOf(owner)
	if err != nil {
		return nil, err
	}
	if err := checkName(id); err != nil {
		return nil, err
	}

	dir := filepath.Join(trashDir(home), id)
	entry, err := readTrashRecord(dir)
	if err != nil {
		return nil, err
	}

	if entry.Kind == KindMcp {
		err = restoreMcp(owner, entry, replace)
	} else {
		err = restorePayload(home, dir, entry, replace)
	}
	if err != nil {
		return nil, err
	}

	os.RemoveAll(dir)
	entry.Entry = nil
	return entry, nil
}

func restorePayload(home string, dir string, entry *TrashEntry, replace bool) error {
	if exists(entry.Path) {
		if !replace {
			return fmt.Errorf("%s already exists again; restore it as a replacement, or move it aside first", entry.Path)
		}
		if err := trashItem(home, occupant(entry)); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(entry.Path), 0o755); err != nil {
		return err
	}
	return os.Rename(filepath.Join(dir, trashPayload), entry.Path)
}

// occupant is whatever took a deleted item's place, described well enough to be
// moved into the trash in its turn.
func occupant(entry *TrashEntry) *Item {
	item := &Item{
		AgentId: entry.AgentId,
		Owner:   entry.Owner,
		Kind:    entry.Kind,
		Name:    entry.Name,
		Path:    entry.Path,
	}
	if entry.Kind == KindSkill {
		stat := measure(entry.Path)
		item.Files, item.Bytes = stat.files, stat.bytes
	}
	return item
}

func restoreMcp(owner string, entry *TrashEntry, replace bool) error {
	found, home, err := resolve(entry.AgentId, owner, KindMcp)
	if err != nil {
		return err
	}
	if found.mcp.readOnly != "" {
		return fmt.Errorf("%s: %s", entry.AgentId, found.mcp.readOnly)
	}

	file := found.mcp.path(home)
	existing, err := found.mcp.store.read(file)
	if err != nil {
		return err
	}
	if taken, ok := existing[entry.Name]; ok {
		if !replace {
			return fmt.Errorf("%s already has an MCP server named %q again; restore it as a replacement, or rename that one first", entry.AgentId, entry.Name)
		}
		if err := trashMcp(home, occupant(entry), taken); err != nil {
			return err
		}
	}
	return found.mcp.store.write(file, entry.Name, entry.Entry)
}

// PurgeTrash deletes one entry for good, or all of them when id is empty.
func PurgeTrash(owner string, id string) error {
	home, err := homeOf(owner)
	if err != nil {
		return err
	}
	if id == "" {
		return os.RemoveAll(trashDir(home))
	}
	if err := checkName(id); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(trashDir(home), id))
}

func newTrashEntry(item *Item) *TrashEntry {
	return &TrashEntry{
		Id:          fmt.Sprintf("%d-%s", time.Now().UnixNano(), slug(item.Name)),
		AgentId:     item.AgentId,
		Owner:       item.Owner,
		Kind:        item.Kind,
		Name:        item.Name,
		Description: item.Description,
		Path:        item.Path,
		Files:       item.Files,
		Bytes:       item.Bytes,
		DeletedAt:   time.Now().Unix(),
	}
}

func trashDir(home string) string {
	return filepath.Join(home, trashRoot, trashFolder)
}

func makeTrashDir(home string, id string) (string, error) {
	dir := filepath.Join(trashDir(home), id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func writeTrashRecord(dir string, entry *TrashEntry) error {
	raw, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(dir, trashRecord), raw, defaultConfigMode)
}

func readTrashRecord(dir string) (*TrashEntry, error) {
	raw, err := os.ReadFile(filepath.Join(dir, trashRecord))
	if err != nil {
		return nil, err
	}
	entry := &TrashEntry{}
	if err := json.Unmarshal(raw, entry); err != nil {
		return nil, err
	}
	if entry.Id == "" || entry.Path == "" {
		return nil, fmt.Errorf("%s is not a Gateway trash entry", dir)
	}
	return entry, nil
}

// slug keeps a deleted item recognizable in the trash directory without letting
// its name decide a path.
func slug(name string) string {
	kept := strings.Map(func(letter rune) rune {
		switch {
		case letter >= 'a' && letter <= 'z', letter >= 'A' && letter <= 'Z',
			letter >= '0' && letter <= '9', letter == '-', letter == '_':
			return letter
		default:
			return '-'
		}
	}, name)
	if len(kept) > 40 {
		kept = kept[:40]
	}
	if kept == "" {
		return "item"
	}
	return kept
}
