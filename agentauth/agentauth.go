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

// Package agentauth saves and puts back the file an agent keeps its sign-in in,
// so one machine holds a subscription account and an API key at the same time
// and moves between them without signing in again.
//
// Codex keeps both in the same ~/.codex/auth.json and its own sign-in
// overwrites whichever was there, which is what makes the swap worth storing.
package agentauth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/apache/casbin-gateway/agent"
	"github.com/apache/casbin-gateway/agentmonitor"
)

// The two ways an agent is signed in: an account with a plan behind it, and a
// key billed by usage.
const (
	KindSubscription = "subscription"
	KindApiKey       = "apikey"
)

// maxCredentialSize caps a read of the credential file.
const maxCredentialSize = 1 << 20

// Credential is one sign-in, kept as the whole file the agent reads it from so
// that putting it back leaves the agent exactly as the sign-in left it.
type Credential struct {
	Kind  string `json:"kind"`
	Data  string `json:"-"`
	Email string `json:"email,omitempty"`
	Name  string `json:"name,omitempty"`
	Plan  string `json:"plan,omitempty"`
}

// Label is what one credential is called when it has not been named by hand.
func (credential Credential) Label() string {
	if credential.Email != "" {
		return credential.Email
	}
	if credential.Name != "" {
		return credential.Name
	}
	return credential.Kind
}

// Supports reports whether Gateway knows where this agent keeps its sign-in.
func Supports(agentId string) bool {
	return agentId == "codex" || agentId == "codex-cli"
}

// HomeOf is the directory holding the sign-in of one installation. The CLI and
// the ChatGPT desktop app share it, so a swap moves both.
func HomeOf(agentId string, path string, owner string) (string, error) {
	if !Supports(agentId) {
		return "", fmt.Errorf("gateway does not know where %s keeps its sign-in", agentId)
	}
	return agentmonitor.ResolveCodexHome(path, owner)
}

// Read is the sign-in in place, nil when the agent has none.
func Read(agentId string, home string) (*Credential, error) {
	data, err := os.ReadFile(filepath.Join(home, credentialFile))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) > maxCredentialSize {
		return nil, fmt.Errorf("%s is larger than a sign-in file should be", credentialFile)
	}
	return Of(agentId, data), nil
}

// Of reads one credential file, nil when it holds no sign-in at all.
func Of(agentId string, data []byte) *Credential {
	kind := kindOf(data)
	if kind == "" {
		return nil
	}
	credential := &Credential{Kind: kind, Data: string(data)}
	if account := agent.AccountOfCredential(agentId, data); account != nil {
		credential.Email = account.Email
		credential.Name = account.Name
		credential.Plan = account.Plan
	}
	return credential
}

// Write puts one saved sign-in back where the agent reads it, replacing
// whatever is there. The caller saves what it replaces first: this is the only
// copy of it.
func Write(home string, data string) error {
	if strings.TrimSpace(data) == "" {
		return errors.New("this account has no sign-in stored to put back")
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		return err
	}

	path := filepath.Join(home, credentialFile)
	temp := path + ".casbin-gateway.tmp"
	if err := os.WriteFile(temp, []byte(data), 0o600); err != nil {
		return err
	}
	if err := os.Rename(temp, path); err != nil {
		os.Remove(temp)
		return err
	}
	return nil
}

// ApiKeyCredential is the credential file Codex writes for a key rather than a
// sign-in, so a key is stored and swapped in like any other account.
func ApiKeyCredential(agentId string, key string) (Credential, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return Credential{}, errors.New("the API key is empty")
	}
	if !Supports(agentId) {
		return Credential{}, fmt.Errorf("gateway does not know where %s keeps its sign-in", agentId)
	}

	// tokens and last_refresh are written as null rather than left out: that is
	// the file Codex writes for a key, and it is what clears a sign-in that was
	// there before.
	data, err := json.MarshalIndent(codexAuth{ApiKey: key}, "", "  ")
	if err != nil {
		return Credential{}, err
	}
	return Credential{Kind: KindApiKey, Data: string(data)}, nil
}

// credentialFile is what Codex keeps its sign-in in, under its home.
const credentialFile = "auth.json"

type codexAuth struct {
	ApiKey      string `json:"OPENAI_API_KEY"`
	Tokens      any    `json:"tokens"`
	LastRefresh any    `json:"last_refresh"`
}

// kindOf tells a signed-in account from a key, empty for a file holding
// neither.
func kindOf(data []byte) string {
	var auth struct {
		ApiKey string `json:"OPENAI_API_KEY"`
		Tokens *struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"tokens"`
	}
	if json.Unmarshal(data, &auth) != nil {
		return ""
	}
	if auth.Tokens != nil && (auth.Tokens.AccessToken != "" || auth.Tokens.RefreshToken != "") {
		return KindSubscription
	}
	if auth.ApiKey != "" {
		return KindApiKey
	}
	return ""
}
