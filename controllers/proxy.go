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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/apache/casbin-gateway/object"
	"github.com/apache/casbin-gateway/proxy"
	"github.com/beego/beego"
)

const (
	// proxyResponseHeaderTimeout bounds the wait for the upstream response
	// headers. It deliberately does not cover the response body: a completion
	// can legitimately take many minutes to finish streaming, so the body is
	// bounded by proxyIdleTimeout instead.
	proxyResponseHeaderTimeout = 60 * time.Second
	// proxyIdleTimeout is the longest gap tolerated between two chunks of the
	// upstream response body before the request is aborted.
	proxyIdleTimeout = 120 * time.Second
)

// hopByHopHeaders are connection-scoped, so a proxy must not pass them on.
// See RFC 7230 section 6.1.
var hopByHopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}

// chatCompletionsRequest contains only the fields needed for routing.
// All other fields (messages, temperature, etc.) are forwarded as-is to the upstream.
type chatCompletionsRequest struct {
	Model  string `json:"model"`
	Stream bool   `json:"stream"`
}

// openaiErrorResponse follows the OpenAI API error format.
type openaiErrorResponse struct {
	Error openaiErrorDetail `json:"error"`
}

type openaiErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

type anthropicErrorResponse struct {
	Type  string               `json:"type"`
	Error anthropicErrorDetail `json:"error"`
}

type anthropicErrorDetail struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// proxyClient is a shared HTTP client for upstream requests.
// Reusing a single instance allows TCP connection pooling across requests.
// It has no overall Timeout on purpose, see proxyResponseHeaderTimeout.
var proxyClient = &http.Client{
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	Transport: &http.Transport{
		Proxy: proxy.Proxy,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: proxyResponseHeaderTimeout,
	},
}

// ChatCompletions is the OpenAI-compatible chat completions proxy endpoint.
// It matches the upstream channels by model name and forwards the request and
// response body as-is (pass-through), trying the channels in priority order
// until one of them answers. Supports SSE streaming when stream=true in the
// request body.
// This endpoint does NOT require Casdoor authentication (auth deferred to milestone 1.3).
func (c *ApiController) ChatCompletions() {
	rawBody := c.Ctx.Input.RequestBody

	// Parse only model and stream; forward everything else as-is.
	var req chatCompletionsRequest
	if err := json.Unmarshal(rawBody, &req); err != nil {
		c.writeOpenAIError(http.StatusBadRequest, "invalid_request_error", "invalid request body")
		return
	}
	if req.Model == "" {
		c.writeOpenAIError(http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}

	// Match the channels globally, without an owner filter.
	channels, err := object.GetChannelsByModel(req.Model)
	if err != nil {
		if errors.Is(err, object.ErrNoChannelAvailable) {
			c.writeOpenAIError(http.StatusBadRequest, "invalid_request_error", err.Error())
		} else {
			beego.Error("channel lookup failed:", err)
			c.writeOpenAIError(http.StatusBadGateway, "server_error", "channel lookup failed")
		}
		return
	}

	// Drop the channels this proxy cannot talk to before forwarding, so that
	// the last usable channel is known and its response can be relayed as-is.
	usableChannels := []*object.Channel{}
	skipReason := ""
	for _, channel := range channels {
		if reason := channelUnusableReason(channel); reason != "" {
			beego.Error("skipped channel", channel.GetId()+":", reason)
			skipReason = reason
			continue
		}
		usableChannels = append(usableChannels, channel)
	}
	if len(usableChannels) == 0 {
		message := fmt.Sprintf("no usable channel for model: %s", req.Model)
		if skipReason != "" {
			message = skipReason
		}
		c.writeOpenAIError(http.StatusBadGateway, "server_error", message)
		return
	}

	// Fail over to the next channel as long as nothing has been written to the
	// client yet. The last channel is never retried, so the client gets the
	// real upstream answer instead of a synthesized error.
	lastStatus, lastMessage := http.StatusBadGateway, "upstream connection failed"
	for i, channel := range usableChannels {
		if c.Ctx.Request.Context().Err() != nil {
			// The client hung up, there is nobody left to fail over for.
			return
		}

		status, message, written := c.forwardToChannel(channel, rawBody, req.Stream, i == len(usableChannels)-1)
		if written {
			return
		}
		lastStatus, lastMessage = status, message
	}

	c.writeOpenAIError(lastStatus, "server_error", lastMessage)
}

// Messages proxies Claude Code's native Anthropic Messages protocol. Client
// authentication is intentionally deferred to the gateway auth milestone;
// client credentials are still stripped before a channel key is injected.
func (c *ApiController) Messages() {
	rawBody := c.Ctx.Input.RequestBody
	var req chatCompletionsRequest
	var body map[string]json.RawMessage
	if err := json.Unmarshal(rawBody, &body); err != nil || json.Unmarshal(rawBody, &req) != nil {
		c.writeAnthropicError(http.StatusBadRequest, "invalid_request_error", "invalid request body")
		return
	}
	if req.Model == "" {
		c.writeAnthropicError(http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}

	channels, err := object.GetAnthropicChannelsByModel(req.Model)
	if err != nil {
		if errors.Is(err, object.ErrNoChannelAvailable) {
			c.writeAnthropicError(http.StatusBadRequest, "invalid_request_error", err.Error())
		} else {
			beego.Error("Anthropic channel lookup failed:", err)
			c.writeAnthropicError(http.StatusBadGateway, "api_error", "channel lookup failed")
		}
		return
	}

	usableChannels := make([]*object.Channel, 0, len(channels))
	for _, channel := range channels {
		if reason := channelUnusableReason(channel); reason != "" {
			beego.Error("skipped channel", channel.GetId()+":", reason)
			continue
		}
		usableChannels = append(usableChannels, channel)
	}
	if len(usableChannels) == 0 {
		c.writeAnthropicError(http.StatusBadGateway, "api_error", "no usable Anthropic channel")
		return
	}

	lastStatus, lastMessage := http.StatusBadGateway, "upstream connection failed"
	for index, channel := range usableChannels {
		if c.Ctx.Request.Context().Err() != nil {
			return
		}
		attemptBody := make(map[string]json.RawMessage, len(body))
		for key, value := range body {
			attemptBody[key] = value
		}
		attemptBody["model"], _ = json.Marshal(channel.AnthropicModel(req.Model))
		encoded, err := json.Marshal(attemptBody)
		if err != nil {
			c.writeAnthropicError(http.StatusBadRequest, "invalid_request_error", "invalid request body")
			return
		}
		status, message, written := c.forwardAnthropicToChannel(channel, encoded, req.Stream, index == len(usableChannels)-1)
		if written {
			return
		}
		lastStatus, lastMessage = status, message
	}
	c.writeAnthropicError(lastStatus, "api_error", lastMessage)
}

func (c *ApiController) forwardAnthropicToChannel(channel *object.Channel, body []byte, stream, isLast bool) (int, string, bool) {
	upstreamURL, err := object.BuildAnthropicMessagesUrl(channel.BaseUrl, c.Ctx.Request.URL.RawQuery)
	if err != nil {
		return http.StatusBadGateway, err.Error(), false
	}
	ctx, cancel := context.WithCancel(c.Ctx.Request.Context())
	defer cancel()

	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		return http.StatusBadGateway, "upstream connection failed", false
	}
	copyRequestHeaders(upstreamReq.Header, c.Ctx.Request.Header)
	upstreamReq.Header.Set("Content-Type", "application/json")
	if channel.AuthType == "x-api-key" {
		upstreamReq.Header.Set("x-api-key", channel.ApiKey)
	} else {
		upstreamReq.Header.Set("Authorization", "Bearer "+channel.ApiKey)
	}

	upstreamResp, err := proxyClient.Do(upstreamReq)
	if err != nil {
		if c.Ctx.Request.Context().Err() != nil {
			return 0, "", true
		}
		beego.Error("upstream request to channel", channel.GetId(), "failed:", err)
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return http.StatusGatewayTimeout, "upstream timeout", false
		}
		return http.StatusBadGateway, "upstream connection failed", false
	}
	defer upstreamResp.Body.Close()
	if !isLast && isRetryableStatus(upstreamResp.StatusCode) {
		_, _ = io.Copy(io.Discard, io.LimitReader(upstreamResp.Body, 4096))
		return http.StatusBadGateway, fmt.Sprintf("upstream returned %s", upstreamResp.Status), false
	}

	responseBody := newIdleTimeoutReader(upstreamResp.Body, proxyIdleTimeout, cancel)
	defer responseBody.Stop()
	c.relayResponse(upstreamResp, responseBody, stream && isEventStream(upstreamResp))
	return 0, "", true
}

func copyRequestHeaders(dst, src http.Header) {
	dynamicHopHeaders := map[string]bool{}
	for _, value := range src.Values("Connection") {
		for _, name := range strings.Split(value, ",") {
			dynamicHopHeaders[http.CanonicalHeaderKey(strings.TrimSpace(name))] = true
		}
	}
	for name, values := range src {
		canonical := http.CanonicalHeaderKey(name)
		if isHopByHopHeader(name) || dynamicHopHeaders[canonical] || strings.EqualFold(name, "Host") ||
			strings.EqualFold(name, "Content-Length") || strings.EqualFold(name, "Authorization") || strings.EqualFold(name, "x-api-key") {
			continue
		}
		for _, value := range values {
			dst.Add(name, value)
		}
	}
}

// forwardToChannel sends the request to a single channel's upstream. It reports
// whether the client response was already written, and when it was not, the
// status and message describing the failure so that the caller can fail over to
// the next channel. The response of the last channel is always relayed, even
// when its status would otherwise be retried.
func (c *ApiController) forwardToChannel(channel *object.Channel, rawBody []byte, stream bool, isLast bool) (int, string, bool) {
	upstreamUrl, err := object.BuildOpenAiUrl(channel.BaseUrl, "/chat/completions")
	if err != nil {
		return http.StatusBadGateway, err.Error(), false
	}

	// The context is cancelled when this function returns, which happens only
	// after the response body has been relayed to the client.
	ctx, cancel := context.WithCancel(c.Ctx.Request.Context())
	defer cancel()

	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamUrl, bytes.NewReader(rawBody))
	if err != nil {
		return http.StatusBadGateway, "upstream connection failed", false
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Authorization", "Bearer "+channel.ApiKey)

	upstreamResp, err := proxyClient.Do(upstreamReq)
	if err != nil {
		if c.Ctx.Request.Context().Err() != nil {
			// The client hung up mid-request, there is nothing left to answer.
			return 0, "", true
		}

		beego.Error("upstream request to channel", channel.GetId(), "failed:", err)
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return http.StatusGatewayTimeout, "upstream timeout", false
		}
		return http.StatusBadGateway, "upstream connection failed", false
	}
	defer upstreamResp.Body.Close()

	if !isLast && isRetryableStatus(upstreamResp.StatusCode) {
		beego.Error("channel", channel.GetId(), "returned a retryable status:", upstreamResp.Status)
		// Drain a bounded amount so the connection can be pooled and reused.
		_, _ = io.Copy(io.Discard, io.LimitReader(upstreamResp.Body, 4096))
		return http.StatusBadGateway, fmt.Sprintf("upstream returned %s", upstreamResp.Status), false
	}

	// Abort the request when the upstream goes silent, rather than capping the
	// total duration, which would cut off long but healthy streams.
	body := newIdleTimeoutReader(upstreamResp.Body, proxyIdleTimeout, cancel)
	defer body.Stop()

	c.relayResponse(upstreamResp, body, stream && isEventStream(upstreamResp))
	return 0, "", true
}

// relayResponse copies the upstream status code, headers and body to the client
// without any transformation (pass-through mode). When flush is true, every
// chunk is written out as soon as it arrives so that SSE clients receive the
// events in real time.
func (c *ApiController) relayResponse(upstreamResp *http.Response, body io.Reader, flush bool) {
	copyResponseHeaders(c.Ctx.ResponseWriter.Header(), upstreamResp.Header)
	if flush {
		c.Ctx.ResponseWriter.Header().Set("Cache-Control", "no-cache")
	}
	c.Ctx.ResponseWriter.WriteHeader(upstreamResp.StatusCode)

	if !flush {
		if _, err := io.Copy(c.Ctx.ResponseWriter, body); err != nil {
			beego.Error("proxy response copy failed:", err)
		}
		return
	}

	buf := make([]byte, 4096)
	for {
		n, err := body.Read(buf)
		if n > 0 {
			if _, writeErr := c.Ctx.ResponseWriter.Write(buf[:n]); writeErr != nil {
				beego.Error("proxy stream write failed:", writeErr)
				return
			}
			c.Ctx.ResponseWriter.Flush()
		}
		if err != nil {
			if err != io.EOF {
				beego.Error("proxy stream read failed:", err)
			}
			return
		}
	}
}

// copyResponseHeaders copies the upstream headers to the client, minus the
// hop-by-hop ones, which belong to the upstream connection and not to the
// response being relayed.
func copyResponseHeaders(dst http.Header, src http.Header) {
	dynamicHopHeaders := map[string]bool{}
	for _, value := range src.Values("Connection") {
		for _, name := range strings.Split(value, ",") {
			dynamicHopHeaders[http.CanonicalHeaderKey(strings.TrimSpace(name))] = true
		}
	}
	for name, values := range src {
		if isHopByHopHeader(name) || dynamicHopHeaders[http.CanonicalHeaderKey(name)] {
			continue
		}
		for _, value := range values {
			dst.Add(name, value)
		}
	}
}

func isHopByHopHeader(name string) bool {
	for _, header := range hopByHopHeaders {
		if strings.EqualFold(name, header) {
			return true
		}
	}
	return false
}

// isEventStream reports whether the upstream response really is an SSE stream.
// An upstream that rejects the request answers with a JSON body even when
// stream=true was asked for, and relaying that as text/event-stream would
// leave the client waiting for events that never come.
func isEventStream(upstreamResp *http.Response) bool {
	if upstreamResp.StatusCode < 200 || upstreamResp.StatusCode > 299 {
		return false
	}
	return strings.Contains(strings.ToLower(upstreamResp.Header.Get("Content-Type")), "text/event-stream")
}

// isRetryableStatus reports whether another channel is worth trying. A rate
// limit or an upstream-side error is transient or specific to that channel,
// while a 4xx caused by the request itself would fail the same way everywhere.
func isRetryableStatus(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode >= 500
}

// channelUnusableReason reports why the proxy cannot forward to a channel, or
// an empty string when it can.
func channelUnusableReason(channel *object.Channel) string {
	if !object.IsChannelTypeSupported(channel) {
		return fmt.Sprintf("the %s channel type is not supported", channel.Type)
	}
	if channel.BaseUrl == "" {
		return "channel base URL is not configured"
	}
	return ""
}

// idleTimeoutReader aborts the upstream request when no data arrives for the
// given duration. It takes the place of an overall request timeout, which would
// cut off a long but healthy streaming completion.
type idleTimeoutReader struct {
	reader  io.Reader
	timeout time.Duration
	timer   *time.Timer
}

func newIdleTimeoutReader(reader io.Reader, timeout time.Duration, onIdle func()) *idleTimeoutReader {
	return &idleTimeoutReader{
		reader:  reader,
		timeout: timeout,
		timer:   time.AfterFunc(timeout, onIdle),
	}
}

func (r *idleTimeoutReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.timer.Reset(r.timeout)
	}
	return n, err
}

func (r *idleTimeoutReader) Stop() {
	r.timer.Stop()
}

// writeOpenAIError writes an OpenAI-compatible JSON error response
// with the given HTTP status code, error type, and message.
func (c *ApiController) writeOpenAIError(statusCode int, errType, message string) {
	resp := openaiErrorResponse{
		Error: openaiErrorDetail{
			Message: message,
			Type:    errType,
		},
	}
	c.Ctx.ResponseWriter.Header().Set("Content-Type", "application/json")
	c.Ctx.ResponseWriter.WriteHeader(statusCode)
	if err := json.NewEncoder(c.Ctx.ResponseWriter).Encode(resp); err != nil {
		beego.Error("proxy error response write failed:", err)
	}
}

func (c *ApiController) writeAnthropicError(statusCode int, errType, message string) {
	resp := anthropicErrorResponse{
		Type:  "error",
		Error: anthropicErrorDetail{Type: errType, Message: message},
	}
	c.Ctx.ResponseWriter.Header().Set("Content-Type", "application/json")
	c.Ctx.ResponseWriter.WriteHeader(statusCode)
	if err := json.NewEncoder(c.Ctx.ResponseWriter).Encode(resp); err != nil {
		beego.Error("proxy error response write failed:", err)
	}
}
