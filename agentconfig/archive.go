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
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/apache/casbin-gateway/proxy"
)

const (
	// A skills repository is text and a few images, so these bounds hold what
	// one plausibly is and stop a mistyped URL from filling the disk.
	maxArchiveBytes     = 128 << 20
	maxArchiveFileBytes = 16 << 20
	maxArchiveFiles     = 20000
	// Downloading a repository takes as long as the link does, and the caller
	// is a browser waiting on it.
	archiveTimeout = 120 * time.Second
)

// githubRefs are the branches tried for a repository given without one. HEAD is
// whatever the repository calls its default branch, so it answers first and the
// rest are there for a host that does not serve it.
var githubRefs = []string{"HEAD", "main", "master"}

// fetchSource fills the store with one source's content, replacing whatever was
// there. A local source is read where it is and has nothing to fetch.
func fetchSource(home string, source *SkillSource) error {
	if source.Kind == SourceLocal {
		return nil
	}
	if source.Kind == SourceUpload {
		return fmt.Errorf("%s was uploaded, so there is nowhere to fetch it from again", source.Name)
	}

	urls, err := downloadUrls(source)
	if err != nil {
		return err
	}

	var data []byte
	var last error
	for _, address := range urls {
		data, last = downloadArchive(address)
		if last == nil {
			break
		}
	}
	if last != nil {
		return last
	}
	return storeArchive(home, source.Id, data)
}

// storeArchive unpacks one archive into a source's store folder, in one step:
// the new content is staged beside the old and swapped in, so a download that
// fails half way leaves the store as it was.
func storeArchive(home string, id string, data []byte) error {
	store := sourceStore(home, id)
	if err := os.MkdirAll(store, 0o755); err != nil {
		return err
	}

	staged := filepath.Join(store, storeTree+".gateway-fetch")
	if err := os.RemoveAll(staged); err != nil {
		return err
	}
	if err := extractArchive(data, staged); err != nil {
		os.RemoveAll(staged)
		return err
	}

	tree := filepath.Join(store, storeTree)
	if err := os.RemoveAll(tree); err != nil {
		os.RemoveAll(staged)
		return err
	}
	if err := os.Rename(staged, tree); err != nil {
		os.RemoveAll(staged)
		return err
	}
	return nil
}

// downloadUrls is where a source's archive is fetched from, in the order they
// are tried. A repository without a ref is tried at each of the usual ones.
func downloadUrls(source *SkillSource) ([]string, error) {
	if source.Kind == SourceArchive {
		return []string{source.Url}, nil
	}

	repo, _, _, err := parseRepoUrl(source.Url)
	if err != nil {
		return nil, err
	}
	refs := githubRefs
	if source.Ref != "" {
		refs = []string{source.Ref}
	}

	urls := []string{}
	for _, ref := range refs {
		urls = append(urls, fmt.Sprintf("https://codeload.github.com/%s/tar.gz/%s", repo, ref))
	}
	return urls, nil
}

func downloadArchive(address string) ([]byte, error) {
	client := &http.Client{Timeout: archiveTimeout, Transport: proxy.Transport()}
	resp, err := client.Get(address)
	if err != nil {
		return nil, fmt.Errorf("%s could not be reached: %s", address, err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s answered %s", address, resp.Status)
	}

	// One byte past the limit is read on purpose: it tells a file that is
	// exactly the limit from one that was cut short.
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxArchiveBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxArchiveBytes {
		return nil, fmt.Errorf("%s is larger than %d MB", address, maxArchiveBytes>>20)
	}
	return data, nil
}

// extractArchive unpacks a .zip or a .tar.gz into dest, deciding which by what
// the bytes start with rather than by the name they arrived under.
func extractArchive(data []byte, dest string) error {
	switch {
	case bytes.HasPrefix(data, []byte("PK\x03\x04")):
		return extractZip(data, dest)
	case bytes.HasPrefix(data, []byte{0x1f, 0x8b}):
		return extractTarGz(data, dest)
	}
	return fmt.Errorf("this is neither a .zip nor a .tar.gz archive")
}

func extractZip(data []byte, dest string) error {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	if len(reader.File) > maxArchiveFiles {
		return fmt.Errorf("this archive holds more than %d files", maxArchiveFiles)
	}

	for _, file := range reader.File {
		target, err := safeJoin(dest, file.Name)
		if err != nil {
			continue
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		// A symbolic link or a device node in an archive is not something a
		// skill needs, and following one would write outside dest.
		if !file.Mode().IsRegular() {
			continue
		}

		opened, err := file.Open()
		if err != nil {
			return err
		}
		err = writeExtracted(target, opened, file.Mode().Perm())
		opened.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func extractTarGz(data []byte, dest string) error {
	unzipped, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer unzipped.Close()

	reader := tar.NewReader(unzipped)
	count := 0
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		count++
		if count > maxArchiveFiles {
			return fmt.Errorf("this archive holds more than %d files", maxArchiveFiles)
		}

		target, err := safeJoin(dest, header.Name)
		if err != nil {
			continue
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := writeExtracted(target, reader, os.FileMode(header.Mode).Perm()); err != nil {
				return err
			}
		}
	}
}

func writeExtracted(target string, content io.Reader, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if mode == 0 {
		mode = 0o644
	}

	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	written, err := io.Copy(file, io.LimitReader(content, maxArchiveFileBytes+1))
	if err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if written > maxArchiveFileBytes {
		os.Remove(target)
		return fmt.Errorf("%s is larger than %d MB", filepath.Base(target), maxArchiveFileBytes>>20)
	}
	return nil
}

// safeJoin resolves one archive entry's name under dest. An archive is somebody
// else's file, and an entry named ../../.ssh/authorized_keys is the reason this
// is not a plain filepath.Join.
func safeJoin(dest string, name string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." || clean == string(filepath.Separator) {
		return "", fmt.Errorf("%q names no file", name)
	}
	if filepath.IsAbs(clean) || filepath.VolumeName(clean) != "" {
		return "", fmt.Errorf("%q is an absolute path", name)
	}

	target := filepath.Join(dest, clean)
	if target != dest && !strings.HasPrefix(target, dest+string(filepath.Separator)) {
		return "", fmt.Errorf("%q points outside the archive", name)
	}
	return target, nil
}

// archiveRoot looks through the folder an archive unpacks into. A repository
// tarball holds everything under one folder named after the branch, and that
// folder is the repository rather than a part of it.
func archiveRoot(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || !entries[0].IsDir() {
		return dir
	}

	inner := filepath.Join(dir, entries[0].Name())
	// A single folder that is itself a skill is the content, not a wrapper.
	if _, ok := manifestPath(inner); ok {
		return dir
	}
	return inner
}

// findSkillFolders lists the skills inside a source's tree, by their path
// relative to root. A skills repository groups them however it likes, so the
// whole tree is searched rather than one level of it.
func findSkillFolders(root string) []string {
	found := []string{}
	filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.IsDir() {
			return nil
		}
		if path != root {
			if strings.HasPrefix(entry.Name(), ".") || pluginNoise[strings.ToLower(entry.Name())] {
				return fs.SkipDir
			}
		}
		if len(found) >= maxPluginSkillDir {
			return filepath.SkipAll
		}
		if depthUnder(root, path) > maxPluginDepth {
			return fs.SkipDir
		}

		if _, ok := manifestPath(path); ok && path != root {
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return fs.SkipDir
			}
			found = append(found, filepath.ToSlash(relative))
			// Whatever a skill ships beside its manifest belongs to it, even
			// when that is another folder with a manifest of its own.
			return fs.SkipDir
		}
		return nil
	})
	sort.Strings(found)
	return found
}
