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
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// KindWebdav is a WebDAV share: Nextcloud, ownCloud, Synology and QNAP, and the
// storage most other NAS firmware offers.
const KindWebdav = "webdav"

// propfindBody asks for the one level below the collection, which is the whole
// of what a listing needs.
const propfindBody = `<?xml version="1.0" encoding="utf-8"?>` +
	`<d:propfind xmlns:d="DAV:"><d:prop>` +
	`<d:getcontentlength/><d:getlastmodified/><d:resourcetype/>` +
	`</d:prop></d:propfind>`

func init() {
	Register(&Kind{
		Name:        KindWebdav,
		DisplayName: "WebDAV",
		Description: "WebDAV description",
		Fields: []Field{
			{Name: "url", Label: "Server URL", Type: FieldText, Required: true, Placeholder: "https://dav.example.com/dav"},
			{Name: "path", Label: "Folder on the server", Type: FieldText, Placeholder: "casbin-gateway", Hint: "Folder on the server hint"},
			{Name: "username", Label: "Username", Type: FieldText},
			{Name: "password", Label: "Password", Type: FieldSecret, Hint: "WebDAV password hint"},
		},
		New: newWebdavTarget,
	})
}

type webdavTarget struct {
	// base always ends in a slash: it names a collection, and a WebDAV server
	// answers a collection asked for without one with a redirect.
	base     string
	username string
	password string
	client   *http.Client
}

func newWebdavTarget(config Config) (Target, error) {
	raw := config.Option("url")
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("this is not a WebDAV URL: %s", config.Option("url"))
	}

	// Assigned decoded: url.URL escapes the path itself when it prints, and a
	// folder escaped here as well would arrive with its percent signs escaped.
	parsed.Path = "/" + strings.Trim(path.Join(parsed.Path, cleanPath(config.Option("path"))), "/") + "/"
	parsed.RawQuery, parsed.Fragment = "", ""

	return &webdavTarget{
		base:     parsed.String(),
		username: config.Option("username"),
		password: config.Option("password"),
		client:   config.client(),
	}, nil
}

// cleanPath is a folder typed by hand, with the segments that would climb out
// of it dropped and a backslash taken to mean the same as a slash.
func cleanPath(raw string) string {
	segments := []string{}
	for _, segment := range strings.Split(strings.Trim(strings.ReplaceAll(raw, "\\", "/"), "/"), "/") {
		if segment != "" && segment != "." && segment != ".." {
			segments = append(segments, segment)
		}
	}
	return strings.Join(segments, "/")
}

func (target *webdavTarget) Describe() string {
	return target.base
}

func (target *webdavTarget) List(ctx context.Context) ([]File, error) {
	req, err := target.request(ctx, "PROPFIND", target.base, []byte(propfindBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Depth", "1")
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")

	body, status, err := target.send(req)
	if err != nil {
		return nil, err
	}
	// The folder is made by the first file written into it, so one that is not
	// there yet is an empty folder rather than a failure.
	if status == http.StatusNotFound {
		return []File{}, nil
	}
	if status != http.StatusMultiStatus && status != http.StatusOK {
		return nil, statusError(status, body)
	}

	multistatus := &davMultistatus{}
	if err := xml.Unmarshal(body, multistatus); err != nil {
		return nil, err
	}

	basePath := target.basePath()
	files := []File{}
	for _, response := range multistatus.Responses {
		name, isDir := response.file(basePath)
		if name == "" || isDir {
			continue
		}
		files = append(files, File{Name: name, Size: response.size(), ModifiedTime: response.modified()})
	}
	return files, nil
}

func (target *webdavTarget) Read(ctx context.Context, name string) ([]byte, error) {
	req, err := target.fileRequest(ctx, http.MethodGet, name, nil)
	if err != nil {
		return nil, err
	}

	body, status, err := target.send(req)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, statusError(status, body)
	}
	return body, nil
}

func (target *webdavTarget) Write(ctx context.Context, name string, data []byte) error {
	if err := target.makeCollections(ctx); err != nil {
		return err
	}

	req, err := target.fileRequest(ctx, http.MethodPut, name, data)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	body, status, err := target.send(req)
	if err != nil {
		return err
	}
	if status/100 != 2 {
		return statusError(status, body)
	}
	return nil
}

func (target *webdavTarget) Remove(ctx context.Context, name string) error {
	req, err := target.fileRequest(ctx, http.MethodDelete, name, nil)
	if err != nil {
		return err
	}

	body, status, err := target.send(req)
	if err != nil {
		return err
	}
	if status/100 != 2 && status != http.StatusNotFound {
		return statusError(status, body)
	}
	return nil
}

// makeCollections creates the folder, and every folder above it, the way
// WebDAV wants it: one MKCOL per level, most of which are already there.
func (target *webdavTarget) makeCollections(ctx context.Context) error {
	parsed, err := url.Parse(target.base)
	if err != nil {
		return err
	}

	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i := range segments {
		if segments[i] == "" {
			continue
		}

		collection := *parsed
		collection.Path = "/" + strings.Join(segments[:i+1], "/") + "/"
		req, err := target.request(ctx, "MKCOL", collection.String(), nil)
		if err != nil {
			return err
		}

		// 405 is the collection already being there, and 403 is a server that
		// will not have one made that high up. Neither stops the PUT below,
		// which reports whatever is really wrong.
		body, status, err := target.send(req)
		if err != nil {
			return err
		}
		if status/100 != 2 && status != http.StatusMethodNotAllowed && status != http.StatusForbidden && status != http.StatusConflict {
			return statusError(status, body)
		}
	}
	return nil
}

func (target *webdavTarget) basePath() string {
	parsed, err := url.Parse(target.base)
	if err != nil {
		return "/"
	}
	return parsed.Path
}

func (target *webdavTarget) fileRequest(ctx context.Context, method string, name string, data []byte) (*http.Request, error) {
	if name == "" || strings.ContainsAny(name, "/\\") {
		return nil, fmt.Errorf("this is not a file name: %s", name)
	}
	return target.request(ctx, method, target.base+url.PathEscape(name), data)
}

func (target *webdavTarget) request(ctx context.Context, method string, rawUrl string, data []byte) (*http.Request, error) {
	var body io.Reader
	if data != nil {
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, rawUrl, body)
	if err != nil {
		return nil, err
	}
	if target.username != "" || target.password != "" {
		req.SetBasicAuth(target.username, target.password)
	}
	return req, nil
}

func (target *webdavTarget) send(req *http.Request) ([]byte, int, error) {
	res, err := target.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, res.StatusCode, err
	}
	return body, res.StatusCode, nil
}

// statusError keeps enough of the answer to tell a wrong password from a wrong
// path, and no more: these bodies are HTML error pages as often as not.
func statusError(status int, body []byte) error {
	message := strings.TrimSpace(string(body))
	if len(message) > 200 {
		message = message[:200] + "..."
	}
	if message == "" {
		return fmt.Errorf("the server answered %d %s", status, http.StatusText(status))
	}
	return fmt.Errorf("the server answered %d: %s", status, message)
}

type davMultistatus struct {
	XMLName   xml.Name      `xml:"DAV: multistatus"`
	Responses []davResponse `xml:"DAV: response"`
}

type davResponse struct {
	Href      string        `xml:"DAV: href"`
	Propstats []davPropstat `xml:"DAV: propstat"`
}

type davPropstat struct {
	Prop davProp `xml:"DAV: prop"`
}

type davProp struct {
	ContentLength int64     `xml:"DAV: getcontentlength"`
	LastModified  string    `xml:"DAV: getlastmodified"`
	Collection    *struct{} `xml:"DAV: resourcetype>collection"`
}

// file is the name this response describes, and whether it is a folder. The
// collection the listing was asked for describes itself first, and is the one
// response with nothing to say.
func (response davResponse) file(basePath string) (string, bool) {
	href := response.Href
	if parsed, err := url.Parse(href); err == nil {
		href = parsed.Path
	}
	if strings.TrimSuffix(href, "/") == strings.TrimSuffix(basePath, "/") {
		return "", true
	}

	name := path.Base(strings.TrimSuffix(href, "/"))
	if unescaped, err := url.PathUnescape(name); err == nil {
		name = unescaped
	}

	for _, propstat := range response.Propstats {
		if propstat.Prop.Collection != nil {
			return name, true
		}
	}
	return name, strings.HasSuffix(href, "/")
}

func (response davResponse) size() int64 {
	for _, propstat := range response.Propstats {
		if propstat.Prop.ContentLength > 0 {
			return propstat.Prop.ContentLength
		}
	}
	return 0
}

func (response davResponse) modified() time.Time {
	for _, propstat := range response.Propstats {
		if propstat.Prop.LastModified == "" {
			continue
		}
		if at, err := http.ParseTime(propstat.Prop.LastModified); err == nil {
			return at
		}
	}
	return time.Time{}
}
