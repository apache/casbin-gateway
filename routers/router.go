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

package routers

import (
	"github.com/apache/casbin-gateway/controllers"
	"github.com/beego/beego"
)

func init() {
	initAPI()
}

func initAPI() {
	ns :=
		beego.NewNamespace("/api",
			beego.NSInclude(
				&controllers.ApiController{},
			),
		)
	beego.AddNamespace(ns)

	beego.Router("/api/signin", &controllers.ApiController{}, "POST:Signin")
	beego.Router("/api/signout", &controllers.ApiController{}, "POST:Signout")
	beego.Router("/api/get-signin-options", &controllers.ApiController{}, "GET:GetSigninOptions")
	beego.Router("/api/get-account", &controllers.ApiController{}, "GET:GetAccount")
	beego.Router("/api/update-account", &controllers.ApiController{}, "POST:UpdateAccount")
	beego.Router("/api/get-casdoor-providers", &controllers.ApiController{}, "GET:GetCasdoorProviders")
	beego.Router("/api/get-gateway-status", &controllers.ApiController{}, "GET:GetGatewayStatus")
	beego.Router("/api/get-relay-token", &controllers.ApiController{}, "GET:GetRelayToken")
	beego.Router("/api/get-version", &controllers.ApiController{}, "GET:GetVersion")
	beego.Router("/api/update-gateway", &controllers.ApiController{}, "POST:UpdateGateway")
	beego.Router("/api/get-update-status", &controllers.ApiController{}, "GET:GetUpdateStatus")
	beego.Router("/api/get-setting", &controllers.ApiController{}, "GET:GetSetting")
	beego.Router("/api/update-setting", &controllers.ApiController{}, "POST:UpdateSetting")

	beego.Router("/api/get-global-nodes", &controllers.ApiController{}, "GET:GetGlobalNodes")
	beego.Router("/api/get-nodes", &controllers.ApiController{}, "GET:GetNodes")
	beego.Router("/api/get-node", &controllers.ApiController{}, "GET:GetNode")
	beego.Router("/api/update-node", &controllers.ApiController{}, "POST:UpdateNode")
	beego.Router("/api/add-node", &controllers.ApiController{}, "POST:AddNode")
	beego.Router("/api/delete-node", &controllers.ApiController{}, "POST:DeleteNode")

	beego.Router("/api/get-global-sites", &controllers.ApiController{}, "GET:GetGlobalSites")
	beego.Router("/api/get-sites", &controllers.ApiController{}, "GET:GetSites")
	beego.Router("/api/get-site", &controllers.ApiController{}, "GET:GetSite")
	beego.Router("/api/update-site", &controllers.ApiController{}, "POST:UpdateSite")
	beego.Router("/api/add-site", &controllers.ApiController{}, "POST:AddSite")
	beego.Router("/api/delete-site", &controllers.ApiController{}, "POST:DeleteSite")

	beego.Router("/api/get-global-certs", &controllers.ApiController{}, "GET:GetGlobalCerts")
	beego.Router("/api/get-certs", &controllers.ApiController{}, "GET:GetCerts")
	beego.Router("/api/get-cert", &controllers.ApiController{}, "GET:GetCert")
	beego.Router("/api/update-cert", &controllers.ApiController{}, "POST:UpdateCert")
	beego.Router("/api/add-cert", &controllers.ApiController{}, "POST:AddCert")
	beego.Router("/api/delete-cert", &controllers.ApiController{}, "POST:DeleteCert")
	beego.Router("/api/update-cert-domain-expire", &controllers.ApiController{}, "POST:UpdateCertDomainExpire")

	beego.Router("/api/get-applications", &controllers.ApiController{}, "GET:GetApplications")
	beego.Router("/api/get-agents", &controllers.ApiController{}, "GET:GetAgents")
	beego.Router("/api/patch-agent", &controllers.ApiController{}, "POST:PatchAgent")
	beego.Router("/api/unpatch-agent", &controllers.ApiController{}, "POST:UnpatchAgent")
	beego.Router("/api/update-agent-routing", &controllers.ApiController{}, "POST:UpdateAgentRouting")
	beego.Router("/api/get-agent-processes", &controllers.ApiController{}, "GET:GetAgentProcesses")
	beego.Router("/api/start-agent", &controllers.ApiController{}, "POST:StartAgent")
	beego.Router("/api/stop-agent", &controllers.ApiController{}, "POST:StopAgent")
	beego.Router("/api/get-agent-records", &controllers.ApiController{}, "GET:GetAgentRecords")
	beego.Router("/api/get-agent-sessions", &controllers.ApiController{}, "GET:GetAgentSessions")
	beego.Router("/api/get-agent-session", &controllers.ApiController{}, "GET:GetAgentSession")
	beego.Router("/api/add-agent-record", &controllers.ApiController{}, "POST:AddAgentRecord")
	beego.Router("/api/get-agent-configs", &controllers.ApiController{}, "GET:GetAgentConfigs")
	beego.Router("/api/get-agent-config-item", &controllers.ApiController{}, "GET:GetAgentConfigItem")
	beego.Router("/api/delete-agent-config-item", &controllers.ApiController{}, "POST:DeleteAgentConfigItem")
	beego.Router("/api/get-agent-config-trash", &controllers.ApiController{}, "GET:GetAgentConfigTrash")
	beego.Router("/api/restore-agent-config-item", &controllers.ApiController{}, "POST:RestoreAgentConfigItem")
	beego.Router("/api/purge-agent-config-trash", &controllers.ApiController{}, "POST:PurgeAgentConfigTrash")
	beego.Router("/api/update-agent-config-skill", &controllers.ApiController{}, "POST:UpdateAgentConfigSkill")
	beego.Router("/api/add-agent-config-mcp", &controllers.ApiController{}, "POST:AddAgentConfigMcp")
	beego.Router("/api/save-agent-config-prompt", &controllers.ApiController{}, "POST:SaveAgentConfigPrompt")
	beego.Router("/api/plan-agent-config-copy", &controllers.ApiController{}, "POST:PlanAgentConfigCopy")
	beego.Router("/api/plan-agent-provider", &controllers.ApiController{}, "POST:PlanAgentProvider")
	beego.Router("/api/apply-agent-provider", &controllers.ApiController{}, "POST:ApplyAgentProvider")
	beego.Router("/api/restore-agent-provider", &controllers.ApiController{}, "POST:RestoreAgentProvider")
	beego.Router("/api/copy-agent-config", &controllers.ApiController{}, "POST:CopyAgentConfig")

	beego.Router("/api/get-records", &controllers.ApiController{}, "GET:GetRecords")
	beego.Router("/api/get-record", &controllers.ApiController{}, "GET:GetRecord")
	beego.Router("/api/delete-record", &controllers.ApiController{}, "POST:DeleteRecord")
	beego.Router("/api/update-record", &controllers.ApiController{}, "POST:UpdateRecord")
	beego.Router("/api/add-record", &controllers.ApiController{}, "POST:AddRecord")

	beego.Router("/api/get-metrics", &controllers.ApiController{}, "GET:GetMetrics")
	beego.Router("/api/get-metrics-over-time", &controllers.ApiController{}, "GET:GetMetricsOverTime")

	beego.Router("/api/get-rules", &controllers.ApiController{}, "GET:GetRules")
	beego.Router("/api/get-rule", &controllers.ApiController{}, "GET:GetRule")
	beego.Router("/api/add-rule", &controllers.ApiController{}, "POST:AddRule")
	beego.Router("/api/update-rule", &controllers.ApiController{}, "POST:UpdateRule")
	beego.Router("/api/delete-rule", &controllers.ApiController{}, "POST:DeleteRule")

	// Provider routes for LLM gateway milestone 1.1.
	beego.Router("/api/get-providers", &controllers.ApiController{}, "GET:GetProviders")
	beego.Router("/api/get-provider", &controllers.ApiController{}, "GET:GetProvider")
	beego.Router("/api/add-provider", &controllers.ApiController{}, "POST:AddProvider")
	beego.Router("/api/parse-provider-link", &controllers.ApiController{}, "POST:ParseProviderLink")
	beego.Router("/api/update-provider", &controllers.ApiController{}, "POST:UpdateProvider")
	beego.Router("/api/delete-provider", &controllers.ApiController{}, "POST:DeleteProvider")
	beego.Router("/api/get-provider-models", &controllers.ApiController{}, "POST:GetProviderModels")
	beego.Router("/api/test-provider", &controllers.ApiController{}, "POST:TestProvider")
	beego.Router("/api/get-provider-health", &controllers.ApiController{}, "GET:GetProviderHealth")
	beego.Router("/api/get-provider-quotas", &controllers.ApiController{}, "GET:GetProviderQuotas")
	beego.Router("/api/refresh-provider-quotas", &controllers.ApiController{}, "POST:RefreshProviderQuotas")

	beego.Router("/api/get-llm-records", &controllers.ApiController{}, "GET:GetLlmRecords")
	beego.Router("/api/get-llm-record", &controllers.ApiController{}, "GET:GetLlmRecord")
	beego.Router("/api/get-llm-record-status", &controllers.ApiController{}, "GET:GetLlmRecordStatus")
	beego.Router("/api/get-llm-record-stats", &controllers.ApiController{}, "GET:GetLlmRecordStats")
	beego.Router("/api/get-llm-agent-stats", &controllers.ApiController{}, "GET:GetLlmAgentStats")
	beego.Router("/api/stream-llm-records", &controllers.ApiController{}, "GET:StreamLlmRecords")
	beego.Router("/api/delete-llm-record", &controllers.ApiController{}, "POST:DeleteLlmRecord")
	beego.Router("/api/clear-llm-records", &controllers.ApiController{}, "POST:ClearLlmRecords")

	// The LLM gateway, in every wire format a client speaks. The agent routes
	// carry the endpoint shapes those clients append to one base URL.
	beego.Router("/v1/chat/completions", &controllers.ApiController{}, "POST:ChatCompletions")
	beego.Router("/v1/responses", &controllers.ApiController{}, "POST:Responses")
	beego.Router("/v1/messages", &controllers.ApiController{}, "POST:Messages")
	beego.Router("/v1/messages/count_tokens", &controllers.ApiController{}, "POST:CountTokens")
	beego.Router("/v1/models", &controllers.ApiController{}, "GET:Models")
	beego.Router("/v1/models/:modelId", &controllers.ApiController{}, "GET:Model")
	beego.Router("/v1/agents/:agentId/chat/completions", &controllers.ApiController{}, "POST:AgentChatCompletions")
	beego.Router("/v1/agents/:agentId/responses", &controllers.ApiController{}, "POST:AgentResponses")
	beego.Router("/v1/agents/:agentId/v1/messages", &controllers.ApiController{}, "POST:AgentMessages")
	beego.Router("/v1/agents/:agentId/v1/messages/count_tokens", &controllers.ApiController{}, "POST:AgentCountTokens")
	beego.Router("/v1/agents/:agentId/models", &controllers.ApiController{}, "GET:AgentModels")
	beego.Router("/v1/agents/:agentId/v1/models", &controllers.ApiController{}, "GET:AgentModels")
	beego.Router("/v1/agents/:agentId/models/:modelId", &controllers.ApiController{}, "GET:AgentModel")
	beego.Router("/v1/agents/:agentId/v1/models/:modelId", &controllers.ApiController{}, "GET:AgentModel")
}
