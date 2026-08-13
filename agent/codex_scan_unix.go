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

//go:build linux || darwin

package agent

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var systemBinDirs = []string{"/usr/bin", "/usr/local/bin"}

func scanCodexStandalone() []Installation {
	var result []Installation
	for _, dir := range systemBinDirs {
		executable := filepath.Join(dir, "codex")
		info, err := os.Stat(executable)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 ||
			!isCodexNativeBinary(executable) {
			continue
		}
		result = append(result, Installation{
			AgentId: "codex-cli", Name: "Codex CLI", Path: executable,
			InstallMethod: "standalone", Owner: fileOwner(executable),
		})
	}
	return result
}

func isCodexNativeBinary(path string) bool {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		normalized := strings.ToLower(filepath.ToSlash(resolved))
		if strings.Contains(normalized, "/node_modules/@openai/codex/") ||
			strings.Contains(normalized, "/caskroom/codex/") ||
			strings.Contains(normalized, "/caskroom/codex@latest/") {
			return false
		}
	}
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	magic := make([]byte, 4)
	if _, err := io.ReadFull(file, magic); err != nil {
		return false
	}
	if bytes.Equal(magic, []byte{0x7f, 'E', 'L', 'F'}) {
		return true
	}
	for _, macho := range [][]byte{
		{0xfe, 0xed, 0xfa, 0xcf},
		{0xcf, 0xfa, 0xed, 0xfe},
		{0xca, 0xfe, 0xba, 0xbe},
		{0xbe, 0xba, 0xfe, 0xca},
	} {
		if bytes.Equal(magic, macho) {
			return true
		}
	}
	return false
}

func scanCodexDarwinApps(homes []homeDir) []Installation {
	if runtime.GOOS != "darwin" {
		return nil
	}
	apps := []struct {
		bundle     string
		executable string
	}{
		{bundle: "ChatGPT.app", executable: "ChatGPT"},
		{bundle: "Codex.app", executable: "Codex"},
	}

	var result []Installation
	for _, app := range apps {
		systemExecutable := filepath.Join("/Applications", app.bundle, "Contents", "MacOS", app.executable)
		if info, err := os.Stat(systemExecutable); err == nil && info.Mode().IsRegular() {
			result = append(result, Installation{
				AgentId: "codex", Name: "ChatGPT Desktop (Codex)", Path: systemExecutable,
				InstallMethod: "app", Owner: fileOwner(systemExecutable),
			})
		}
		for _, home := range homes {
			executable := filepath.Join(home.path, "Applications", app.bundle, "Contents", "MacOS", app.executable)
			if info, err := os.Stat(executable); err != nil || !info.Mode().IsRegular() {
				continue
			}
			result = append(result, Installation{
				AgentId: "codex", Name: "ChatGPT Desktop (Codex)", Path: executable,
				InstallMethod: "app", Owner: home.owner,
			})
		}
	}
	return result
}

func expandSharedCodexInstallations(installations []Installation, homes []homeDir) []Installation {
	var result []Installation
	for _, installation := range installations {
		clean := filepath.Clean(installation.Path)
		if installation.AgentId == "codex-cli" && installation.InstallMethod == "native" &&
			!isCodexNativeBinary(clean) {
			continue
		}
		shared := installation.AgentId == "codex" && strings.HasPrefix(clean, "/Applications/") ||
			installation.AgentId == "codex-cli" &&
				(strings.HasPrefix(clean, "/usr/") ||
					strings.HasPrefix(clean, "/opt/") ||
					strings.HasPrefix(clean, "/home/linuxbrew/"))
		if !shared || len(homes) == 0 {
			result = append(result, installation)
			continue
		}
		for _, home := range homes {
			if home.path != "/root" && home.path != "/var/root" &&
				!strings.HasPrefix(home.path, "/home/") &&
				!strings.HasPrefix(home.path, "/Users/") {
				continue
			}
			expanded := installation
			expanded.Owner = home.owner
			result = append(result, expanded)
		}
	}
	return result
}
