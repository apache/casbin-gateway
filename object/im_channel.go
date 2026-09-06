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
	"errors"
	"fmt"
	"sort"

	"github.com/apache/casbin-gateway/imbridge"
	"github.com/apache/casbin-gateway/util"
	"github.com/xorm-io/core"
)

// ImChannel is one chat platform Gateway listens on, and what a conversation
// there is bound to. The credential is held here rather than in a file, the same
// way a connection's is, and never leaves the process in the clear.
type ImChannel struct {
	Owner string `xorm:"varchar(100) notnull pk" json:"owner"`
	// Name is what the operator called this channel. One platform can be
	// listened to more than once - two bots, two projects - so the name rather
	// than the platform is the key.
	Name        string `xorm:"varchar(100) notnull pk" json:"name"`
	CreatedTime string `xorm:"varchar(100)" json:"createdTime"`
	UpdatedTime string `xorm:"varchar(100)" json:"updatedTime"`

	Platform string `xorm:"varchar(50)" json:"platform"`
	Enabled  bool   `json:"enabled"`
	// Token is the bot credential: Telegram's bot token, or the token a WeChat
	// account handed over when it scanned the code. Ciphertext at rest when
	// "apiKeyEncryptionKey" is set, and masked on the way out.
	Token string `xorm:"varchar(500)" json:"token"`

	// The installation a conversation on this channel talks to.
	AgentId   string `xorm:"varchar(100)" json:"agentId"`
	AgentPath string `xorm:"varchar(500)" json:"agentPath"`
	AgentUser string `xorm:"varchar(100)" json:"agentUser"`
	WorkDir   string `xorm:"varchar(500)" json:"workDir"`
	Model     string `xorm:"varchar(200)" json:"model"`

	// AllowedUsers are the platform ids that may drive the agent. Empty lets
	// anybody who finds the bot drive it.
	AllowedUsers []string `xorm:"mediumtext json" json:"allowedUsers"`
}

func (channel *ImChannel) GetId() string {
	return fmt.Sprintf("%s/%s", channel.Owner, channel.Name)
}

// imChannelAad binds the ciphertext to its own row, so a token copied into
// another channel no longer decrypts.
func imChannelAad(channel *ImChannel) string {
	return "im-channel/" + channel.GetId()
}

func (channel *ImChannel) decrypted() *ImChannel {
	copied := *channel
	if copied.Token == "" {
		return &copied
	}
	// A token that will not decrypt is left as it is, matching how a provider's
	// key behaves when the encryption key changed: the row is still listed, and
	// using it is what reports the problem.
	if plain, err := util.DecryptWithKey(apiKeyEncryptionSecret(), copied.Token, imChannelAad(&copied)); err == nil {
		copied.Token = plain
	}
	return &copied
}

// Masked is a copy safe to send to the browser.
func (channel *ImChannel) Masked() *ImChannel {
	masked := *channel
	if masked.Token != "" {
		masked.Token = ApiKeyMask
	}
	return &masked
}

func GetImChannels() ([]*ImChannel, error) {
	channels := []*ImChannel{}
	if err := ormer.Engine.Where("owner = ?", AgentOwner).Find(&channels); err != nil {
		return nil, err
	}

	sort.Slice(channels, func(i, j int) bool { return channels[i].Name < channels[j].Name })
	return channels, nil
}

func GetImChannel(name string) (*ImChannel, error) {
	channel := &ImChannel{Owner: AgentOwner, Name: name}
	existed, err := ormer.Engine.Get(channel)
	if err != nil {
		return nil, err
	}
	if !existed {
		return nil, nil
	}
	return channel, nil
}

// SaveImChannel writes one channel, inserting it the first time. A masked token
// means the browser is sending back what it was shown, so the stored one is
// kept rather than overwritten with the mask.
func SaveImChannel(channel *ImChannel) error {
	if channel.Name == "" {
		return errors.New("the channel needs a name")
	}
	if channel.Platform != imbridge.PlatformTelegram && channel.Platform != imbridge.PlatformWeixin {
		return fmt.Errorf("no chat platform named %q", channel.Platform)
	}

	channel.Owner = AgentOwner
	channel.UpdatedTime = util.GetCurrentTime()

	existing, err := GetImChannel(channel.Name)
	if err != nil {
		return err
	}
	if existing == nil {
		channel.CreatedTime = channel.UpdatedTime
	} else {
		channel.CreatedTime = existing.CreatedTime
		if channel.Token == "" || channel.Token == ApiKeyMask {
			channel.Token = existing.Token
		}
	}

	if channel.Token != "" && channel.Token != ApiKeyMask && !util.IsEncrypted(channel.Token) {
		encrypted, err := util.EncryptWithKey(apiKeyEncryptionSecret(), channel.Token, imChannelAad(channel))
		if err != nil {
			return err
		}
		channel.Token = encrypted
	}

	if existing == nil {
		_, err = ormer.Engine.Insert(channel)
	} else {
		_, err = ormer.Engine.ID(core.PK{channel.Owner, channel.Name}).AllCols().Update(channel)
	}
	if err != nil {
		return err
	}
	return ReloadImChannels()
}

func DeleteImChannel(name string) error {
	if _, err := ormer.Engine.ID(core.PK{AgentOwner, name}).Delete(&ImChannel{}); err != nil {
		return err
	}
	return ReloadImChannels()
}

// ReloadImChannels brings the listeners in line with what is stored. It is
// called on every change, so a channel switched on starts listening without a
// restart.
func ReloadImChannels() error {
	channels, err := GetImChannels()
	if err != nil {
		return err
	}

	wanted := []imbridge.Channel{}
	for _, channel := range channels {
		if !channel.Enabled {
			continue
		}
		plain := channel.decrypted()
		wanted = append(wanted, imbridge.Channel{
			Name:         plain.Name,
			Platform:     plain.Platform,
			Token:        plain.Token,
			AgentId:      plain.AgentId,
			AgentPath:    plain.AgentPath,
			AgentUser:    plain.AgentUser,
			WorkDir:      plain.WorkDir,
			Model:        plain.Model,
			AllowedUsers: plain.AllowedUsers,
		})
	}

	imbridge.Reload(wanted)
	return nil
}

// InitImChannels starts listening on the stored channels and tells the bridge
// how to forget a conversation ended from a chat.
func InitImChannels() {
	imbridge.SetSessionForgetter(func(id string) {
		DeleteAgentSession(id)
	})
	if err := ReloadImChannels(); err != nil {
		panic(err)
	}
}
