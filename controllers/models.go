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
	"time"

	"github.com/apache/casbin-gateway/object"
	"github.com/apache/casbin-gateway/protocol"
	"github.com/beego/beego"
)

// Models answers the model list every client asks for to fill its model picker.
// OpenAI and Anthropic both call GET /v1/models and read a different shape out
// of the answer, so one entry carries the fields of both.
func (c *ApiController) Models() {
	if !c.allowModelListing() {
		return
	}

	models, err := object.ListEnabledModels()
	if err != nil {
		beego.Error("model listing failed:", err)
		c.writeModelsError(http.StatusBadGateway, "server_error", "provider lookup failed")
		return
	}
	c.writeModelList(models)
}

// Model answers the retrieve-a-model endpoint an Anthropic client checks the
// configured model with. A name reaches an upstream as long as one provider is
// enabled or a rule routes it, see PlanModelRoute(), so that is what
// "available" means here: answering 404 for a name the gateway would happily
// route would tell the client its model is missing when it is not.
func (c *ApiController) Model() {
	if !c.allowModelListing() {
		return
	}

	model := c.Ctx.Input.Param(":modelId")
	if _, err := object.PlanModelRoute(model); err != nil {
		if errors.Is(err, object.ErrNoProviderAvailable) {
			c.writeModelsError(http.StatusNotFound, "not_found_error", err.Error())
		} else {
			beego.Error("model lookup failed:", err)
			c.writeModelsError(http.StatusBadGateway, "server_error", "provider lookup failed")
		}
		return
	}
	c.writeProxyBody(http.StatusOK, mustEncode(modelEntry(model)))
}

// AgentModel is the same check against the agent's own provider chain.
func (c *ApiController) AgentModel() {
	if !c.allowModelListing() {
		return
	}

	agentId := c.Ctx.Input.Param(":agentId")
	if _, err := object.GetProvidersByAgent(agentId); err != nil {
		c.writeModelsError(http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	c.writeProxyBody(http.StatusOK, mustEncode(modelEntry(c.Ctx.Input.Param(":modelId"))))
}

// AgentModels is the same list narrowed to what the agent's own provider chain
// serves, for a client pointed at ".../v1/agents/<agentId>".
func (c *ApiController) AgentModels() {
	if !c.allowModelListing() {
		return
	}

	agentId := c.Ctx.Input.Param(":agentId")
	providers, err := object.GetProvidersByAgent(agentId)
	if err != nil {
		c.writeModelsError(http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	c.writeModelList(object.ModelsWithRoutes(object.ModelsOfProviders(providers), agentId))
}

// allowModelListing gates the listing on the same token the proxy endpoints ask
// an off-box caller for.
func (c *ApiController) allowModelListing() bool {
	if c.allowRelay() {
		return true
	}
	c.writeModelsError(http.StatusUnauthorized, "authentication_error",
		"this relay is reachable from the network, so it needs the token shown next to the provider in Casbin Gateway")
	return false
}

func (c *ApiController) writeModelList(models []string) {
	entries := []map[string]any{}
	for _, model := range models {
		entries = append(entries, modelEntry(model))
	}

	payload := map[string]any{"object": "list", "data": entries, "has_more": false}
	if len(models) > 0 {
		payload["first_id"] = models[0]
		payload["last_id"] = models[len(models)-1]
	}
	c.writeProxyBody(http.StatusOK, mustEncode(payload))
}

// modelEntry carries the fields of both APIs, since one endpoint answers the
// clients of both.
func modelEntry(model string) map[string]any {
	created := time.Now().Unix()
	return map[string]any{
		"id":           model,
		"object":       "model",
		"type":         "model",
		"display_name": model,
		"created":      created,
		"created_at":   time.Unix(created, 0).UTC().Format(time.RFC3339),
		"owned_by":     "casbin-gateway",
	}
}

func mustEncode(payload map[string]any) []byte {
	body, err := json.Marshal(payload)
	if err != nil {
		beego.Error("model encoding failed:", err)
		return []byte(`{"object":"list","data":[],"has_more":false}`)
	}
	return body
}

// writeModelsError uses the Anthropic error shape, whose "error" object is the
// one an OpenAI client reads too.
func (c *ApiController) writeModelsError(statusCode int, kind string, message string) {
	c.writeProxyError(protocol.Of(protocol.Anthropic), statusCode, kind, message)
}
