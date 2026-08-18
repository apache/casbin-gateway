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

package agentconfig

// Status describes the configuration managed by an Adapter.
type Status struct {
	TakenOver bool   `json:"takenOver"`
	Endpoint  string `json:"endpoint,omitempty"`
}

// Adapter switches an agent to Gateway and restores its previous configuration.
type Adapter interface {
	Takeover(endpoint, token string) error
	Restore() error
	Status() (Status, error)
}
