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

import "strings"

// ProbeEndpoint runs the shipped suite against a bare endpoint and key. Nothing
// is stored: this is the probe for someone who has not installed Gateway.
func ProbeEndpoint(baseUrl string, apiKey string, protocol string, model string) *ProviderProbe {
	provider := &Provider{
		Owner:    "probe",
		Name:     "endpoint",
		Type:     "custom",
		BaseUrl:  strings.TrimSpace(baseUrl),
		ApiKey:   strings.TrimSpace(apiKey),
		AuthMode: ProviderAuthProvider,
		Status:   "enabled",
	}
	if protocol == ProtocolAnthropic {
		provider.Type = "anthropic"
	}
	if model = strings.TrimSpace(model); model != "" {
		provider.Models = []string{model}
	}
	return runProviderProbe(provider, ProbeTriggerManual)
}
