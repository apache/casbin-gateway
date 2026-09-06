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

	"github.com/apache/casbin-gateway/conf"
	"github.com/apache/casbin-gateway/object"
	"github.com/apache/casbin-gateway/protocol"
	"github.com/apache/casbin-gateway/proxy"
	"github.com/apache/casbin-gateway/util"
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

// proxyTarget is one API this proxy answers on: the wire format the client
// speaks there and the path it called, which is what a record names it by.
type proxyTarget struct {
	protocol string
	endpoint string
	// countTokens marks the Anthropic token-counting endpoint, which is not a
	// completion at all. No other API serves one, so a provider that does not
	// speak Anthropic is answered by the gateway itself.
	countTokens bool
}

// countTokensEndpoint is where the Anthropic clients ask how much of their
// context a request would take.
const countTokensEndpoint = "/v1/messages/count_tokens"

// maxErrorBodyBytes bounds the upstream error body read to write the failure
// back out in the client's own format.
const maxErrorBodyBytes = 64 * 1024

var (
	openAiChat           = proxyTarget{protocol: protocol.OpenAi, endpoint: "/chat/completions"}
	openAiResponses      = proxyTarget{protocol: protocol.Responses, endpoint: "/responses"}
	anthropicMessages    = proxyTarget{protocol: protocol.Anthropic, endpoint: "/v1/messages"}
	anthropicCountTokens = proxyTarget{protocol: protocol.Anthropic, endpoint: countTokensEndpoint, countTokens: true}
)

// proxyRoute is one client request being relayed. A provider speaking the
// format the request arrived in is forwarded to as-is; any other provider is
// reached through the canonical form, which is what makes a client of one API
// and a provider of another able to talk at all.
type proxyRoute struct {
	target proxyTarget
	// codec reads the client's request and writes its answer.
	codec  protocol.Codec
	body   []byte
	model  string
	stream bool
	// source describes how the providers were chosen, for the error a client
	// sees when none of them can be used.
	source string
	start  time.Time
	// record accumulates what is written to the LLM record of this request. It
	// is nil while recording is off.
	record *object.LlmRecord
	// request is the body in canonical form, decoded on first use: a provider
	// speaking the client's own format never needs one.
	request    *protocol.Request
	requestErr error
}

// canonical is the request in the form every format is translated through.
func (route *proxyRoute) canonical() (*protocol.Request, error) {
	if route.request == nil && route.requestErr == nil {
		route.request, route.requestErr = route.codec.DecodeRequest(route.body)
	}
	return route.request, route.requestErr
}

// passthrough reports whether an upstream speaks the format the request arrived
// in. Such a request is relayed byte for byte, so nothing the canonical form
// does not model - a cache breakpoint, a field added last week - is lost.
func (route *proxyRoute) passthrough(upstream protocol.Upstream) bool {
	return route.target.protocol == upstream.Name()
}

// upstreamBody is the request body an upstream speaking the given format takes,
// asking for the model that upstream's provider serves.
func (route *proxyRoute) upstreamBody(upstream protocol.Upstream, model string) ([]byte, error) {
	if route.passthrough(upstream) {
		if model == route.model {
			return route.body, nil
		}
		return setBodyModel(route.body, model)
	}

	request, err := route.canonical()
	if err != nil {
		return nil, err
	}
	if model != request.Model {
		// The canonical form is decoded once and forwarded to every provider of
		// the chain, each of which names its models differently.
		swapped := *request
		swapped.Model = model
		request = &swapped
	}
	return upstream.EncodeRequest(request)
}

// setBodyModel rewrites the model of a body relayed as-is, leaving every other
// field exactly as the client wrote it.
func setBodyModel(body []byte, model string) ([]byte, error) {
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, err
	}
	name, err := json.Marshal(model)
	if err != nil {
		return nil, err
	}
	fields["model"] = name
	return json.Marshal(fields)
}

// upstreamEndpoint is the path on the provider that answers this request.
func (route *proxyRoute) upstreamEndpoint(upstream protocol.Upstream) string {
	if route.target.countTokens {
		return countTokensEndpoint
	}
	return upstream.Endpoint()
}

// routingFields are the only fields read out of the request body. Both the
// OpenAI and the Anthropic body carry them under the same names.
type routingFields struct {
	Model  string `json:"model"`
	Stream bool   `json:"stream"`
}

// proxyClient is a shared HTTP client for upstream requests.
// Reusing a single instance allows TCP connection pooling across requests.
// It has no overall Timeout on purpose, see proxyResponseHeaderTimeout.
var proxyClient = &http.Client{
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	Transport: proxy.WithUserAgent(&http.Transport{
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
	}),
}

// ChatCompletions is the OpenAI-compatible chat completions proxy endpoint.
// It matches the upstream providers by model name, trying them in priority
// order until one of them answers. A provider serving this same API is relayed
// to as-is; one serving another is translated for, in both directions. Supports
// SSE streaming when stream=true in the request body.
// This endpoint does NOT require Casdoor authentication (auth deferred to milestone 1.3).
func (c *ApiController) ChatCompletions() {
	c.proxyByModel(openAiChat)
}

// Responses is the OpenAI Responses API entry point, which recent Codex
// versions speak and no provider serves: the request is translated on the way
// out and the answer back on the way in, whichever API the provider serves.
func (c *ApiController) Responses() {
	c.proxyByModel(openAiResponses)
}

// Messages is the Anthropic-compatible counterpart of ChatCompletions, for the
// agents that speak that API rather than OpenAI's.
func (c *ApiController) Messages() {
	c.proxyByModel(anthropicMessages)
}

// CountTokens answers the Anthropic token-counting endpoint, which clients call
// alongside Messages to size their context. Only an Anthropic provider is asked;
// for any other the gateway estimates the count itself.
func (c *ApiController) CountTokens() {
	c.proxyByModel(anthropicCountTokens)
}

// AgentChatCompletions is the per-agent entry point of the same proxy: an agent
// pointed at ".../v1/agents/<agentId>" reaches the provider bound to it rather
// than one chosen by model name.
func (c *ApiController) AgentChatCompletions() {
	c.proxyByAgent(openAiChat)
}

// AgentResponses is the per-agent entry point of the Responses API, which is
// the one Codex reaches the gateway on.
func (c *ApiController) AgentResponses() {
	c.proxyByAgent(openAiResponses)
}

// AgentMessages is the per-agent entry point for Anthropic clients. One base URL
// serves every API: an OpenAI client appends /chat/completions to it, Codex
// appends /responses, and an Anthropic one appends /v1/messages.
func (c *ApiController) AgentMessages() {
	c.proxyByAgent(anthropicMessages)
}

// AgentCountTokens is the per-agent Anthropic token-counting endpoint.
func (c *ApiController) AgentCountTokens() {
	c.proxyByAgent(anthropicCountTokens)
}

// proxyByModel forwards to the providers that serve the model the request names.
func (c *ApiController) proxyByModel(target proxyTarget) {
	route, ok := c.readProxyRoute(target)
	if !ok {
		return
	}
	c.forwardByModel(route)
}

// forwardByModel is the rest of that, for an entry point that read its route
// somewhere other than the request body.
func (c *ApiController) forwardByModel(route *proxyRoute) {
	route.source = "model: " + route.model
	// Every way out of a proxy entry point ends the client request, which is
	// what a record describes, so this is the only place one is written.
	defer c.finishLlmRecord(route)

	// Match the providers globally, without an owner filter. A routing rule
	// covering this model puts its own ladder in front of them.
	attempts, err := object.PlanModelRoute(route.model)
	if err != nil {
		if errors.Is(err, object.ErrNoProviderAvailable) {
			route.recordOutcome(http.StatusBadRequest, err.Error())
			c.writeProxyError(route.codec, http.StatusBadRequest, "invalid_request_error", err.Error())
		} else {
			beego.Error("provider lookup failed:", err)
			route.recordOutcome(http.StatusBadGateway, "provider lookup failed")
			c.writeProxyError(route.codec, http.StatusBadGateway, "server_error", "provider lookup failed")
		}
		return
	}

	c.forwardAttempts(attempts, route)
}

// proxyByAgent forwards to the provider chain bound to the agent in the path.
func (c *ApiController) proxyByAgent(target proxyTarget) {
	route, ok := c.readProxyRoute(target)
	if !ok {
		return
	}
	c.forwardByAgent(route)
}

// forwardByAgent is the rest of that, for an entry point that read its route
// somewhere other than the request body.
func (c *ApiController) forwardByAgent(route *proxyRoute) {
	agentId := c.Ctx.Input.Param(":agentId")
	route.source = "agent: " + agentId
	if route.record != nil {
		route.record.Agent = agentId
	}
	defer c.finishLlmRecord(route)

	// What this agent is allowed to ask for. A guard is nil for an agent whose
	// permissions are off, which is every agent until someone turns them on.
	guard, err := object.LoadAgentGuard(agentId)
	if err != nil {
		beego.Error("agent permission lookup failed:", err)
		route.recordOutcome(http.StatusBadGateway, err.Error())
		c.writeProxyError(route.codec, http.StatusBadGateway, "server_error", err.Error())
		return
	}
	if guard != nil && !c.allowAgentRequest(guard, agentId, route) {
		return
	}

	// The whole chain is forwarded to, so a bound provider that is down fails
	// over to the agent's fallbacks instead of failing the request.
	providers, err := object.GetProvidersByAgent(agentId)
	if err != nil {
		if errors.Is(err, object.ErrAgentNoProvider) {
			route.recordOutcome(http.StatusBadRequest, err.Error())
			c.writeProxyError(route.codec, http.StatusBadRequest, "invalid_request_error", err.Error())
		} else {
			beego.Error("agent provider lookup failed:", err)
			route.recordOutcome(http.StatusBadGateway, err.Error())
			c.writeProxyError(route.codec, http.StatusBadGateway, "server_error", err.Error())
		}
		return
	}

	// The ladder of a rule covering this agent comes first, resolved against the
	// chain, and the chain itself is what it falls back to.
	attempts, err := object.PlanAgentRoute(agentId, route.model, providers)
	if err != nil {
		beego.Error("model route lookup failed:", err)
		route.recordOutcome(http.StatusBadGateway, err.Error())
		c.writeProxyError(route.codec, http.StatusBadGateway, "server_error", err.Error())
		return
	}

	// Held to the permissions after planning rather than before: a rule may name
	// a provider outside the chain and may ask for another model, and neither is
	// a way around what this agent is allowed.
	if guard != nil {
		allowed, reason := guard.FilterAttempts(attempts)
		if len(allowed) == 0 {
			route.recordOutcome(http.StatusForbidden, reason)
			c.writeProxyError(route.codec, http.StatusForbidden, "permission_error", reason)
			return
		}
		attempts = allowed
	}

	c.forwardAttempts(attempts, route)
}

// allowAgentRequest holds one relayed request to the agent's permissions: a
// model it may not ask for is refused outright, and the tools of a group it may
// not use are taken back out of the body before any provider sees it. A tool
// the model is never offered is one the agent cannot run.
func (c *ApiController) allowAgentRequest(guard *object.AgentGuard, agentId string, route *proxyRoute) bool {
	if !guard.AllowModel(route.model) {
		message := fmt.Sprintf("the permissions of agent %s do not allow the model %s", agentId, route.model)
		route.recordOutcome(http.StatusForbidden, message)
		c.writeProxyError(route.codec, http.StatusForbidden, "permission_error", message)
		return false
	}

	if filtered, removed := guard.FilterTools(route.body); len(removed) > 0 {
		// The record is written from the body, so it shows what was sent rather
		// than what the agent asked to send.
		route.body = filtered
		beego.Info("agent", agentId, "is not allowed these tools, they were removed from the request:", strings.Join(removed, ", "))
	}
	return true
}

func (c *ApiController) readProxyRoute(target proxyTarget) (*proxyRoute, bool) {
	if !c.allowRelayFor(target) {
		return nil, false
	}

	var fields routingFields
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &fields); err != nil {
		c.writeProxyError(protocol.Of(target.protocol), http.StatusBadRequest,
			"invalid_request_error", "invalid request body")
		return nil, false
	}
	return c.newProxyRoute(target, fields.Model, fields.Stream)
}

// allowRelayFor answers an off-box caller without the relay token in the format
// it called in, and reports whether the request may go on.
func (c *ApiController) allowRelayFor(target proxyTarget) bool {
	if c.allowRelay() {
		return true
	}
	c.writeProxyError(protocol.Of(target.protocol), http.StatusUnauthorized, "authentication_error",
		"this relay is reachable from the network, so it needs the token shown next to the provider in Casbin Gateway")
	return false
}

// newProxyRoute builds the route the forwarding works off. The model and the
// stream flag come out of the body for most APIs and out of the URL for Gemini,
// which names them there.
func (c *ApiController) newProxyRoute(target proxyTarget, model string, stream bool) (*proxyRoute, bool) {
	codec := protocol.Of(target.protocol)
	if model == "" {
		c.writeProxyError(codec, http.StatusBadRequest, "invalid_request_error", "model is required")
		return nil, false
	}

	route := &proxyRoute{
		target: target, codec: codec, body: c.Ctx.Input.RequestBody,
		model: model, stream: stream, start: time.Now(),
	}
	if object.IsLlmRecording() {
		route.record = &object.LlmRecord{
			Protocol: target.protocol,
			Endpoint: target.endpoint,
			Model:    model,
			ClientIp: util.GetClientIp(c.Ctx.Request),
			Stream:   stream,
		}
	}
	return route, true
}

// forwardAttempts relays the request down the plan until one attempt answers.
// An attempt is a provider and the model to ask it for, so failing over can
// change either: to another upstream serving the same model, or down to a
// lesser model when nothing above it can answer.
func (c *ApiController) forwardAttempts(attempts []object.RouteAttempt, route *proxyRoute) {
	// Drop the providers this proxy cannot talk to before forwarding, so that
	// the last usable attempt is known and its response can be relayed as-is.
	usable := []object.RouteAttempt{}
	skipReason := ""
	reported := map[string]bool{}
	for _, attempt := range attempts {
		if reason := c.providerUnusableReason(attempt.Provider); reason != "" {
			if id := attempt.Provider.GetId(); !reported[id] {
				reported[id] = true
				beego.Error("skipped provider", id+":", reason)
			}
			skipReason = reason
			continue
		}
		usable = append(usable, attempt)
	}
	if len(usable) == 0 {
		message := fmt.Sprintf("no usable provider for %s", route.source)
		if skipReason != "" {
			message = skipReason
		}
		route.recordOutcome(http.StatusBadGateway, message)
		c.writeProxyError(route.codec, http.StatusBadGateway, "server_error", message)
		return
	}

	// A provider inside its failure cooldown goes last, so a dead upstream stops
	// costing every request the time it takes to time out.
	usable = object.SortAttemptsByHealth(usable)

	// Fail over to the next attempt as long as nothing has been written to the
	// client yet. The last one is never retried, so the client gets the real
	// upstream answer instead of a synthesized error.
	lastStatus, lastMessage := http.StatusBadGateway, "upstream connection failed"
	for i, attempt := range usable {
		if c.Ctx.Request.Context().Err() != nil {
			// The client hung up, there is nobody left to fail over for.
			route.recordOutcome(0, "client disconnected")
			return
		}

		status, message, written := c.forwardToProvider(attempt, route, i == len(usable)-1)
		if written {
			return
		}
		route.recordFailure(attempt, status, message)
		lastStatus, lastMessage = status, message
	}

	route.recordOutcome(lastStatus, lastMessage)
	c.writeProxyError(route.codec, lastStatus, "server_error", lastMessage)
}

// forwardToProvider sends the request to a single provider's upstream, asking
// for the model the attempt names. It reports whether the client response was
// already written, and when it was not, the status and message describing the
// failure so that the caller can fail over to the next attempt. The response of
// the last attempt is always relayed, even when its status would otherwise be
// retried.
func (c *ApiController) forwardToProvider(attempt object.RouteAttempt, route *proxyRoute, isLast bool) (int, string, bool) {
	provider := attempt.Provider
	route.recordAttempt(attempt)

	upstream, err := protocol.UpstreamOf(object.ProviderProtocol(provider))
	if err != nil {
		return http.StatusBadGateway, err.Error(), false
	}
	// Counting tokens is an Anthropic endpoint alone. A provider serving another
	// API has none to ask, so rather than failing a request the client needs to
	// size its context, the gateway answers with an estimate of its own.
	if route.target.countTokens && !route.passthrough(upstream) {
		return c.answerCountTokens(route)
	}

	requestBody, err := route.upstreamBody(upstream, attempt.Model)
	if err != nil {
		return http.StatusBadRequest, err.Error(), false
	}
	// A subscription endpoint answers the client its token was granted to, so
	// the body is put in the shape that client would have sent.
	requestBody = object.ShapeSubscriptionBody(provider, requestBody)

	upstreamUrl, err := object.BuildProviderUrl(provider.BaseUrl, upstream.Name(), route.upstreamEndpoint(upstream))
	if err != nil {
		object.ReportProviderFailure(provider.GetId(), err.Error())
		return http.StatusBadGateway, err.Error(), false
	}
	// The query selects a variant of the endpoint the client called, which only
	// means the same thing on an upstream serving that same API.
	if route.passthrough(upstream) {
		upstreamUrl = object.AppendQuery(upstreamUrl, c.Ctx.Request.URL.RawQuery)
	}
	upstreamUrl = object.AppendQuery(upstreamUrl, object.SubscriptionQuery(provider))

	// The context is cancelled when this function returns, which happens only
	// after the response body has been relayed to the client.
	ctx, cancel := context.WithCancel(c.Ctx.Request.Context())
	defer cancel()

	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamUrl, bytes.NewReader(requestBody))
	if err != nil {
		return http.StatusBadGateway, "upstream connection failed", false
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	// The sign-in a subscription provider spends is renewed here rather than on
	// a timer, so a token is only refreshed when something is about to send it.
	if object.UsesSubscription(provider) {
		if err := object.EnsureSubscription(provider); err != nil {
			object.ReportProviderFailure(provider.GetId(), err.Error())
			return http.StatusBadGateway, err.Error(), false
		}
	}
	object.SetProviderAuth(upstreamReq.Header, provider)
	if object.UsesClientAuth(provider) {
		c.copyClientAuthHeaders(upstreamReq.Header, upstream)
	}
	if upstream.Name() == object.ProtocolAnthropic {
		c.copyAnthropicHeaders(upstreamReq.Header)
	}

	upstreamResp, err := proxyClient.Do(upstreamReq)
	if err != nil {
		if c.Ctx.Request.Context().Err() != nil {
			// The client hung up mid-request, there is nothing left to answer.
			route.recordOutcome(0, "client disconnected")
			return 0, "", true
		}

		beego.Error("upstream request to provider", provider.GetId(), "failed:", err)
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			object.ReportProviderFailure(provider.GetId(), "upstream timeout")
			return http.StatusGatewayTimeout, "upstream timeout", false
		}
		object.ReportProviderFailure(provider.GetId(), "upstream connection failed")
		return http.StatusBadGateway, "upstream connection failed", false
	}
	defer upstreamResp.Body.Close()

	reportProviderStatus(provider, upstreamResp.StatusCode)

	if !isLast && isRetryableStatus(upstreamResp.StatusCode) {
		beego.Error("provider", provider.GetId(), "returned a retryable status:", upstreamResp.Status)
		// Read a bounded amount rather than discarding it: the connection can
		// still be pooled, and the reason this provider was skipped is in there.
		skipped, _ := io.ReadAll(io.LimitReader(upstreamResp.Body, maxErrorBodyBytes))
		message := fmt.Sprintf("upstream returned %s", upstreamResp.Status)
		if _, detail := protocol.ReadError(skipped, ""); detail != "" {
			message += ": " + detail
		}
		return upstreamResp.StatusCode, message, false
	}

	// Abort the request when the upstream goes silent, rather than capping the
	// total duration, which would cut off long but healthy streams.
	body := newIdleTimeoutReader(upstreamResp.Body, proxyIdleTimeout, cancel)
	defer body.Stop()

	route.recordOutcome(upstreamResp.StatusCode, "")
	if route.record == nil {
		c.relayResponse(route, upstream, upstreamResp, body)
		return 0, "", true
	}

	// The tap reads the counters out of the upstream answer as it passes, so it
	// sees them in the provider's own spelling, translated or not. On a failure
	// the same bytes are the error the client was handed.
	tap := &usageTap{reader: body}
	c.relayResponse(route, upstream, upstreamResp, tap)
	if isSuccessStatus(upstreamResp.StatusCode) {
		route.recordUsage(tap.tail)
	} else {
		route.recordErrorBody(tap.tail)
	}
	return 0, "", true
}

// answerCountTokens answers the token-counting endpoint out of the gateway's
// own estimate, for a provider whose API has no endpoint to ask.
func (c *ApiController) answerCountTokens(route *proxyRoute) (int, string, bool) {
	request, err := route.canonical()
	if err != nil {
		return http.StatusBadRequest, err.Error(), false
	}

	body, err := json.Marshal(countTokensBody(route.target.protocol, protocol.EstimateTokens(request)))
	if err != nil {
		return http.StatusBadGateway, "token count failed", false
	}
	route.recordOutcome(http.StatusOK, "")
	c.writeProxyBody(http.StatusOK, body)
	return 0, "", true
}

// countTokensBody is a token count as the client's own API reports one.
func countTokensBody(name string, tokens int) map[string]any {
	if name == protocol.Gemini {
		return map[string]any{"totalTokens": tokens}
	}
	return map[string]any{"input_tokens": tokens}
}

// reportProviderStatus feeds the breaker that decides in which order providers
// are tried. Every status worth failing over from counts as a provider failure:
// a wrong key or an exhausted quota is not something the next request will do
// better.
func reportProviderStatus(provider *object.Provider, statusCode int) {
	if !isRetryableStatus(statusCode) {
		object.ReportProviderSuccess(provider.GetId())
		return
	}
	object.ReportProviderFailure(provider.GetId(), fmt.Sprintf("upstream returned %d", statusCode))
}

// allowRelay decides whether a request may use the providers stored here. A
// request from this machine always may — that is the whole point of a local
// gateway, and a client-auth provider carries the caller's own vendor
// credential in the same header a token would use. Anything off-box has to
// present the relay token instead.
func (c *ApiController) allowRelay() bool {
	if util.IsLoopbackRequest(c.Ctx.Request) {
		return true
	}

	token := conf.GetRelayToken()
	return token != "" && c.relayCredential() == token
}

// relayCredential is the token the client sent, in either of the two headers
// the OpenAI and Anthropic clients use.
func (c *ApiController) relayCredential() string {
	header := c.Ctx.Request.Header
	if key := strings.TrimSpace(header.Get("X-Api-Key")); key != "" {
		return key
	}
	if key := strings.TrimSpace(header.Get("X-Goog-Api-Key")); key != "" {
		return key
	}

	authorization := strings.TrimSpace(header.Get("Authorization"))
	return strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
}

// clientAuthHeaders are forwarded verbatim by a provider that authenticates with
// the caller's own credentials: the credential itself, plus what the vendors
// expect beside a token issued to a CLI rather than to an API account. It is an
// allowlist, so nothing else the client sent (a browser cookie, say) leaks
// upstream.
var clientAuthHeaders = []string{
	"Authorization",
	"X-Api-Key",
	"X-Goog-Api-Key",
	"User-Agent",
	"X-App",
	"Openai-Beta",
	"Openai-Organization",
	"Openai-Project",
	"Chatgpt-Account-Id",
	"Originator",
	"Session-Id",
	"Thread-Id",
}

// hasClientCredentials reports whether the client request carries a credential
// a client-auth provider could forward.
func (c *ApiController) hasClientCredentials() bool {
	header := c.Ctx.Request.Header
	return header.Get("Authorization") != "" || header.Get("X-Api-Key") != "" ||
		header.Get("X-Goog-Api-Key") != ""
}

func (c *ApiController) copyClientAuthHeaders(dst http.Header, upstream protocol.Upstream) {
	for _, name := range clientAuthHeaders {
		dst.Del(name)
		for _, value := range c.Ctx.Request.Header.Values(name) {
			dst.Add(name, value)
		}
	}

	// A Gemini client sends its key in a header no provider reads, so it is
	// copied into the two the upstreams do.
	if key := dst.Get("X-Goog-Api-Key"); key != "" {
		if dst.Get("X-Api-Key") == "" {
			dst.Set("X-Api-Key", key)
		}
		if dst.Get("Authorization") == "" {
			dst.Set("Authorization", "Bearer "+key)
		}
	}

	// The two APIs carry the credential in different headers, so a client
	// speaking one of them and a provider serving the other would end up
	// authenticating with nothing at all.
	if upstream.Name() == object.ProtocolAnthropic {
		if bearer := strings.TrimPrefix(dst.Get("Authorization"), "Bearer "); dst.Get("X-Api-Key") == "" && bearer != "" {
			dst.Set("X-Api-Key", bearer)
		}
		return
	}
	if key := dst.Get("X-Api-Key"); dst.Get("Authorization") == "" && key != "" {
		dst.Set("Authorization", "Bearer "+key)
	}
}

// copyAnthropicHeaders passes the client's API version and beta opt-ins on to
// the upstream: they select response features, so dropping them would answer a
// different request than the one that was made.
func (c *ApiController) copyAnthropicHeaders(dst http.Header) {
	version := c.Ctx.Request.Header.Get("Anthropic-Version")
	if version == "" {
		version = object.AnthropicVersion
	}
	dst.Set("Anthropic-Version", version)

	for _, beta := range c.Ctx.Request.Header.Values("Anthropic-Beta") {
		dst.Add("Anthropic-Beta", beta)
	}
}

// relayResponse writes the upstream answer back to the client. A provider
// speaking the format the request arrived in is relayed byte for byte, headers
// and all; any other provider is read into the canonical form and written back
// out in the format the client asked in.
func (c *ApiController) relayResponse(route *proxyRoute, upstream protocol.Upstream, upstreamResp *http.Response, body io.Reader) {
	streamed := isEventStream(upstreamResp)
	if route.passthrough(upstream) {
		c.relayVerbatim(upstreamResp, body, route.stream && streamed)
		return
	}
	if !isSuccessStatus(upstreamResp.StatusCode) {
		// An error body names the same two things in both formats, so it is
		// read out of the one and written back in the other.
		raw, _ := io.ReadAll(io.LimitReader(body, maxErrorBodyBytes))
		kind, message := protocol.ReadError(raw, "upstream returned "+upstreamResp.Status)
		c.writeProxyError(route.codec, upstreamResp.StatusCode, kind, message)
		return
	}

	c.translateResponse(route, upstream, body, streamed)
}

// translateResponse rewrites a successful answer into the client's own format.
// Both sides may be streamed or whole, and the four ways round are covered: an
// upstream that ignored stream=true still owes the client its events, and one
// that streamed at a client waiting for a body is collected into one.
func (c *ApiController) translateResponse(route *proxyRoute, upstream protocol.Upstream, body io.Reader, streamed bool) {
	if streamed && route.stream {
		writer := route.codec.NewStreamWriter(c.startEventStream(), c.Ctx.ResponseWriter.Flush, route.model)
		writer.Open()
		err := upstream.DecodeStream(body, func(event protocol.Event) bool {
			writer.Write(event)
			return true
		})
		if err != nil {
			beego.Error("proxy stream read failed:", err)
			writer.Write(protocol.Event{Kind: protocol.EventFailure, Failure: &protocol.Failure{
				Kind: "server_error", Message: err.Error(),
			}})
		}
		writer.Close()
		return
	}

	response, err := c.readUpstreamAnswer(route, upstream, body, streamed)
	if err != nil {
		c.writeProxyError(route.codec, http.StatusBadGateway, "server_error", err.Error())
		return
	}
	if response.Model == "" {
		response.Model = route.model
	}

	if route.stream {
		// The upstream answered in one piece at a client waiting for events, so
		// the whole answer is written out as a stream of one turn.
		writer := route.codec.NewStreamWriter(c.startEventStream(), c.Ctx.ResponseWriter.Flush, route.model)
		protocol.WriteStream(writer, response)
		return
	}

	data, err := route.codec.EncodeResponse(response)
	if err != nil {
		c.writeProxyError(route.codec, http.StatusBadGateway, "server_error", err.Error())
		return
	}
	c.writeProxyBody(http.StatusOK, data)
}

// readUpstreamAnswer reads a whole answer, however the upstream sent it.
func (c *ApiController) readUpstreamAnswer(route *proxyRoute, upstream protocol.Upstream, body io.Reader, streamed bool) (*protocol.Response, error) {
	if streamed {
		collector := protocol.NewCollector(route.model)
		err := upstream.DecodeStream(body, func(event protocol.Event) bool {
			collector.Add(event)
			return true
		})
		if err != nil {
			return nil, err
		}
		return collector.Response(), nil
	}

	data, err := io.ReadAll(body)
	if err != nil {
		return nil, errors.New("upstream read failed")
	}
	return upstream.DecodeResponse(data)
}

// relayVerbatim copies the upstream status code, headers and body to the client
// without any transformation. When flush is true, every chunk is written out as
// soon as it arrives so that SSE clients receive the events in real time.
func (c *ApiController) relayVerbatim(upstreamResp *http.Response, body io.Reader, flush bool) {
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
	for name, values := range src {
		if isHopByHopHeader(name) {
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
	if !isSuccessStatus(upstreamResp.StatusCode) {
		return false
	}
	return strings.Contains(strings.ToLower(upstreamResp.Header.Get("Content-Type")), "text/event-stream")
}

// isSuccessStatus reports whether the upstream answered rather than refused.
func isSuccessStatus(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}

// isRetryableStatus reports whether another provider is worth trying. A rate
// limit, an upstream-side error, a credential this provider rejected, a quota it
// has run out of and a model it does not serve are all specific to that
// provider, while a 4xx caused by the request itself would fail the same way
// everywhere.
func isRetryableStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusUnauthorized, http.StatusPaymentRequired, http.StatusForbidden,
		http.StatusNotFound, http.StatusRequestTimeout, http.StatusTooManyRequests:
		return true
	}
	return statusCode >= 500
}

// providerUnusableReason reports why the proxy cannot forward to a provider, or
// an empty string when it can. The wire format the provider speaks is not one
// of the reasons: a request in another one is translated for it.
func (c *ApiController) providerUnusableReason(provider *object.Provider) string {
	if !object.IsProviderTypeSupported(provider) {
		return fmt.Sprintf("the %s provider type is not supported", provider.Type)
	}
	if provider.BaseUrl == "" {
		return "provider base URL is not configured"
	}
	// Without a credential to forward the upstream would answer 401, which
	// reads as a broken provider rather than a client that sent no key.
	if object.UsesClientAuth(provider) && !c.hasClientCredentials() {
		return fmt.Sprintf("provider %s forwards the credentials of the caller, but the request carries none", provider.GetId())
	}
	if object.UsesSubscription(provider) && !object.HasSubscription(provider) {
		return fmt.Sprintf("provider %s spends a subscription, but it is not signed in", provider.GetId())
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

// writeProxyError writes a JSON error response in the format the client that
// made the request reads failures in.
func (c *ApiController) writeProxyError(codec protocol.Codec, statusCode int, kind string, message string) {
	c.writeProxyBody(statusCode, codec.EncodeError(kind, message))
}

// writeProxyBody writes an answer of the gateway's own making, which carries
// none of the upstream headers: the body relayed under them is gone.
func (c *ApiController) writeProxyBody(statusCode int, body []byte) {
	c.Ctx.ResponseWriter.Header().Set("Content-Type", "application/json")
	c.Ctx.ResponseWriter.WriteHeader(statusCode)
	if _, err := c.Ctx.ResponseWriter.Write(body); err != nil {
		beego.Error("proxy response write failed:", err)
	}
}

// startEventStream begins an SSE response of this gateway's own making.
func (c *ApiController) startEventStream() io.Writer {
	header := c.Ctx.ResponseWriter.Header()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache")
	c.Ctx.ResponseWriter.WriteHeader(http.StatusOK)
	return c.Ctx.ResponseWriter
}
