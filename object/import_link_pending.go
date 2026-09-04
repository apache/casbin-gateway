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

package object

import (
	"sync"
	"time"
)

// pendingLinkTtl is how long a link waits for the page to come and get it. A
// browser that has to start Gateway first takes a few seconds; anything longer
// than this is a link nobody opened a page for, and it is dropped rather than
// shown to whoever opens one next.
const pendingLinkTtl = 2 * time.Minute

// MaxImportLinkLength caps what is accepted as a link, which arrives from a
// browser and is only ever a URL.
const MaxImportLinkLength = 8192

// pendingLink is the one link the URL scheme handler has handed over and the
// page has not read yet. It lives in memory: a link is on its way to a page
// that is opening right now, and a Gateway that restarts in between has lost
// the browser window it was for anyway.
var pendingLink struct {
	sync.Mutex
	raw     string
	expires time.Time
}

// SetPendingImportLink records the link a page is about to be opened for,
// replacing one nobody came for.
func SetPendingImportLink(raw string) {
	pendingLink.Lock()
	defer pendingLink.Unlock()

	pendingLink.raw = raw
	pendingLink.expires = time.Now().Add(pendingLinkTtl)
}

// TakePendingImportLink returns the waiting link and forgets it, so a page that
// is reloaded does not import the same link twice.
func TakePendingImportLink() string {
	pendingLink.Lock()
	defer pendingLink.Unlock()

	raw := pendingLink.raw
	expired := time.Now().After(pendingLink.expires)
	pendingLink.raw = ""
	if expired {
		return ""
	}
	return raw
}
