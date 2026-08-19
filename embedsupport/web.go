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

package embedsupport

import (
	"bytes"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
)

// ServeWeb answers a request for a frontend asset from the embedded web/build
// tree. urlPath is the request path, e.g. "/" or "/static/js/main.abc123.js".
// Anything the tree does not contain falls back to index.html, because the web
// UI is a single-page app whose routes ("/sites", "/rules", ...) exist only in
// the browser.
//
// Callers must check HasWeb() first.
func ServeWeb(w http.ResponseWriter, r *http.Request, urlPath string) {
	name := path.Clean("/" + urlPath)[1:]
	if name == "" || name == "." {
		name = "index.html"
	}

	data, err := fs.ReadFile(webFS, name)
	if err != nil {
		name = "index.html"
		data, err = fs.ReadFile(webFS, name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}

	if contentType := mime.TypeByExtension(strings.ToLower(path.Ext(name))); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}

	// The zero modtime keeps ServeContent from sending Last-Modified: every
	// asset that can be cached already carries a content hash in its name.
	http.ServeContent(w, r, path.Base(name), time.Time{}, bytes.NewReader(data))
}
