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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"slices"

	"github.com/apache/casbin-gateway/agentprovider"
)

// A stable name and an insert-only save make a retried import idempotent.
func ImportNativeProvider(owner, agentId string, native *agentprovider.NativeProvider) (string, error) {
	digest := sha256.Sum256([]byte(native.BaseUrl))
	candidate := &Provider{
		Owner:               owner,
		Name:                "native-" + agentId + "-" + hex.EncodeToString(digest[:6]),
		DisplayName:         native.DisplayName,
		Protocol:            native.Protocol,
		BaseUrl:             native.BaseUrl,
		AnthropicAuthHeader: native.AnthropicAuthHeader,
		ApiKey:              native.ApiKey,
		Status:              "enabled",
		AuthMode:            ProviderAuthProvider,
		Type:                "custom",
	}
	switch native.Protocol {
	case ProtocolAnthropic:
		candidate.Type = "anthropic"
	case ProtocolResponses:
		candidate.Type = ProviderTypeOpenAiResponses
	}
	if native.Model != "" {
		candidate.Models = []string{native.Model}
	}
	if err := validateProvider(candidate); err != nil {
		return "", err
	}
	id := candidate.GetId()
	stored, err := getProvider(owner, candidate.Name)
	if err != nil {
		return "", err
	}
	if stored == nil {
		toInsert := *candidate
		added, insertErr := insertProvider(&toInsert)
		if insertErr == nil && added {
			return id, nil
		}
		// Another import may have just inserted the same provider.
		stored, err = getProvider(owner, candidate.Name)
		if err != nil || stored == nil {
			return "", errors.New("could not save the native provider")
		}
	}
	if stored.BaseUrl != candidate.BaseUrl || stored.DisplayName != candidate.DisplayName ||
		ProviderProtocol(stored) != candidate.Protocol || stored.AuthMode != candidate.AuthMode ||
		stored.AnthropicAuthHeader != candidate.AnthropicAuthHeader || stored.ApiKey != candidate.ApiKey ||
		!slices.Equal(stored.Models, candidate.Models) {
		return "", errors.New("a previously imported provider has different settings; review it on the Providers page")
	}
	return id, nil
}
