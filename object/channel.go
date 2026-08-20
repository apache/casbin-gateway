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

package object

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/apache/casbin-gateway/conf"
	"github.com/apache/casbin-gateway/proxy"
	"github.com/apache/casbin-gateway/util"
	"github.com/xorm-io/core"
)

// ErrNoChannelAvailable is returned by GetChannelsByModel when no enabled
// channel matches the requested model name. It is a sentinel error so
// callers can distinguish "no match" (client error, HTTP 400) from
// database failures (server error, HTTP 502).
var ErrNoChannelAvailable = errors.New("no available channel")

// ApiKeyMask is what the API returns in place of a stored API key. Sending it
// back in an update means "keep the existing key"; sending anything else
// (including an empty string) overwrites the stored key.
const ApiKeyMask = "***"

// apiKeyEncryptionSecret is empty when encryption is off, which keeps keys
// stored as plaintext like before.
func apiKeyEncryptionSecret() string {
	return conf.GetConfigString("apiKeyEncryptionKey")
}

// apiKeyAad binds the ciphertext to its own row, so a value copied into another
// channel's api_key column no longer decrypts.
func apiKeyAad(channel *Channel) string {
	return channel.GetId()
}

// encryptApiKey needs channel.Owner and channel.Name to be set already.
func encryptApiKey(channel *Channel) error {
	encrypted, err := util.EncryptWithKey(apiKeyEncryptionSecret(), channel.ApiKey, apiKeyAad(channel))
	if err != nil {
		return err
	}
	channel.ApiKey = encrypted
	return nil
}

// decryptChannel restores the plaintext ApiKey on a channel just read from the
// database. A failure leaves the stored value in place rather than dropping the
// channel, but is logged: otherwise a changed key looks exactly like a healthy
// channel whose upstream answers 401.
func decryptChannel(channel *Channel) {
	if channel == nil {
		return
	}

	secret := apiKeyEncryptionSecret()
	stored := channel.ApiKey

	plain, err := util.DecryptWithKey(secret, stored, apiKeyAad(channel))
	if err != nil {
		fmt.Printf("decryptChannel(): channel [%s]: %v\n", channel.GetId(), err)
		return
	}
	channel.ApiKey = plain

	if util.NeedsReEncryption(secret, stored) {
		upgradeStoredApiKey(channel)
	}
}

// apiKeyUpgrades collapses concurrent upgrades of the same row: GetChannelsByModel()
// runs on every proxied request.
var apiKeyUpgrades sync.Map

// upgradeStoredApiKey rewrites a plaintext or older-format key in the current
// format. Only api_key is touched, so UpdatedTime keeps reflecting the last real
// edit. A failure is logged and ignored, and retried on the next read.
func upgradeStoredApiKey(channel *Channel) {
	id := channel.GetId()
	if _, busy := apiKeyUpgrades.LoadOrStore(id, struct{}{}); busy {
		return
	}
	defer apiKeyUpgrades.Delete(id)

	encrypted, err := util.EncryptWithKey(apiKeyEncryptionSecret(), channel.ApiKey, apiKeyAad(channel))
	if err != nil {
		fmt.Printf("upgradeStoredApiKey(): channel [%s]: %v\n", id, err)
		return
	}

	_, err = ormer.Engine.ID(core.PK{channel.Owner, channel.Name}).
		Cols("api_key").Update(&Channel{ApiKey: encrypted})
	if err != nil {
		fmt.Printf("upgradeStoredApiKey(): channel [%s]: %v\n", id, err)
	}
}

func decryptChannels(channels []*Channel) {
	for _, channel := range channels {
		decryptChannel(channel)
	}
}

const (
	maxChannelModels     = 200
	maxChannelModelChars = 100
	maxProviderChars     = 100
	maxMappedModelChars  = 255
)

var (
	channelTypes     = []string{"openai", "custom", "anthropic"}
	channelStatuses  = []string{"enabled", "disabled"}
	channelAuthTypes = []string{"bearer", "x-api-key"}
)

// IsChannelTypeSupported reports whether the gateway can talk to the channel's
// upstream.
func IsChannelTypeSupported(channel *Channel) bool {
	return containsString(channelTypes, channel.Type)
}

func containsString(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

// Channel is an upstream AI provider channel. (Milestone 1.1)
type Channel struct {
	Owner       string `xorm:"varchar(100) notnull pk" json:"owner"`
	Name        string `xorm:"varchar(100) notnull pk" json:"name"`
	CreatedTime string `xorm:"varchar(100)" json:"createdTime"`
	UpdatedTime string `xorm:"varchar(100)" json:"updatedTime"`

	DisplayName string `xorm:"varchar(100)" json:"displayName"`
	Type        string `xorm:"varchar(100)" json:"type"`
	Provider    string `xorm:"varchar(100)" json:"provider"`
	AuthType    string `xorm:"varchar(30)" json:"authType"`
	BaseUrl     string `xorm:"varchar(255)" json:"baseUrl"`
	// ApiKey holds base64 ciphertext, not the bare key, when
	// "apiKeyEncryptionKey" is set in app.conf, hence the wider column.
	ApiKey string `xorm:"varchar(1000)" json:"apiKey"`
	// Models is JSON-serialized by xorm, so it needs a text column rather than
	// a varchar: the serialized form is longer than the joined model names.
	Models       []string `xorm:"mediumtext" json:"models"`
	DefaultModel string   `xorm:"varchar(255)" json:"defaultModel"`
	HaikuModel   string   `xorm:"varchar(255)" json:"haikuModel"`
	SonnetModel  string   `xorm:"varchar(255)" json:"sonnetModel"`
	OpusModel    string   `xorm:"varchar(255)" json:"opusModel"`
	// TODO(1.2): Priority routing strategy will be defined in milestone 1.2.
	Priority int    `xorm:"int" json:"priority"`
	Status   string `xorm:"varchar(100)" json:"status"`
}

func (channel *Channel) GetId() string {
	return fmt.Sprintf("%s/%s", channel.Owner, channel.Name)
}

func GetChannels(owner string) ([]*Channel, error) {
	channels := []*Channel{}
	session := GetSession(owner, -1, -1, "", "", "", "")
	err := session.Find(&channels)
	decryptChannels(channels)
	return channels, err
}

func GetChannelCount(owner, field, value string) (int64, error) {
	session := GetSession(owner, -1, -1, field, value, "", "")
	return session.Count(&Channel{})
}

func GetPaginationChannels(owner string, offset, limit int, field, value, sortField, sortOrder string) ([]*Channel, error) {
	channels := []*Channel{}
	session := GetSession(owner, offset, limit, field, value, sortField, sortOrder)
	err := session.Find(&channels)
	decryptChannels(channels)
	return channels, err
}

func getChannel(owner, name string) (*Channel, error) {
	channel := &Channel{Owner: owner, Name: name}
	existed, err := ormer.Engine.Get(channel)
	if err != nil {
		return nil, err
	}
	if !existed {
		return nil, nil
	}
	decryptChannel(channel)
	return channel, nil
}

func GetChannel(id string) (*Channel, error) {
	owner, name := util.GetOwnerAndNameFromId(id)
	return getChannel(owner, name)
}

// GetMaskedChannel returns a copy of the channel with the API key replaced by
// ApiKeyMask, so the stored key never reaches the browser.
func GetMaskedChannel(channel *Channel) *Channel {
	if channel == nil {
		return nil
	}

	masked := *channel
	if masked.ApiKey != "" {
		masked.ApiKey = ApiKeyMask
	}
	return &masked
}

func GetMaskedChannels(channels []*Channel) []*Channel {
	maskedChannels := make([]*Channel, 0, len(channels))
	for _, channel := range channels {
		maskedChannels = append(maskedChannels, GetMaskedChannel(channel))
	}
	return maskedChannels
}

func validateChannel(channel *Channel) error {
	if channel.Type == "" {
		channel.Type = "openai"
	}
	if channel.Status == "" {
		channel.Status = "enabled"
	}
	if channel.AuthType == "" {
		channel.AuthType = "bearer"
	}
	if channel.Models == nil {
		channel.Models = []string{}
	}

	if !containsString(channelTypes, channel.Type) {
		return fmt.Errorf("invalid channel type: %s", channel.Type)
	}
	if !containsString(channelStatuses, channel.Status) {
		return fmt.Errorf("invalid channel status: %s", channel.Status)
	}
	if !containsString(channelAuthTypes, channel.AuthType) {
		return fmt.Errorf("invalid channel auth type: %s", channel.AuthType)
	}
	if len(channel.Provider) > maxProviderChars {
		return fmt.Errorf("provider is too long")
	}

	if channel.BaseUrl != "" {
		if err := validateBaseUrl(channel.BaseUrl); err != nil {
			return err
		}
	}

	if len(channel.Models) > maxChannelModels {
		return fmt.Errorf("too many models: %d, at most %d are allowed", len(channel.Models), maxChannelModels)
	}
	for _, model := range channel.Models {
		if strings.TrimSpace(model) == "" {
			return fmt.Errorf("model name cannot be empty")
		}
		if len(model) > maxChannelModelChars {
			return fmt.Errorf("model name is too long: %s", model)
		}
	}
	if channel.Type == "anthropic" {
		mappedModels := []*string{&channel.DefaultModel, &channel.HaikuModel, &channel.SonnetModel, &channel.OpusModel}
		for _, model := range mappedModels {
			*model = strings.TrimSpace(*model)
			if *model == "" {
				return fmt.Errorf("all Anthropic model mappings are required")
			}
			if len(*model) > maxMappedModelChars {
				return fmt.Errorf("Anthropic model mapping is too long: %s", *model)
			}
		}
	}

	return nil
}

func validateBaseUrl(baseUrl string) error {
	u, err := url.Parse(baseUrl)
	if err != nil {
		return fmt.Errorf("invalid base URL: %s", err.Error())
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("invalid base URL: only the http and https schemes are supported")
	}
	if u.Hostname() == "" {
		return fmt.Errorf("invalid base URL: the host is empty")
	}
	return nil
}

// BuildOpenAiUrl joins an OpenAI-compatible endpoint onto a channel base URL.
// The base URL may be bare, already carry the /v1 prefix or already end with
// the endpoint itself; none of those forms are doubled.
func BuildOpenAiUrl(baseUrl string, endpoint string) (string, error) {
	u, err := url.Parse(baseUrl)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %s", err.Error())
	}

	path := strings.TrimSuffix(strings.TrimRight(u.Path, "/"), endpoint)
	if !strings.HasSuffix(path, "/v1") {
		path += "/v1"
	}

	u.Path = path + endpoint
	u.RawPath = ""
	return u.String(), nil
}

// BuildAnthropicMessagesUrl resolves the native Messages endpoint and merges
// the client query into any query already present on the configured base URL.
func BuildAnthropicMessagesUrl(baseUrl, requestQuery string) (string, error) {
	u, err := url.Parse(baseUrl)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %s", err.Error())
	}

	path := strings.TrimRight(u.Path, "/")
	if !strings.HasSuffix(path, "/v1/messages") {
		if !strings.HasSuffix(path, "/v1") {
			path += "/v1"
		}
		path += "/messages"
	}
	u.Path = path
	u.RawPath = ""
	u.Fragment = ""

	query := u.Query()
	clientQuery, err := url.ParseQuery(requestQuery)
	if err != nil {
		return "", fmt.Errorf("invalid request query: %s", err.Error())
	}
	for name, values := range clientQuery {
		for _, value := range values {
			query.Add(name, value)
		}
	}
	u.RawQuery = query.Encode()
	return u.String(), nil
}

// BuildModelEndpointCandidates returns the same-origin model endpoints used by
// Anthropic-compatible providers, including the root fallback for compatibility
// prefixes such as /anthropic.
type providerModelEndpoint struct {
	Url      string
	AuthType string
	Models   []string
}

var providerModelEndpoints = map[string]providerModelEndpoint{
	"anthropic":       {Url: "https://api.anthropic.com/v1/models", AuthType: "x-api-key"},
	"deepseek":        {Url: "https://api.deepseek.com/models", AuthType: "bearer"},
	"kimi-for-coding": {Url: "https://api.kimi.com/coding/v1/models", AuthType: "bearer"},
	"minimax":         {Url: "https://api.minimaxi.com/anthropic/v1/models", AuthType: "x-api-key"},
	"openrouter":      {Url: "https://openrouter.ai/api/v1/models", AuthType: "bearer"},
	"longcat":         {Url: "https://api.longcat.chat/anthropic/v1/models", AuthType: "bearer"},
	// Zhipu documents the models available to Coding Plan users, but does not
	// publish a model-list API for its Anthropic-compatible endpoint.
	"zhipu": {AuthType: "x-api-key", Models: []string{"glm-4.7", "glm-5-turbo", "glm-5.3"}},
}

// ProviderModels returns the documented static model list for providers that
// do not expose a model-list API.
func ProviderModels(provider string) ([]string, bool) {
	endpoint, ok := providerModelEndpoints[provider]
	if !ok || len(endpoint.Models) == 0 {
		return nil, false
	}
	return append([]string(nil), endpoint.Models...), true
}

func BuildModelEndpointCandidates(baseUrl, provider string) ([]string, error) {
	base, err := url.Parse(baseUrl)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Hostname() == "" {
		return nil, fmt.Errorf("invalid base URL")
	}
	if endpoint, ok := providerModelEndpoints[provider]; ok {
		if endpoint.Url == "" {
			return nil, fmt.Errorf("the provider does not publish a model-list endpoint")
		}
		explicit, parseErr := url.Parse(endpoint.Url)
		if parseErr != nil || explicit.Scheme != base.Scheme || !strings.EqualFold(explicit.Host, base.Host) {
			return nil, fmt.Errorf("provider model endpoint must use the channel origin")
		}
		return []string{explicit.String()}, nil
	}
	base.Fragment = ""
	base.RawQuery = ""
	path := strings.TrimRight(base.Path, "/")
	path = strings.TrimSuffix(path, "/v1/messages")
	path = strings.TrimSuffix(path, "/messages")
	path = strings.TrimSuffix(path, "/v1")

	prefixes := []string{path}
	if strings.HasSuffix(path, "/anthropic") {
		prefixes = append(prefixes, strings.TrimSuffix(path, "/anthropic"))
	}
	seen := map[string]bool{}
	result := []string{}
	for _, prefix := range prefixes {
		for _, suffix := range []string{"/v1/models", "/models"} {
			candidate := *base
			candidate.Path = strings.TrimRight(prefix, "/") + suffix
			candidate.RawPath = ""
			text := candidate.String()
			if !seen[text] {
				seen[text] = true
				result = append(result, text)
			}
		}
	}
	return result, nil
}

// ModelEndpointAuthType returns the authentication required by a known
// provider's model-list endpoint. Unknown providers use the Channel setting.
func ModelEndpointAuthType(provider, channelAuthType string) string {
	if endpoint, ok := providerModelEndpoints[provider]; ok {
		return endpoint.AuthType
	}
	return channelAuthType
}

func AddChannel(channel *Channel) (bool, error) {
	if err := validateChannel(channel); err != nil {
		return false, err
	}

	now := util.GetCurrentTime()
	if channel.CreatedTime == "" {
		channel.CreatedTime = now
	}
	channel.UpdatedTime = now

	if err := encryptApiKey(channel); err != nil {
		return false, err
	}

	affected, err := ormer.Engine.Insert(channel)
	return affected != 0, err
}

func UpdateChannel(id string, channel *Channel) (bool, error) {
	owner, name := util.GetOwnerAndNameFromId(id)
	if stored, err := getChannel(owner, name); err != nil {
		return false, err
	} else if stored == nil {
		return false, nil
	}

	if err := validateChannel(channel); err != nil {
		return false, err
	}

	channel.Owner = owner
	channel.Name = name
	channel.UpdatedTime = util.GetCurrentTime()

	session := ormer.Engine.ID(core.PK{owner, name})
	// The browser only ever sees the mask, so getting it back means the user
	// did not touch the field. Any other value (including "") is written, which
	// is what makes clearing a key possible.
	if channel.ApiKey == ApiKeyMask {
		session = session.Omit("api_key")
	} else if err := encryptApiKey(channel); err != nil {
		return false, err
	}

	affected, err := session.AllCols().Update(channel)
	return affected != 0, err
}

func DeleteChannel(channel *Channel) (bool, error) {
	affected, err := ormer.Engine.ID(core.PK{channel.Owner, channel.Name}).Delete(&Channel{})
	return affected != 0, err
}

// TestChannelConnectivity performs a read-only probe against the channel's
// upstream. It returns whether the probe succeeded, the upstream HTTP status
// code (0 when no response was received) and a human-readable message.
func TestChannelConnectivity(channel *Channel) (bool, int, string) {
	stored, err := getChannel(channel.Owner, channel.Name)
	if err != nil {
		return false, 0, err.Error()
	}
	if stored == nil {
		return false, 0, "the channel does not exist"
	}

	if !IsChannelTypeSupported(stored) {
		return false, 0, fmt.Sprintf("the %s channel type is not supported", stored.Type)
	}

	if stored.BaseUrl == "" {
		return false, 0, "the base URL is empty"
	}
	if err = validateBaseUrl(stored.BaseUrl); err != nil {
		return false, 0, err.Error()
	}

	probeUrls := []string{}
	if stored.Type == "anthropic" {
		probeUrls, err = BuildModelEndpointCandidates(stored.BaseUrl, stored.Provider)
	} else {
		var probeUrl string
		probeUrl, err = BuildOpenAiUrl(stored.BaseUrl, "/models")
		probeUrls = []string{probeUrl}
	}
	if err != nil {
		return false, 0, err.Error()
	}

	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: proxy.Transport(),
		// Do not follow redirects, so the reported status is the one the
		// configured base URL actually returns.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	lastStatus, lastMessage := 0, "no model endpoint succeeded"
	modelAuthType := ModelEndpointAuthType(stored.Provider, stored.AuthType)
	for _, probeUrl := range probeUrls {
		req, requestErr := http.NewRequest(http.MethodGet, probeUrl, nil)
		if requestErr != nil {
			return false, 0, requestErr.Error()
		}
		if stored.ApiKey != "" {
			if modelAuthType == "x-api-key" {
				req.Header.Set("x-api-key", stored.ApiKey)
			} else {
				req.Header.Set("Authorization", "Bearer "+stored.ApiKey)
			}
		}
		req.Header.Set("anthropic-version", "2023-06-01")
		resp, requestErr := client.Do(req)
		if requestErr != nil {
			var urlErr *url.Error
			if errors.As(requestErr, &urlErr) {
				lastMessage = urlErr.Err.Error()
			} else {
				lastMessage = requestErr.Error()
			}
			continue
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		lastStatus, lastMessage = resp.StatusCode, resp.Status
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return true, resp.StatusCode, resp.Status
		}
	}
	return false, lastStatus, lastMessage
}

// GetChannelsByModel returns every enabled channel that supports the given
// model name, ordered by priority (ascending, so the lowest value comes first)
// so that the caller can fail over from one channel to the next. It queries all
// channels globally (no owner filter) because /v1/chat/completions is an
// unauthenticated public endpoint.
func GetChannelsByModel(model string) ([]*Channel, error) {
	return getChannelsByProtocolAndModel("openai", model)
}

// GetAnthropicChannelsByModel returns enabled native Anthropic channels for a
// public Claude Code alias, ordered for failover.
func GetAnthropicChannelsByModel(model string) ([]*Channel, error) {
	return getChannelsByProtocolAndModel("anthropic", model)
}

func getChannelsByProtocolAndModel(protocol, model string) ([]*Channel, error) {
	channels := []*Channel{}
	err := ormer.Engine.Where("status = ?", "enabled").Asc("priority").Find(&channels)
	if err != nil {
		return nil, fmt.Errorf("channel query failed: %w", err)
	}

	// The models are JSON-serialized into a single column, so the match cannot
	// be pushed down into the query.
	matchedChannels := []*Channel{}
	for _, channel := range channels {
		if protocol == "anthropic" {
			if channel.Type == "anthropic" && channel.AnthropicModel(model) != "" {
				matchedChannels = append(matchedChannels, channel)
			}
			continue
		}
		if channel.Type == "openai" || channel.Type == "custom" {
			for _, channelModel := range channel.Models {
				if channelModel == model {
					matchedChannels = append(matchedChannels, channel)
					break
				}
			}
		}
	}

	decryptChannels(matchedChannels)

	if len(matchedChannels) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrNoChannelAvailable, model)
	}
	return matchedChannels, nil
}

// AnthropicModel maps a stable Claude Code alias to this channel's upstream
// model. An empty result means the alias is not supported.
func (channel *Channel) AnthropicModel(alias string) string {
	switch alias {
	case "casbin-default":
		return channel.DefaultModel
	case "casbin-haiku":
		return channel.HaikuModel
	case "casbin-sonnet":
		return channel.SonnetModel
	case "casbin-opus":
		return channel.OpusModel
	default:
		return ""
	}
}
