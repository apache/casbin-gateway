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

// Package webui says where this process looks for the compiled frontend. The
// router and the startup summary both need the answer, and they must agree on
// it.
package webui

import "github.com/apache/casbin-gateway/util"

// BuildDir is where the compiled web UI is expected, relative to the working
// directory.
const BuildDir = "web/build"

// GetBuildDir returns BuildDir when the UI has been built there, or "" when it
// has not. A build made with -tags embed carries its own copy, which is used
// only in that last case.
func GetBuildDir() string {
	if util.FileExist(BuildDir + "/index.html") {
		return BuildDir
	}

	return ""
}
