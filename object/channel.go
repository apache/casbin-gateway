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
	"fmt"
	"strings"

	"github.com/casbin/caswaf/util"
	"github.com/xorm-io/core"
)

// Channel is an upstream LLM service configuration (e.g. OpenAI, DeepSeek, Anthropic).
// Type is a free-form string by convention: openai / anthropic / deepseek / custom.
type Channel struct {
	Owner       string `xorm:"varchar(100) notnull pk" json:"owner"`
	Name        string `xorm:"varchar(100) notnull pk" json:"name"`
	CreatedTime string `xorm:"varchar(100)" json:"createdTime"`
	UpdatedTime string `xorm:"varchar(100)" json:"updatedTime"`
	DisplayName string `xorm:"varchar(100)" json:"displayName"`

	Type      string   `xorm:"varchar(100)" json:"type"`
	BaseURL   string   `xorm:"varchar(200) 'base_url'" json:"baseUrl"`
	SecretKey string   `xorm:"varchar(500)" json:"secretKey"`
	Models    []string `xorm:"varchar(1000)" json:"models"`
	Status    string   `xorm:"varchar(100)" json:"status"`
}

func GetGlobalChannels() ([]*Channel, error) {
	channels := []*Channel{}
	err := ormer.Engine.Desc("created_time").Find(&channels)
	if err != nil {
		return nil, err
	}

	return channels, nil
}

func GetChannels(owner string) ([]*Channel, error) {
	channels := []*Channel{}
	err := ormer.Engine.Asc("created_time").Find(&channels, &Channel{Owner: owner})
	if err != nil {
		return nil, err
	}

	return channels, nil
}

func getChannel(owner string, name string) (*Channel, error) {
	channel := Channel{Owner: owner, Name: name}
	existed, err := ormer.Engine.Get(&channel)
	if err != nil {
		return nil, err
	}

	if existed {
		return &channel, nil
	}
	return nil, nil
}

func GetChannel(id string) (*Channel, error) {
	owner, name := util.GetOwnerAndNameFromId(id)
	channel, err := getChannel(owner, name)
	if err != nil {
		return nil, err
	}

	return channel, nil
}

func UpdateChannel(id string, channel *Channel) (bool, error) {
	owner, name := util.GetOwnerAndNameFromId(id)
	if c, err := getChannel(owner, name); err != nil {
		return false, err
	} else if c == nil {
		return false, nil
	}

	channel.UpdatedTime = util.GetCurrentTime()

	session := ormer.Engine.ID(core.PK{owner, name})
	if channel.SecretKey == "" || strings.Contains(channel.SecretKey, "****") {
		// The secret key was returned masked (or left empty) by the frontend:
		// keep the stored key untouched instead of overwriting it with the masked value.
		_, err := session.Omit("secret_key").AllCols().Update(channel)
		if err != nil {
			return false, err
		}
	} else {
		_, err := session.AllCols().Update(channel)
		if err != nil {
			return false, err
		}
	}

	return true, nil
}

func AddChannel(channel *Channel) (bool, error) {
	affected, err := ormer.Engine.Insert(channel)
	if err != nil {
		return false, err
	}

	return affected != 0, nil
}

func DeleteChannel(channel *Channel) (bool, error) {
	affected, err := ormer.Engine.ID(core.PK{channel.Owner, channel.Name}).Delete(&Channel{})
	if err != nil {
		return false, err
	}

	return affected != 0, nil
}

func (channel *Channel) GetId() string {
	return fmt.Sprintf("%s/%s", channel.Owner, channel.Name)
}

func maskSecretKey(secretKey string) string {
	if len(secretKey) <= 8 {
		return "****"
	}

	return fmt.Sprintf("%s****%s", secretKey[:4], secretKey[len(secretKey)-4:])
}

func GetMaskedChannel(channel *Channel) *Channel {
	if channel == nil {
		return nil
	}

	channel.SecretKey = maskSecretKey(channel.SecretKey)
	return channel
}

func GetMaskedChannels(channels []*Channel) []*Channel {
	for _, channel := range channels {
		channel = GetMaskedChannel(channel)
	}
	return channels
}

func GetChannelCount(owner, field, value string) (int64, error) {
	session := GetSession(owner, -1, -1, field, value, "", "")
	return session.Count(&Channel{})
}

func GetPaginationChannels(owner string, offset, limit int, field, value, sortField, sortOrder string) ([]*Channel, error) {
	channels := []*Channel{}
	session := GetSession(owner, offset, limit, field, value, sortField, sortOrder)
	err := session.Where("owner = ? or owner = ?", "admin", owner).Find(&channels)
	if err != nil {
		return channels, err
	}

	return channels, nil
}
