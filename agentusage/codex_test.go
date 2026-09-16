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

package agentusage

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// signedIn is a ChatGPT sign-in cut down to the fields the usage call reads.
const signedIn = `{"tokens":{"access_token":"token-abc","account_id":"account-1","refresh_token":"r"}}`

const usageReply = `{
  "plan_type": "plus",
  "rate_limit": {
    "allowed": true,
    "primary_window": {"used_percent": 28.5, "limit_window_seconds": 18000, "reset_after_seconds": 7971, "reset_at": 1789485149},
    "secondary_window": {"used_percent": 6, "limit_window_seconds": 604800, "reset_after_seconds": 560539, "reset_at": 1790037717}
  },
  "credits": {"has_credits": false, "balance": "0"},
  "rate_limit_reset_credits": {"available_count": 2, "applicable_available_count": 0}
}`

// serveUsage points the usage call at a local handler for one test.
func serveUsage(t *testing.T, handler http.HandlerFunc) {
	t.Helper()

	server := httptest.NewServer(handler)
	previousUrl, previousClient := codexUsageUrl, usageClient
	codexUsageUrl, usageClient = server.URL, server.Client()
	t.Cleanup(func() {
		codexUsageUrl, usageClient = previousUrl, previousClient
		server.Close()
	})
}

func TestCodexReadsWhatIsLeft(t *testing.T) {
	var got *http.Request
	serveUsage(t, func(writer http.ResponseWriter, request *http.Request) {
		got = request.Clone(request.Context())
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(usageReply))
	})

	usage, err := Codex(signedIn)
	if err != nil {
		t.Fatalf("Codex() = %v", err)
	}

	if got.Header.Get("Authorization") != "Bearer token-abc" {
		t.Errorf("Authorization = %q", got.Header.Get("Authorization"))
	}
	if got.Header.Get("Chatgpt-Account-Id") != "account-1" {
		t.Errorf("Chatgpt-Account-Id = %q", got.Header.Get("Chatgpt-Account-Id"))
	}
	if got.Header.Get("Originator") != codexOriginator {
		t.Errorf("Originator = %q", got.Header.Get("Originator"))
	}

	// The endpoint reports what has been spent; this is what is left.
	if usage.Primary == nil || usage.Primary.RemainingPercent != 71.5 {
		t.Fatalf("Primary = %+v, want 71.5 percent left", usage.Primary)
	}
	if usage.Primary.WindowMinutes != 300 {
		t.Errorf("Primary.WindowMinutes = %d, want 300", usage.Primary.WindowMinutes)
	}
	if want := time.Unix(1789485149, 0).UTC().Format(time.RFC3339); usage.Primary.ResetTime != want {
		t.Errorf("Primary.ResetTime = %q, want %q", usage.Primary.ResetTime, want)
	}
	if usage.Secondary == nil || usage.Secondary.RemainingPercent != 94 {
		t.Fatalf("Secondary = %+v, want 94 percent left", usage.Secondary)
	}
	if usage.Secondary.WindowMinutes != 10080 {
		t.Errorf("Secondary.WindowMinutes = %d, want 10080", usage.Secondary.WindowMinutes)
	}
	if usage.ResetCredits != 2 {
		t.Errorf("ResetCredits = %d, want 2", usage.ResetCredits)
	}
}

func TestCodexClampsAndFallsBackToCountdown(t *testing.T) {
	serveUsage(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{
		  "plan_type": "pro",
		  "rate_limit": {
		    "primary_window": {"used_percent": 130, "limit_window_seconds": 18000, "reset_after_seconds": 600},
		    "secondary_window": null
		  },
		  "rate_limit_reset_credits": {"available_count": 0}
		}`))
	})

	usage, err := Codex(signedIn)
	if err != nil {
		t.Fatalf("Codex() = %v", err)
	}
	if usage.Primary.RemainingPercent != 0 {
		t.Errorf("RemainingPercent = %v, want 0", usage.Primary.RemainingPercent)
	}
	if usage.Secondary != nil {
		t.Errorf("Secondary = %+v, want nil", usage.Secondary)
	}

	reset, err := time.Parse(time.RFC3339, usage.Primary.ResetTime)
	if err != nil {
		t.Fatalf("ResetTime = %q: %v", usage.Primary.ResetTime, err)
	}
	if left := time.Until(reset); left < 9*time.Minute || left > 11*time.Minute {
		t.Errorf("ResetTime is %v away, want about 10m", left)
	}
}

func TestCodexRefusesAnApiKey(t *testing.T) {
	serveUsage(t, func(writer http.ResponseWriter, _ *http.Request) {
		t.Error("an API key must not be asked about")
		writer.WriteHeader(http.StatusInternalServerError)
	})

	if _, err := Codex(`{"OPENAI_API_KEY":"sk-test","tokens":null,"last_refresh":null}`); err == nil {
		t.Fatal("Codex() accepted an API key")
	}
}

func TestCodexReportsARefusedSignIn(t *testing.T) {
	serveUsage(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte("<html>sign in</html>"))
	})

	_, err := Codex(signedIn)
	if err == nil {
		t.Fatal("Codex() accepted a refused sign-in")
	}
	if !strings.Contains(err.Error(), "sign in again") {
		t.Errorf("err = %v, want it to say the sign-in has to be renewed", err)
	}
}

func TestCodexRejectsAnEmptyAnswer(t *testing.T) {
	serveUsage(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"rate_limit": {}}`))
	})

	if _, err := Codex(signedIn); err == nil {
		t.Fatal("Codex() accepted an answer with no limits in it")
	}
}
