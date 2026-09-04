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

package cloudsync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// KindFolder is a directory on this machine. It is how Dropbox, OneDrive,
// iCloud Drive, Google Drive and a mounted NAS share are all supported without
// a line of code for any of them: their desktop clients already sync a folder,
// so writing the snapshots into it is the whole integration.
const KindFolder = "folder"

func init() {
	Register(&Kind{
		Name:        KindFolder,
		DisplayName: "Folder",
		Description: "Folder description",
		Fields: []Field{
			{
				Name:        "path",
				Label:       "Folder path",
				Type:        FieldText,
				Required:    true,
				Placeholder: defaultFolderExample(),
				Hint:        "Folder path hint",
			},
		},
		New: newFolderTarget,
	})
}

type folderTarget struct {
	dir string
}

func newFolderTarget(config Config) (Target, error) {
	dir := ExpandPath(config.Option("path"))
	if dir == "" {
		return nil, fmt.Errorf("the folder has no path")
	}
	return &folderTarget{dir: dir}, nil
}

func (target *folderTarget) Describe() string {
	return target.dir
}

func (target *folderTarget) List(ctx context.Context) ([]File, error) {
	entries, err := os.ReadDir(target.dir)
	if err != nil {
		// A folder that is not there yet is an empty one: it is created by the
		// first file written into it, and a reachable target with nothing in it
		// is not an error.
		if os.IsNotExist(err) {
			return []File{}, nil
		}
		return nil, err
	}

	files := []File{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, File{Name: entry.Name(), Size: info.Size(), ModifiedTime: info.ModTime()})
	}
	return files, nil
}

func (target *folderTarget) Read(ctx context.Context, name string) ([]byte, error) {
	path, err := target.path(name)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func (target *folderTarget) Write(ctx context.Context, name string, data []byte) error {
	path, err := target.path(name)
	if err != nil {
		return err
	}
	return writeFile(path, data)
}

func (target *folderTarget) Remove(ctx context.Context, name string) error {
	path, err := target.path(name)
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// path keeps a name a name. Nothing outside the folder is read, written or
// deleted, whatever the target listed or the settings row was made to say.
func (target *folderTarget) path(name string) (string, error) {
	if name == "" || name != filepath.Base(name) || strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("this is not a file name: %s", name)
	}
	return filepath.Join(target.dir, name), nil
}

// ExpandPath resolves the "~" and the environment variables a path typed by
// hand carries, so "%OneDrive%\Backups" and "~/Dropbox" both land where the
// person meant.
func ExpandPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}

	path = os.ExpandEnv(path)
	path = expandWindowsEnv(path)
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, strings.TrimPrefix(path[1:], string(filepath.Separator)))
		}
	}
	return filepath.Clean(path)
}

// winEnvPattern is the %NAME% spelling of an environment variable, which
// os.ExpandEnv leaves alone and which is the one people paste on Windows.
var winEnvPattern = regexp.MustCompile(`%([A-Za-z_][A-Za-z0-9_]*)%`)

// expandWindowsEnv fills in the variables it knows and leaves the rest of the
// path exactly as it was typed.
func expandWindowsEnv(path string) string {
	return winEnvPattern.ReplaceAllStringFunc(path, func(match string) string {
		if value, found := os.LookupEnv(strings.Trim(match, "%")); found {
			return value
		}
		return match
	})
}
