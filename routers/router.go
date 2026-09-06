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
	beego.Router("/api/get-relay-token", &controllers.ApiController{}, "GET:GetRelayToken")
	beego.Router("/api/get-version", &controllers.ApiController{}, "GET:GetVersion")
	beego.Router("/api/update-gateway", &controllers.ApiController{}, "POST:UpdateGateway")
	beego.Router("/api/get-update-status", &controllers.ApiController{}, "GET:GetUpdateStatus")
	beego.Router("/api/get-setting", &controllers.ApiController{}, "GET:GetSetting")
	beego.Router("/api/update-setting", &controllers.ApiController{}, "POST:UpdateSetting")
	beego.Router("/api/test-outbound-proxy", &controllers.ApiController{}, "POST:TestOutboundProxy")
	beego.Router("/api/get-autostart", &controllers.ApiController{}, "GET:GetAutostart")
	beego.Router("/api/update-autostart", &controllers.ApiController{}, "POST:UpdateAutostart")

	beego.Router("/api/export-snapshot", &controllers.ApiController{}, "POST:ExportSnapshot")
	beego.Router("/api/import-snapshot", &controllers.ApiController{}, "POST:ImportSnapshot")
	beego.Router("/api/get-backup-state", &controllers.ApiController{}, "GET:GetBackupState")
	beego.Router("/api/get-backup", &controllers.ApiController{}, "GET:GetBackup")
	beego.Router("/api/create-backup", &controllers.ApiController{}, "POST:CreateBackup")
	beego.Router("/api/restore-backup", &controllers.ApiController{}, "POST:RestoreBackup")
	beego.Router("/api/delete-backup", &controllers.ApiController{}, "POST:DeleteBackup")
	beego.Router("/api/update-backup-schedule", &controllers.ApiController{}, "POST:UpdateBackupSchedule")
	beego.Router("/api/get-cloud-sync-state", &controllers.ApiController{}, "GET:GetCloudSyncState")
	beego.Router("/api/update-cloud-sync", &controllers.ApiController{}, "POST:UpdateCloudSync")
	beego.Router("/api/test-cloud-sync", &controllers.ApiController{}, "POST:TestCloudSync")
	beego.Router("/api/run-cloud-sync", &controllers.ApiController{}, "POST:RunCloudSync")

	beego.Router("/api/get-agents", &controllers.ApiController{}, "GET:GetAgents")
	beego.Router("/api/get-agent-catalog", &controllers.ApiController{}, "GET:GetAgentCatalog")
	beego.Router("/api/get-agent-install-jobs", &controllers.ApiController{}, "GET:GetAgentInstallJobs")
	beego.Router("/api/install-agent", &controllers.ApiController{}, "POST:InstallAgent")
	beego.Router("/api/upgrade-agent", &controllers.ApiController{}, "POST:UpgradeAgent")
	beego.Router("/api/set-agent-version", &controllers.ApiController{}, "POST:SetAgentVersion")
	beego.Router("/api/uninstall-agent", &controllers.ApiController{}, "POST:UninstallAgent")
	beego.Router("/api/get-agent-versions", &controllers.ApiController{}, "GET:GetAgentVersions")
	beego.Router("/api/browse-local-path", &controllers.ApiController{}, "GET:BrowseLocalPath")
	beego.Router("/api/add-agent-path", &controllers.ApiController{}, "POST:AddAgentPath")
	beego.Router("/api/remove-agent-path", &controllers.ApiController{}, "POST:RemoveAgentPath")
	beego.Router("/api/get-agent-updates", &controllers.ApiController{}, "GET:GetAgentUpdates")
	beego.Router("/api/patch-agent", &controllers.ApiController{}, "POST:PatchAgent")
	beego.Router("/api/unpatch-agent", &controllers.ApiController{}, "POST:UnpatchAgent")
	beego.Router("/api/update-agent-routing", &controllers.ApiController{}, "POST:UpdateAgentRouting")
	beego.Router("/api/get-agent-permission", &controllers.ApiController{}, "GET:GetAgentPermission")
	beego.Router("/api/get-agent-permissions", &controllers.ApiController{}, "GET:GetAgentPermissions")
	beego.Router("/api/update-agent-permission", &controllers.ApiController{}, "POST:UpdateAgentPermission")
	beego.Router("/api/get-agent-processes", &controllers.ApiController{}, "GET:GetAgentProcesses")
	beego.Router("/api/start-agent", &controllers.ApiController{}, "POST:StartAgent")
	beego.Router("/api/stop-agent", &controllers.ApiController{}, "POST:StopAgent")
	beego.Router("/api/get-agent-instances", &controllers.ApiController{}, "GET:GetAgentInstances")
	beego.Router("/api/add-agent-instance", &controllers.ApiController{}, "POST:AddAgentInstance")
	beego.Router("/api/update-agent-instance", &controllers.ApiController{}, "POST:UpdateAgentInstance")
	beego.Router("/api/delete-agent-instance", &controllers.ApiController{}, "POST:DeleteAgentInstance")
	beego.Router("/api/start-agent-instance", &controllers.ApiController{}, "POST:StartAgentInstance")
	beego.Router("/api/stop-agent-instance", &controllers.ApiController{}, "POST:StopAgentInstance")
	beego.Router("/api/capture-agent-instance-link", &controllers.ApiController{}, "POST:CaptureAgentInstanceLink")

	// Driving an agent: Gateway hands it a prompt and reads back what it says.
	beego.Router("/api/get-drivable-agents", &controllers.ApiController{}, "GET:GetDrivableAgents")
	beego.Router("/api/get-driven-sessions", &controllers.ApiController{}, "GET:GetDrivenSessions")
	beego.Router("/api/open-driven-session", &controllers.ApiController{}, "POST:OpenDrivenSession")
	beego.Router("/api/send-driven-session", &controllers.ApiController{}, "POST:SendDrivenSession")
	beego.Router("/api/interrupt-driven-session", &controllers.ApiController{}, "POST:InterruptDrivenSession")
	beego.Router("/api/close-driven-session", &controllers.ApiController{}, "POST:CloseDrivenSession")
	beego.Router("/api/stream-driven-session", &controllers.ApiController{}, "GET:StreamDrivenSession")
	beego.Router("/api/get-agent-accounts", &controllers.ApiController{}, "GET:GetAgentAccounts")
	beego.Router("/api/save-agent-account", &controllers.ApiController{}, "POST:SaveAgentAccount")
	beego.Router("/api/add-agent-account", &controllers.ApiController{}, "POST:AddAgentAccount")
	beego.Router("/api/switch-agent-account", &controllers.ApiController{}, "POST:SwitchAgentAccount")
	beego.Router("/api/update-agent-account", &controllers.ApiController{}, "POST:UpdateAgentAccount")
	beego.Router("/api/delete-agent-account", &controllers.ApiController{}, "POST:DeleteAgentAccount")
	beego.Router("/api/sign-in-agent-account", &controllers.ApiController{}, "POST:SignInAgentAccount")
	beego.Router("/api/get-agent-signin", &controllers.ApiController{}, "GET:GetAgentSignin")
	beego.Router("/api/get-agent-records", &controllers.ApiController{}, "GET:GetAgentRecords")
	beego.Router("/api/get-agent-sessions", &controllers.ApiController{}, "GET:GetAgentSessions")
	beego.Router("/api/get-agent-session", &controllers.ApiController{}, "GET:GetAgentSession")
	beego.Router("/api/get-agent-usage", &controllers.ApiController{}, "GET:GetAgentUsage")
	beego.Router("/api/add-agent-record", &controllers.ApiController{}, "POST:AddAgentRecord")
	beego.Router("/api/check-agent-tool", &controllers.ApiController{}, "POST:CheckAgentTool")
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

	beego.Router("/api/get-connectors", &controllers.ApiController{}, "GET:GetConnectors")
	beego.Router("/api/get-connection", &controllers.ApiController{}, "GET:GetConnection")
	beego.Router("/api/connect", &controllers.ApiController{}, "POST:Connect")
	beego.Router("/api/disconnect", &controllers.ApiController{}, "POST:Disconnect")
	beego.Router("/api/resolve-connection", &controllers.ApiController{}, "POST:ResolveConnection")
	beego.Router("/api/get-connector-redirect-uri", &controllers.ApiController{}, "GET:GetConnectorRedirectUri")
	beego.Router("/api/start-connector-auth", &controllers.ApiController{}, "POST:StartConnectorAuth")
	beego.Router("/api/test-connection", &controllers.ApiController{}, "POST:TestConnection")
	beego.Router("/api/retest-connections", &controllers.ApiController{}, "POST:RetestConnections")
	// Opened by the vendor's redirect rather than by the UI, so it is reached
	// without a session: what proves the caller is the unguessable state it
	// carries, which is the whole point of that parameter.
	beego.Router("/api/connector-auth-callback", &controllers.ApiController{}, "GET:ConnectorAuthCallback")

	// The desktop tray, which holds the local token instead of a session.
	beego.Router("/api/get-tray-menu", &controllers.ApiController{}, "GET:GetTrayMenu")
	beego.Router("/api/set-agent-provider", &controllers.ApiController{}, "POST:SetAgentProvider")

	beego.Router("/api/get-skill-sources", &controllers.ApiController{}, "GET:GetSkillSources")
	beego.Router("/api/add-skill-source", &controllers.ApiController{}, "POST:AddSkillSource")
	beego.Router("/api/upload-skill-source", &controllers.ApiController{}, "POST:UploadSkillSource")
	beego.Router("/api/delete-skill-source", &controllers.ApiController{}, "POST:DeleteSkillSource")
	beego.Router("/api/get-skill-catalog", &controllers.ApiController{}, "GET:GetSkillCatalog")
	beego.Router("/api/install-skills", &controllers.ApiController{}, "POST:InstallSkills")
	beego.Router("/api/get-unmanaged-skills", &controllers.ApiController{}, "GET:GetUnmanagedSkills")
	beego.Router("/api/adopt-skills", &controllers.ApiController{}, "POST:AdoptSkills")

	// A vendor's "add this to Gateway" link, whether it was pasted into a page
	// or clicked outside the browser and routed here by the URL scheme handler.
	beego.Router("/api/parse-import-link", &controllers.ApiController{}, "POST:ParseImportLink")
	beego.Router("/api/open-import-link", &controllers.ApiController{}, "POST:OpenImportLink")
	beego.Router("/api/get-pending-import-link", &controllers.ApiController{}, "GET:GetPendingImportLink")

	// Everything a CC Switch installation on this machine holds, brought over
	// in one go by somebody moving to Gateway.
	beego.Router("/api/get-ccswitch-import", &controllers.ApiController{}, "GET:GetCcSwitchImport")
	beego.Router("/api/import-ccswitch", &controllers.ApiController{}, "POST:ImportCcSwitch")

	// Provider routes for LLM gateway milestone 1.1.
	beego.Router("/api/get-providers", &controllers.ApiController{}, "GET:GetProviders")
	beego.Router("/api/get-provider", &controllers.ApiController{}, "GET:GetProvider")
	beego.Router("/api/add-provider", &controllers.ApiController{}, "POST:AddProvider")
	beego.Router("/api/parse-provider-link", &controllers.ApiController{}, "POST:ParseProviderLink")
	beego.Router("/api/update-provider", &controllers.ApiController{}, "POST:UpdateProvider")
	beego.Router("/api/delete-provider", &controllers.ApiController{}, "POST:DeleteProvider")
	beego.Router("/api/get-provider-models", &controllers.ApiController{}, "POST:GetProviderModels")
	beego.Router("/api/test-provider", &controllers.ApiController{}, "POST:TestProvider")
	beego.Router("/api/sign-in-provider", &controllers.ApiController{}, "POST:SignInProvider")
	beego.Router("/api/get-provider-signin", &controllers.ApiController{}, "GET:GetProviderSignin")
	beego.Router("/api/get-provider-health", &controllers.ApiController{}, "GET:GetProviderHealth")
	beego.Router("/api/get-provider-quotas", &controllers.ApiController{}, "GET:GetProviderQuotas")
	beego.Router("/api/refresh-provider-quotas", &controllers.ApiController{}, "POST:RefreshProviderQuotas")
	beego.Router("/api/get-provider-probes", &controllers.ApiController{}, "GET:GetProviderProbes")
	beego.Router("/api/get-provider-probe-history", &controllers.ApiController{}, "GET:GetProviderProbeHistory")
	beego.Router("/api/probe-provider", &controllers.ApiController{}, "POST:ProbeProvider")
	beego.Router("/api/get-probe-cases", &controllers.ApiController{}, "GET:GetProbeCases")
	beego.Router("/api/add-probe-case", &controllers.ApiController{}, "POST:AddProbeCase")
	beego.Router("/api/update-probe-case", &controllers.ApiController{}, "POST:UpdateProbeCase")
	beego.Router("/api/delete-probe-case", &controllers.ApiController{}, "POST:DeleteProbeCase")
	beego.Router("/api/reset-probe-cases", &controllers.ApiController{}, "POST:ResetProbeCases")

	// The rules deciding which model a request is sent, and what it steps down
	// to when that cannot answer.
	beego.Router("/api/get-model-routes", &controllers.ApiController{}, "GET:GetModelRoutes")
	beego.Router("/api/add-model-route", &controllers.ApiController{}, "POST:AddModelRoute")
	beego.Router("/api/update-model-route", &controllers.ApiController{}, "POST:UpdateModelRoute")
	beego.Router("/api/delete-model-route", &controllers.ApiController{}, "POST:DeleteModelRoute")
	beego.Router("/api/preview-model-route", &controllers.ApiController{}, "GET:PreviewModelRoute")

	beego.Router("/api/get-llm-records", &controllers.ApiController{}, "GET:GetLlmRecords")
	beego.Router("/api/get-llm-record", &controllers.ApiController{}, "GET:GetLlmRecord")
	beego.Router("/api/get-llm-record-status", &controllers.ApiController{}, "GET:GetLlmRecordStatus")
	beego.Router("/api/get-llm-record-stats", &controllers.ApiController{}, "GET:GetLlmRecordStats")
	beego.Router("/api/get-llm-agent-stats", &controllers.ApiController{}, "GET:GetLlmAgentStats")
	beego.Router("/api/get-llm-usage-trend", &controllers.ApiController{}, "GET:GetLlmUsageTrend")
	beego.Router("/api/get-llm-provider-audit", &controllers.ApiController{}, "GET:GetLlmProviderAudit")
	beego.Router("/api/stream-llm-records", &controllers.ApiController{}, "GET:StreamLlmRecords")
	beego.Router("/api/delete-llm-record", &controllers.ApiController{}, "POST:DeleteLlmRecord")
	beego.Router("/api/clear-llm-records", &controllers.ApiController{}, "POST:ClearLlmRecords")

	beego.Router("/api/get-llm-prices", &controllers.ApiController{}, "GET:GetLlmPrices")
	beego.Router("/api/update-llm-price", &controllers.ApiController{}, "POST:UpdateLlmPrice")
	beego.Router("/api/delete-llm-price", &controllers.ApiController{}, "POST:DeleteLlmPrice")
	beego.Router("/api/search-models-dev-models", &controllers.ApiController{}, "GET:SearchModelsDevModels")
	beego.Router("/api/sync-models-dev-prices", &controllers.ApiController{}, "POST:SyncModelsDevPrices")
	beego.Router("/api/get-models-dev-sync", &controllers.ApiController{}, "GET:GetModelsDevSync")
	beego.Router("/api/update-models-dev-sync", &controllers.ApiController{}, "POST:UpdateModelsDevSync")

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
	// The Gemini API, which names the model and the call in the path: the
	// Google client library appends "/v1beta/models/<model>:<action>" to the
	// base URL it is given.
	beego.Router("/v1beta/models", &controllers.ApiController{}, "GET:GeminiModels")
	beego.Router("/v1beta/models/:model", &controllers.ApiController{}, "GET:GeminiModel;POST:GeminiGenerate")
	beego.Router("/v1/agents/:agentId/v1beta/models", &controllers.ApiController{}, "GET:AgentGeminiModels")
	beego.Router("/v1/agents/:agentId/v1beta/models/:model", &controllers.ApiController{}, "GET:AgentGeminiModel;POST:AgentGeminiGenerate")
}
