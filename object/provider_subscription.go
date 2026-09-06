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
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/apache/casbin-gateway/agent"
	"github.com/apache/casbin-gateway/proxy"
	"github.com/xorm-io/core"
)

// The vendors whose subscription Gateway can hold. The sign-in is the one that
// vendor's own client uses, because the endpoint behind a subscription accepts
// no other OAuth application.
const (
	SubscriptionOpenAi    = "openai"
	SubscriptionAnthropic = "anthropic"
)

// What a subscription's requests have to look like to be served: the vendor
// hands these tokens to one client of its own, and its endpoint answers that
// client. They are the same values the client on this machine would send.
const (
	codexOriginator = "codex_cli_rs"
	claudeUserAgent = "claude-cli/2.1.170 (external, cli)"
	claudeBetas     = "oauth-2025-04-20,claude-code-20250219,interleaved-thinking-2025-05-14"
	claudePreamble  = "You are Claude Code, Anthropic's official CLI for Claude."
)

// SubscriptionVendor is one sign-in Gateway can hold: where it is granted, and
// what the provider holding it then talks to.
type SubscriptionVendor struct {
	Id    string `json:"id"`
	Label string `json:"label"`
	// BaseUrl and Protocol are the endpoint a subscription is served from,
	// which is the vendor's own client backend rather than its API.
	BaseUrl  string `json:"baseUrl"`
	Protocol string `json:"protocol"`

	clientId     string
	authorizeUrl string
	tokenUrl     string
	// redirectUri is registered by the client this sign-in belongs to, port and
	// path included, so the browser has to come back to exactly this.
	redirectUri  string
	listenAddr   string
	callbackPath string
	scope        string
	extraParams  map[string]string
	// codeJson posts the first exchange as JSON. A renewal is JSON everywhere.
	codeJson bool
	// stateInFragment marks a vendor that returns the state after a "#", which
	// never reaches a server, so the callback page hands it back itself.
	stateInFragment bool
	// tokenHeaders are what the token endpoint wants beside the body.
	tokenHeaders map[string]string
	// query is appended to every upstream URL, for an endpoint that selects a
	// variant that way.
	query string
	// apply puts the sign-in on an upstream request, in the headers this vendor
	// reads it from.
	apply func(http.Header, *ProviderSubscription)
	// shape rewrites a request body the vendor's endpoint would otherwise
	// refuse. Nil where nothing has to be changed.
	shape func([]byte) []byte
}

var subscriptionVendors = []*SubscriptionVendor{
	{
		Id:       SubscriptionOpenAi,
		Label:    "ChatGPT",
		BaseUrl:  "https://chatgpt.com" + ChatGptCodexPath,
		Protocol: ProtocolResponses,

		clientId:     "app_EMoamEEZ73f0CkXaXp7hrann",
		authorizeUrl: "https://auth.openai.com/oauth/authorize",
		tokenUrl:     "https://auth.openai.com/oauth/token",
		redirectUri:  "http://localhost:1455/auth/callback",
		listenAddr:   "127.0.0.1:1455",
		callbackPath: "/auth/callback",
		scope:        "openid profile email offline_access",
		extraParams: map[string]string{
			"id_token_add_organizations": "true",
			"originator":                 codexOriginator,
		},
		tokenHeaders: map[string]string{"Originator": codexOriginator},
		apply: func(header http.Header, subscription *ProviderSubscription) {
			header.Set("Authorization", "Bearer "+subscription.AccessToken)
			if subscription.AccountId != "" {
				header.Set("Chatgpt-Account-Id", subscription.AccountId)
			}
			if header.Get("Originator") == "" {
				header.Set("Originator", codexOriginator)
			}
		},
	},
	{
		Id:       SubscriptionAnthropic,
		Label:    "Claude",
		BaseUrl:  "https://api.anthropic.com",
		Protocol: ProtocolAnthropic,

		clientId:        "9d1c250a-e61b-44d9-88ed-5944d1962f5e",
		authorizeUrl:    "https://claude.ai/oauth/authorize",
		tokenUrl:        "https://api.anthropic.com/v1/oauth/token",
		redirectUri:     "http://localhost:54545/callback",
		listenAddr:      "127.0.0.1:54545",
		callbackPath:    "/callback",
		scope:           "org:create_api_key user:profile user:inference",
		codeJson:        true,
		stateInFragment: true,
		tokenHeaders:    map[string]string{"User-Agent": claudeUserAgent},
		query:           "beta=true",
		apply: func(header http.Header, subscription *ProviderSubscription) {
			// The subscription endpoint reads a bearer token, not the x-api-key
			// an API account authenticates with.
			header.Del("X-Api-Key")
			header.Set("Authorization", "Bearer "+subscription.AccessToken)
			header.Set("Anthropic-Version", AnthropicVersion)
			header.Set("Anthropic-Beta", mergeBetas(header.Get("Anthropic-Beta"), claudeBetas))
			header.Set("User-Agent", claudeUserAgent)
		},
		shape: withClaudePreamble,
	},
}

func SubscriptionVendors() []*SubscriptionVendor { return subscriptionVendors }

func subscriptionVendorOf(id string) (*SubscriptionVendor, error) {
	for _, vendor := range subscriptionVendors {
		if vendor.Id == id {
			return vendor, nil
		}
	}
	return nil, fmt.Errorf("gateway cannot sign in to %q", id)
}

// UsesSubscription reports whether the provider authenticates with a sign-in
// Gateway holds rather than with a key. What reaches the upstream is the same
// either way: the difference is only where the credential came from.
func UsesSubscription(provider *Provider) bool {
	return provider.AuthMode == ProviderAuthSubscription
}

// ProviderSubscription is the sign-in a subscription provider authenticates
// with. It is stored encrypted in one column and never leaves the server.
type ProviderSubscription struct {
	Vendor       string `json:"vendor"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	// AccountId is the workspace the requests are billed to, which the vendor
	// wants beside the token.
	AccountId string `json:"accountId"`
	Account   string `json:"account"`
	Plan      string `json:"plan"`
	ExpiresAt string `json:"expiresAt"`
}

func (subscription *ProviderSubscription) expiry() time.Time {
	at, err := time.Parse(time.RFC3339, subscription.ExpiresAt)
	if err != nil {
		return time.Time{}
	}
	return at
}

// subscriptionOf reads the stored sign-in of a decrypted provider, nil when it
// holds none.
func subscriptionOf(provider *Provider) *ProviderSubscription {
	if provider == nil || strings.TrimSpace(provider.Subscription) == "" {
		return nil
	}

	subscription := &ProviderSubscription{}
	if err := json.Unmarshal([]byte(provider.Subscription), subscription); err != nil {
		fmt.Printf("subscriptionOf(): provider [%s]: %v\n", provider.GetId(), err)
		return nil
	}
	if subscription.AccessToken == "" && subscription.RefreshToken == "" {
		return nil
	}
	return subscription
}

// HasSubscription reports whether a subscription provider is signed in at all.
func HasSubscription(provider *Provider) bool {
	return subscriptionOf(provider) != nil
}

// describeSubscription fills the fields the page shows about a stored sign-in.
// The tokens themselves are left behind: json:"-" keeps the column out of every
// answer, and these two are all a page needs to say whose account it is.
func describeSubscription(provider *Provider) {
	subscription := subscriptionOf(provider)
	if subscription == nil {
		return
	}
	provider.SubscriptionVendor = subscription.Vendor
	provider.SubscriptionAccount = subscription.Account
	provider.SubscriptionPlan = subscription.Plan
}

// subscriptionRefreshes serializes the renewals of one provider, which the
// proxy would otherwise start once per concurrent request.
var subscriptionRefreshes sync.Map

// EnsureSubscription renews the access token of a subscription provider when it
// is spent or about to be, and puts the new one back in the row. It is called
// where the credential is about to be used, so a token is renewed only when
// something is really going to send it.
func EnsureSubscription(provider *Provider) error {
	subscription := subscriptionOf(provider)
	if subscription == nil {
		return fmt.Errorf("provider %s is not signed in", provider.GetId())
	}
	expiry := subscription.expiry()
	if subscription.AccessToken != "" && (expiry.IsZero() || time.Until(expiry) > refreshMargin) {
		return nil
	}
	if subscription.RefreshToken == "" {
		return fmt.Errorf("the sign-in of provider %s has expired, sign in again", provider.GetId())
	}

	id := provider.GetId()
	lock, _ := subscriptionRefreshes.LoadOrStore(id, &sync.Mutex{})
	mutex := lock.(*sync.Mutex)
	mutex.Lock()
	defer mutex.Unlock()

	// Another request may have renewed it while this one waited.
	if stored, err := GetProvider(id); err == nil && stored != nil {
		if fresh := subscriptionOf(stored); fresh != nil && time.Until(fresh.expiry()) > refreshMargin {
			provider.Subscription = stored.Subscription
			return nil
		}
	}

	vendor, err := subscriptionVendorOf(subscription.Vendor)
	if err != nil {
		return err
	}
	granted, err := refreshSubscription(vendor, subscription)
	if err != nil {
		return err
	}

	stored, err := json.Marshal(granted)
	if err != nil {
		return err
	}
	provider.Subscription = string(stored)
	return storeSubscription(provider, granted)
}

// storeSubscription writes the sign-in column alone, so a renewal in the middle
// of a request does not touch what somebody is editing on the page.
func storeSubscription(provider *Provider, subscription *ProviderSubscription) error {
	encrypted, err := encryptedSubscription(provider, subscription)
	if err != nil {
		return err
	}
	_, err = ormer.Engine.ID(core.PK{provider.Owner, provider.Name}).
		Cols("subscription").Update(&Provider{Subscription: encrypted})
	return err
}

// SetSubscriptionAuth puts a held sign-in on an upstream request, in the shape
// the vendor's endpoint reads it: beside the token it wants the account it is
// spent on and the client it was granted to.
func SetSubscriptionAuth(header http.Header, provider *Provider) {
	subscription := subscriptionOf(provider)
	if subscription == nil {
		return
	}
	vendor, err := subscriptionVendorOf(subscription.Vendor)
	if err != nil || vendor.apply == nil {
		return
	}
	vendor.apply(header, subscription)
}

// SubscriptionQuery is what a subscription endpoint wants on the URL, empty for
// one that wants nothing.
func SubscriptionQuery(provider *Provider) string {
	if vendor := vendorOfProvider(provider); vendor != nil {
		return vendor.query
	}
	return ""
}

// ShapeSubscriptionBody rewrites a request the subscription endpoint would
// refuse as it stands. The body is returned unchanged where nothing has to be.
func ShapeSubscriptionBody(provider *Provider, body []byte) []byte {
	vendor := vendorOfProvider(provider)
	if vendor == nil || vendor.shape == nil {
		return body
	}
	return vendor.shape(body)
}

func vendorOfProvider(provider *Provider) *SubscriptionVendor {
	subscription := subscriptionOf(provider)
	if subscription == nil {
		return nil
	}
	vendor, err := subscriptionVendorOf(subscription.Vendor)
	if err != nil {
		return nil
	}
	return vendor
}

// mergeBetas keeps the betas the client asked for and adds the ones the
// subscription endpoint needs, without repeating any.
func mergeBetas(asked string, needed string) string {
	seen := map[string]bool{}
	merged := []string{}
	for _, list := range []string{asked, needed} {
		for _, beta := range strings.Split(list, ",") {
			beta = strings.TrimSpace(beta)
			if beta == "" || seen[beta] {
				continue
			}
			seen[beta] = true
			merged = append(merged, beta)
		}
	}
	return strings.Join(merged, ",")
}

// withClaudePreamble puts the line Anthropic answers a subscription token for at
// the front of the system prompt. Without it the endpoint refuses the request,
// and it is the same line the client this token was granted to sends.
func withClaudePreamble(body []byte) []byte {
	fields := map[string]json.RawMessage{}
	if json.Unmarshal(body, &fields) != nil {
		return body
	}

	preamble := []map[string]string{{"type": "text", "text": claudePreamble}}
	blocks := []map[string]any{}
	switch existing := fields["system"]; {
	case len(existing) == 0:
	default:
		var text string
		if json.Unmarshal(existing, &text) == nil {
			if strings.HasPrefix(strings.TrimSpace(text), claudePreamble) {
				return body
			}
			blocks = append(blocks, map[string]any{"type": "text", "text": text})
			break
		}
		var carried []map[string]any
		if json.Unmarshal(existing, &carried) != nil {
			return body
		}
		if len(carried) > 0 {
			if first, ok := carried[0]["text"].(string); ok && strings.HasPrefix(strings.TrimSpace(first), claudePreamble) {
				return body
			}
		}
		blocks = carried
	}

	system := make([]any, 0, len(blocks)+1)
	system = append(system, preamble[0])
	for _, block := range blocks {
		system = append(system, block)
	}
	encoded, err := json.Marshal(system)
	if err != nil {
		return body
	}
	fields["system"] = encoded

	shaped, err := json.Marshal(fields)
	if err != nil {
		return body
	}
	return shaped
}

// SubscriptionLogin is one sign-in Gateway started, running or finished. It
// takes as long as whoever is at the browser, so the page polls this rather
// than holding a request open.
type SubscriptionLogin struct {
	Id     string `json:"id"`
	Vendor string `json:"vendor"`
	// Url is where the sign-in is approved. The page opens it.
	Url     string `json:"url"`
	Running bool   `json:"running"`
	Ok      bool   `json:"ok"`
	Account string `json:"account,omitempty"`
	Plan    string `json:"plan,omitempty"`
	Error   string `json:"error,omitempty"`

	// granted is what the finished sign-in brought back, taken out of here by
	// the save that stores it on a provider. It never reaches the browser.
	granted *ProviderSubscription
}

type loginSession struct {
	sync.Mutex
	SubscriptionLogin
	verifier string
	state    string
	server   *http.Server
	started  time.Time
}

var logins = struct {
	sync.Mutex
	byId map[string]*loginSession
}{byId: map[string]*loginSession{}}

// StartSubscriptionLogin opens the callback the vendor's client is registered
// with and returns the address to approve the sign-in at.
func StartSubscriptionLogin(vendorId string) (*SubscriptionLogin, error) {
	vendor, err := subscriptionVendorOf(vendorId)
	if err != nil {
		return nil, err
	}

	id, err := randomString()
	if err != nil {
		return nil, err
	}
	state, err := randomString()
	if err != nil {
		return nil, err
	}
	verifier, err := randomString()
	if err != nil {
		return nil, err
	}

	listener, err := listenForCallback(vendor)
	if err != nil {
		return nil, err
	}

	session := &loginSession{
		SubscriptionLogin: SubscriptionLogin{
			Id:      id,
			Vendor:  vendor.Id,
			Url:     authorizeSubscriptionUrl(vendor, state, verifier),
			Running: true,
		},
		verifier: verifier,
		state:    state,
		started:  time.Now(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc(vendor.callbackPath, func(writer http.ResponseWriter, request *http.Request) {
		query := request.URL.Query()
		// This vendor returns the state after a "#", which no browser sends to a
		// server. The page below asks for the same address again with it in the
		// query, and that request is the one that finishes the sign-in.
		if vendor.stateInFragment && query.Get("state") == "" && query.Get("error") == "" {
			writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(writer, fragmentPage)
			return
		}
		finishSubscriptionLogin(session, vendor, query, writer)
	})
	session.server = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	logins.Lock()
	forgetFinishedLoginsLocked()
	logins.byId[id] = session
	logins.Unlock()

	go func() {
		_ = session.server.Serve(listener)
	}()
	// A sign-in nobody finishes must not hold the port forever: the vendor's own
	// client wants it back.
	time.AfterFunc(pendingTtl, func() {
		session.Lock()
		running := session.Running
		if running {
			session.Running = false
			session.Error = "the sign-in was not finished in time"
		}
		session.Unlock()
		_ = session.server.Close()
	})

	answer := session.snapshot()
	return &answer, nil
}

// listenForCallback takes the address this vendor's client is registered with.
// A sign-in nobody finished is still holding it, so it is dropped first rather
// than left to fail the next attempt for ten minutes.
func listenForCallback(vendor *SubscriptionVendor) (net.Listener, error) {
	cancelRunningLogins(vendor.Id)

	var err error
	for attempt := 0; attempt < 10; attempt++ {
		var listener net.Listener
		listener, err = net.Listen("tcp", vendor.listenAddr)
		if err == nil {
			return listener, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil, fmt.Errorf("gateway cannot take %s to finish the sign-in on, which is the address %s registered: %v",
		vendor.listenAddr, vendor.Label, err)
}

// cancelRunningLogins drops the sign-ins of one vendor that are still waiting
// for a browser, which is what frees its port.
func cancelRunningLogins(vendorId string) {
	logins.Lock()
	started := make([]*loginSession, 0, len(logins.byId))
	for _, session := range logins.byId {
		if session.Vendor == vendorId {
			started = append(started, session)
		}
	}
	logins.Unlock()

	for _, session := range started {
		session.Lock()
		running := session.Running
		if running {
			session.Running = false
			session.Error = "another sign-in was started"
		}
		session.Unlock()
		if running {
			_ = session.server.Close()
		}
	}
}

// SubscriptionLoginSession is how a sign-in that was started is getting on.
func SubscriptionLoginSession(id string) (SubscriptionLogin, bool) {
	logins.Lock()
	session, ok := logins.byId[id]
	logins.Unlock()
	if !ok {
		return SubscriptionLogin{}, false
	}
	return session.snapshot(), true
}

// takeSubscriptionLogin hands the credential of a finished sign-in to the save
// that stores it, and forgets the session: one sign-in fills one provider.
func takeSubscriptionLogin(id string) (*ProviderSubscription, error) {
	logins.Lock()
	session, ok := logins.byId[id]
	logins.Unlock()
	if !ok {
		return nil, fmt.Errorf("no sign-in was started under this id")
	}

	session.Lock()
	defer session.Unlock()
	if session.Running {
		return nil, fmt.Errorf("this sign-in is not finished yet")
	}
	if session.granted == nil {
		return nil, fmt.Errorf("this sign-in did not finish: %s", session.Error)
	}
	granted := session.granted
	session.granted = nil

	logins.Lock()
	delete(logins.byId, id)
	logins.Unlock()
	return granted, nil
}

func (session *loginSession) snapshot() SubscriptionLogin {
	session.Lock()
	defer session.Unlock()
	answer := session.SubscriptionLogin
	answer.granted = nil
	return answer
}

func (session *loginSession) finish(granted *ProviderSubscription, err error) {
	session.Lock()
	session.Running = false
	if err != nil {
		session.Error = err.Error()
	} else {
		session.Ok = true
		session.granted = granted
		session.Account = granted.Account
		session.Plan = granted.Plan
	}
	session.Unlock()
	go func() {
		_ = session.server.Close()
	}()
}

func finishSubscriptionLogin(session *loginSession, vendor *SubscriptionVendor, query url.Values, writer http.ResponseWriter) {
	granted, err := grantOf(session, vendor, query)
	session.finish(granted, err)

	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(writer, callbackPage, "Sign-in failed", err.Error())
		return
	}
	fmt.Fprintf(writer, callbackPage, "Signed in", "You can close this page and go back to Casbin Gateway.")
}

func grantOf(session *loginSession, vendor *SubscriptionVendor, query url.Values) (*ProviderSubscription, error) {
	if failure := query.Get("error"); failure != "" {
		return nil, fmt.Errorf("%s %s", failure, query.Get("error_description"))
	}
	if query.Get("state") != session.state {
		return nil, fmt.Errorf("this sign-in was not the one started here")
	}
	code := query.Get("code")
	if code == "" {
		return nil, fmt.Errorf("the vendor sent no authorization code back")
	}

	if vendor.codeJson {
		body, err := json.Marshal(map[string]string{
			"grant_type":    "authorization_code",
			"code":          code,
			"state":         session.state,
			"redirect_uri":  vendor.redirectUri,
			"client_id":     vendor.clientId,
			"code_verifier": session.verifier,
		})
		if err != nil {
			return nil, err
		}
		return postSubscriptionToken(vendor, "application/json", strings.NewReader(string(body)), nil)
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", vendor.redirectUri)
	form.Set("client_id", vendor.clientId)
	form.Set("code_verifier", session.verifier)

	return postSubscriptionToken(vendor, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()), nil)
}

func refreshSubscription(vendor *SubscriptionVendor, subscription *ProviderSubscription) (*ProviderSubscription, error) {
	body, err := json.Marshal(map[string]string{
		"client_id":     vendor.clientId,
		"grant_type":    "refresh_token",
		"refresh_token": subscription.RefreshToken,
	})
	if err != nil {
		return nil, err
	}
	return postSubscriptionToken(vendor, "application/json", strings.NewReader(string(body)), subscription)
}

// postSubscriptionToken asks the vendor's token endpoint for tokens and reads
// who they belong to out of them. previous carries what a renewal keeps: the
// account details, and the refresh token when a renewal does not issue one.
func postSubscriptionToken(vendor *SubscriptionVendor, contentType string, body io.Reader, previous *ProviderSubscription) (*ProviderSubscription, error) {
	request, err := http.NewRequest(http.MethodPost, vendor.tokenUrl, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("Accept", "application/json")
	for name, value := range vendor.tokenHeaders {
		request.Header.Set(name, value)
	}

	client := &http.Client{Timeout: exchangeTimeout, Transport: proxy.Transport()}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	answer, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
	if err != nil {
		return nil, err
	}
	var granted struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IdToken      string `json:"id_token"`
		ExpiresIn    int64  `json:"expires_in"`
		Account      struct {
			EmailAddress string `json:"email_address"`
		} `json:"account"`
		Error     string `json:"error"`
		ErrorText string `json:"error_description"`
	}
	readable := json.Unmarshal(answer, &granted) == nil

	if readable && granted.Error != "" {
		return nil, fmt.Errorf("the sign-in was refused: %s %s", granted.Error, granted.ErrorText)
	}
	if response.StatusCode >= 400 {
		return nil, fmt.Errorf("the token endpoint answered %s: %s", response.Status, summarize(answer))
	}
	if !readable || granted.AccessToken == "" {
		return nil, fmt.Errorf("the token endpoint returned no access token: %s", summarize(answer))
	}

	subscription := &ProviderSubscription{
		Vendor:       vendor.Id,
		AccessToken:  granted.AccessToken,
		RefreshToken: granted.RefreshToken,
	}
	if previous != nil {
		subscription.AccountId = previous.AccountId
		subscription.Account = previous.Account
		subscription.Plan = previous.Plan
		if subscription.RefreshToken == "" {
			subscription.RefreshToken = previous.RefreshToken
		}
	}
	if granted.ExpiresIn > 0 {
		subscription.ExpiresAt = time.Now().Add(time.Duration(granted.ExpiresIn) * time.Second).UTC().Format(time.RFC3339)
	}
	fillClaim(&subscription.Account, granted.Account.EmailAddress)
	describeGrant(subscription, granted.IdToken)
	return subscription, nil
}

// describeGrant reads whose account this is out of the tokens themselves. The
// id token carries the profile, the access token carries when it dies.
func describeGrant(subscription *ProviderSubscription, idToken string) {
	if expiry, ok := agent.DecodeJWTClaims(subscription.AccessToken)["exp"].(float64); ok && subscription.ExpiresAt == "" {
		subscription.ExpiresAt = time.Unix(int64(expiry), 0).UTC().Format(time.RFC3339)
	}

	for _, token := range []string{idToken, subscription.AccessToken} {
		claims := agent.DecodeJWTClaims(token)
		if claims == nil {
			continue
		}
		if auth, ok := claims["https://api.openai.com/auth"].(map[string]any); ok {
			fillClaim(&subscription.AccountId, auth["chatgpt_account_id"])
			fillClaim(&subscription.Plan, auth["chatgpt_plan_type"])
		}
		if profile, ok := claims["https://api.openai.com/profile"].(map[string]any); ok {
			fillClaim(&subscription.Account, profile["email"])
		}
		fillClaim(&subscription.Account, claims["email"])
	}
}

func fillClaim(target *string, value any) {
	if *target != "" {
		return
	}
	if text, ok := value.(string); ok && text != "" {
		*target = text
	}
}

func authorizeSubscriptionUrl(vendor *SubscriptionVendor, state string, verifier string) string {
	query := url.Values{}
	query.Set("response_type", "code")
	query.Set("client_id", vendor.clientId)
	query.Set("redirect_uri", vendor.redirectUri)
	query.Set("scope", vendor.scope)
	query.Set("code_challenge", challengeOf(verifier))
	query.Set("code_challenge_method", "S256")
	query.Set("state", state)
	for name, value := range vendor.extraParams {
		query.Set(name, value)
	}
	return vendor.authorizeUrl + "?" + query.Encode()
}

// forgetFinishedLoginsLocked drops what is too old to still be polled. A sign-in
// that finished stays until then, so the page that started it reads how it went
// rather than "no such sign-in".
func forgetFinishedLoginsLocked() {
	for id, session := range logins.byId {
		if time.Since(session.started) > pendingTtl {
			delete(logins.byId, id)
		}
	}
}

// fragmentPage hands back the part of the address the browser kept to itself.
const fragmentPage = `<!doctype html><meta charset="utf-8"><title>Signing in</title>` +
	`<body style="font:16px system-ui;margin:4rem auto;max-width:32rem;text-align:center">` +
	`<p style="color:#666">Finishing the sign-in...</p><script>` +
	`var hash=location.hash.slice(1);` +
	`if(hash){location.replace(location.pathname+location.search+"&state="+encodeURIComponent(hash));}` +
	`else{document.body.innerHTML="<p>This sign-in carried no state, so it cannot be finished. Start it again.</p>";}` +
	`</script>`

const callbackPage = `<!doctype html><meta charset="utf-8"><title>%[1]s</title>` +
	`<body style="font:16px system-ui;margin:4rem auto;max-width:32rem;text-align:center">` +
	`<h1 style="font-size:1.25rem">%[1]s</h1><p style="color:#666">%[2]s</p>`
