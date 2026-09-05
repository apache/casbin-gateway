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
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/apache/casbin-gateway/agentpatch"
	"github.com/apache/casbin-gateway/connector"
	"github.com/apache/casbin-gateway/proxy"
	"github.com/apache/casbin-gateway/util"
)

// CallbackPath is where a vendor sends the operator back after they approve.
// It is loopback, so the application they register on the vendor's own platform
// names this machine and nothing of ours is in the middle.
const CallbackPath = "/api/connector-auth-callback"

const (
	// pendingTtl is how long an operator has to finish approving before the
	// request they started is forgotten.
	pendingTtl = 10 * time.Minute
	// refreshMargin renews a token that is about to expire rather than one that
	// already has, so a session does not start on a credential that dies during
	// its first call.
	refreshMargin   = 2 * time.Minute
	exchangeTimeout = 30 * time.Second
)

// pendingAuth is one authorization the operator has been sent off to approve.
// It lives in memory only: a request nobody finished is nothing to keep, and a
// verifier written to disk would outlive the exchange it protects.
type pendingAuth struct {
	Owner    string
	Name     string
	Verifier string
	Started  time.Time
	Redirect string
}

var pending = struct {
	sync.Mutex
	byState map[string]pendingAuth
}{byState: map[string]pendingAuth{}}

// RedirectUri is what the operator registers on the vendor's platform. It is
// shown in the connect dialog, because an application registered with a
// different one fails at the end of the flow rather than the start.
func RedirectUri() (string, error) {
	base, err := agentpatch.GatewayBaseUrl()
	if err != nil {
		return "", err
	}
	return base + CallbackPath, nil
}

// StartConnectorAuth stores the client application the operator registered and
// returns the address to send them to. Nothing is authorized yet: the
// connection exists with its client credentials and no token.
func StartConnectorAuth(owner string, name string, credentials map[string]string) (string, error) {
	found, ok := connector.Get(name)
	if !ok {
		return "", fmt.Errorf("no connector named %q", name)
	}
	if found.Auth.Kind != connector.AuthOauth2 {
		return "", fmt.Errorf("%s is not authorized this way", name)
	}

	connection, err := GetConnection(owner, name)
	if err != nil {
		return "", err
	}
	if connection == nil {
		connection = &Connection{Owner: owner, Name: name}
	}
	connection.Credentials = mergeCredentials(connection.Credentials, credentials)
	if strings.TrimSpace(connection.Credentials[connector.KeyClientId]) == "" {
		return "", fmt.Errorf("the client application's %s is needed first", connector.KeyClientId)
	}
	// Saved before the operator leaves for the vendor, so the callback has the
	// client secret to exchange the code with when they come back.
	if err := saveWithoutRendering(connection); err != nil {
		return "", err
	}

	redirect, err := RedirectUri()
	if err != nil {
		return "", err
	}
	state, err := randomString()
	if err != nil {
		return "", err
	}
	verifier, err := randomString()
	if err != nil {
		return "", err
	}

	pending.Lock()
	forgetExpiredLocked()
	pending.byState[state] = pendingAuth{
		Owner: owner, Name: name, Verifier: verifier, Started: time.Now(), Redirect: redirect,
	}
	pending.Unlock()

	return authorizeUrl(found, connection.Credentials[connector.KeyClientId], redirect, state, verifier)
}

func authorizeUrl(found connector.Connector, clientId string, redirect string, state string, verifier string) (string, error) {
	parsed, err := url.Parse(found.Auth.AuthorizeUrl)
	if err != nil {
		return "", err
	}

	query := parsed.Query()
	query.Set("response_type", "code")
	query.Set("client_id", clientId)
	query.Set("redirect_uri", redirect)
	query.Set("state", state)
	if scopes := found.ScopeList(); scopes != "" {
		query.Set("scope", scopes)
	}
	// PKCE costs nothing where it is ignored and is what stops a code seen on
	// this machine from being redeemed by anything else.
	query.Set("code_challenge", challengeOf(verifier))
	query.Set("code_challenge_method", "S256")
	for name, value := range found.Auth.ExtraParams {
		query.Set(name, value)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

// CompleteConnectorAuth exchanges the code the vendor sent back and stores the
// tokens. It returns the connection the grant belongs to so the caller can say
// which one just became usable.
func CompleteConnectorAuth(state string, code string) (*Connection, error) {
	pending.Lock()
	forgetExpiredLocked()
	request, found := pending.byState[state]
	delete(pending.byState, state)
	pending.Unlock()

	if !found {
		return nil, fmt.Errorf("this authorization was not started here, or took too long")
	}

	connection, err := GetConnection(request.Owner, request.Name)
	if err != nil {
		return nil, err
	}
	if connection == nil {
		return nil, fmt.Errorf("%q is no longer connected", request.Name)
	}
	entry, ok := connector.Get(request.Name)
	if !ok {
		return nil, fmt.Errorf("no connector named %q", request.Name)
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", request.Redirect)
	form.Set("code_verifier", request.Verifier)

	granted, err := exchange(entry, connection.Credentials, form)
	if err != nil {
		return nil, err
	}
	connection.Credentials = mergeCredentials(connection.Credentials, granted)
	if err := saveWithoutRendering(connection); err != nil {
		return nil, err
	}
	return connection, nil
}

// refreshIfNeeded renews an access token that has expired or is about to. It is
// called where the credential is handed out rather than on a timer, so a token
// is only renewed when something is actually about to use it.
func refreshIfNeeded(connection *Connection, found connector.Connector) error {
	if found.Auth.Kind != connector.AuthOauth2 {
		return nil
	}
	refreshToken := strings.TrimSpace(connection.Credentials[connector.KeyRefreshToken])
	if refreshToken == "" {
		// A grant that never expires, or one the service will not renew. Either
		// way there is nothing to do until it stops working.
		return nil
	}
	expiry := strings.TrimSpace(connection.Credentials[connector.KeyExpiresAt])
	if expiry == "" {
		return nil
	}
	at, err := time.Parse(time.RFC3339, expiry)
	if err != nil || time.Until(at) > refreshMargin {
		return nil
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)

	granted, err := exchange(found, connection.Credentials, form)
	if err != nil {
		return fmt.Errorf("renewing the authorization of %s failed: %w", connection.Name, err)
	}
	connection.Credentials = mergeCredentials(connection.Credentials, granted)
	return saveWithoutRendering(connection)
}

// exchange posts to the token endpoint and reads what came back. It is shared
// by the first exchange and every renewal, which differ only in the grant.
func exchange(found connector.Connector, credentials map[string]string, form url.Values) (map[string]string, error) {
	clientId := credentials[connector.KeyClientId]
	clientSecret := credentials[connector.KeyClientSecret]

	if found.Auth.TokenAuth != connector.TokenAuthBasic {
		form.Set("client_id", clientId)
		if clientSecret != "" {
			form.Set("client_secret", clientSecret)
		}
	}

	request, err := http.NewRequest(http.MethodPost, found.Auth.TokenUrl, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Some token endpoints answer in form encoding unless asked otherwise.
	request.Header.Set("Accept", "application/json")
	if found.Auth.TokenAuth == connector.TokenAuthBasic {
		request.SetBasicAuth(clientId, clientSecret)
	}

	client := &http.Client{Timeout: exchangeTimeout, Transport: proxy.Transport()}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
	if err != nil {
		return nil, err
	}
	var granted struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		Error        string `json:"error"`
		ErrorText    string `json:"error_description"`
	}
	readable := json.Unmarshal(body, &granted) == nil

	// The body is read before the status, because the status is the less
	// informative half of the answer: an error comes back with 200 more often
	// than it should, and GitHub answers an application it does not know with
	// 404 and nothing but {"error":"Not Found"}. Reporting the status first
	// would send somebody looking at the URL when the client id is the problem.
	if readable && granted.Error != "" {
		return nil, fmt.Errorf("the token endpoint refused this application: %s %s",
			granted.Error, granted.ErrorText)
	}
	if response.StatusCode >= 400 {
		return nil, fmt.Errorf("the token endpoint answered %s: %s", response.Status, summarize(body))
	}
	if !readable {
		return nil, fmt.Errorf("the token endpoint did not answer with JSON: %s", summarize(body))
	}
	if granted.AccessToken == "" {
		return nil, fmt.Errorf("the token endpoint returned no access token")
	}

	result := map[string]string{connector.KeyAccessToken: granted.AccessToken}
	// A renewal that returns no new refresh token keeps the one it was given,
	// which is what merging leaves alone.
	if granted.RefreshToken != "" {
		result[connector.KeyRefreshToken] = granted.RefreshToken
	}
	if granted.ExpiresIn > 0 {
		result[connector.KeyExpiresAt] = time.Now().Add(time.Duration(granted.ExpiresIn) * time.Second).Format(time.RFC3339)
	}
	return result, nil
}

// saveWithoutRendering stores a connection that is not usable yet. SaveConnection
// refuses one whose credentials do not render, which is right when the operator
// is finishing a form and wrong halfway through an authorization.
func saveWithoutRendering(connection *Connection) error {
	connection.UpdatedTime = util.GetCurrentTime()
	if connection.Credentials == nil {
		connection.Credentials = map[string]string{}
	}

	existing, err := GetConnection(connection.Owner, connection.Name)
	if err != nil {
		return err
	}
	if err := encryptCredentials(connection); err != nil {
		return err
	}
	if existing == nil {
		connection.CreatedTime = util.GetCurrentTime()
		_, err = ormer.Engine.Insert(connection)
		return err
	}
	return updateConnection(connection)
}

// mergeCredentials layers new values over old, ignoring a masked value and an
// empty one: an edit that only resends the client id must not erase the token
// beside it.
func mergeCredentials(existing map[string]string, incoming map[string]string) map[string]string {
	merged := map[string]string{}
	for key, value := range existing {
		merged[key] = value
	}
	for key, value := range incoming {
		if value == "" || value == ApiKeyMask {
			continue
		}
		merged[key] = value
	}
	return merged
}

func forgetExpiredLocked() {
	for state, request := range pending.byState {
		if time.Since(request.Started) > pendingTtl {
			delete(pending.byState, state)
		}
	}
}

func randomString() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func challengeOf(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// summarize keeps an endpoint's error readable in a message without pasting a
// whole page into it.
func summarize(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) > 300 {
		return text[:300] + "..."
	}
	return text
}
