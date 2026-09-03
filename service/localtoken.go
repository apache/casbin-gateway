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

package service

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/apache/casbin-gateway/util"
)

// LocalTokenHeader carries the credential a program on this machine presents in
// place of a session. The tray is one: it draws its menu from the same API the
// pages use, but it is not a browser and has no session to send.
const LocalTokenHeader = "X-Casbin-Gateway-Local-Token"

// LocalTokenPath is relative to the working directory, which is where Gateway
// keeps the rest of its state, so a tray started from the same directory finds
// it without being told.
const LocalTokenPath = "./tmp/local-token"

var localToken struct {
	sync.RWMutex
	value string
}

// IssueLocalToken writes a fresh token where only this account can read it. It
// is new on every start, so one left behind by a Gateway that is gone grants
// nothing.
func IssueLocalToken() error {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return err
	}
	token := base64.RawURLEncoding.EncodeToString(buffer)

	if err := os.MkdirAll(filepath.Dir(LocalTokenPath), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(LocalTokenPath, []byte(token), 0o600); err != nil {
		return err
	}
	// WriteFile only applies the mode to a file it creates, and this one
	// outlives the run that created it.
	if err := os.Chmod(LocalTokenPath, 0o600); err != nil {
		return err
	}

	localToken.Lock()
	localToken.value = token
	localToken.Unlock()
	return nil
}

// IsLocalRequest reports whether a request came from a program on this machine
// holding that token. Loopback alone would not do: a page in the operator's
// browser reaches loopback too, and cannot read the file.
func IsLocalRequest(r *http.Request) bool {
	if r == nil || !util.IsLoopbackRequest(r) {
		return false
	}

	localToken.RLock()
	expected := localToken.value
	localToken.RUnlock()
	if expected == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(r.Header.Get(LocalTokenHeader)), []byte(expected)) == 1
}
