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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/apache/casbin-gateway/proxy"
	"github.com/apache/casbin-gateway/util"
)

// ProviderQuota is what the vendor behind a provider says is left on the
// account. Like ProviderHealth it lives in memory: it is the vendor's answer at
// one point in time, not a stored setting.
//
// The numbers are pointers because a vendor that reports only what is left must
// not be shown as having used nothing.
type ProviderQuota struct {
	Provider string `json:"provider"`
	// Supported is false when Gateway knows no balance endpoint for the vendor
	// and the provider names none either, which is not a failure.
	Supported bool     `json:"supported"`
	Remaining *float64 `json:"remaining"`
	Used      *float64 `json:"used"`
	Total     *float64 `json:"total"`
	Unit      string   `json:"unit"`
	Error     string   `json:"error"`
	Time      string   `json:"time"`
}

// QuotaConfig points the probe at a vendor's own balance endpoint, for the
// vendors and the aggregator sites that have no built-in entry here. It reads
// the answer by JSON path rather than by running code: the configuration comes
// from whoever added the provider, and this runs on a shared server.
type QuotaConfig struct {
	// Url is a path resolved against the base URL's origin, or an absolute URL
	// that has to be on that same origin.
	Url     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	// Token is a credential of its own, for the aggregator sites whose balance
	// endpoint does not take the inference key. It is stored and masked like
	// Provider.ApiKey, and headers reach it as {{token}}.
	Token string `json:"token"`
	// Remaining, Used and Total are dotted paths into the JSON answer, where a
	// numeric segment indexes an array: "balance_infos.0.total_balance".
	Remaining string `json:"remaining"`
	Used      string `json:"used"`
	Total     string `json:"total"`
	// Unit is the currency itself, not a path: a vendor does not vary it.
	Unit string `json:"unit"`
	// Scale divides every number read, for a vendor that reports in an internal
	// unit of its own. 0 and 1 both mean "as reported".
	Scale float64 `json:"scale"`

	// Manual asks no endpoint at all: the balance is Initial, drawn down by what
	// the relay recorded spending through this provider since Since. It is the
	// answer for a vendor with no balance API of any kind. The recorded cost is
	// priced from the models table, so Unit only matches reality when that table
	// is kept in the same currency.
	Manual  bool    `json:"manual"`
	Initial float64 `json:"initial"`
	// Since is where the drawdown starts counting, in util.GetCurrentTime form.
	// Set when Manual is first saved; resetting the counter moves it forward.
	Since string `json:"since"`
}

func (config *QuotaConfig) isEmpty() bool {
	return config == nil || strings.TrimSpace(config.Url) == ""
}

func (config *QuotaConfig) isManual() bool {
	return config != nil && config.Manual
}

// quotaVendor is a vendor whose balance endpoint is documented, so that a
// provider pointed at it needs nothing configured.
type quotaVendor struct {
	hosts []string
	path  string
	read  func(body any, quota *ProviderQuota)
}

// The endpoints and the field names below were each checked against the live
// API. A vendor without a balance endpoint (OpenAI, Anthropic) has no entry:
// there is nothing to ask.
var quotaVendors = []quotaVendor{
	{
		hosts: []string{"api.deepseek.com"},
		path:  "/user/balance",
		read: func(body any, quota *ProviderQuota) {
			quota.Remaining = quotaNumber(body, "balance_infos.0.total_balance")
			quota.Unit = quotaString(body, "balance_infos.0.currency")
		},
	},
	{
		hosts: []string{"api.siliconflow.cn"},
		path:  "/v1/user/info",
		read: func(body any, quota *ProviderQuota) {
			quota.Remaining = quotaNumber(body, "data.totalBalance")
			quota.Unit = "CNY"
		},
	},
	{
		hosts: []string{"api.siliconflow.com"},
		path:  "/v1/user/info",
		read: func(body any, quota *ProviderQuota) {
			quota.Remaining = quotaNumber(body, "data.totalBalance")
			quota.Unit = "USD"
		},
	},
	{
		hosts: []string{"api.moonshot.cn", "api.moonshot.ai"},
		path:  "/v1/users/me/balance",
		read: func(body any, quota *ProviderQuota) {
			quota.Remaining = quotaNumber(body, "data.available_balance")
			quota.Unit = "CNY"
		},
	},
	{
		hosts: []string{"openrouter.ai"},
		path:  "/api/v1/credits",
		read: func(body any, quota *ProviderQuota) {
			quota.Total = quotaNumber(body, "data.total_credits")
			quota.Used = quotaNumber(body, "data.total_usage")
			quota.Remaining = quotaMinus(quota.Total, quota.Used)
			quota.Unit = "USD"
		},
	},
	{
		hosts: []string{"api.stepfun.com", "api.stepfun.ai"},
		path:  "/v1/accounts",
		read: func(body any, quota *ProviderQuota) {
			quota.Remaining = quotaNumber(body, "balance")
			quota.Unit = "CNY"
		},
	},
	{
		hosts: []string{"api.novita.ai"},
		path:  "/v3/user/balance",
		// Novita reports in units of 0.0001 USD.
		read: func(body any, quota *ProviderQuota) {
			quota.Remaining = quotaScale(quotaNumber(body, "availableBalance"), 10000)
			quota.Unit = "USD"
		},
	},
}

func quotaVendorOf(baseUrl string) *quotaVendor {
	parsed, err := url.Parse(baseUrl)
	if err != nil {
		return nil
	}

	host := strings.ToLower(parsed.Hostname())
	for i := range quotaVendors {
		for _, candidate := range quotaVendors[i].hosts {
			if host == candidate || strings.HasSuffix(host, "."+candidate) {
				return &quotaVendors[i]
			}
		}
	}
	return nil
}

// quotaValue walks a dotted path into a decoded JSON body, where a numeric
// segment indexes an array.
func quotaValue(body any, path string) any {
	if path == "" {
		return nil
	}

	current := body
	for _, key := range strings.Split(path, ".") {
		switch node := current.(type) {
		case map[string]any:
			current = node[key]
		case []any:
			index, err := strconv.Atoi(key)
			if err != nil || index < 0 || index >= len(node) {
				return nil
			}
			current = node[index]
		default:
			return nil
		}
	}
	return current
}

// quotaNumber reads an amount, which a vendor may have encoded as a string:
// DeepSeek answers "110.00" where OpenRouter answers 110.
func quotaNumber(body any, path string) *float64 {
	switch value := quotaValue(body, path).(type) {
	case float64:
		return &value
	case string:
		number, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			return nil
		}
		return &number
	}
	return nil
}

func quotaString(body any, path string) string {
	if value, ok := quotaValue(body, path).(string); ok {
		return value
	}
	return ""
}

func quotaMinus(total *float64, used *float64) *float64 {
	if total == nil || used == nil {
		return nil
	}
	remaining := *total - *used
	return &remaining
}

func quotaScale(value *float64, divisor float64) *float64 {
	if value == nil || divisor == 0 || divisor == 1 {
		return value
	}
	scaled := *value / divisor
	return &scaled
}

// quotaOrigin is the scheme and host a provider's requests go to, which is the
// only origin its quota may be asked of.
func quotaOrigin(baseUrl string) (string, error) {
	parsed, err := url.Parse(baseUrl)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %s", err.Error())
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("invalid base URL: only the http and https schemes are supported")
	}
	if parsed.Host == "" {
		return "", errors.New("invalid base URL: the host is empty")
	}
	return parsed.Scheme + "://" + parsed.Host, nil
}

// quotaUrl resolves a configured endpoint against the provider's own origin. An
// absolute URL is allowed but has to stay on that origin: a quota endpoint
// belongs to the vendor being billed, and following it anywhere else would turn
// the provider form into a way of making Gateway fetch arbitrary addresses.
func quotaUrl(configured string, origin string) (string, error) {
	configured = strings.TrimSpace(configured)
	if strings.HasPrefix(configured, "/") {
		return origin + configured, nil
	}

	parsed, err := url.Parse(configured)
	if err != nil {
		return "", fmt.Errorf("invalid quota URL: %s", err.Error())
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("the quota URL has to be a path or an absolute URL")
	}
	if parsed.Scheme+"://"+parsed.Host != origin {
		return "", fmt.Errorf("the quota URL has to be on %s, the provider's own origin", origin)
	}
	return parsed.String(), nil
}

// probeQuota asks the vendor what is left. Everything it can go wrong with is
// reported on the quota itself rather than as an error: a vendor that is down
// is a thing the page shows, not a broken request.
func probeQuota(provider *Provider) *ProviderQuota {
	quota := &ProviderQuota{
		Provider: provider.GetId(),
		Time:     util.GetCurrentTime(),
	}

	if UsesClientAuth(provider) {
		quota.Error = "the caller's own login is forwarded, so there is no account here to ask about"
		return quota
	}

	if provider.Quota.isManual() {
		return manualQuota(provider, quota)
	}

	origin, err := quotaOrigin(provider.BaseUrl)
	if err != nil {
		quota.Error = err.Error()
		return quota
	}

	config := provider.Quota
	vendor := quotaVendorOf(provider.BaseUrl)
	if config.isEmpty() && vendor == nil {
		return quota
	}
	quota.Supported = true

	endpoint := ""
	if config.isEmpty() {
		endpoint = origin + vendor.path
	} else if endpoint, err = quotaUrl(config.Url, origin); err != nil {
		quota.Error = err.Error()
		return quota
	}

	body, err := fetchQuota(endpoint, provider, config)
	if err != nil {
		quota.Error = err.Error()
		return quota
	}

	if config.isEmpty() {
		vendor.read(body, quota)
	} else {
		scale := config.Scale
		quota.Remaining = quotaScale(quotaNumber(body, config.Remaining), scale)
		quota.Used = quotaScale(quotaNumber(body, config.Used), scale)
		quota.Total = quotaScale(quotaNumber(body, config.Total), scale)
		quota.Unit = config.Unit
	}

	if quota.Remaining == nil && quota.Used == nil && quota.Total == nil {
		quota.Error = "the vendor answered, but none of the configured fields were in the answer"
	}
	return quota
}

// manualQuota draws the operator's starting figure down by what the relayed
// records say was spent through this provider. Nothing is asked of any vendor.
func manualQuota(provider *Provider, quota *ProviderQuota) *ProviderQuota {
	config := provider.Quota
	quota.Supported = true
	quota.Unit = config.Unit

	spent, err := sumProviderCost(provider.GetId(), config.Since)
	if err != nil {
		quota.Error = err.Error()
		return quota
	}

	total := config.Initial
	remaining := total - spent
	quota.Total = &total
	quota.Used = &spent
	quota.Remaining = &remaining
	return quota
}

// fetchQuota performs the GET and decodes the answer. The provider's own key
// authenticates it unless the configuration sets its own Authorization.
func fetchQuota(endpoint string, provider *Provider, config *QuotaConfig) (any, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	for name, value := range config.headers() {
		req.Header.Set(name, quotaExpand(value, provider))
	}
	if req.Header.Get("Authorization") == "" {
		if token := config.token(); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		} else if provider.ApiKey != "" {
			req.Header.Set("Authorization", "Bearer "+provider.ApiKey)
		}
	}

	client := &http.Client{Timeout: 15 * time.Second, Transport: proxy.Transport()}
	resp, err := client.Do(req)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			return nil, urlErr.Err
		}
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxProbeBody))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("the vendor answered %s%s", resp.Status, probeDetail(raw))
	}

	var body any
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, fmt.Errorf("the vendor did not answer with JSON: %s", err.Error())
	}
	return body, nil
}

func (config *QuotaConfig) headers() map[string]string {
	if config == nil {
		return nil
	}
	return config.Headers
}

func (config *QuotaConfig) token() string {
	if config == nil {
		return ""
	}
	return config.Token
}

// quotaExpand fills in the placeholders a header value may carry, so that a
// credential is not written into the configuration a second time.
func quotaExpand(value string, provider *Provider) string {
	value = strings.ReplaceAll(value, "{{apiKey}}", provider.ApiKey)
	return strings.ReplaceAll(value, "{{token}}", provider.Quota.token())
}

// providerQuotaTtl is how long an answer is reused. A balance moves as requests
// are relayed, but not so fast that every page view should cost the vendor a
// request.
const providerQuotaTtl = 10 * time.Minute

type cachedQuota struct {
	quota *ProviderQuota
	time  time.Time
}

var (
	providerQuotaMutex sync.Mutex
	providerQuotaMap   = map[string]*cachedQuota{}
)

// GetProviderQuotas returns what is already known, without asking any vendor.
func GetProviderQuotas(providers []*Provider) []*ProviderQuota {
	providerQuotaMutex.Lock()
	defer providerQuotaMutex.Unlock()

	result := []*ProviderQuota{}
	for _, provider := range providers {
		if cached, ok := providerQuotaMap[provider.GetId()]; ok {
			result = append(result, cached.quota)
		}
	}
	return result
}

// maxQuotaProbes bounds the fan-out, so refreshing a long list of providers does
// not open one connection per provider at once.
const maxQuotaProbes = 6

// RefreshProviderQuotas asks the vendors and caches what they answer. Without
// force, a provider whose answer is still fresh is left alone, which is what
// makes it safe to call on every page load.
func RefreshProviderQuotas(providers []*Provider, force bool) []*ProviderQuota {
	stale := []*Provider{}
	for _, provider := range providers {
		if force || isQuotaStale(provider.GetId()) {
			stale = append(stale, provider)
		}
	}

	var wait sync.WaitGroup
	tokens := make(chan struct{}, maxQuotaProbes)
	for _, provider := range stale {
		wait.Add(1)
		go func(provider *Provider) {
			defer wait.Done()
			tokens <- struct{}{}
			defer func() { <-tokens }()

			quota := probeQuota(provider)
			providerQuotaMutex.Lock()
			providerQuotaMap[quota.Provider] = &cachedQuota{quota: quota, time: time.Now()}
			providerQuotaMutex.Unlock()
		}(provider)
	}
	wait.Wait()

	return GetProviderQuotas(providers)
}

func isQuotaStale(providerId string) bool {
	providerQuotaMutex.Lock()
	defer providerQuotaMutex.Unlock()

	cached, ok := providerQuotaMap[providerId]
	return !ok || time.Since(cached.time) > providerQuotaTtl
}

// ClearProviderQuota forgets a vendor's answer, which an edited or deleted
// provider deserves: the key it was read with may be the thing that changed.
func ClearProviderQuota(providerId string) {
	providerQuotaMutex.Lock()
	defer providerQuotaMutex.Unlock()
	delete(providerQuotaMap, providerId)
}
