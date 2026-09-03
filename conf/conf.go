// Copyright 2023 The casbin Authors. All Rights Reserved.
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

package conf

import (
	_ "embed"
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/beego/beego"
)

func init() {
	ApplyEnvOverrides()
}

// ApplyEnvOverrides copies the beego configuration items that may be modified
// via env into beego's config. It runs at init, and again whenever the config
// is reloaded afterwards — beego.LoadAppConfig() builds a new config from the
// file alone, so anything set here would otherwise be lost.
func ApplyEnvOverrides() {
	presetConfigItems := []string{"httpport", "httpaddr", "appname"}
	for _, key := range presetConfigItems {
		if value, ok := os.LookupEnv(key); ok {
			err := beego.AppConfig.Set(key, value)
			if err != nil {
				panic(err)
			}
		}
	}
}

var (
	settingOverrides = map[string]string{}
	settingMutex     sync.RWMutex
)

// SetSettingOverrides hands over the settings held in the database. The
// built-in Setting row is the source of truth for everything the web UI can
// change, so a key it carries wins over conf/app.conf even when it is empty:
// the row was seeded from the file, and clearing a value in the UI has to
// clear it for good rather than fall back to what the file still says.
func SetSettingOverrides(overrides map[string]string) {
	settingMutex.Lock()
	defer settingMutex.Unlock()

	settingOverrides = overrides
}

func getSettingOverride(key string) (string, bool) {
	settingMutex.RLock()
	defer settingMutex.RUnlock()

	value, ok := settingOverrides[key]
	return value, ok
}

func GetConfigString(key string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}

	if value, ok := getSettingOverride(key); ok {
		return value
	}

	res := beego.AppConfig.String(key)
	if res == "" {
		if key == "staticBaseUrl" {
			res = "https://cdn.casbin.org"
		} else if key == "logConfig" {
			res = fmt.Sprintf("{\"filename\": \"logs/%s.log\", \"maxdays\":99999, \"perm\":\"0770\"}", beego.AppConfig.String("appname"))
		}
	}

	return res
}

// GetConfigStringUnquoted drops the quotes conf/app.conf writes around a value.
// Settings stored in the database never carry them.
func GetConfigStringUnquoted(key string) string {
	return strings.Trim(GetConfigString(key), `"' `)
}

func GetConfigBool(key string) bool {
	value := GetConfigString(key)
	if value == "true" {
		return true
	} else {
		return false
	}
}

func GetConfigInt(key string) int {
	value := GetConfigString(key)
	num, err := strconv.Atoi(value)
	if err != nil {
		panic(err)
	}
	return num
}

// GetConfigIntDefault reads an int setting, falling back to defaultValue when
// the key is missing or unparsable. Unlike GetConfigInt it never panics, so it
// suits settings that have a sensible built-in value.
func GetConfigIntDefault(key string, defaultValue int) int {
	value := strings.Trim(GetConfigString(key), `"' `)
	num, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return num
}

// GetHttpPort is the port serving the management UI and the REST API.
func GetHttpPort() int {
	return GetConfigIntDefault("httpport", 17000)
}

// GetHttpAddr is the interface the management UI and the REST API bind to. It
// defaults to loopback: the UI signs the local admin in without a password and
// the relay hands out the provider keys, so reaching those from the network is
// opt-in rather than the default.
func GetHttpAddr() string {
	addr := strings.Trim(GetConfigString("httpaddr"), `"' `)
	if addr == "" {
		return "127.0.0.1"
	}
	return addr
}

// IsHttpAddrLoopback reports whether the management port is reachable only from
// this machine, which is what lets the local admin skip the sign-in form.
func IsHttpAddrLoopback() bool {
	addr := GetHttpAddr()
	if addr == "localhost" {
		return true
	}
	ip := net.ParseIP(addr)
	return ip != nil && ip.IsLoopback()
}

// GetAllowedHosts is the extra Host header values the management UI and the
// REST API answer to. This host's own addresses and names are always answered
// to; a name is only listed here when Gateway is reached through one that is
// not this machine's, such as a reverse proxy in front of it.
func GetAllowedHosts() []string {
	return getConfigList("allowedHosts")
}

// GetAllowedOrigins is the browser origins allowed to call the API from another
// site. The web UI is served by Gateway itself, so it needs none of them: this
// is for a page of your own that talks to the relay.
func GetAllowedOrigins() []string {
	return getConfigList("allowedOrigins")
}

// getConfigList reads a comma-separated setting.
func getConfigList(key string) []string {
	raw := strings.Trim(GetConfigString(key), `"' `)
	if raw == "" {
		return nil
	}

	items := []string{}
	for _, item := range strings.Split(raw, ",") {
		if trimmed := strings.Trim(item, `"' `); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

// GetRelayToken is the token an agent sends to the local relay. It is generated
// on first start and stored with the settings, so it survives restarts and can
// be rotated from the Settings page.
func GetRelayToken() string {
	return strings.Trim(GetConfigString("relayToken"), `"' `)
}

func GetConfigInt64(key string) (int64, error) {
	value := GetConfigString(key)
	num, err := strconv.ParseInt(value, 10, 64)
	return num, err
}

// DefaultSqliteDataSourceName is used when "dataSourceName" is empty.
const DefaultSqliteDataSourceName = "./data/casbin-gateway.db"

func GetConfigDriverName() string {
	driverName := strings.Trim(GetConfigString("driverName"), `"' `)
	if driverName == "" {
		return "sqlite"
	}

	return driverName
}

// IsSqliteDriver accepts both spellings: modernc.org/sqlite registers itself as
// "sqlite" while XORM calls its dialect "sqlite3".
func IsSqliteDriver(driverName string) bool {
	return driverName == "sqlite" || driverName == "sqlite3"
}

func GetConfigDataSourceName() string {
	dataSourceName := GetConfigString("dataSourceName")

	// A SQLite data source is a file path, so the Docker host rewrite below
	// does not apply to it.
	if IsSqliteDriver(GetConfigDriverName()) {
		dataSourceName = strings.Trim(dataSourceName, `"' `)
		if dataSourceName == "" {
			return DefaultSqliteDataSourceName
		}

		return dataSourceName
	}

	runningInDocker := os.Getenv("RUNNING_IN_DOCKER")
	if runningInDocker == "true" {
		// https://stackoverflow.com/questions/48546124/what-is-linux-equivalent-of-host-docker-internal
		if runtime.GOOS == "linux" {
			dataSourceName = strings.ReplaceAll(dataSourceName, "localhost", "172.17.0.1")
		} else {
			dataSourceName = strings.ReplaceAll(dataSourceName, "localhost", "host.docker.internal")
		}
	}

	return dataSourceName
}

func GetLanguage(language string) string {
	if language == "" || language == "*" {
		return "en"
	}

	if len(language) != 2 || language == "nu" {
		return "en"
	} else {
		return language
	}
}

func IsDemoMode() bool {
	return strings.ToLower(GetConfigString("isDemoMode")) == "true"
}

func GetConfigBatchSize() int {
	res, err := strconv.Atoi(GetConfigString("batchSize"))
	if err != nil {
		res = 100
	}
	return res
}

// GetAgentPatchStateDir is the local directory that holds agent patch
// manifests, backups, and monitor cursors. It is operational state only; agent
// behaviour records remain in memory.
func GetAgentPatchStateDir() string {
	dir := GetConfigString("agentPatchStateDir")
	if dir == "" {
		return "./data/agent-patches"
	}
	return dir
}

// GetAgentRecordCapacity is how many agent monitoring records Gateway keeps in
// memory. Records are never written to disk, so this value alone bounds the
// memory the live window can occupy.
func GetAgentRecordCapacity() int {
	res, err := strconv.Atoi(GetConfigString("agentRecordCapacity"))
	if err != nil || res <= 0 {
		res = 1000
	}
	return res
}

// GetAgentMonitorPollSeconds is how often Gateway rescans the append-only logs
// of agents that are monitored by tailing files. Larger values trade detection
// latency for far less disk traffic on hosts with a long agent history.
func GetAgentMonitorPollSeconds() int {
	res, err := strconv.Atoi(GetConfigString("agentMonitorPollSeconds"))
	if err != nil || res <= 0 {
		res = 5
	}
	return res
}

// GetModelsDevSyncIntervalHours is how often an automatic models.dev sync runs.
// The catalogue only changes when a vendor publishes a price, so once a day is
// already more often than it has anything new to say.
func GetModelsDevSyncIntervalHours() int {
	res, err := strconv.Atoi(GetConfigString("modelsDevSyncIntervalHours"))
	if err != nil || res <= 0 {
		res = 24
	}
	return res
}

// How much of an LLM request Gateway retains.
const (
	LlmRecordOff      = "off"
	LlmRecordMetadata = "metadata"
	LlmRecordFull     = "full"
)

// GetLlmRecordMode reports whether LLM requests are recorded, and whether the
// request body is kept along with the metadata. Recording is on by default, so
// anything but an explicit "off" or "metadata" keeps the whole request.
func GetLlmRecordMode() string {
	switch strings.ToLower(strings.Trim(GetConfigString("llmRecordMode"), `"' `)) {
	case LlmRecordOff:
		return LlmRecordOff
	case LlmRecordMetadata:
		return LlmRecordMetadata
	default:
		return LlmRecordFull
	}
}

// GetLlmRecordQueueCapacity bounds the records waiting to be written. A full
// queue drops records rather than slowing the proxy down.
func GetLlmRecordQueueCapacity() int {
	res := GetConfigIntDefault("llmRecordQueueCapacity", 1000)
	if res <= 0 {
		res = 1000
	}
	return res
}

// GetLlmRecordRetentionDays is how long a recorded request is kept.
func GetLlmRecordRetentionDays() int {
	res := GetConfigIntDefault("llmRecordRetentionDays", 30)
	if res <= 0 {
		res = 30
	}
	return res
}

// GetLlmRecordMaxPayloadBytes bounds one retained request body. A coding agent
// sends its whole system prompt, tool schemas and conversation every turn, so
// the limit is far above what a chat message needs.
func GetLlmRecordMaxPayloadBytes() int {
	res := GetConfigIntDefault("llmRecordMaxPayloadBytes", 1024*1024)
	if res < 64*1024 {
		res = 64 * 1024
	}
	if res > 32*1024*1024 {
		res = 32 * 1024 * 1024
	}
	return res
}

// GetLlmRecordMaxRecords caps the table regardless of the retention window.
func GetLlmRecordMaxRecords() int {
	res := GetConfigIntDefault("llmRecordMaxRecords", 10000)
	if res <= 0 {
		res = 10000
	}
	return res
}

// GetLlmPricingFile is an optional JSON file of token prices overriding the
// built-in table.
func GetLlmPricingFile() string {
	return strings.Trim(GetConfigString("llmPricingFile"), `"' `)
}

func GetConfigRealDataSourceName(driverName string) string {
	var dataSourceName string
	if driverName != "mysql" {
		dataSourceName = GetConfigDataSourceName()
	} else {
		dataSourceName = GetConfigDataSourceName() + GetConfigString("dbName")
	}
	return dataSourceName
}
