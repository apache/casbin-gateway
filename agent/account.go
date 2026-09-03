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

package agent

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Account is the cloud account an agent is signed in to, read from the state
// the agent keeps on disk. Fields are best-effort: an agent may record only an
// email, or none of these.
type Account struct {
	Email string `json:"email,omitempty"`
	Name  string `json:"name,omitempty"`
	Plan  string `json:"plan,omitempty"`
}

// maxAccountFileSize caps reads of the state files accounts are read from.
const maxAccountFileSize = 8 * 1024 * 1024

// fillAccounts attaches the signed-in account to each installation whose agent
// keeps one, reading the owner's home directory. It is read per agent id and
// home, so installations that share both share one read.
func fillAccounts(installations []Installation, homes []homeDir) {
	homeByOwner := make(map[string]string, len(homes))
	for _, home := range homes {
		if _, ok := homeByOwner[home.owner]; !ok {
			homeByOwner[home.owner] = home.path
		}
	}

	cache := map[string]*Account{}
	for i := range installations {
		kind := accountKind(installations[i].AgentId)
		if kind == "" {
			continue
		}
		home := homeByOwner[installations[i].Owner]
		if home == "" {
			continue
		}
		key := kind + "\x00" + home
		account, ok := cache[key]
		if !ok {
			account = readAccount(kind, home)
			cache[key] = account
		}
		installations[i].Account = account
	}
}

// accountKind maps an agent id to the reader that knows where its account is,
// empty for an agent Gateway cannot read an account from.
func accountKind(agentId string) string {
	switch {
	case agentId == "claude-code":
		return "claude-code"
	case agentId == "claude-desktop":
		return "claude-desktop"
	case strings.HasPrefix(agentId, "codex"):
		return "codex"
	default:
		return ""
	}
}

func readAccount(kind, home string) *Account {
	switch kind {
	case "codex":
		return readCodexAccount(home)
	case "claude-code":
		return readClaudeCodeAccount(home)
	case "claude-desktop":
		return readClaudeDesktopAccount(home)
	default:
		return nil
	}
}

// readCodexAccount decodes the profile carried in the JWT that Codex and the
// ChatGPT desktop app store after a ChatGPT sign-in.
func readCodexAccount(home string) *Account {
	data := readCapped(filepath.Join(home, ".codex", "auth.json"))
	if data == nil {
		return nil
	}
	var auth struct {
		Tokens struct {
			AccessToken string `json:"access_token"`
			IDToken     string `json:"id_token"`
		} `json:"tokens"`
	}
	if json.Unmarshal(data, &auth) != nil {
		return nil
	}

	account := &Account{}
	// The id token carries email and name as standard claims, the access token
	// carries them nested in a profile object; either may be present.
	for _, token := range []string{auth.Tokens.IDToken, auth.Tokens.AccessToken} {
		claims := decodeJWTClaims(token)
		if claims == nil {
			continue
		}
		if profile, ok := claims["https://api.openai.com/profile"].(map[string]any); ok {
			fillString(&account.Email, profile["email"])
			fillString(&account.Name, profile["name"])
		}
		fillString(&account.Email, claims["email"])
		fillString(&account.Name, claims["name"])
		if authClaim, ok := claims["https://api.openai.com/auth"].(map[string]any); ok {
			fillString(&account.Plan, authClaim["chatgpt_plan_type"])
		}
	}
	if account.Email == "" && account.Name == "" {
		return nil
	}
	return account
}

// fillString sets target from value only when target is still empty and value
// is a non-empty string.
func fillString(target *string, value any) {
	if *target != "" {
		return
	}
	if text, ok := value.(string); ok && text != "" {
		*target = text
	}
}

// readClaudeCodeAccount reads the oauth account Claude Code records in its
// top-level config file.
func readClaudeCodeAccount(home string) *Account {
	data := readCapped(filepath.Join(home, ".claude.json"))
	if data == nil {
		return nil
	}
	var config struct {
		OauthAccount struct {
			EmailAddress     string `json:"emailAddress"`
			DisplayName      string `json:"displayName"`
			FullName         string `json:"fullName"`
			OrganizationType string `json:"organizationType"`
		} `json:"oauthAccount"`
	}
	if json.Unmarshal(data, &config) != nil {
		return nil
	}

	oauth := config.OauthAccount
	account := &Account{
		Email: oauth.EmailAddress,
		Name:  firstNonEmpty(oauth.DisplayName, oauth.FullName),
		Plan:  strings.TrimPrefix(oauth.OrganizationType, "claude_"),
	}
	if account.Email == "" && account.Name == "" {
		return nil
	}
	return account
}

// readClaudeDesktopAccount pulls the profile the desktop app leaves in its
// IndexedDB store. The store is a Chromium format with no stable reader while
// the app holds it open, so the fields are lifted out by their markers.
func readClaudeDesktopAccount(home string) *Account {
	dir := claudeDesktopDataDir(home)
	if dir == "" {
		return nil
	}
	account := &Account{}
	scanClaudeIndexedDB(filepath.Join(dir, "IndexedDB"), account)
	if account.Email == "" && account.Name == "" {
		return nil
	}
	return account
}

// scanClaudeIndexedDB walks the IndexedDB tree looking for the account profile,
// stopping at the first file that carries both an email and a name.
func scanClaudeIndexedDB(root string, account *Account) {
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || info.Size() > maxAccountFileSize {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		if account.Email == "" {
			account.Email = idbMarkedString(data, "email_address")
		}
		if account.Name == "" {
			account.Name = idbMarkedString(data, "full_name")
		}
		if account.Email != "" && account.Name != "" {
			return filepath.SkipAll
		}
		return nil
	})
}

// idbMarkedString reads the length-prefixed string stored after a field name in
// Chromium's IndexedDB value blobs: the field name, a 0x22 tag, a single length
// byte, then that many bytes of the value.
func idbMarkedString(data []byte, field string) string {
	marker := bytes.Index(data, []byte(field))
	if marker < 0 {
		return ""
	}
	p := marker + len(field)
	tag := bytes.IndexByte(data[p:min(len(data), p+3)], '"')
	if tag < 0 {
		return ""
	}
	p += tag + 1
	if p >= len(data) {
		return ""
	}
	n := int(data[p])
	p++
	if n <= 0 || n > 128 || p+n > len(data) {
		return ""
	}
	value := data[p : p+n]
	for _, b := range value {
		if b < 0x20 || b >= 0x7f {
			return ""
		}
	}
	return string(value)
}

// decodeJWTClaims returns the claim set of a JWT without verifying it: the
// account fields are read from local state the user already trusts.
func decodeJWTClaims(token string) map[string]any {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return nil
	}
	var claims map[string]any
	if json.Unmarshal(payload, &claims) != nil {
		return nil
	}
	return claims
}

func readCapped(path string) []byte {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxAccountFileSize {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return data
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
