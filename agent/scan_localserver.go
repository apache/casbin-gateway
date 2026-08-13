// Copyright 2025 The casbin Authors. All Rights Reserved.
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

package agent

import (
	"context"
	"os"
	"strings"

	"github.com/apache/casbin-gateway/localserver"
)

const localServerInstallMethod = "source"

// scanLocalServers detects agents listening on configured loopback ports.
func scanLocalServers(ctx context.Context) []Installation {
	var installations []Installation
	for i := range fingerprints {
		fingerprint := &fingerprints[i]
		if ctx.Err() != nil {
			return installations
		}
		if fingerprint.LocalServer == nil {
			continue
		}
		mark := len(installations)
		for _, port := range fingerprint.LocalServer.Ports {
			installations = append(installations, scanLocalServerPort(ctx, fingerprint, port)...)
		}
		stampAgentId(installations, mark, fingerprint.ID)
		fillMissingVersions(installations, mark, fingerprint)
	}
	return installations
}

func scanLocalServerPort(ctx context.Context, fingerprint *Fingerprint, port int) []Installation {
	base, ok := fingerprint.LocalServer.Confirm(ctx, port)
	if !ok {
		return nil
	}
	version := sanitizeVersion(fingerprint.LocalServer.Version(ctx, base))

	var result []Installation
	seen := map[string]bool{}
	for _, process := range localserver.Listeners(ctx, port) {
		if ctx.Err() != nil {
			return result
		}
		key := strings.ToLower(process.Path)
		if process.Path == "" || seen[key] {
			continue
		}
		seen[key] = true
		if info, err := os.Stat(process.Path); err != nil || !info.Mode().IsRegular() {
			continue
		}
		result = append(result, Installation{
			Name: fingerprint.DisplayName, Version: version, Path: process.Path,
			InstallMethod: localServerInstallMethod, Owner: process.Owner,
		})
	}
	return result
}
