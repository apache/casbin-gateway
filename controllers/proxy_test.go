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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/apache/casbin-gateway/object"
	beegoContext "github.com/beego/beego/context"
)

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
	for _, statusCode := range []int{429, 500, 502, 503} {
		if !isRetryableStatus(statusCode) {
			t.Errorf("status %d should be retryable", statusCode)
		}
	}
	for _, statusCode := range []int{200, 400, 401, 404} {
		if isRetryableStatus(statusCode) {
			t.Errorf("status %d should not be retryable", statusCode)
		}
	}
}

func TestChannelUnusableReason(t *testing.T) {
	if reason := channelUnusableReason(&object.Channel{Type: "claude", BaseUrl: "https://example.com"}); !strings.Contains(reason, "not supported") {
		t.Errorf("the claude channel type should be rejected, got: %s", reason)
	}
	if reason := channelUnusableReason(&object.Channel{Type: "openai", BaseUrl: ""}); !strings.Contains(reason, "base URL") {
		t.Errorf("an empty base URL should be rejected, got: %s", reason)
	}
	if reason := channelUnusableReason(&object.Channel{Type: "custom", BaseUrl: "https://example.com"}); reason != "" {
		t.Errorf("the custom channel type should be usable, got: %s", reason)
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
	c.relayResponse(upstreamResp, strings.NewReader(`{"error":{"message":"slow down"}}`), false)

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
	c.relayResponse(upstreamResp, strings.NewReader("data: a\n\ndata: [DONE]\n\n"), true)

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

func TestForwardToChannel(t *testing.T) {
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

	overloadedChannel := &object.Channel{Owner: "admin", Name: "overloaded", Type: "openai", BaseUrl: overloadedServer.URL, ApiKey: "sk-bad"}
	healthyChannel := &object.Channel{Owner: "admin", Name: "healthy", Type: "openai", BaseUrl: healthyServer.URL + "/", ApiKey: "sk-good"}
	rawBody := []byte(`{"model":"gpt-4","messages":[]}`)

	// A retryable status fails over instead of reaching the client.
	c, recorder := newTestApiController()
	statusCode, message, written := c.forwardToChannel(overloadedChannel, rawBody, false, false)
	if written {
		t.Fatal("a retryable status was relayed instead of failing over")
	}
	if statusCode != http.StatusBadGateway || !strings.Contains(message, "503") {
		t.Errorf("statusCode = %d, message = %s", statusCode, message)
	}
	if recorder.Body.Len() != 0 {
		t.Errorf("a body was written before failing over: %s", recorder.Body.String())
	}

	// The last channel is relayed as-is, even with a retryable status, so that
	// the client sees the real upstream answer.
	c, recorder = newTestApiController()
	_, _, written = c.forwardToChannel(overloadedChannel, rawBody, false, true)
	if !written || recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), "overloaded") {
		t.Errorf("the last channel was not relayed: written = %v, statusCode = %d, body = %s", written, recorder.Code, recorder.Body.String())
	}

	// A healthy channel, with a trailing slash in its base URL.
	c, recorder = newTestApiController()
	_, _, written = c.forwardToChannel(healthyChannel, rawBody, false, true)
	if !written || recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "choices") {
		t.Errorf("the healthy channel failed: written = %v, statusCode = %d, body = %s", written, recorder.Code, recorder.Body.String())
	}

	// stream=true, but the upstream rejected the request: the JSON error must
	// not be dressed up as an SSE stream.
	c, recorder = newTestApiController()
	c.forwardToChannel(overloadedChannel, rawBody, true, true)
	if header := recorder.Header().Get("Content-Type"); header != "application/json" {
		t.Errorf("Content-Type = %s, expected application/json", header)
	}
}

func TestForwardAnthropicToChannel(t *testing.T) {
	for _, tc := range []struct {
		name     string
		authType string
		header   string
		expected string
	}{
		{name: "bearer", authType: "bearer", header: "Authorization", expected: "Bearer upstream-key"},
		{name: "x-api-key", authType: "x-api-key", header: "X-Api-Key", expected: "upstream-key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/anthropic/v1/messages" || r.URL.Query().Get("beta") != "true" {
					t.Errorf("unexpected upstream URL: %s", r.URL.String())
				}
				if actual := r.Header.Get(tc.header); actual != tc.expected {
					t.Errorf("%s = %q, want %q", tc.header, actual, tc.expected)
				}
				otherHeader := "Authorization"
				if tc.header == otherHeader {
					otherHeader = "X-Api-Key"
				}
				if actual := r.Header.Get(otherHeader); actual != "" {
					t.Errorf("client credential leaked in %s: %q", otherHeader, actual)
				}
				if r.Header.Get("Anthropic-Beta") != "tools-2026" || r.Header.Get("Anthropic-Version") != "2023-06-01" {
					t.Error("Anthropic compatibility headers were not preserved")
				}
				body, _ := io.ReadAll(r.Body)
				if !strings.Contains(string(body), `"model":"upstream-model"`) {
					t.Errorf("mapped body was not forwarded: %s", body)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"content":[]}`))
			}))
			defer server.Close()

			controller, recorder := newTestApiController()
			controller.Ctx.Request.URL.RawQuery = "beta=true"
			controller.Ctx.Request.Header.Set("Authorization", "Bearer client-placeholder")
			controller.Ctx.Request.Header.Set("X-Api-Key", "client-placeholder")
			controller.Ctx.Request.Header.Set("Anthropic-Beta", "tools-2026")
			controller.Ctx.Request.Header.Set("Anthropic-Version", "2023-06-01")
			channel := &object.Channel{
				Owner: "admin", Name: tc.name, Type: "anthropic", BaseUrl: server.URL + "/anthropic",
				ApiKey: "upstream-key", AuthType: tc.authType,
			}
			_, _, written := controller.forwardAnthropicToChannel(channel, []byte(`{"model":"upstream-model","messages":[]}`), false, true)
			if !written || recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "content") {
				t.Fatalf("Anthropic response was not relayed: status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}
