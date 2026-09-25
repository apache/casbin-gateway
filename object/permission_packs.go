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

// PermissionPack is a named set of guards. Agents store the name, so pack updates reach everyone.
type PermissionPack struct {
	Name   string   `json:"name"`
	Guards []string `json:"guards"`
}

// segmentStart is where a shell command can begin, so "echo rm -rf" is not a delete.
const segmentStart = `(?:^|[;&|(\x60\n]|\$\()\s*(?:sudo\s+|command\s+|exec\s+|xargs\s+)*`

func PermissionPacks() []PermissionPack {
	return []PermissionPack{
		{
			Name: "no-destructive-commands",
			Guards: []string{
				"command, " + segmentStart + `rm\s(?:[^;&|\n]*\s)?(?:-[a-zA-Z]*[rR][a-zA-Z]*|--recursive)(?:\s|$), deny`,
				"command, (?i)" + segmentStart + `(?:remove-item|rm|ri|del|erase|rd|rmdir)\s[^;&|\n]*-recurse\b, deny`,
				"command, (?i)" + segmentStart + `(?:rd|rmdir|del|erase)\s(?:[^;&|\n]*\s)?/s\b, deny`,
				"command, " + segmentStart + `find\s[^;&|\n]*\s-delete\b, deny`,
				"command, " + segmentStart + `git\s+reset\s(?:[^;&|\n]*\s)?--hard\b, deny`,
				"command, " + segmentStart + `git\s+clean\s(?:[^;&|\n]*\s)?-[a-zA-Z]*f, deny`,
				"command, " + segmentStart + `git\s+push\s(?:[^;&|\n]*\s)?(?:-f|--force|--force-with-lease|--mirror|--delete)(?:[=\s]|$), deny`,
				"command, " + segmentStart + `git\s+(?:checkout|restore)\s(?:[^;&|\n]*\s)?(?:--\s+)?\.(?:\s|$), deny`,
				"command, " + segmentStart + `(?:mkfs(?:\.\w+)?|fdisk|parted|wipefs|shred)\s, deny`,
				"command, " + segmentStart + `dd\s[^;&|\n]*\bof=/dev/, deny`,
				"command, " + `>\s*/dev/(?:sd|nvme|disk|hd)\w*, deny`,
				"command, " + segmentStart + `chmod\s+(?:-[a-zA-Z]*R[a-zA-Z]*\s+)?(?:0?777|a\+rwx)\s+/(?:\s|$), deny`,
				"command, (?i)" + segmentStart + `(?:format|format-volume|clear-disk)\s, deny`,
				"command, " + segmentStart + `(?:shutdown|reboot|halt|poweroff)(?:\s|$), deny`,
				"command, " + `:\(\)\s*\{\s*:\s*\|\s*:\s*&\s*\}\s*;\s*:, deny`,
			},
		},
		{
			Name: "protect-secrets",
			Guards: []string{
				// Env templates hold no secrets.
				"path, **/.env.example, allow",
				"path, **/.env.sample, allow",
				"path, **/.env.template, allow",
				"path, **/.env, deny",
				"path, **/.env.*, deny",
				"path, ~/.ssh/**, deny",
				"path, ~/.aws/**, deny",
				"path, ~/.azure/**, deny",
				"path, ~/.config/gcloud/**, deny",
				"path, ~/.gnupg/**, deny",
				"path, ~/.kube/**, deny",
				"path, ~/.docker/config.json, deny",
				"path, ~/.netrc, deny",
				"path, ~/.git-credentials, deny",
				"path, ~/.npmrc, deny",
				"path, ~/.pypirc, deny",
				"path, ~/.config/gh/**, deny",
				"path, ~/.claude/.credentials.json, deny",
				"path, ~/.codex/auth.json, deny",
				"path, ~/.gemini/oauth_creds.json, deny",
				"path, **/*.pem, deny",
				"path, **/*.key, deny",
				// The same files reached through the shell.
				`command, (?i)(?:[/\\]\.ssh[/\\]|\bid_(?:rsa|ed25519|ecdsa|dsa)\b|[/\\]\.aws[/\\]credentials|[/\\]\.gnupg[/\\]|\.git-credentials|[/\\]\.netrc\b|[/\\]\.kube[/\\]config|[/\\]\.codex[/\\]auth\.json|[/\\]\.claude[/\\]\.credentials\.json|oauth_creds\.json), deny`,
				`command, (?:^|[\s'"/\\=<])\.env(?:\.(?:local|dev|development|prod|production|staging|test))?(?:$|[\s'";|&>)]), deny`,
				`command, (?i)` + segmentStart + `(?:printenv|env|set|get-childitem\s+env:|gci\s+env:|dir\s+env:)\s*(?:$|[;&|>]), deny`,
			},
		},
		{
			Name: "egress-allowlist",
			Guards: []string{
				"host, localhost, allow",
				"host, 127.0.0.1, allow",
				"host, ::1, allow",
				"host, *.github.com, allow",
				"host, *.githubusercontent.com, allow",
				"host, *.gitlab.com, allow",
				"host, *.npmjs.org, allow",
				"host, *.npmjs.com, allow",
				"host, *.yarnpkg.com, allow",
				"host, pypi.org, allow",
				"host, *.pythonhosted.org, allow",
				"host, proxy.golang.org, allow",
				"host, sum.golang.org, allow",
				"host, *.go.dev, allow",
				"host, crates.io, allow",
				"host, *.crates.io, allow",
				"host, *.rust-lang.org, allow",
				"host, *.maven.org, allow",
				"host, *.docker.io, allow",
				"host, *.npmmirror.com, allow",
				"host, goproxy.cn, allow",
				"host, *.tuna.tsinghua.edu.cn, allow",
				"host, *, deny",
			},
		},
		{
			Name: "no-publishing",
			Guards: []string{
				"command, " + segmentStart + `git\s+push(?:\s|$), deny`,
				"command, " + segmentStart + `(?:npm|pnpm|yarn)\s+publish\b, deny`,
				"command, " + segmentStart + `cargo\s+publish\b, deny`,
				"command, " + segmentStart + `(?:twine\s+upload|poetry\s+publish|uv\s+publish|flit\s+publish)\b, deny`,
				"command, " + segmentStart + `(?:docker|podman)\s+push\b, deny`,
				"command, " + segmentStart + `gh\s+(?:release\s+(?:create|upload|edit|delete)|pr\s+(?:merge|create)|repo\s+(?:create|delete|edit)|gist\s+create)\b, deny`,
				"command, " + segmentStart + `(?:kubectl|helm)\s+(?:apply|delete|install|upgrade|uninstall|rollout)\b, deny`,
				"command, " + segmentStart + `terraform\s+(?:apply|destroy)\b, deny`,
			},
		},
	}
}

func PermissionPackNames() []string {
	names := []string{}
	for _, pack := range PermissionPacks() {
		names = append(names, pack.Name)
	}
	return names
}
