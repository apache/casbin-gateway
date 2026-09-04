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
	"sort"
)

// Which way one run copies. Both is the useful default: a machine that only
// pushes never learns what another one backed up.
const (
	DirectionBoth = "both"
	DirectionUp   = "up"
	DirectionDown = "down"
)

// Options are what one run may do.
type Options struct {
	// Match keeps a run to the files it understands. The directory is on disk
	// and anything can drop a file in it.
	Match func(name string) bool
	// Retention is how many files the target keeps, the oldest dropped first
	// by name. Zero keeps every file that ever landed there.
	Retention int
	Direction string
	// MaxBytes is the largest file that is copied. A bigger one is left where
	// it is rather than sent through memory.
	MaxBytes int64
}

// Result is what one run did, and what the Settings page reports.
type Result struct {
	Uploaded   []string `json:"uploaded"`
	Downloaded []string `json:"downloaded"`
	// Removed is what the retention dropped at the target. Nothing is ever
	// deleted locally: a file missing here is one this machine never took.
	Removed []string `json:"removed"`
	Skipped int      `json:"skipped"`
	// Errors are the files that failed. One bad file does not stop the run:
	// the point of a sync is the other twenty that went over.
	Errors []string `json:"errors"`
	// Remote is what the target holds afterwards.
	Remote []File `json:"remote"`
}

func (result *Result) failed(name string, err error) {
	result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", name, err.Error()))
}

func newResult() *Result {
	return &Result{Uploaded: []string{}, Downloaded: []string{}, Removed: []string{}, Errors: []string{}, Remote: []File{}}
}

// SyncDir makes a directory and a target hold the same files. Snapshots are
// immutable, so a name present on both sides is the same file on both sides
// and is never copied again, and neither side is ever asked which copy is
// newer.
func SyncDir(ctx context.Context, target Target, dir string, options Options) (*Result, error) {
	result := newResult()

	remoteFiles, err := target.List(ctx)
	if err != nil {
		return nil, err
	}

	remote := map[string]File{}
	for _, file := range remoteFiles {
		if options.Match == nil || options.Match(file.Name) {
			remote[file.Name] = file
		} else {
			result.Skipped++
		}
	}

	local, err := listDir(dir, options.Match)
	if err != nil {
		return nil, err
	}

	if options.Direction != DirectionDown {
		for _, name := range sortedNames(local) {
			if _, there := remote[name]; there {
				continue
			}

			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				result.failed(name, err)
				continue
			}
			if options.MaxBytes > 0 && int64(len(data)) > options.MaxBytes {
				result.Skipped++
				continue
			}
			if err := target.Write(ctx, name, data); err != nil {
				result.failed(name, err)
				continue
			}

			remote[name] = File{Name: name, Size: int64(len(data))}
			result.Uploaded = append(result.Uploaded, name)
		}
	}

	if options.Direction != DirectionUp {
		for _, name := range sortedNames(remote) {
			if _, here := local[name]; here {
				continue
			}
			if options.MaxBytes > 0 && remote[name].Size > options.MaxBytes {
				result.Skipped++
				continue
			}

			data, err := target.Read(ctx, name)
			if err != nil {
				result.failed(name, err)
				continue
			}
			if err := writeFile(filepath.Join(dir, name), data); err != nil {
				result.failed(name, err)
				continue
			}

			local[name] = int64(len(data))
			result.Downloaded = append(result.Downloaded, name)
		}
	}

	// Only after the pull: a file dropped for the retention has to have been
	// somewhere else first.
	if options.Direction != DirectionDown {
		prune(ctx, target, remote, options.Retention, result)
	}

	for _, name := range sortedNames(remote) {
		result.Remote = append(result.Remote, remote[name])
	}
	return result, nil
}

// prune keeps the newest files at the target. The names start with the time
// they were taken, so they sort the way the contents would.
func prune(ctx context.Context, target Target, remote map[string]File, retention int, result *Result) {
	if retention <= 0 || len(remote) <= retention {
		return
	}

	names := sortedNames(remote)
	for i := 0; i < len(names)-retention; i++ {
		name := names[i]
		if err := target.Remove(ctx, name); err != nil {
			result.failed(name, err)
			continue
		}

		delete(remote, name)
		result.Removed = append(result.Removed, name)
	}
}

func listDir(dir string, match func(string) bool) (map[string]int64, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]int64{}, nil
		}
		return nil, err
	}

	files := map[string]int64{}
	for _, entry := range entries {
		if entry.IsDir() || (match != nil && !match(entry.Name())) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}
		files[entry.Name()] = info.Size()
	}
	return files, nil
}

// writeFile lands a file under its own name only once it is whole, so that a
// folder somebody else's client is watching never uploads half a snapshot.
func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	partial := path + ".part"
	if err := os.WriteFile(partial, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(partial, path); err != nil {
		os.Remove(partial)
		return err
	}
	return nil
}

func sortedNames[V any](files map[string]V) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
