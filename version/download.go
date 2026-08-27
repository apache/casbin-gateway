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

package version

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// A single connection to the release CDN is throttled, and on some networks
// stalls outright, so the archive is fetched as byte ranges over several
// connections at once and a range that stops moving is retried on its own.
const (
	chunkSize     = 4 << 20
	chunkWorkers  = 8
	chunkAttempts = 5
	retryDelay    = time.Second

	// A transfer is judged by whether bytes keep arriving, not by how long the
	// whole thing takes: a slow but moving download is still working.
	stallTimeout   = 20 * time.Second
	connectTimeout = 15 * time.Second
	headerTimeout  = 30 * time.Second

	readBuffer = 128 << 10
)

// download writes the release archive to path, keeping Status in step so the
// web UI can draw a progress bar.
func download(release *Release, path string) error {
	client := newDownloadClient()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	total, ranged, err := probe(ctx, client, release)
	if err != nil {
		return err
	}
	if total <= 0 {
		total = release.AssetSize
	}

	statusLock.Lock()
	status.Total = total
	status.Downloaded = 0
	status.Percent = 0
	statusLock.Unlock()

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	if ranged && total > 0 {
		err = downloadRanges(ctx, client, release, file, total)
	} else {
		err = downloadWhole(ctx, client, release, file)
	}
	if err != nil {
		return err
	}

	if err := file.Close(); err != nil {
		return err
	}

	// A short file is a download that was cut off. Catching it here turns a
	// broken installation into a message the reader can retry from.
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if release.AssetSize > 0 && info.Size() != release.AssetSize {
		return fmt.Errorf("downloaded %d of %d bytes of %s", info.Size(), release.AssetSize, release.AssetName)
	}

	return nil
}

// newDownloadClient gives every range its own TCP connection. HTTP/2 would
// multiplex them all onto one, which is the connection being throttled.
func newDownloadClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           (&net.Dialer{Timeout: connectTimeout, KeepAlive: 30 * time.Second}).DialContext,
			TLSNextProto:          map[string]func(string, *tls.Conn) http.RoundTripper{},
			TLSHandshakeTimeout:   connectTimeout,
			ResponseHeaderTimeout: headerTimeout,
			MaxConnsPerHost:       chunkWorkers,
			MaxIdleConnsPerHost:   chunkWorkers,
			IdleConnTimeout:       30 * time.Second,
		},
	}
}

func newDownloadRequest(ctx context.Context, release *Release) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", release.AssetUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "casbin-gateway/"+Current().Version)

	return req, nil
}

// probe asks for the first byte, which answers both questions at once: how big
// the asset is, and whether ranges are served at all.
func probe(ctx context.Context, client *http.Client, release *Release) (int64, bool, error) {
	req, err := newDownloadRequest(ctx, release)
	if err != nil {
		return 0, false, err
	}
	req.Header.Set("Range", "bytes=0-0")

	resp, err := client.Do(req)
	if err != nil {
		return 0, false, err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1))

	switch resp.StatusCode {
	case http.StatusPartialContent:
		return rangeTotal(resp.Header.Get("Content-Range")), true, nil
	case http.StatusOK:
		return resp.ContentLength, false, nil
	default:
		return 0, false, fmt.Errorf("downloading %s answered HTTP %d", release.AssetName, resp.StatusCode)
	}
}

// rangeTotal reads the size out of a "bytes 0-0/12345" response header.
func rangeTotal(contentRange string) int64 {
	_, size, found := strings.Cut(contentRange, "/")
	if !found {
		return 0
	}

	var total int64
	if _, err := fmt.Sscanf(strings.TrimSpace(size), "%d", &total); err != nil {
		return 0
	}

	return total
}

// downloadRanges splits the asset into chunks and keeps chunkWorkers of them in
// flight. The first chunk that cannot be fetched stops the rest.
func downloadRanges(ctx context.Context, client *http.Client, release *Release, file *os.File, total int64) error {
	if err := file.Truncate(total); err != nil {
		return err
	}

	count := (total + chunkSize - 1) / chunkSize
	workers := int64(chunkWorkers)
	if workers > count {
		workers = count
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		next    atomic.Int64
		wait    sync.WaitGroup
		once    sync.Once
		failure error
	)

	for worker := int64(0); worker < workers; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()

			for {
				index := next.Add(1) - 1
				if index >= count || ctx.Err() != nil {
					return
				}

				start := index * chunkSize
				end := start + chunkSize - 1
				if end > total-1 {
					end = total - 1
				}

				if err := fetchRange(ctx, client, release, file, start, end); err != nil {
					once.Do(func() {
						failure = err
						cancel()
					})
					return
				}
			}
		}()
	}
	wait.Wait()

	return failure
}

// fetchRange retries one chunk on its own, so a connection that dies or stops
// moving costs that chunk rather than the whole download.
func fetchRange(ctx context.Context, client *http.Client, release *Release, file *os.File, start int64, end int64) error {
	var last error

	for attempt := 0; attempt < chunkAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(retryDelay):
			}
		}

		written, err := fetchRangeOnce(ctx, client, release, file, start, end)
		if err == nil {
			return nil
		}
		// What this attempt counted is about to be fetched again.
		addProgress(-written)

		if ctx.Err() != nil {
			return ctx.Err()
		}
		last = err
	}

	return fmt.Errorf("downloading bytes %d-%d of %s: %w", start, end, release.AssetName, last)
}

func fetchRangeOnce(ctx context.Context, client *http.Client, release *Release, file *os.File, start int64, end int64) (int64, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	req, err := newDownloadRequest(ctx, release)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		return 0, fmt.Errorf("answered HTTP %d", resp.StatusCode)
	}

	written, err := copyInto(file, resp.Body, start, cancel)
	if err != nil {
		return written, err
	}
	if wanted := end - start + 1; written != wanted {
		return written, fmt.Errorf("answered %d of %d bytes", written, wanted)
	}

	return written, nil
}

func downloadWhole(ctx context.Context, client *http.Client, release *Release, file *os.File) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	req, err := newDownloadRequest(ctx, release)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %s answered HTTP %d", release.AssetName, resp.StatusCode)
	}

	_, err = copyInto(file, resp.Body, 0, cancel)
	return err
}

// copyInto writes the body at its place in the file and gives up on a
// connection that has delivered nothing for stallTimeout.
func copyInto(file *os.File, body io.Reader, offset int64, cancel context.CancelFunc) (int64, error) {
	watchdog := time.AfterFunc(stallTimeout, cancel)
	defer watchdog.Stop()

	buffer := make([]byte, readBuffer)
	written := int64(0)

	for {
		read, err := body.Read(buffer)
		if read > 0 {
			watchdog.Reset(stallTimeout)

			if _, writeErr := file.WriteAt(buffer[:read], offset+written); writeErr != nil {
				return written, writeErr
			}
			written += int64(read)
			addProgress(int64(read))
		}
		if err == io.EOF {
			return written, nil
		}
		if err != nil {
			return written, err
		}
	}
}

// addProgress publishes the running byte count. A retried chunk hands back what
// it had already counted, so the bar never runs ahead of the file.
func addProgress(delta int64) {
	statusLock.Lock()
	defer statusLock.Unlock()

	status.Downloaded += delta
	if status.Downloaded < 0 {
		status.Downloaded = 0
	}
	if status.Total > 0 {
		status.Percent = int(status.Downloaded * 100 / status.Total)
		if status.Percent > 100 {
			status.Percent = 100
		}
	}
}

// extractBinary writes the one executable inside the archive to path. Only that
// entry is taken, by name, so nothing else in the archive can land on disk.
func extractBinary(archive string, path string) error {
	wanted := filepath.Base(path)

	if strings.HasSuffix(archive, ".zip") {
		return extractFromZip(archive, wanted, path)
	}

	return extractFromTarGz(archive, wanted, path)
}

func extractFromTarGz(archive string, wanted string, path string) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("%s is not a gzip archive: %w", filepath.Base(archive), err)
	}
	defer gzipReader.Close()

	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if header.Typeflag != tar.TypeReg || filepath.Base(header.Name) != wanted {
			continue
		}

		return writeBinary(path, reader)
	}

	return fmt.Errorf("%s holds no %s", filepath.Base(archive), wanted)
}

func extractFromZip(archive string, wanted string, path string) error {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return fmt.Errorf("%s is not a zip archive: %w", filepath.Base(archive), err)
	}
	defer reader.Close()

	for _, entry := range reader.File {
		if entry.FileInfo().IsDir() || filepath.Base(entry.Name) != wanted {
			continue
		}

		content, err := entry.Open()
		if err != nil {
			return err
		}
		defer content.Close()

		return writeBinary(path, content)
	}

	return fmt.Errorf("%s holds no %s", filepath.Base(archive), wanted)
}

func writeBinary(path string, content io.Reader) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	defer file.Close()

	written, err := io.Copy(file, io.LimitReader(content, maxBinarySize+1))
	if err != nil {
		return err
	}
	if written > maxBinarySize {
		return fmt.Errorf("the executable in the archive is larger than %d bytes", int64(maxBinarySize))
	}
	if written == 0 {
		return fmt.Errorf("the executable in the archive is empty")
	}

	return file.Close()
}
