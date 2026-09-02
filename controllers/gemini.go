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
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/apache/casbin-gateway/object"
	"github.com/apache/casbin-gateway/protocol"
	"github.com/beego/beego"
)

// The Gemini API puts the model and the call in one path segment, spelled
// "<model>:<action>", and asks for a stream by the action rather than by a
// field of the body.
const (
	geminiGenerateAction    = "generateContent"
	geminiStreamAction      = "streamGenerateContent"
	geminiCountTokensAction = "countTokens"
)

var (
	geminiGenerate    = proxyTarget{protocol: protocol.Gemini, endpoint: "/v1beta/models:generateContent"}
	geminiStream      = proxyTarget{protocol: protocol.Gemini, endpoint: "/v1beta/models:streamGenerateContent"}
	geminiCountTokens = proxyTarget{
		protocol: protocol.Gemini, endpoint: "/v1beta/models:countTokens", countTokens: true,
	}
)

// The context a model entry claims, for the client that sizes its own prompt
// against the answer. The gateway does not know what the bound provider serves,
// so these are the limits of the Gemini models the clients expect here.
const (
	geminiInputTokenLimit  = 1048576
	geminiOutputTokenLimit = 65536
)

// GeminiGenerate is the Gemini API entry point, which the Gemini CLI reaches
// its provider on. No provider serves this API, so the request is translated on
// the way out and the answer back on the way in.
func (c *ApiController) GeminiGenerate() {
	route, ok := c.readGeminiRoute()
	if !ok {
		return
	}
	c.forwardByModel(route)
}

// AgentGeminiGenerate is the per-agent entry point of the same API: an agent
// pointed at ".../v1/agents/<agentId>" appends "/v1beta/models/<model>:<action>"
// to it, which is the path the Google client library builds.
func (c *ApiController) AgentGeminiGenerate() {
	route, ok := c.readGeminiRoute()
	if !ok {
		return
	}
	c.forwardByAgent(route)
}

// readGeminiRoute reads the model and the call out of the path, since this API
// names neither in the body.
func (c *ApiController) readGeminiRoute() (*proxyRoute, bool) {
	model, action := geminiCall(c.Ctx.Input.Param(":model"))

	target := geminiGenerate
	switch action {
	case geminiGenerateAction:
	case geminiStreamAction:
		target = geminiStream
	case geminiCountTokensAction:
		target = geminiCountTokens
	default:
		c.writeGeminiError(http.StatusNotFound, "not_found_error",
			"the gateway does not serve the "+action+" method")
		return nil, false
	}

	if !c.allowRelayFor(target) {
		return nil, false
	}
	if !json.Valid(c.Ctx.Input.RequestBody) {
		c.writeGeminiError(http.StatusBadRequest, "invalid_request_error", "invalid request body")
		return nil, false
	}
	return c.newProxyRoute(target, model, action == geminiStreamAction)
}

// geminiCall splits "<model>:<action>" into the two. A path segment without an
// action names the model alone, which is what the retrieve-a-model endpoint is.
func geminiCall(param string) (string, string) {
	param = strings.TrimPrefix(param, "models/")
	model, action, found := strings.Cut(param, ":")
	if !found {
		return param, ""
	}
	return model, action
}

// GeminiModels answers the model list a Gemini client fills its picker from.
func (c *ApiController) GeminiModels() {
	if !c.allowGeminiListing() {
		return
	}

	models, err := object.ListEnabledModels()
	if err != nil {
		beego.Error("model listing failed:", err)
		c.writeGeminiError(http.StatusBadGateway, "server_error", "provider lookup failed")
		return
	}
	c.writeProxyBody(http.StatusOK, mustEncode(geminiModelList(models)))
}

// AgentGeminiModels is the same list narrowed to what the agent's own provider
// chain serves.
func (c *ApiController) AgentGeminiModels() {
	if !c.allowGeminiListing() {
		return
	}

	providers, err := object.GetProvidersByAgent(c.Ctx.Input.Param(":agentId"))
	if err != nil {
		c.writeGeminiError(http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	c.writeProxyBody(http.StatusOK, mustEncode(geminiModelList(object.ModelsOfProviders(providers))))
}

// GeminiModel answers the retrieve-a-model endpoint a Gemini client checks its
// configured model with. A name reaches an upstream as long as one provider is
// enabled, which is what "available" means here.
func (c *ApiController) GeminiModel() {
	if !c.allowGeminiListing() {
		return
	}

	model, _ := geminiCall(c.Ctx.Input.Param(":model"))
	if _, err := object.GetProvidersByModel(model); err != nil {
		if errors.Is(err, object.ErrNoProviderAvailable) {
			c.writeGeminiError(http.StatusNotFound, "not_found_error", err.Error())
		} else {
			beego.Error("model lookup failed:", err)
			c.writeGeminiError(http.StatusBadGateway, "server_error", "provider lookup failed")
		}
		return
	}
	c.writeProxyBody(http.StatusOK, mustEncode(geminiModelEntry(model)))
}

// AgentGeminiModel is the same check against the agent's own provider chain.
func (c *ApiController) AgentGeminiModel() {
	if !c.allowGeminiListing() {
		return
	}

	if _, err := object.GetProvidersByAgent(c.Ctx.Input.Param(":agentId")); err != nil {
		c.writeGeminiError(http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	model, _ := geminiCall(c.Ctx.Input.Param(":model"))
	c.writeProxyBody(http.StatusOK, mustEncode(geminiModelEntry(model)))
}

func geminiModelList(models []string) map[string]any {
	entries := []map[string]any{}
	for _, model := range models {
		entries = append(entries, geminiModelEntry(model))
	}
	return map[string]any{"models": entries}
}

func geminiModelEntry(model string) map[string]any {
	return map[string]any{
		"name":                       "models/" + model,
		"baseModelId":                model,
		"version":                    "001",
		"displayName":                model,
		"description":                "Served through Casbin Gateway",
		"inputTokenLimit":            geminiInputTokenLimit,
		"outputTokenLimit":           geminiOutputTokenLimit,
		"supportedGenerationMethods": []string{geminiGenerateAction, geminiStreamAction, geminiCountTokensAction},
	}
}

// allowGeminiListing gates the listing on the same token the proxy endpoints ask
// an off-box caller for.
func (c *ApiController) allowGeminiListing() bool {
	if c.allowRelay() {
		return true
	}
	c.writeGeminiError(http.StatusUnauthorized, "authentication_error",
		"this relay is reachable from the network, so it needs the token shown next to the provider in Casbin Gateway")
	return false
}

func (c *ApiController) writeGeminiError(statusCode int, kind string, message string) {
	c.writeProxyError(protocol.Of(protocol.Gemini), statusCode, kind, message)
}
