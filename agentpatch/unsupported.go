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

package agentpatch

func init() {
	for _, id := range []string{"cursor", "cursor-agent", "windsurf"} {
		register(unimplemented{id: id})
	}
}

type unimplemented struct {
	id string
}

func (p unimplemented) AgentId() string { return p.id }

func (unimplemented) Supported() bool { return false }

func (unimplemented) Status(Target) (Status, error) {
	return Status{Detail: "not implemented yet"}, nil
}

func (unimplemented) Patch(Target) error { return ErrNotSupported }

func (unimplemented) Unpatch(Target) error { return ErrNotSupported }
