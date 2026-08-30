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

// Package auth is a thin wrapper around the identity-provider SDK.
// All calls to the upstream SDK go through this package, so the rest of the
// codebase never imports casdoorsdk directly and the "no provider configured"
// guards live in one place.
package auth

import (
	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
	"golang.org/x/oauth2"
)

// Type aliases — re-export all SDK types used across the codebase so that
// other packages only need to import this package, not casdoorsdk directly.
type (
	Claims = casdoorsdk.Claims
	User   = casdoorsdk.User
)

// InitConfig initialises the identity-provider SDK client.
func InitConfig(endpoint, clientId, clientSecret, jwtPublicKey, organization, application string) {
	casdoorsdk.InitConfig(endpoint, clientId, clientSecret, jwtPublicKey, organization, application)
}

func GetOAuthToken(code, state string) (*oauth2.Token, error) {
	return casdoorsdk.GetOAuthToken(code, state)
}

func ParseJwtToken(token string) (*Claims, error) {
	return casdoorsdk.ParseJwtToken(token)
}
