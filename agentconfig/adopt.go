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
	"path/filepath"
)

// UnmanagedSkill is one skill an agent reads that Gateway has no record of:
// installed by hand, by another tool, or through a Gateway whose records are
// gone. It works either way, but nothing says which version of it this is, so
// the update badge stays blank and Update has nothing to copy from.
type UnmanagedSkill struct {
	AgentId string `json:"agentId"`
	Owner   string `json:"owner"`
	Name    string `json:"name"`
	Path    string `json:"path"`

	Description string `json:"description,omitempty"`
	Files       int    `json:"files,omitempty"`
	Bytes       int64  `json:"bytes,omitempty"`
	Digest      string `json:"digest,omitempty"`
	Modified    int64  `json:"modified,omitempty"`

	// Match is the source skill this one was recognized as, when a source on
	// this machine holds it. Adopting it is what records the two as one.
	Match *SkillMatch `json:"match,omitempty"`
}

// SkillMatch is where an unmanaged skill appears to have come from.
type SkillMatch struct {
	SourceId   string `json:"sourceId"`
	SourceName string `json:"sourceName"`
	// Skill is the name inside the source, and Path the folder it is in.
	Skill string `json:"skill"`
	Path  string `json:"path"`

	// Same marks a source holding exactly this content. False is the same skill
	// at another version, which adopting turns into an offered update.
	Same bool `json:"same"`
	// ByDigest marks a skill recognized by its content after being renamed here.
	ByDigest bool `json:"byDigest,omitempty"`
}

// AdoptRequest records some scanned skills against the sources they came from.
type AdoptRequest struct {
	Owner string      `json:"owner"`
	Items []AdoptItem `json:"items"`
}

// AdoptItem is one skill and the source skill it is to be recorded against.
type AdoptItem struct {
	AgentId string `json:"agentId"`
	Name    string `json:"name"`

	SourceId string `json:"sourceId"`
	Skill    string `json:"skill"`
}

// storeSkill is one skill of one source, as the store holds it now.
type storeSkill struct {
	source *SkillSource
	skill  *CatalogSkill
}

// ScanUnmanaged lists the skills on this host that Gateway did not install and
// has no origin record for, each with the source skill it looks like a copy of.
// Only what is already downloaded is compared against: a scan reads the store
// as it stands rather than fetching every source again.
func ScanUnmanaged(inventories []*Inventory) []*UnmanagedSkill {
	stores := map[string][]*storeSkill{}
	found := []*UnmanagedSkill{}

	for _, inventory := range inventories {
		for _, item := range inventory.Skills {
			if !untracked(item) {
				continue
			}

			skills, ok := stores[inventory.Owner]
			if !ok {
				skills = storeSkills(inventory.Owner)
				stores[inventory.Owner] = skills
			}
			found = append(found, &UnmanagedSkill{
				AgentId:     item.AgentId,
				Owner:       item.Owner,
				Name:        item.Name,
				Path:        item.Path,
				Description: item.Description,
				Files:       item.Files,
				Bytes:       item.Bytes,
				Digest:      item.Digest,
				Modified:    item.Modified,
				Match:       matchStore(item, skills),
			})
		}
	}
	return found
}

// untracked is a skill the operator keeps that says nothing about where it came
// from. A plugin's or a checkout's skills are left out: those belong to the
// thing that ships them, and Gateway does not update them either way.
func untracked(item *Item) bool {
	return item.Update == nil && item.Scope == ScopeUser && !item.Managed && !item.Missing
}

// matchStore finds the source skill an unmanaged one came from. Identical
// content is the answer wherever it is found, because a skill renamed here is
// still that skill; failing that, a source holding the same name is the copy it
// has moved on from.
func matchStore(item *Item, skills []*storeSkill) *SkillMatch {
	var named *SkillMatch
	for _, held := range skills {
		if item.Digest != "" && held.skill.Digest == item.Digest {
			return newMatch(held, true, targetName(KindSkill, held.skill.Name) != targetName(KindSkill, item.Name))
		}
		if named == nil && targetName(KindSkill, held.skill.Name) == targetName(KindSkill, item.Name) {
			named = newMatch(held, false, false)
		}
	}
	return named
}

func newMatch(held *storeSkill, same bool, byDigest bool) *SkillMatch {
	return &SkillMatch{
		SourceId:   held.source.Id,
		SourceName: held.source.Name,
		Skill:      held.skill.Name,
		Path:       held.skill.Path,
		Same:       same,
		ByDigest:   byDigest,
	}
}

// storeSkills is every skill of every source that is on this machine already.
// A source that has never been fetched is skipped rather than downloaded: a
// scan of what is installed should not reach the network.
func storeSkills(owner string) []*storeSkill {
	home, err := homeOf(owner)
	if err != nil {
		return nil
	}
	sources, err := ListSources(owner)
	if err != nil {
		return nil
	}

	skills := []*storeSkill{}
	for _, source := range sources {
		root, err := catalogRoot(home, source)
		if err != nil {
			continue
		}
		for _, relative := range findSkillFolders(root) {
			skills = append(skills, &storeSkill{source: source, skill: readCatalogSkill(root, relative)})
		}
	}
	return skills
}

// AdoptSkills records each scanned skill as a copy of the source skill it was
// matched to, which is the same record an install writes. From then on the
// listing can say whether it is current, and Update has somewhere to copy from.
func AdoptSkills(request AdoptRequest) ([]*PlanItem, error) {
	if len(request.Items) == 0 {
		return nil, errors.New("no skill was picked to import")
	}
	home, err := homeOf(request.Owner)
	if err != nil {
		return nil, err
	}
	sources, err := ListSources(request.Owner)
	if err != nil {
		return nil, err
	}

	held := map[string]*SkillSource{}
	for _, source := range sources {
		held[source.Id] = source
	}

	planned := []*PlanItem{}
	for _, adopt := range request.Items {
		plan := &PlanItem{AgentId: adopt.AgentId, Name: adopt.Name}
		planned = append(planned, plan)

		item, err := findItem(adopt.AgentId, request.Owner, KindSkill, adopt.Name)
		if err != nil {
			plan.Action, plan.Reason = ActionSkip, err.Error()
			continue
		}
		path, err := sourceSkillPath(home, held[adopt.SourceId], adopt.Skill)
		if err != nil {
			plan.Action, plan.Reason = ActionSkip, err.Error()
			continue
		}

		plan.Path = item.Path
		plan.Action = ActionCreate
		recordSkillOrigin(home, item.Path, adopt.SourceId, sourceItemName(held[adopt.SourceId], &CatalogSkill{Name: adopt.Skill}), path)
	}
	return planned, nil
}

// sourceSkillPath is where one named skill of a source is on this machine, and
// an error when the source or the skill is no longer there.
func sourceSkillPath(home string, source *SkillSource, skill string) (string, error) {
	if source == nil {
		return "", errors.New("that skill source is no longer on the list")
	}
	if skill == "" {
		return "", errors.New("no skill of that source was chosen")
	}

	root, err := catalogRoot(home, source)
	if err != nil {
		return "", err
	}
	path, err := safeJoin(root, skill)
	if err != nil {
		return "", err
	}
	if _, ok := manifestPath(path); !ok {
		return "", fmt.Errorf("%s no longer holds %s", source.Name, filepath.Base(skill))
	}
	return path, nil
}
