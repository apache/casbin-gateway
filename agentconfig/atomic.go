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
	"os"
	"path/filepath"
)

// writeFileAtomic writes data to path by creating a temporary file in the same
// directory, fsyncing it, then renaming it over path. A crash or power loss can
// therefore never leave a half-written config: path always contains either the
// old file or the complete new one. The parent directory is fsynced so the
// rename itself survives a crash, and mode is applied to the final file.
//
// The temp file must share path's directory so the rename stays on one
// filesystem and is atomic; a rename across filesystems is a copy and is not.
func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".casbin-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	// Remove the temp file if anything below fails; a no-op once the rename has
	// consumed it.
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	fsyncDir(dir)
	return nil
}

// fsyncDir flushes a directory entry so a rename into it is durable. It is
// best-effort: several platforms and filesystems cannot sync a directory, and
// that must not fail an otherwise successful write.
func fsyncDir(dir string) {
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	defer d.Close()
	_ = d.Sync()
}
