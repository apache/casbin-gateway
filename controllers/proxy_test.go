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
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/apache/casbin-gateway/object"
	"github.com/apache/casbin-gateway/protocol"
	beegoContext "github.com/beego/beego/context"
)

// newProxyRoute is the route one request would be relayed on, as readProxyRoute
// builds it.
func newProxyRoute(target proxyTarget, body []byte, stream bool) *proxyRoute {
	var fields routingFields
	_ = json.Unmarshal(body, &fields)
	return &proxyRoute{
		target: target, codec: protocol.Of(target.protocol),
		body: body, model: fields.Model, stream: stream,
	}
}

func newTestApiController() (*ApiController, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx := beegoContext.NewContext()
	ctx.Reset(recorder, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("{}")))

	c := &ApiController{}
	c.Ctx = ctx
	return c, recorder
}

func TestIsEventStream(t *testing.T) {
	cases := []struct {
		statusCode  int
		contentType string
		expected    bool
	}{
		{200, "text/event-stream", true},
		{200, "text/event-stream; charset=utf-8", true},
		{200, "application/json", false},
		// An upstream that rejects the request answers with JSON even when
		// stream=true was asked for.
		{429, "application/json", false},
		{500, "text/event-stream", false},
	}

	for _, tc := range cases {
		resp := &http.Response{StatusCode: tc.statusCode, Header: http.Header{"Content-Type": {tc.contentType}}}
		if got := isEventStream(resp); got != tc.expected {
			t.Errorf("isEventStream(%d, %s) = %v, expected %v", tc.statusCode, tc.contentType, got, tc.expected)
		}
	}
}

func TestIsRetryableStatus(t *testing.T) {
	for _, statusCode := range []int{401, 402, 403, 404, 429, 500, 502, 503} {
		if !isRetryableStatus(statusCode) {
			t.Errorf("status %d should be retryable", statusCode)
		}
	}
	for _, statusCode := range []int{200, 400, 422} {
		if isRetryableStatus(statusCode) {
			t.Errorf("status %d should not be retryable", statusCode)
		}
	}
}

func TestProviderUnusableReason(t *testing.T) {
	c, _ := newTestApiController()
	providerUnusableReason := c.providerUnusableReason

	if reason := providerUnusableReason(&object.Provider{Type: "claude", BaseUrl: "https://example.com"}); !strings.Contains(reason, "not supported") {
		t.Errorf("the claude provider type should be rejected, got: %s", reason)
	}
	if reason := providerUnusableReason(&object.Provider{Type: "openai", BaseUrl: ""}); !strings.Contains(reason, "base URL") {
		t.Errorf("an empty base URL should be rejected, got: %s", reason)
	}
	if reason := providerUnusableReason(&object.Provider{Type: "custom", BaseUrl: "https://example.com"}); reason != "" {
		t.Errorf("the custom provider type should be usable, got: %s", reason)
	}

	// The wire format is no longer a reason: a request that arrived in the other
	// one is translated for the provider.
	other := &object.Provider{Owner: "admin", Name: "claude", Type: "anthropic", BaseUrl: "https://api.anthropic.com"}
	if reason := providerUnusableReason(other); reason != "" {
		t.Errorf("an anthropic provider should be usable, got: %s", reason)
	}

	passthrough := &object.Provider{
		Owner:    "admin",
		Name:     "passthrough",
		Type:     "openai",
		BaseUrl:  "https://api.openai.com/v1",
		AuthMode: object.ProviderAuthClient,
	}
	if reason := providerUnusableReason(passthrough); !strings.Contains(reason, "carries none") {
		t.Errorf("a client-auth provider should be rejected without a credential, got: %s", reason)
	}
	c.Ctx.Request.Header.Set("Authorization", "Bearer token")
	if reason := providerUnusableReason(passthrough); reason != "" {
		t.Errorf("a client-auth provider should be usable with a credential, got: %s", reason)
	}
}

func TestRelayResponse(t *testing.T) {
	c, recorder := newTestApiController()
	upstreamResp := &http.Response{
		StatusCode: 429,
		Header: http.Header{
			"Content-Type":          {"application/json"},
			"Connection":            {"keep-alive"},
			"X-Request-Id":          {"req-123"},
			"X-Ratelimit-Remaining": {"0"},
		},
	}
	c.relayVerbatim(upstreamResp, strings.NewReader(`{"error":{"message":"slow down"}}`), false)

	if recorder.Code != 429 {
		t.Errorf("status code = %d, expected 429", recorder.Code)
	}
	if header := recorder.Header().Get("Connection"); header != "" {
		t.Errorf("the hop-by-hop Connection header was relayed: %s", header)
	}
	if header := recorder.Header().Get("X-Request-Id"); header != "req-123" {
		t.Errorf("X-Request-Id = %s, expected req-123", header)
	}
	if header := recorder.Header().Get("X-Ratelimit-Remaining"); header != "0" {
		t.Errorf("X-Ratelimit-Remaining = %s, expected 0", header)
	}
	if body := recorder.Body.String(); !strings.Contains(body, "slow down") {
		t.Errorf("body = %s", body)
	}
}

func TestRelayResponseStream(t *testing.T) {
	c, recorder := newTestApiController()
	upstreamResp := &http.Response{
		StatusCode: 200,
		Header: http.Header{
			"Content-Type": {"text/event-stream"},
			"X-Request-Id": {"req-abc"},
			"Connection":   {"keep-alive"},
		},
	}
	c.relayVerbatim(upstreamResp, strings.NewReader("data: a\n\ndata: [DONE]\n\n"), true)

	if header := recorder.Header().Get("Content-Type"); header != "text/event-stream" {
		t.Errorf("Content-Type = %s, expected text/event-stream", header)
	}
	if header := recorder.Header().Get("Cache-Control"); header != "no-cache" {
		t.Errorf("Cache-Control = %s, expected no-cache", header)
	}
	if header := recorder.Header().Get("X-Request-Id"); header != "req-abc" {
		t.Errorf("the upstream headers were dropped, X-Request-Id = %s", header)
	}
	if header := recorder.Header().Get("Connection"); header != "" {
		t.Errorf("the hop-by-hop Connection header was relayed: %s", header)
	}
	if body := recorder.Body.String(); !strings.Contains(body, "[DONE]") {
		t.Errorf("body = %s", body)
	}
	if !recorder.Flushed {
		t.Error("the stream was not flushed")
	}
}

// A stream that is slow but alive must not be cut off, no matter how long it
// lasts in total. Only a stalled upstream is aborted.
func TestIdleTimeoutReader(t *testing.T) {
	idleTimeout := 150 * time.Millisecond
	aborted := make(chan struct{})
	reader, writer := io.Pipe()
	idleReader := newIdleTimeoutReader(reader, idleTimeout, func() {
		close(aborted)
		_ = writer.CloseWithError(io.ErrUnexpectedEOF)
	})
	defer idleReader.Stop()

	chunkCount := 5
	go func() {
		// The whole stream takes longer than the idle timeout, while every
		// single gap stays below it.
		for i := 0; i < chunkCount; i++ {
			time.Sleep(idleTimeout * 2 / 3)
			if _, err := writer.Write([]byte("data: chunk\n\n")); err != nil {
				return
			}
		}
	}()

	buf := make([]byte, 64)
	for i := 0; i < chunkCount; i++ {
		if _, err := idleReader.Read(buf); err != nil {
			t.Fatalf("reading chunk %d failed: %s", i, err.Error())
		}
	}
	select {
	case <-aborted:
		t.Fatal("a slow but healthy stream was aborted")
	default:
	}

	// Nothing is written anymore, so the idle timeout has to fire.
	if _, err := idleReader.Read(buf); err == nil {
		t.Fatal("the read on a stalled stream returned no error")
	}
	select {
	case <-aborted:
	default:
		t.Fatal("the idle timeout did not fire on a stalled stream")
	}
}

func attemptOf(provider *object.Provider, model string) object.RouteAttempt {
	return object.RouteAttempt{Provider: provider, Model: model}
}

func TestForwardToProvider(t *testing.T) {
	overloadedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":{"message":"overloaded"}}`))
	}))
	defer overloadedServer.Close()

	healthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if header := r.Header.Get("Authorization"); header != "Bearer sk-good" {
			t.Errorf("Authorization = %s, expected Bearer sk-good", header)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("upstream path = %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "gpt-4") {
			t.Errorf("the request body was not forwarded as-is: %s", body)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer healthyServer.Close()

	overloadedProvider := &object.Provider{Owner: "admin", Name: "overloaded", Type: "openai", BaseUrl: overloadedServer.URL, ApiKey: "sk-bad"}
	healthyProvider := &object.Provider{Owner: "admin", Name: "healthy", Type: "openai", BaseUrl: healthyServer.URL + "/", ApiKey: "sk-good"}
	rawBody := []byte(`{"model":"gpt-4","messages":[]}`)

	route := newProxyRoute(openAiChat, rawBody, false)

	// A retryable status fails over instead of reaching the client.
	c, recorder := newTestApiController()
	statusCode, message, written := c.forwardToProvider(attemptOf(overloadedProvider, "gpt-4"), route, false)
	if written {
		t.Fatal("a retryable status was relayed instead of failing over")
	}
	if statusCode != http.StatusServiceUnavailable || !strings.Contains(message, "overloaded") {
		t.Errorf("statusCode = %d, message = %s", statusCode, message)
	}
	if recorder.Body.Len() != 0 {
		t.Errorf("a body was written before failing over: %s", recorder.Body.String())
	}

	// The last provider is relayed as-is, even with a retryable status, so that
	// the client sees the real upstream answer.
	c, recorder = newTestApiController()
	_, _, written = c.forwardToProvider(attemptOf(overloadedProvider, "gpt-4"), route, true)
	if !written || recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), "overloaded") {
		t.Errorf("the last provider was not relayed: written = %v, statusCode = %d, body = %s", written, recorder.Code, recorder.Body.String())
	}

	// A healthy provider, with a trailing slash in its base URL.
	c, recorder = newTestApiController()
	_, _, written = c.forwardToProvider(attemptOf(healthyProvider, "gpt-4"), route, true)
	if !written || recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "choices") {
		t.Errorf("the healthy provider failed: written = %v, statusCode = %d, body = %s", written, recorder.Code, recorder.Body.String())
	}

	// stream=true, but the upstream rejected the request: the JSON error must
	// not be dressed up as an SSE stream.
	c, recorder = newTestApiController()
	c.forwardToProvider(attemptOf(overloadedProvider, "gpt-4"), newProxyRoute(openAiChat, rawBody, true), true)
	if header := recorder.Header().Get("Content-Type"); header != "application/json" {
		t.Errorf("Content-Type = %s, expected application/json", header)
	}
}

func TestForwardToProviderAnthropic(t *testing.T) {
	var gotPath, gotKey, gotVersion, gotAuth string
	var gotBeta []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotKey = r.Header.Get("X-Api-Key")
		gotVersion = r.Header.Get("Anthropic-Version")
		gotBeta = r.Header.Values("Anthropic-Beta")
		gotAuth = r.Header.Get("Authorization")

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[]}`))
	}))
	defer server.Close()

	provider := &object.Provider{Owner: "admin", Name: "claude", Type: "anthropic", BaseUrl: server.URL, ApiKey: "sk-ant-test"}
	route := newProxyRoute(anthropicMessages, []byte(`{"model":"claude-opus-5","messages":[]}`), false)

	c, recorder := newTestApiController()
	c.Ctx.Request.Header.Add("Anthropic-Beta", "fine-grained-tool-streaming-2025-05-14")
	if _, _, written := c.forwardToProvider(attemptOf(provider, "claude-opus-5"), route, true); !written {
		t.Fatal("the anthropic provider was not relayed")
	}

	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "content") {
		t.Errorf("statusCode = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if gotPath != "/v1/messages" {
		t.Errorf("upstream path = %s, expected /v1/messages", gotPath)
	}
	if gotKey != "sk-ant-test" {
		t.Errorf("X-Api-Key = %s", gotKey)
	}
	if gotAuth != "" {
		t.Errorf("the OpenAI Authorization header was sent to an anthropic upstream: %s", gotAuth)
	}
	if gotVersion != object.AnthropicVersion {
		t.Errorf("Anthropic-Version = %s, expected %s", gotVersion, object.AnthropicVersion)
	}
	if len(gotBeta) != 1 || gotBeta[0] != "fine-grained-tool-streaming-2025-05-14" {
		t.Errorf("Anthropic-Beta = %v, expected the client value to be passed on", gotBeta)
	}
}

func TestWriteProxyError(t *testing.T) {
	c, recorder := newTestApiController()
	c.writeProxyError(protocol.Of(protocol.OpenAi), http.StatusBadRequest, "invalid_request_error", "nope")
	if body := recorder.Body.String(); !strings.Contains(body, `"error":{"message":"nope"`) || strings.Contains(body, `"type":"error"`) {
		t.Errorf("openai error body = %s", body)
	}

	c, recorder = newTestApiController()
	c.writeProxyError(protocol.Of(protocol.Anthropic), http.StatusBadRequest, "invalid_request_error", "nope")
	if body := recorder.Body.String(); !strings.Contains(body, `"type":"error"`) {
		t.Errorf("anthropic error body = %s", body)
	}
}
