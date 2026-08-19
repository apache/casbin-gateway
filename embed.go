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

//go:build embed

// This file is compiled only with -tags embed. It bakes conf/app.conf and the
// compiled web UI into the executable and hands them to embedsupport, so that
// the binary runs on its own with no files next to it. Files on disk still win
// over the embedded copies at runtime — see the embedsupport package.
//
// web/build must exist before building with this tag, or go:embed fails to
// compile: build the frontend first with
//
//	cd web && yarn install && yarn build
//
// Everything under web/build ends up inside the binary, which is why the Vite
// config turns source maps off: they are several times the size of the code
// they map.

package main

import (
	"embed"
	"io/fs"

	// The IANA time zone database, which util.GetCurrentTime() needs. A
	// self-contained binary cannot count on the host for it: Windows ships no
	// zoneinfo at all, and -trimpath drops the GOROOT copy Go falls back to,
	// so without this the first timestamp panics with "unknown time zone".
	_ "time/tzdata"

	"github.com/apache/casbin-gateway/embedsupport"
)

//go:embed conf/app.conf
var embeddedAppConf string

//go:embed web/build
var embeddedWeb embed.FS

//go:embed ip/17monipdb.dat
var embeddedIpDb []byte

func init() {
	webFS, err := fs.Sub(embeddedWeb, "web/build")
	if err != nil {
		panic(err)
	}

	embedsupport.Setup(embeddedAppConf, webFS, embeddedIpDb)
}
