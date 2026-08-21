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

package controllers

import (
	"testing"

	"github.com/apache/casbin-gateway/object"
)

func chId(channels []*object.Channel) []string {
	ids := make([]string, len(channels))
	for i, channel := range channels {
		ids[i] = channel.GetId()
	}
	return ids
}

func TestAgentChannels(t *testing.T) {
	bound := &object.Channel{Owner: "admin", Name: "primary"}
	c1 := &object.Channel{Owner: "admin", Name: "alt1"}
	c2 := &object.Channel{Owner: "admin", Name: "alt2"}

	tests := []struct {
		name      string
		fallbacks []*object.Channel
		want      []string
	}{
		{
			name: "no fallbacks",
			want: []string{"admin/primary"},
		},
		{
			name:      "bound first then fallbacks in order",
			fallbacks: []*object.Channel{c1, c2},
			want:      []string{"admin/primary", "admin/alt1", "admin/alt2"},
		},
		{
			name:      "bound is dropped from fallbacks so it is not retried twice",
			fallbacks: []*object.Channel{c1, bound, c2},
			want:      []string{"admin/primary", "admin/alt1", "admin/alt2"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := chId(agentChannels(bound, test.fallbacks))
			if len(got) != len(test.want) {
				t.Fatalf("got %v, want %v", got, test.want)
			}
			for i, id := range test.want {
				if got[i] != id {
					t.Errorf("position %d = %q, want %q (full: %v)", i, got[i], id, got)
				}
			}
		})
	}
}
