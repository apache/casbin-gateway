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

// Package embedsupport wires up the assets that are baked into the binary when
// it is built with -tags embed: conf/app.conf, the compiled web UI in
// web/build, and the IP location database. Those are every file Gateway needs
// to find on disk at startup, so embedding them turns it into a single
// executable that runs with nothing next to it.
//
// The embedded copies are a fallback, never an override: whenever the matching
// file exists on disk it wins, so an operator can drop a conf/app.conf beside
// the binary, or keep developing against a live web/build, without rebuilding.
//
// A build without -tags embed never calls Setup, so both accessors report
// "nothing embedded" and every caller keeps its original on-disk behaviour.
package embedsupport

import "io/fs"

var (
	webFS fs.FS
	ipDb  []byte
)

// Setup must be called before any config value is read or any request is
// served. appConf is the contents of the embedded conf/app.conf, web is the
// embedded web/build tree, and ipDb is the embedded IP location database; each
// of them may be empty or nil.
func Setup(appConf string, web fs.FS, ipDatabase []byte) {
	webFS = web
	ipDb = ipDatabase
	setupConf(appConf)
}

// WebFS returns the embedded web/build filesystem, or nil when the binary was
// built without embedded web assets.
func WebFS() fs.FS { return webFS }

// HasWeb reports whether a compiled web UI is embedded in this binary.
func HasWeb() bool { return webFS != nil }

// IpDb returns the embedded IP location database, or nil when this binary
// carries none.
func IpDb() []byte { return ipDb }
