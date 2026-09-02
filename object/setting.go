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
	"strconv"

	"github.com/apache/casbin-gateway/conf"
	"github.com/apache/casbin-gateway/util"
	"github.com/xorm-io/core"
)

// BuiltInSettingId is the one row this table holds. Settings are global, so
// there is nothing to add or delete: the row is created on first start from
// conf/app.conf and edited from the Settings page afterwards.
const BuiltInSettingId = "admin/setting-built-in"

type Setting struct {
	Owner       string `xorm:"varchar(100) notnull pk" json:"owner"`
	Name        string `xorm:"varchar(100) notnull pk" json:"name"`
	CreatedTime string `xorm:"varchar(100)" json:"createdTime"`
	DisplayName string `xorm:"varchar(100)" json:"displayName"`

	LlmRecordMode            string `xorm:"varchar(100)" json:"llmRecordMode"`
	LlmRecordQueueCapacity   int    `xorm:"int" json:"llmRecordQueueCapacity"`
	LlmRecordRetentionDays   int    `xorm:"int" json:"llmRecordRetentionDays"`
	LlmRecordMaxRecords      int    `xorm:"int" json:"llmRecordMaxRecords"`
	LlmRecordMaxPayloadBytes int    `xorm:"int" json:"llmRecordMaxPayloadBytes"`
	LlmPricingFile           string `xorm:"varchar(500)" json:"llmPricingFile"`

	// ProviderProbeMode is "auto", "manual" or "off". An automatic probe spends
	// a few cents of the account's own credit, so it is one setting away.
	ProviderProbeMode string `xorm:"varchar(20)" json:"providerProbeMode"`

	AgentPatchStateDir      string `xorm:"varchar(500)" json:"agentPatchStateDir"`
	AgentRecordCapacity     int    `xorm:"int" json:"agentRecordCapacity"`
	AgentMonitorPollSeconds int    `xorm:"int" json:"agentMonitorPollSeconds"`

	CasdoorEndpoint     string `xorm:"varchar(500)" json:"casdoorEndpoint"`
	ClientId            string `xorm:"varchar(100)" json:"clientId"`
	ClientSecret        string `xorm:"varchar(200)" json:"clientSecret"`
	CasdoorOrganization string `xorm:"varchar(100)" json:"casdoorOrganization"`
	CasdoorApplication  string `xorm:"varchar(100)" json:"casdoorApplication"`

	ApiKeyEncryptionKey string `xorm:"varchar(200)" json:"apiKeyEncryptionKey"`
	// RelayToken is what an agent sends to the local relay. It is generated on
	// first start; clearing it on the Settings page issues a new one.
	RelayToken string `xorm:"varchar(200)" json:"relayToken"`

	HttpProxy string `xorm:"varchar(200)" json:"httpProxy"`
}

// SyncSettingToConf makes the row the answer conf.GetConfigString() gives, so
// every existing caller of conf keeps working and none of them has to know that
// the value now comes from the database. Keys not listed here stay in
// conf/app.conf: the database connection and the management port are read
// before this row can be, and "isDemoMode" turns the API read-only, which would
// take the Settings page that turns it off down with it.
func SyncSettingToConf(setting *Setting) {
	conf.SetSettingOverrides(map[string]string{
		"llmRecordMode":            setting.LlmRecordMode,
		"llmRecordQueueCapacity":   strconv.Itoa(setting.LlmRecordQueueCapacity),
		"llmRecordRetentionDays":   strconv.Itoa(setting.LlmRecordRetentionDays),
		"llmRecordMaxRecords":      strconv.Itoa(setting.LlmRecordMaxRecords),
		"llmRecordMaxPayloadBytes": strconv.Itoa(setting.LlmRecordMaxPayloadBytes),
		"llmPricingFile":           setting.LlmPricingFile,

		"providerProbeMode": setting.ProviderProbeMode,

		"agentPatchStateDir":      setting.AgentPatchStateDir,
		"agentRecordCapacity":     strconv.Itoa(setting.AgentRecordCapacity),
		"agentMonitorPollSeconds": strconv.Itoa(setting.AgentMonitorPollSeconds),

		"casdoorEndpoint":     setting.CasdoorEndpoint,
		"clientId":            setting.ClientId,
		"clientSecret":        setting.ClientSecret,
		"casdoorOrganization": setting.CasdoorOrganization,
		"casdoorApplication":  setting.CasdoorApplication,

		"apiKeyEncryptionKey": setting.ApiKeyEncryptionKey,
		"relayToken":          setting.RelayToken,

		"httpProxy": setting.HttpProxy,
	})
}

// relayTokenLength is long enough that the token cannot be guessed and short
// enough to stay readable in a shell snippet.
const relayTokenLength = 32

func newRelayToken() string {
	return "cg-" + util.GenerateToken(relayTokenLength)
}

func getSetting(owner string, name string) (*Setting, error) {
	setting := Setting{Owner: owner, Name: name}
	existed, err := ormer.Engine.Get(&setting)
	if err != nil {
		return nil, err
	}

	if existed {
		return &setting, nil
	}
	return nil, nil
}

func GetSetting(id string) (*Setting, error) {
	owner, name := util.GetOwnerAndNameFromId(id)
	return getSetting(owner, name)
}

func GetBuiltInSetting() (*Setting, error) {
	return GetSetting(BuiltInSettingId)
}

func UpdateSetting(id string, setting *Setting) (bool, error) {
	owner, name := util.GetOwnerAndNameFromId(id)
	if s, err := getSetting(owner, name); err != nil {
		return false, err
	} else if s == nil {
		return false, nil
	}

	setting.Owner, setting.Name = owner, name
	// An empty token would leave the relay open, so clearing the field is taken
	// as "issue a new one" rather than "turn the check off".
	if setting.RelayToken == "" {
		setting.RelayToken = newRelayToken()
	}
	_, err := ormer.Engine.ID(core.PK{owner, name}).AllCols().Update(setting)
	if err != nil {
		return false, err
	}

	SyncSettingToConf(setting)
	return true, nil
}

// InitBuiltInSetting creates the row on first start, seeded from conf/app.conf
// so an existing installation keeps the settings it already had, and loads it
// into conf. It runs before anything else reads conf, because from here on the
// row is what conf answers with.
func InitBuiltInSetting() {
	setting, err := GetBuiltInSetting()
	if err != nil {
		panic(err)
	}

	if setting == nil {
		setting = newSettingFromConf()
		if _, err = ormer.Engine.Insert(setting); err != nil {
			panic(err)
		}
	}

	// An installation that predates the relay token has an empty column, so the
	// token is issued here rather than only when the row is created.
	if setting.RelayToken == "" {
		setting.RelayToken = newRelayToken()
		if _, err = ormer.Engine.ID(core.PK{setting.Owner, setting.Name}).Cols("relay_token").Update(setting); err != nil {
			panic(err)
		}
	}

	// Likewise for a column added after the row was written: an empty value
	// would show as an empty choice on the Settings page rather than as the
	// default the code already applies.
	if setting.ProviderProbeMode == "" {
		setting.ProviderProbeMode = GetProviderProbeMode()
		if _, err = ormer.Engine.ID(core.PK{setting.Owner, setting.Name}).Cols("provider_probe_mode").Update(setting); err != nil {
			panic(err)
		}
	}

	SyncSettingToConf(setting)
}

// newSettingFromConf reads the defaults through conf's own getters, so a key
// that conf/app.conf never mentioned is stored as the default the code would
// have used anyway rather than as an empty value.
func newSettingFromConf() *Setting {
	return &Setting{
		Owner:       "admin",
		Name:        "setting-built-in",
		CreatedTime: util.GetCurrentTime(),
		DisplayName: "Built-in Setting",

		LlmRecordMode:            conf.GetLlmRecordMode(),
		LlmRecordQueueCapacity:   conf.GetLlmRecordQueueCapacity(),
		LlmRecordRetentionDays:   conf.GetLlmRecordRetentionDays(),
		LlmRecordMaxRecords:      conf.GetLlmRecordMaxRecords(),
		LlmRecordMaxPayloadBytes: conf.GetLlmRecordMaxPayloadBytes(),
		LlmPricingFile:           conf.GetLlmPricingFile(),

		ProviderProbeMode: GetProviderProbeMode(),

		AgentPatchStateDir:      conf.GetAgentPatchStateDir(),
		AgentRecordCapacity:     conf.GetAgentRecordCapacity(),
		AgentMonitorPollSeconds: conf.GetAgentMonitorPollSeconds(),

		CasdoorEndpoint:     conf.GetConfigStringUnquoted("casdoorEndpoint"),
		ClientId:            conf.GetConfigStringUnquoted("clientId"),
		ClientSecret:        conf.GetConfigStringUnquoted("clientSecret"),
		CasdoorOrganization: conf.GetConfigStringUnquoted("casdoorOrganization"),
		CasdoorApplication:  conf.GetConfigStringUnquoted("casdoorApplication"),

		ApiKeyEncryptionKey: conf.GetConfigStringUnquoted("apiKeyEncryptionKey"),
		RelayToken:          newRelayToken(),

		HttpProxy: conf.GetConfigStringUnquoted("httpProxy"),
	}
}
