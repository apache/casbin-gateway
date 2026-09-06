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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/apache/casbin-gateway/conf"
	"github.com/apache/casbin-gateway/protocol"
	"github.com/apache/casbin-gateway/proxy"
	"github.com/apache/casbin-gateway/util"
	"github.com/xorm-io/core"
)

// ErrNoProviderAvailable is returned by GetProvidersByModel when no enabled
// provider matches the requested model name. It is a sentinel error so
// callers can distinguish "no match" (client error, HTTP 400) from
// database failures (server error, HTTP 502).
var ErrNoProviderAvailable = errors.New("no available provider")

// ApiKeyMask is what the API returns in place of a stored API key. Sending it
// back in an update means "keep the existing key"; sending anything else
// (including an empty string) overwrites the stored key.
const ApiKeyMask = "***"

// AnthropicVersion is the API version sent upstream when the client did not
// pick one. The Anthropic API rejects a request without it.
const AnthropicVersion = "2023-06-01"

// apiKeyEncryptionSecret is empty when encryption is off, which keeps keys
// stored as plaintext like before.
func apiKeyEncryptionSecret() string {
	return conf.GetConfigString("apiKeyEncryptionKey")
}

// apiKeyAad binds the ciphertext to its own row, so a value copied into another
// provider's api_key column no longer decrypts.
func apiKeyAad(provider *Provider) string {
	return provider.GetId()
}

// encryptApiKey needs provider.Owner and provider.Name to be set already.
func encryptApiKey(provider *Provider) error {
	encrypted, err := util.EncryptWithKey(apiKeyEncryptionSecret(), provider.ApiKey, apiKeyAad(provider))
	if err != nil {
		return err
	}
	provider.ApiKey = encrypted
	return nil
}

// subscriptionAad keeps a sign-in's ciphertext from being usable in the api_key
// column of the same row, or in any other row.
func subscriptionAad(provider *Provider) string {
	return provider.GetId() + "/subscription"
}

// encryptedSubscription is one sign-in as it is stored, empty for none.
func encryptedSubscription(provider *Provider, subscription *ProviderSubscription) (string, error) {
	if subscription == nil {
		return "", nil
	}

	plain, err := json.Marshal(subscription)
	if err != nil {
		return "", err
	}
	return util.EncryptWithKey(apiKeyEncryptionSecret(), string(plain), subscriptionAad(provider))
}

func encryptSubscription(provider *Provider) error {
	if provider.Subscription == "" {
		return nil
	}

	encrypted, err := util.EncryptWithKey(apiKeyEncryptionSecret(), provider.Subscription, subscriptionAad(provider))
	if err != nil {
		return err
	}
	provider.Subscription = encrypted
	return nil
}

func decryptSubscription(provider *Provider) {
	if provider.Subscription == "" {
		return
	}

	plain, err := util.DecryptWithKey(apiKeyEncryptionSecret(), provider.Subscription, subscriptionAad(provider))
	if err != nil {
		fmt.Printf("decryptSubscription(): provider [%s]: %v\n", provider.GetId(), err)
		return
	}
	provider.Subscription = plain
}

// quotaTokenAad keeps the quota token's ciphertext from being usable in the
// api_key column of the same row, or in any other row.
func quotaTokenAad(provider *Provider) string {
	return provider.GetId() + "/quota"
}

// encryptQuotaToken mirrors encryptApiKey for the credential a quota endpoint
// needs when the inference key is not the one it takes.
func encryptQuotaToken(provider *Provider) error {
	if provider.Quota == nil || provider.Quota.Token == "" {
		return nil
	}

	encrypted, err := util.EncryptWithKey(apiKeyEncryptionSecret(), provider.Quota.Token, quotaTokenAad(provider))
	if err != nil {
		return err
	}
	provider.Quota.Token = encrypted
	return nil
}

// stampQuotaSince fixes the point a manual balance counts spending from, the
// first time one is saved. A client that clears the field is asking to start
// over, so an empty Since is always refilled with now.
func stampQuotaSince(provider *Provider) {
	if provider.Quota != nil && provider.Quota.Manual && provider.Quota.Since == "" {
		provider.Quota.Since = util.GetCurrentTime()
	}
}

func decryptQuotaToken(provider *Provider) {
	if provider.Quota == nil || provider.Quota.Token == "" {
		return
	}

	plain, err := util.DecryptWithKey(apiKeyEncryptionSecret(), provider.Quota.Token, quotaTokenAad(provider))
	if err != nil {
		fmt.Printf("decryptQuotaToken(): provider [%s]: %v\n", provider.GetId(), err)
		return
	}
	provider.Quota.Token = plain
}

// decryptProvider restores the plaintext ApiKey on a provider just read from the
// database. A failure leaves the stored value in place rather than dropping the
// provider, but is logged: otherwise a changed key looks exactly like a healthy
// provider whose upstream answers 401.
func decryptProvider(provider *Provider) {
	if provider == nil {
		return
	}

	decryptQuotaToken(provider)
	decryptSubscription(provider)

	secret := apiKeyEncryptionSecret()
	stored := provider.ApiKey

	plain, err := util.DecryptWithKey(secret, stored, apiKeyAad(provider))
	if err != nil {
		fmt.Printf("decryptProvider(): provider [%s]: %v\n", provider.GetId(), err)
		return
	}
	provider.ApiKey = plain

	if util.NeedsReEncryption(secret, stored) {
		upgradeStoredApiKey(provider)
	}
}

// apiKeyUpgrades collapses concurrent upgrades of the same row: GetProvidersByModel()
// runs on every proxied request.
var apiKeyUpgrades sync.Map

// upgradeStoredApiKey rewrites a plaintext or older-format key in the current
// format. Only api_key is touched, so UpdatedTime keeps reflecting the last real
// edit. A failure is logged and ignored, and retried on the next read.
func upgradeStoredApiKey(provider *Provider) {
	id := provider.GetId()
	if _, busy := apiKeyUpgrades.LoadOrStore(id, struct{}{}); busy {
		return
	}
	defer apiKeyUpgrades.Delete(id)

	encrypted, err := util.EncryptWithKey(apiKeyEncryptionSecret(), provider.ApiKey, apiKeyAad(provider))
	if err != nil {
		fmt.Printf("upgradeStoredApiKey(): provider [%s]: %v\n", id, err)
		return
	}

	_, err = ormer.Engine.ID(core.PK{provider.Owner, provider.Name}).
		Cols("api_key").Update(&Provider{ApiKey: encrypted})
	if err != nil {
		fmt.Printf("upgradeStoredApiKey(): provider [%s]: %v\n", id, err)
	}
}

func decryptProviders(providers []*Provider) {
	for _, provider := range providers {
		decryptProvider(provider)
	}
}

const (
	maxProviderModels     = 200
	maxProviderModelChars = 100
)

var (
	providerTypes    = []string{"openai", "custom", "anthropic"}
	providerStatuses = []string{"enabled", "disabled"}
)

// The two ways a provider authenticates upstream. In ProviderAuthClient mode the
// gateway forwards the credentials the caller sent instead of a stored key, so
// an agent already signed in with a subscription keeps its own login.
const (
	ProviderAuthProvider = "provider"
	ProviderAuthClient   = "client"
	// ProviderAuthSubscription is a sign-in Gateway holds and renews itself, so
	// a subscription is used from here exactly like a key.
	ProviderAuthSubscription = "subscription"
)

var providerAuthModes = []string{ProviderAuthProvider, ProviderAuthClient, ProviderAuthSubscription}

// UsesClientAuth reports whether the provider authenticates with the caller's
// own credentials. An empty AuthMode is a row written before the field existed.
func UsesClientAuth(provider *Provider) bool {
	return provider.AuthMode == ProviderAuthClient
}

// ServesAnyModel reports whether the provider names no models on purpose: what
// a sign-in may ask for is decided by the account behind it, not by this table.
func ServesAnyModel(provider *Provider) bool {
	return UsesClientAuth(provider) || UsesSubscription(provider)
}

// The wire formats a provider's upstream can serve. A request that arrived in
// another one is translated by the proxy, see the protocol package.
const (
	ProtocolOpenAi    = protocol.OpenAi
	ProtocolAnthropic = protocol.Anthropic
	ProtocolResponses = protocol.Responses
)

// IsProviderTypeSupported reports whether the gateway can talk to the provider's
// upstream.
func IsProviderTypeSupported(provider *Provider) bool {
	return containsString(providerTypes, provider.Type)
}

// ProviderProtocol is the wire format a provider's upstream is talked to in.
// The provider names it when its upstream serves more than its type implies;
// otherwise everything that is not Anthropic is sent an OpenAI request.
func ProviderProtocol(provider *Provider) string {
	if protocol.IsUpstream(provider.Protocol) {
		return provider.Protocol
	}
	if provider.Type == "anthropic" {
		return ProtocolAnthropic
	}
	return ProtocolOpenAi
}

// ProviderApiFamily is the family a provider's base URL belongs to, which is
// what an agent configuration is written against: an agent pointed straight at
// a Responses provider still writes an OpenAI base URL.
func ProviderApiFamily(provider *Provider) string {
	if spoken := ProviderProtocol(provider); spoken != ProtocolResponses {
		return spoken
	}
	return ProtocolOpenAi
}

// ServesResponsesApi reports whether the provider's own upstream answers on the
// OpenAI Responses API. The other vendors stop at chat completions, which the
// gateway translates for the clients that speak nothing else.
func ServesResponsesApi(provider *Provider) bool {
	return provider.Type == "openai" || ProviderProtocol(provider) == ProtocolResponses
}

func containsString(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

// Provider is an upstream AI provider provider. (Milestone 1.1)
type Provider struct {
	Owner       string `xorm:"varchar(100) notnull pk" json:"owner"`
	Name        string `xorm:"varchar(100) notnull pk" json:"name"`
	CreatedTime string `xorm:"varchar(100)" json:"createdTime"`
	UpdatedTime string `xorm:"varchar(100)" json:"updatedTime"`

	DisplayName string `xorm:"varchar(100)" json:"displayName"`
	Type        string `xorm:"varchar(100)" json:"type"`
	BaseUrl     string `xorm:"varchar(255)" json:"baseUrl"`
	// Protocol is the wire format the upstream is talked to in, when it is not
	// the one Type implies.
	Protocol string `xorm:"varchar(100)" json:"protocol"`
	// ApiKey holds base64 ciphertext, not the bare key, when
	// "apiKeyEncryptionKey" is set in app.conf, hence the wider column.
	ApiKey string `xorm:"varchar(1000)" json:"apiKey"`
	// AuthMode selects whose credentials reach the upstream, see UsesClientAuth.
	AuthMode string `xorm:"varchar(100)" json:"authMode"`
	// Subscription is the sign-in of a ProviderAuthSubscription provider, stored
	// as ciphertext like the key. Only the sign-in flow writes it, and it leaves
	// this process no more than the key does.
	Subscription string `xorm:"mediumtext" json:"-"`
	// LoginId names a finished sign-in whose credential this save takes over,
	// which is how a token reaches the row without passing through the browser.
	LoginId string `xorm:"-" json:"loginId,omitempty"`
	// These three describe the stored sign-in, so the page can say whose account
	// a provider spends and offer to sign in again where it was granted.
	SubscriptionVendor  string `xorm:"-" json:"subscriptionVendor,omitempty"`
	SubscriptionAccount string `xorm:"-" json:"subscriptionAccount,omitempty"`
	SubscriptionPlan    string `xorm:"-" json:"subscriptionPlan,omitempty"`
	// Models is JSON-serialized by xorm, so it needs a text column rather than
	// a varchar: the serialized form is longer than the joined model names.
	Models []string `xorm:"mediumtext" json:"models"`
	// Icon is a site the vendor's favicon is taken from, or an image URL. Empty
	// means the icon is derived from BaseUrl.
	Icon string `xorm:"varchar(255)" json:"icon"`
	// Notes is whatever the person who added the provider wants to remember
	// about it: which account the key belongs to, when it expires, who pays.
	Notes string `xorm:"varchar(500)" json:"notes"`
	// Quota names the vendor's balance endpoint when there is no built-in one
	// for it. Nil leaves the built-in table in provider_quota.go to answer.
	Quota *QuotaConfig `xorm:"mediumtext json" json:"quota"`
	// TODO(1.2): Priority routing strategy will be defined in milestone 1.2.
	Priority int    `xorm:"int" json:"priority"`
	Status   string `xorm:"varchar(100)" json:"status"`
}

func (provider *Provider) GetId() string {
	return fmt.Sprintf("%s/%s", provider.Owner, provider.Name)
}

func GetProviders(owner string) ([]*Provider, error) {
	providers := []*Provider{}
	session := GetSession(owner, -1, -1, "", "", "", "")
	err := session.Find(&providers)
	decryptProviders(providers)
	return providers, err
}

func GetProviderCount(owner, field, value string) (int64, error) {
	session := GetSession(owner, -1, -1, field, value, "", "")
	return session.Count(&Provider{})
}

func GetPaginationProviders(owner string, offset, limit int, field, value, sortField, sortOrder string) ([]*Provider, error) {
	providers := []*Provider{}
	session := GetSession(owner, offset, limit, field, value, sortField, sortOrder)
	err := session.Find(&providers)
	decryptProviders(providers)
	return providers, err
}

func getProvider(owner, name string) (*Provider, error) {
	provider := &Provider{Owner: owner, Name: name}
	existed, err := ormer.Engine.Get(provider)
	if err != nil {
		return nil, err
	}
	if !existed {
		return nil, nil
	}
	decryptProvider(provider)
	return provider, nil
}

func GetProvider(id string) (*Provider, error) {
	owner, name := util.GetOwnerAndNameFromId(id)
	return getProvider(owner, name)
}

// GetMaskedProvider returns a copy of the provider with the API key replaced by
// ApiKeyMask, so the stored key never reaches the browser.
func GetMaskedProvider(provider *Provider) *Provider {
	if provider == nil {
		return nil
	}

	masked := *provider
	if masked.ApiKey != "" {
		masked.ApiKey = ApiKeyMask
	}
	describeSubscription(&masked)
	masked.Subscription = ""
	// The quota token shares a column with the rest of the quota configuration,
	// so the whole struct is copied before the token is replaced.
	if masked.Quota != nil && masked.Quota.Token != "" {
		quota := *masked.Quota
		quota.Token = ApiKeyMask
		masked.Quota = &quota
	}
	return &masked
}

func GetMaskedProviders(providers []*Provider) []*Provider {
	maskedProviders := make([]*Provider, 0, len(providers))
	for _, provider := range providers {
		maskedProviders = append(maskedProviders, GetMaskedProvider(provider))
	}
	return maskedProviders
}

func validateProvider(provider *Provider) error {
	if provider.Type == "" {
		provider.Type = "openai"
	}
	if provider.Status == "" {
		provider.Status = "enabled"
	}
	if provider.Models == nil {
		provider.Models = []string{}
	}
	if provider.AuthMode == "" {
		provider.AuthMode = ProviderAuthProvider
	}

	if !containsString(providerTypes, provider.Type) {
		return fmt.Errorf("invalid provider type: %s", provider.Type)
	}
	if provider.Protocol != "" && !protocol.IsUpstream(provider.Protocol) {
		return fmt.Errorf("the gateway cannot speak the %s API to a provider", provider.Protocol)
	}
	if !containsString(providerStatuses, provider.Status) {
		return fmt.Errorf("invalid provider status: %s", provider.Status)
	}
	if !containsString(providerAuthModes, provider.AuthMode) {
		return fmt.Errorf("invalid provider auth mode: %s", provider.AuthMode)
	}

	// A provider that forwards the caller's credentials, or spends a sign-in of
	// its own, has no use for a stored key, and one left behind would be sent
	// upstream again after a switch back.
	if provider.AuthMode != ProviderAuthProvider {
		provider.ApiKey = ""
	}

	if provider.BaseUrl != "" {
		if err := validateBaseUrl(provider.BaseUrl); err != nil {
			return err
		}
	}

	if len(provider.Models) > maxProviderModels {
		return fmt.Errorf("too many models: %d, at most %d are allowed", len(provider.Models), maxProviderModels)
	}
	for _, model := range provider.Models {
		if strings.TrimSpace(model) == "" {
			return fmt.Errorf("model name cannot be empty")
		}
		if len(model) > maxProviderModelChars {
			return fmt.Errorf("model name is too long: %s", model)
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

// ChatGptCodexPath is where the Codex requests of a ChatGPT subscription go.
// It names no version and serves the endpoint straight off the path, so the
// /v1 a versionless base URL gets would miss it.
const ChatGptCodexPath = "/backend-api/codex"

// apiVersionSegment matches a path segment that names an API version, which is
// how a base URL says it is already the root of the API: v1, v3, v1beta.
var apiVersionSegment = regexp.MustCompile(`^v[0-9]`)

// namesApiVersion reports whether any segment of the path is a version. The
// version is not always last: an OpenAI-compatible API served beside a native
// one lives at /v1beta/openai or /v3/openai.
func namesApiVersion(path string) bool {
	for _, segment := range strings.Split(path, "/") {
		if apiVersionSegment.MatchString(segment) {
			return true
		}
	}
	return false
}

// BuildOpenAiUrl joins an OpenAI-compatible endpoint onto a provider base URL.
// The base URL may be bare, already carry the version prefix or already end
// with the endpoint itself; none of those forms are doubled.
//
// The /v1 is only supplied for a base URL that names no version of its own,
// because plenty of vendors do not serve their OpenAI-compatible API under
// /v1: Gemini uses /v1beta/openai, Zhipu /api/paas/v4, DeepInfra /v1/openai.
// A path that names no version at all still gets one, so a relay configured as
// https://relay.example.com/openai keeps working.
func BuildOpenAiUrl(baseUrl string, endpoint string) (string, error) {
	u, err := url.Parse(baseUrl)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %s", err.Error())
	}

	path := strings.TrimSuffix(strings.TrimRight(u.Path, "/"), endpoint)
	if !namesApiVersion(path) && !strings.HasSuffix(path, ChatGptCodexPath) {
		path += "/v1"
	}

	u.Path = path + endpoint
	u.RawPath = ""
	return u.String(), nil
}

// BuildAnthropicUrl joins an Anthropic endpoint onto a provider base URL. Unlike
// an OpenAI base URL, an Anthropic one is bare and the endpoint carries the /v1
// prefix; a base URL that already has one is not doubled.
func BuildAnthropicUrl(baseUrl string, endpoint string) (string, error) {
	u, err := url.Parse(baseUrl)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %s", err.Error())
	}

	path := strings.TrimSuffix(strings.TrimRight(u.Path, "/"), endpoint)
	path = strings.TrimSuffix(strings.TrimRight(path, "/"), "/v1")

	u.Path = path + endpoint
	u.RawPath = ""
	return u.String(), nil
}

// BuildProviderUrl is the upstream URL a request in the given protocol is sent
// to. The endpoint is the protocol's own, not a shared one.
func BuildProviderUrl(baseUrl string, protocol string, endpoint string) (string, error) {
	if protocol == ProtocolAnthropic {
		return BuildAnthropicUrl(baseUrl, endpoint)
	}
	return BuildOpenAiUrl(baseUrl, endpoint)
}

// AppendQuery puts the query of the client request back on an upstream URL.
// The query selects a variant of the endpoint — the Anthropic clients ask for
// the beta one with "?beta=true" — so dropping it would forward a different
// request than the one that was made. A base URL carrying a query of its own
// keeps it, with the client's appended.
func AppendQuery(rawUrl string, rawQuery string) string {
	if rawQuery == "" {
		return rawUrl
	}

	u, err := url.Parse(rawUrl)
	if err != nil {
		return rawUrl
	}
	if u.RawQuery != "" {
		rawQuery = u.RawQuery + "&" + rawQuery
	}
	u.RawQuery = rawQuery
	return u.String()
}

// SetProviderAuth puts the provider's credentials on an upstream request, in the
// header the provider's protocol authenticates with.
func SetProviderAuth(header http.Header, provider *Provider) {
	// The caller's own credentials are already on the request, and the proxy
	// copies them across itself.
	if UsesClientAuth(provider) {
		return
	}
	if UsesSubscription(provider) {
		SetSubscriptionAuth(header, provider)
		return
	}

	if ProviderProtocol(provider) == ProtocolAnthropic {
		header.Set("X-Api-Key", provider.ApiKey)
		return
	}
	header.Set("Authorization", "Bearer "+provider.ApiKey)
}

// takeProviderLogin moves the credential of a finished sign-in onto the row
// being saved. A save that brings none leaves the stored one alone, which is
// what makes editing the rest of a signed-in provider harmless.
func takeProviderLogin(provider *Provider) error {
	loginId := provider.LoginId
	provider.LoginId = ""
	provider.Subscription = ""

	if loginId != "" {
		granted, err := takeSubscriptionLogin(loginId)
		if err != nil {
			return err
		}
		plain, err := json.Marshal(granted)
		if err != nil {
			return err
		}
		provider.Subscription = string(plain)
	}

	if !UsesSubscription(provider) {
		provider.Subscription = ""
	}
	return nil
}

func AddProvider(provider *Provider) (bool, error) {
	if err := validateProvider(provider); err != nil {
		return false, err
	}

	name, suffix, err := freeProviderName(provider.Owner, provider.Name)
	if err != nil {
		return false, err
	}
	provider.Name = name
	// The name the user sees follows the one that was free, so two accounts with
	// the same vendor are told apart in the list rather than only in the URL.
	if suffix > 1 && provider.DisplayName != "" {
		provider.DisplayName = fmt.Sprintf("%s %d", provider.DisplayName, suffix)
	}

	now := util.GetCurrentTime()
	if provider.CreatedTime == "" {
		provider.CreatedTime = now
	}
	provider.UpdatedTime = now

	stampQuotaSince(provider)
	if err := encryptQuotaToken(provider); err != nil {
		return false, err
	}
	if err := takeProviderLogin(provider); err != nil {
		return false, err
	}
	// The probe needs the key as it was typed, and encryption is in place.
	probeTarget := *provider
	if err := encryptApiKey(provider); err != nil {
		return false, err
	}
	if err := encryptSubscription(provider); err != nil {
		return false, err
	}

	affected, err := ormer.Engine.Insert(provider)
	if err == nil && affected != 0 {
		ProbeNewProvider(&probeTarget)
	}
	return affected != 0, err
}

// freeProviderName keeps the readable name the user asked for and appends a
// number only when it is taken, so a second DeepSeek account becomes "deepseek-2"
// rather than something nobody can read.
func freeProviderName(owner string, name string) (string, int, error) {
	for suffix := 1; suffix < 1000; suffix++ {
		candidate := name
		if suffix > 1 {
			candidate = fmt.Sprintf("%s-%d", name, suffix)
		}

		existing, err := getProvider(owner, candidate)
		if err != nil {
			return "", 0, err
		}
		if existing == nil {
			return candidate, suffix, nil
		}
	}

	return "", 0, fmt.Errorf("too many providers named %s", name)
}

func UpdateProvider(id string, provider *Provider) (bool, error) {
	owner, name := util.GetOwnerAndNameFromId(id)
	stored, err := getProvider(owner, name)
	if err != nil {
		return false, err
	}
	if stored == nil {
		return false, nil
	}

	if err := validateProvider(provider); err != nil {
		return false, err
	}

	provider.Owner = owner
	provider.Name = name
	provider.UpdatedTime = util.GetCurrentTime()

	// Snapshotted before the key is encrypted, and before the mask that means
	// "the field was not touched" is resolved back to the stored key.
	keyChanged := provider.ApiKey != ApiKeyMask
	probeTarget := *provider
	if !keyChanged {
		probeTarget.ApiKey = stored.ApiKey
	}

	// The quota token is masked on read like the API key, but it shares its
	// column with the rest of the quota configuration, so the stored one is put
	// back rather than the column left out of the write.
	if provider.Quota != nil && provider.Quota.Token == ApiKeyMask {
		provider.Quota.Token = stored.Quota.token()
	}
	stampQuotaSince(provider)
	if err := encryptQuotaToken(provider); err != nil {
		return false, err
	}

	if err := takeProviderLogin(provider); err != nil {
		return false, err
	}

	session := ormer.Engine.ID(core.PK{owner, name})
	// The sign-in never reaches the browser, so a save that carries none is
	// leaving the stored one alone rather than clearing it. Switching the
	// provider off a subscription does clear it: it is spent by nothing now.
	if provider.Subscription == "" && UsesSubscription(provider) {
		session = session.Omit("subscription")
	} else if err = encryptSubscription(provider); err != nil {
		return false, err
	}
	// The browser only ever sees the mask, so getting it back means the user
	// did not touch the field. Any other value (including "") is written, which
	// is what makes clearing a key possible.
	if provider.ApiKey == ApiKeyMask {
		session = session.Omit("api_key")
	} else if err = encryptApiKey(provider); err != nil {
		return false, err
	}

	var affected int64

	affected, err = session.AllCols().Update(provider)
	if err == nil {
		// The edit may be the fix for whatever the proxy last saw, so the
		// provider starts from a clean slate.
		ClearProviderHealth(provider.GetId())
		ClearProviderQuota(provider.GetId())
		ProbeEditedProvider(stored, &probeTarget, keyChanged)
	}
	return affected != 0, err
}

func DeleteProvider(provider *Provider) (bool, error) {
	affected, err := ormer.Engine.ID(core.PK{provider.Owner, provider.Name}).Delete(&Provider{})
	if err == nil {
		ClearProviderHealth(provider.GetId())
		ClearProviderQuota(provider.GetId())
		if err := DeleteProviderProbes(provider.GetId()); err != nil {
			fmt.Printf("DeleteProvider(): probes of [%s]: %v\n", provider.GetId(), err)
		}
	}
	return affected != 0, err
}

// maxProbeBody caps what is read back from an upstream: a model list from an
// aggregator is long, and an error page can be arbitrarily long.
const maxProbeBody = 1 << 20

// providerProbe is what a read-only GET against a provider's models endpoint
// returned. The same probe answers both "is this upstream reachable" and
// "which models does it serve".
type providerProbe struct {
	statusCode int
	status     string
	body       []byte
}

func (probe *providerProbe) ok() bool {
	return probe.statusCode >= 200 && probe.statusCode < 300
}

// checkProviderTarget rejects what cannot be asked anything at all, so an
// unsupported type or an unusable base URL is reported as itself rather than as
// whatever the request layer makes of it.
func checkProviderTarget(provider *Provider) error {
	if !IsProviderTypeSupported(provider) {
		return fmt.Errorf("the %s provider type is not supported", provider.Type)
	}
	if provider.BaseUrl == "" {
		return errors.New("the base URL is empty")
	}
	return validateBaseUrl(provider.BaseUrl)
}

// probeProvider performs the read-only GET against the provider's models
// endpoint. The provider is used as given rather than read back from the
// database, so a provider that is not saved yet can be probed too.
func probeProvider(provider *Provider) (*providerProbe, error) {
	if err := checkProviderTarget(provider); err != nil {
		return nil, err
	}

	protocol := ProviderProtocol(provider)
	probeEndpoint := "/models"
	if protocol == ProtocolAnthropic {
		probeEndpoint = "/v1/models"
	}

	probeUrl, err := BuildProviderUrl(provider.BaseUrl, protocol, probeEndpoint)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, probeUrl, nil)
	if err != nil {
		return nil, err
	}
	if protocol == ProtocolAnthropic {
		req.Header.Set("Anthropic-Version", AnthropicVersion)
	}
	if provider.ApiKey != "" {
		SetProviderAuth(req.Header, provider)
	}

	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: proxy.Transport(),
		// Do not follow redirects, so the reported status is the one the
		// configured base URL actually returns.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	resp, err := client.Do(req)
	if err != nil {
		// Unwrap the *url.Error so the reason is not buried behind the URL.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			return nil, urlErr.Err
		}
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxProbeBody))
	if err != nil {
		return nil, err
	}

	return &providerProbe{statusCode: resp.StatusCode, status: resp.Status, body: body}, nil
}

// testCompletionMaxTokens is the shortest answer worth asking for: what is
// being checked is that the model answers, not what it says.
const testCompletionMaxTokens = 16

// TestProviderConnectivity checks that the provider can actually serve a
// completion. It returns whether it did, the upstream HTTP status code (0 when
// no response was received) and a human-readable message. The provider is used
// as given, so a form can be checked before it is saved.
//
// A models list is not enough to answer this: a vendor that lists a model it
// will not serve - or serves only under another name, as the endpoint-id
// vendors do - answers that list exactly like one that works, and the person
// filling in the form finds out at the first real request instead.
func TestProviderConnectivity(provider *Provider) (bool, int, string) {
	if err := checkProviderTarget(provider); err != nil {
		return false, 0, err.Error()
	}

	// A provider authenticated by a sign-in names no model to ask for, and a
	// client-auth one carries no credential of its own. All that can be checked
	// either way is that the endpoint is there.
	if ServesAnyModel(provider) || provider.ApiKey == "" {
		return testProviderReachable(provider)
	}

	model, err := probeModelOf(provider)
	if err != nil {
		return false, 0, err.Error()
	}
	return testProviderCompletion(provider, model)
}

// testProviderReachable is the read-only GET against the models endpoint, which
// is all there is to ask when there is no key to ask with.
func testProviderReachable(provider *Provider) (bool, int, string) {
	probe, err := probeProvider(provider)
	if err != nil {
		return false, 0, err.Error()
	}

	// An upstream that rejects the unauthenticated probe has still proven it is
	// reachable, which is the whole of what a provider with no key can show.
	if ServesAnyModel(provider) && (probe.statusCode == http.StatusUnauthorized || probe.statusCode == http.StatusForbidden) {
		if UsesSubscription(provider) {
			return true, probe.statusCode, "reachable, and spending the sign-in stored here"
		}
		return true, probe.statusCode, "reachable, and authenticated with the caller's own credentials"
	}

	if !probe.ok() {
		return false, probe.statusCode, probe.status + probeDetail(probe.body)
	}
	return true, probe.statusCode, probe.status
}

// testProviderCompletion asks the model the shortest question its API defines.
func testProviderCompletion(provider *Provider, model string) (bool, int, string) {
	body := map[string]any{
		"model":      model,
		"max_tokens": testCompletionMaxTokens,
		"messages":   []any{map[string]any{"role": "user", "content": probeStreamPrompt}},
	}

	answer, err := sendProbe(provider, body, false)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return false, 0, err.Error()
	}

	// An error envelope is read before the status code: several vendors answer
	// a refused model with 200 and the reason in the body.
	if _, message := readProbeError(answer.body); message != "" {
		return false, answer.status, model + " was refused" + probeDetail([]byte(message))
	}
	if !answer.ok() {
		return false, answer.status, model + " was refused" + probeDetail(answer.body)
	}

	parsed := probeBody{}
	if err := json.Unmarshal(answer.body, &parsed); err != nil ||
		(parsed.Model == "" && len(parsed.Choices) == 0 && len(parsed.Content) == 0) {
		return false, answer.status, model + " did not answer with a completion" + probeDetail(answer.body)
	}
	return true, answer.status, model + " answered"
}

// FetchProviderModels lists what the provider's upstream reports at its models
// endpoint, so the model names do not have to be typed by hand.
func FetchProviderModels(provider *Provider) ([]string, error) {
	probe, err := probeProvider(provider)
	if err != nil {
		return nil, err
	}
	if !probe.ok() {
		return nil, fmt.Errorf("the upstream answered %s%s", probe.status, probeDetail(probe.body))
	}

	models, err := parseModelList(probe.body)
	if err != nil {
		return nil, err
	}
	if len(models) == 0 {
		return nil, errors.New("the upstream did not report any model")
	}
	return models, nil
}

// parseModelList reads the model ids out of a models response. OpenAI and
// Anthropic both answer {"data": [{"id": ...}]}, and an OpenAI-compatible
// vendor follows OpenAI.
func parseModelList(body []byte) ([]string, error) {
	var payload struct {
		Data []struct {
			Id string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("the upstream did not answer with a model list: %s", err.Error())
	}

	models := []string{}
	seen := map[string]bool{}
	for _, item := range payload.Data {
		id := strings.TrimSpace(item.Id)
		if id == "" || len(id) > maxProviderModelChars || seen[id] {
			continue
		}
		seen[id] = true
		models = append(models, id)
	}
	// The upstream order is kept: Anthropic returns its newest model first.
	return models, nil
}

// probeDetail is the upstream's own error text, trimmed to what fits in a
// toast: a status code alone rarely says which of key, URL or plan is wrong.
func probeDetail(body []byte) string {
	text := strings.Join(strings.Fields(string(body)), " ")
	if text == "" {
		return ""
	}
	if runes := []rune(text); len(runes) > 200 {
		text = string(runes[:200]) + "..."
	}
	return ": " + text
}

// GetProvidersByModel returns the enabled providers a request naming this model
// is forwarded to, ordered by priority (ascending, so the lowest value comes
// first) so that the caller can fail over from one provider to the next. It
// queries all providers globally (no owner filter) because /v1/chat/completions
// is an unauthenticated public endpoint.
//
// A model no provider names falls back to every enabled provider, which is then
// sent a model it does serve, see ProviderModel(). An Anthropic client asks for
// model names of its own whatever it is configured with - claude-haiku-4-5 for
// its background work, and whichever built-in it probes availability with - so
// rejecting them leaves the client unable to talk to any third-party provider
// at all.
func GetProvidersByModel(model string) ([]*Provider, error) {
	providers, err := listEnabledProviders()
	if err != nil {
		return nil, err
	}

	// The models are JSON-serialized into a single column, so the match cannot
	// be pushed down into the query.
	matchedProviders := []*Provider{}
	// A provider authenticated with a sign-in rather than a key cannot know
	// which models the account behind it may use, so an empty model list there
	// means "any model". Those providers are tried after the ones that name the
	// model, so a wildcard never takes traffic from an exact match.
	wildcardProviders := []*Provider{}
	for _, provider := range providers {
		if len(provider.Models) == 0 {
			if ServesAnyModel(provider) {
				wildcardProviders = append(wildcardProviders, provider)
			}
			continue
		}
		for _, providerModel := range provider.Models {
			if providerModel == model {
				matchedProviders = append(matchedProviders, provider)
				break
			}
		}
	}
	matchedProviders = append(matchedProviders, wildcardProviders...)

	if len(matchedProviders) == 0 {
		if len(providers) == 0 {
			return nil, fmt.Errorf("%w: %s", ErrNoProviderAvailable, model)
		}
		matchedProviders = providers
	}

	decryptProviders(matchedProviders)
	return matchedProviders, nil
}

// ListEnabledModels is every model the enabled providers serve, in priority
// order and without duplicates, plus the ones the routing rules answer for. It
// is what the models endpoint answers with, so that a client fills its model
// picker with what this gateway can actually reach.
func ListEnabledModels() ([]string, error) {
	providers, err := listEnabledProviders()
	if err != nil {
		return nil, err
	}
	return ModelsWithRoutes(ModelsOfProviders(providers), ""), nil
}

// listEnabledProviders is every provider in rotation, lowest priority value
// first, which is the order requests are tried against them. The keys are still
// ciphertext: only the providers a request actually reaches are decrypted.
func listEnabledProviders() ([]*Provider, error) {
	providers := []*Provider{}
	if err := ormer.Engine.Where("status = ?", "enabled").Asc("priority").Find(&providers); err != nil {
		return nil, fmt.Errorf("provider query failed: %w", err)
	}
	return providers, nil
}

// ModelsOfProviders flattens the model lists of providers into one, keeping the
// order they are tried in.
func ModelsOfProviders(providers []*Provider) []string {
	models := []string{}
	seen := map[string]bool{}
	for _, provider := range providers {
		for _, model := range provider.Models {
			if model == "" || seen[model] {
				continue
			}
			seen[model] = true
			models = append(models, model)
		}
	}
	return models
}

// ProviderModel is the model name to send a provider for the model the client
// asked for. An agent picks its model on its own - Codex Desktop remembers the
// one chosen in its own state, whatever the config file says - so a provider
// that never heard of it answers with an error naming the models it does serve,
// rather than with a completion. That agent is sent the closest model the
// provider does serve instead, see PickProviderModel. A provider that serves no
// named model, which is the client-auth case, takes whatever arrives.
func ProviderModel(provider *Provider, model string) string {
	if len(provider.Models) == 0 {
		return model
	}
	return PickProviderModel(provider.Models, model)
}
