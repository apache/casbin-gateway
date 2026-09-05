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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/apache/casbin-gateway/agentauth"
	"github.com/apache/casbin-gateway/util"
	"github.com/xorm-io/core"
)

// AgentAccount is one sign-in of an agent, kept aside so the agent can be moved
// between accounts. An agent overwrites its own credential file when it signs
// in, so a subscription account and an API key can only be held at once by
// storing both here.
//
// Credential is the whole file the agent reads its sign-in from. It carries a
// refresh token, so it is stored the way a provider key is.
type AgentAccount struct {
	Owner       string `xorm:"varchar(100) notnull pk" json:"owner"`
	Name        string `xorm:"varchar(300) notnull pk" json:"name"`
	CreatedTime string `xorm:"varchar(100)" json:"createdTime"`
	UpdatedTime string `xorm:"varchar(100)" json:"updatedTime"`

	AgentId string `xorm:"varchar(100) index" json:"agentId"`
	// Kind is a subscription account or an API key.
	Kind        string `xorm:"varchar(50)" json:"kind"`
	DisplayName string `xorm:"varchar(200)" json:"displayName"`
	Email       string `xorm:"varchar(200)" json:"email"`
	Plan        string `xorm:"varchar(100)" json:"plan"`
	// Fingerprint is what tells the account in place from the ones stored,
	// without holding the credential itself up to the page.
	Fingerprint string `xorm:"varchar(100) index" json:"fingerprint"`
	Credential  string `xorm:"mediumtext" json:"-"`
	// LastUsedTime is when this one was last written into the agent.
	LastUsedTime string `xorm:"varchar(100)" json:"lastUsedTime"`
}

func (account *AgentAccount) GetId() string {
	return fmt.Sprintf("%s/%s", account.Owner, account.Name)
}

// agentAccountAad binds the stored credential to its own row, so a value copied
// into another account no longer decrypts.
func agentAccountAad(account *AgentAccount) string {
	return account.GetId() + "/credential"
}

// AgentAccountFingerprint names one credential by its contents, which is how a
// sign-in found in the agent is recognised as one already stored.
func AgentAccountFingerprint(credential string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(credential)))
	return hex.EncodeToString(sum[:16])
}

// Matches reports whether this row holds the sign-in given: the same credential,
// or the same account after the agent renewed its tokens in place, which
// rewrites the file without signing anybody else in.
func (account *AgentAccount) Matches(credential agentauth.Credential) bool {
	if account.Fingerprint == AgentAccountFingerprint(credential.Data) {
		return true
	}
	// A key is its own account, so only a signed-in one is recognised again by
	// who it belongs to.
	return credential.Kind == agentauth.KindSubscription &&
		account.Kind == credential.Kind && account.Email != "" && account.Email == credential.Email
}

// GetAgentAccounts returns the accounts stored for one agent, or for every
// agent when agentId is empty, oldest first so the list does not reshuffle.
func GetAgentAccounts(agentId string) ([]*AgentAccount, error) {
	accounts := []*AgentAccount{}
	session := ormer.Engine.Where("owner = ?", AgentOwner)
	if agentId != "" {
		session = session.And("agent_id = ?", agentId)
	}
	if err := session.Asc("created_time").Find(&accounts); err != nil {
		return nil, err
	}
	return accounts, nil
}

// GetAgentAccount returns one stored account with its credential decrypted,
// which is what a swap writes back into the agent.
func GetAgentAccount(name string) (*AgentAccount, error) {
	account := &AgentAccount{Owner: AgentOwner, Name: name}
	existed, err := ormer.Engine.Get(account)
	if err != nil {
		return nil, err
	}
	if !existed {
		return nil, nil
	}

	credential, err := util.DecryptWithKey(apiKeyEncryptionSecret(), account.Credential, agentAccountAad(account))
	if err != nil {
		return nil, err
	}
	account.Credential = credential
	return account, nil
}

// SaveAgentAccount stores one sign-in, replacing the row holding the same
// credential. Signing the same account in again renews its tokens rather than
// adding a second entry for it.
func SaveAgentAccount(agentId string, credential agentauth.Credential, displayName string) (*AgentAccount, error) {
	if agentId == "" || credential.Data == "" {
		return nil, errors.New("the agent and the sign-in are required")
	}

	account := &AgentAccount{
		Owner:       AgentOwner,
		AgentId:     agentId,
		Kind:        credential.Kind,
		DisplayName: strings.TrimSpace(displayName),
		Email:       credential.Email,
		Plan:        credential.Plan,
		Fingerprint: AgentAccountFingerprint(credential.Data),
		Credential:  credential.Data,
		UpdatedTime: util.GetCurrentTime(),
	}
	if account.DisplayName == "" {
		account.DisplayName = credential.Label()
	}

	stored, err := findAgentAccount(agentId, credential)
	if err != nil {
		return nil, err
	}
	if stored != nil {
		account.Name = stored.Name
		account.CreatedTime = stored.CreatedTime
		account.LastUsedTime = stored.LastUsedTime
		if stored.DisplayName != "" && strings.TrimSpace(displayName) == "" {
			account.DisplayName = stored.DisplayName
		}
	} else {
		account.Name, err = freeAgentAccountName(agentId, credential)
		if err != nil {
			return nil, err
		}
		account.CreatedTime = account.UpdatedTime
	}

	// The row is written encrypted; the caller is handed back the plain one it
	// passed in, since that is what a swap has to write into the agent.
	row := *account
	if row.Credential, err = util.EncryptWithKey(apiKeyEncryptionSecret(), row.Credential, agentAccountAad(&row)); err != nil {
		return nil, err
	}
	if stored != nil {
		_, err = ormer.Engine.ID(core.PK{AgentOwner, row.Name}).AllCols().Update(&row)
	} else {
		_, err = ormer.Engine.Insert(&row)
	}
	if err != nil {
		return nil, err
	}
	return account, nil
}

// findAgentAccount is the stored row holding this sign-in already: the same
// credential, or the same account signed in again.
func findAgentAccount(agentId string, credential agentauth.Credential) (*AgentAccount, error) {
	accounts, err := GetAgentAccounts(agentId)
	if err != nil {
		return nil, err
	}
	for _, account := range accounts {
		if account.Matches(credential) {
			return account, nil
		}
	}
	return nil, nil
}

// DeleteAgentAccount forgets one stored sign-in. What the agent is using right
// now is left alone: this drops the copy, not the session.
func DeleteAgentAccount(name string) error {
	_, err := ormer.Engine.Delete(&AgentAccount{Owner: AgentOwner, Name: name})
	return err
}

// SetAgentAccountDisplayName renames one stored account in the lists.
func SetAgentAccountDisplayName(name string, displayName string) error {
	stored, err := GetAgentAccount(name)
	if err != nil {
		return err
	}
	if stored == nil {
		return fmt.Errorf("no agent account is stored under this name: %s", name)
	}

	// Cols() is what writes an empty label: xorm skips zero values otherwise.
	_, err = ormer.Engine.ID(core.PK{AgentOwner, name}).
		Cols("display_name", "updated_time").
		Update(&AgentAccount{DisplayName: displayName, UpdatedTime: util.GetCurrentTime()})
	return err
}

// TouchAgentAccount records that this account is the one now in the agent.
func TouchAgentAccount(name string) error {
	now := util.GetCurrentTime()
	_, err := ormer.Engine.ID(core.PK{AgentOwner, name}).
		Cols("last_used_time", "updated_time").
		Update(&AgentAccount{LastUsedTime: now, UpdatedTime: now})
	return err
}

// freeAgentAccountName is the stored name of a new account: the agent it
// belongs to and a slug of whoever it signs in, numbered when that is taken.
func freeAgentAccountName(agentId string, credential agentauth.Credential) (string, error) {
	base := agentId + "/" + agentAccountSlug(credential)
	for suffix := 0; suffix < 100; suffix++ {
		name := base
		if suffix > 0 {
			name = fmt.Sprintf("%s-%d", base, suffix+1)
		}
		stored := &AgentAccount{Owner: AgentOwner, Name: name}
		existed, err := ormer.Engine.Get(stored)
		if err != nil {
			return "", err
		}
		if !existed {
			return name, nil
		}
	}
	return "", errors.New("too many accounts are stored for this agent")
}

// agentAccountSlug is a label reduced to what is safe in a stored name.
func agentAccountSlug(credential agentauth.Credential) string {
	slug := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_', r == '.', r == '@':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		default:
			return '-'
		}
	}, credential.Label())
	slug = strings.Trim(slug, "-")
	if len(slug) > 64 {
		slug = slug[:64]
	}
	if slug == "" {
		return credential.Kind
	}
	return slug
}
