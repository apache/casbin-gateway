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
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// maxPromptBytes bounds what the editor loads and what a save accepts.
	maxPromptBytes = 256 * 1024
	// maxPromptSummary bounds the first line shown beside the file name.
	maxPromptSummary = 160
	// promptShared matches one agent's instruction file to every other's. They
	// hold the same thing under a different name, which is the whole reason for
	// listing them side by side.
	promptShared = "instructions"
	// promptMissing explains a file that is not there, and doubles as the
	// reason there is nothing to delete.
	promptMissing = "this agent has no instructions yet"
)

// readPrompt describes one agent's instruction file. A file nobody has written
// yet is listed as missing rather than left out: what the agent is told is
// "nothing", and that is the answer the page exists to give.
func readPrompt(agentId string, owner string, path string) *Item {
	item := &Item{
		AgentId: agentId,
		Owner:   owner,
		Kind:    KindPrompt,
		Name:    filepath.Base(path),
		Shared:  promptShared,
		Path:    path,
		Scope:   ScopeUser,
	}

	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		item.Missing = true
		item.ReadOnly = promptMissing
		return item
	}

	stat := measure(path)
	item.Files, item.Bytes, item.Digest, item.Modified = stat.files, stat.bytes, stat.digest, stat.modified
	if raw, err := readPromptContent(path); err == nil {
		item.Description = promptSummary(string(raw))
	}
	return item
}

func promptDetail(item *Item) (*Detail, error) {
	if item.Missing {
		return &Detail{Item: item, Content: ""}, nil
	}

	raw, err := readPromptContent(item.Path)
	if err != nil {
		return nil, err
	}
	return &Detail{Item: item, Content: string(raw)}, nil
}

// SavePrompt replaces one agent's instruction file, creating it when it is not
// there yet. There is one such file per agent, so it is named by the agent
// rather than by itself.
func SavePrompt(agentId string, owner string, content string) (*Item, error) {
	found, home, err := resolve(agentId, owner, KindPrompt)
	if err != nil {
		return nil, err
	}
	if len(content) > maxPromptBytes {
		return nil, fmt.Errorf("these instructions are larger than the %d bytes Gateway edits", maxPromptBytes)
	}

	path := found.prompt.path(home)
	_, mode, err := readFile(path)
	if err != nil {
		return nil, err
	}
	if err := writeFile(path, []byte(content), mode); err != nil {
		return nil, err
	}
	return readPrompt(agentId, owner, path), nil
}

// copyPrompt writes one agent's instructions into another agent's own file,
// under whatever name that agent reads. What it replaces goes to the recycle
// bin first: an instruction file is written by hand and there is no other copy.
func copyPrompt(home string, agentId string, owner string, path string, from *Item) (string, error) {
	raw, err := readPromptContent(from.Path)
	if err != nil {
		return "", err
	}

	_, mode, err := readFile(path)
	if err != nil {
		return "", err
	}
	if existing := readPrompt(agentId, owner, path); !existing.Missing {
		if err := trashPath(home, existing); err != nil {
			return "", err
		}
	}
	return path, writeFile(path, raw, mode)
}

func readPromptContent(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxPromptBytes {
		return nil, fmt.Errorf("%s is larger than the %d bytes Gateway edits", path, maxPromptBytes)
	}
	return os.ReadFile(path)
}

// promptSummary is the first line of the file with any heading marks taken off,
// so a row says what these instructions are about.
func promptSummary(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#"))
		if line == "" {
			continue
		}
		if letters := []rune(line); len(letters) > maxPromptSummary {
			line = string(letters[:maxPromptSummary]) + "..."
		}
		return line
	}
	return ""
}
