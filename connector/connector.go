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

// Package connector holds the catalog of applications an agent can be
// connected to. A connector is data, not code: one JSON file naming the
// service, how it is authenticated, and the MCP server that reaches it.
package connector

import "strings"

// How a connection is authenticated.
const (
	// AuthNone is a server that needs no credential at all.
	AuthNone = "none"
	// AuthToken is a credential the operator pastes in: an API key, a personal
	// access token, a database URL.
	AuthToken = "token"
	// AuthOauth2 is the authorization code flow. The operator registers an
	// application on the vendor's own platform and Gateway runs the flow with
	// it, so no client secret of ours is shipped in the binary and the grant
	// stays inside their own tenant.
	AuthOauth2 = "oauth2"
)

// The catalog's own sections, which the page groups its cards by.
const (
	CategoryDev     = "dev"
	CategoryDocs    = "docs"
	CategoryOffice  = "office"
	CategoryMail    = "mail"
	CategoryStorage = "storage"
	CategoryLife    = "life"
	// CategoryPro is what a business runs on rather than what it builds with:
	// payments, filings, the services somebody is billed for using.
	CategoryPro = "pro"
)

var categories = map[string]bool{
	CategoryDev:     true,
	CategoryDocs:    true,
	CategoryOffice:  true,
	CategoryMail:    true,
	CategoryStorage: true,
	CategoryLife:    true,
	CategoryPro:     true,
}

// Connector is one entry in the catalog.
type Connector struct {
	Id          string `json:"id"`
	DisplayName string `json:"displayName"`
	Category    string `json:"category"`
	// Icon is a site the favicon is taken from, spelled like a provider's.
	Icon        string `json:"icon"`
	Description string `json:"description"`
	DocsUrl     string `json:"docsUrl,omitempty"`
	// Paid marks a service that bills for its own API, which the card says
	// before anybody connects it rather than after.
	Paid bool `json:"paid,omitempty"`
	// Unverified marks an entry whose server has not been run against the live
	// service by us. It is listed, and the card says so.
	Unverified bool `json:"unverified,omitempty"`

	Auth   Auth       `json:"auth"`
	Server ServerSpec `json:"server"`
}

// The credential keys Gateway itself fills in once an authorization has been
// granted. They are reserved: a connector's own fields may not use these names,
// and a server template refers to them like any other value.
const (
	KeyAccessToken  = "accessToken"
	KeyRefreshToken = "refreshToken"
	// KeyExpiresAt is RFC3339, empty when the grant does not expire.
	KeyExpiresAt = "expiresAt"
)

// The client application an oauth2 connector is authorized through. The
// operator registers it on the vendor's own platform and fills these in, which
// is what keeps the grant inside their tenant and no secret of ours in this
// binary.
const (
	KeyClientId     = "clientId"
	KeyClientSecret = "clientSecret"
)

// ReservedKeys is every credential Gateway writes rather than the operator.
var ReservedKeys = []string{KeyAccessToken, KeyRefreshToken, KeyExpiresAt}

// How the client credentials are presented when a code is exchanged. Most
// services take them in the form body; some want HTTP Basic instead, and one
// spelled the wrong way is refused with an error that names neither.
const (
	TokenAuthBody  = "body"
	TokenAuthBasic = "basic"
)

// Auth is how one connector proves who it is.
type Auth struct {
	Kind string `json:"kind"`
	// Fields is what the operator fills in, for AuthToken and for the client
	// application half of AuthOauth2.
	Fields []Field `json:"fields,omitempty"`

	// The rest is the OAuth flow, empty for every other kind.
	AuthorizeUrl string            `json:"authorizeUrl,omitempty"`
	TokenUrl     string            `json:"tokenUrl,omitempty"`
	Scopes       []string          `json:"scopes,omitempty"`
	ExtraParams  map[string]string `json:"extraParams,omitempty"`
	// TokenAuth is how the client credentials reach the token endpoint, empty
	// for the usual form body.
	TokenAuth string `json:"tokenAuth,omitempty"`
	// ScopeSeparator is what joins the scopes, empty for the space the
	// specification asks for. A few services only accept commas.
	ScopeSeparator string `json:"scopeSeparator,omitempty"`
	// RegisterUrl is where the operator creates the application these
	// credentials come from, linked from the connect dialog.
	RegisterUrl string `json:"registerUrl,omitempty"`
}

// Field is one value the operator supplies.
type Field struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Placeholder string `json:"placeholder,omitempty"`
	Help        string `json:"help,omitempty"`
	// Secret keeps the value out of every response once it is stored.
	Secret   bool `json:"secret,omitempty"`
	Required bool `json:"required,omitempty"`
}

// ServerSpec is the MCP server that reaches the service, as a template. A
// "${field}" in any value is replaced by the credential of that name.
type ServerSpec struct {
	// Name is what the entry is called in an agent's own configuration.
	Name      string            `json:"name"`
	Transport string            `json:"transport"`
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Url       string            `json:"url,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
}

// NeedsAuth reports whether connecting this one asks for anything.
func (c Connector) NeedsAuth() bool { return c.Auth.Kind != AuthNone }

// SecretKeys is every credential of this connector that is never returned once
// stored: the secret fields the operator filled in, and every token Gateway
// obtained on their behalf.
func (c Connector) SecretKeys() []string {
	keys := []string{}
	for _, field := range c.Auth.Fields {
		if field.Secret {
			keys = append(keys, field.Key)
		}
	}
	if c.Auth.Kind == AuthOauth2 {
		keys = append(keys, KeyAccessToken, KeyRefreshToken)
	}
	return keys
}

// ScopeList is the scope parameter this connector asks for.
func (c Connector) ScopeList() string {
	separator := c.Auth.ScopeSeparator
	if separator == "" {
		separator = " "
	}
	return strings.Join(c.Auth.Scopes, separator)
}
