// Copyright 2023 The casbin Authors. All Rights Reserved.
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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apache/casbin-gateway/object"
)

func TestModelEndpointCandidates(t *testing.T) {
	actual, err := object.BuildModelEndpointCandidates("https://api.deepseek.com/anthropic", "custom")
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{
		"https://api.deepseek.com/anthropic/v1/models",
		"https://api.deepseek.com/anthropic/models",
		"https://api.deepseek.com/v1/models",
		"https://api.deepseek.com/models",
	}
	if len(actual) != len(expected) {
		t.Fatalf("got %v, want %v", actual, expected)
	}
	for index := range expected {
		if actual[index] != expected[index] {
			t.Errorf("candidate %d = %q, want %q", index, actual[index], expected[index])
		}
	}
}

func TestFetchModelsCandidateUsesProviderHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "provider-key" {
			t.Errorf("x-api-key = %q", r.Header.Get("X-Api-Key"))
		}
		if r.Header.Get("Authorization") != "" {
			t.Errorf("unexpected Authorization header: %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Anthropic-Version") != "2023-06-01" {
			t.Errorf("anthropic-version = %q", r.Header.Get("Anthropic-Version"))
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"model-b"},{"id":"model-a"}]}`))
	}))
	defer server.Close()

	models, err := fetchModelsCandidate(server.Client(), server.URL, "provider-key", "x-api-key")
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 || models[0] != "model-a" || models[1] != "model-b" {
		t.Fatalf("models = %v", models)
	}
}

func TestCopyRequestHeaders(t *testing.T) {
	source := http.Header{
		"Authorization":     {"client-token"},
		"X-Api-Key":         {"client-key"},
		"Connection":        {"keep-alive, X-Remove-Me"},
		"X-Remove-Me":       {"secret"},
		"Anthropic-Beta":    {"tools-2026"},
		"Anthropic-Version": {"2023-06-01"},
		"User-Agent":        {"claude-code"},
	}
	destination := http.Header{}
	copyRequestHeaders(destination, source)
	for _, name := range []string{"Authorization", "X-Api-Key", "Connection", "X-Remove-Me"} {
		if destination.Get(name) != "" {
			t.Errorf("%s was forwarded", name)
		}
	}
	for _, name := range []string{"Anthropic-Beta", "Anthropic-Version", "User-Agent"} {
		if destination.Get(name) != source.Get(name) {
			t.Errorf("%s was not preserved", name)
		}
	}
}
