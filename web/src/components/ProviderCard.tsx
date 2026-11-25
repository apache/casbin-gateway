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

import * as React from "react";
import {Bot, FileCog, RotateCcw} from "lucide-react";
import i18next from "i18next";

import * as AgentBackend from "@/backend/AgentBackend";
import {EnvSnippet} from "@/components/EnvSnippet";
import {EnvWarningBanner} from "@/components/EnvWarningBanner";
import {AgentIcon} from "@/components/AgentIcon";
import {FallbackQueue} from "@/components/FallbackQueue";
import {ProviderIcon} from "@/components/ProviderIcon";
import {ProviderStreamCheck} from "@/components/ProviderStreamCheck";
import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {Loading} from "@/components/shared/loading";
import {SimpleSelect} from "@/components/shared/simple-select";
import {MessageAlert} from "@/components/ui/alert";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Card, CardContent, CardHeader, CardTitle} from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {Switch} from "@/components/ui/switch";
import {
  agentBuiltin,
  agentGatewayOnlyKey,
  agentProxyBaseUrl,
  agentSetupNoteKey,
  builtinProvider,
  directMode,
  gatewayMode,
} from "@/lib/agents";
import {
  providerIdOf,
  providerProtocol,
  providerWireApi,
  shellForPath,
  usesClientAuth,
} from "@/lib/providers";
import type {Agent, AgentProviderFile, Provider, ProviderHealth} from "@/types";

/** The file preview, loaded when the dialog opens rather than on every render. */
function PreviewDialog({
  agent,
  open,
  onOpenChange,
}: {
  agent: Agent;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const [files, setFiles] = React.useState<AgentProviderFile[]>([]);
  const [error, setError] = React.useState("");
  const [loading, setLoading] = React.useState(false);

  React.useEffect(() => {
    if (!open) {
      return;
    }

    setLoading(true);
    setError("");
    AgentBackend.planAgentProvider({agentId: agent.agentId, path: agent.path, owner: agent.owner})
      .then(res => {
        if (res.status === "ok") {
          setFiles(res.data ?? []);
        } else {
          setError(res.msg || i18next.t("agent:Failed to write the agent configuration"));
        }
      })
      .catch(err => setError(err.message || String(err)))
      .then(() => setLoading(false));
  }, [open, agent.agentId, agent.path, agent.owner]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] gap-4 sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{i18next.t("agent:Configuration preview")}</DialogTitle>
          <DialogDescription>{i18next.t("agent:Preview hint")}</DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-4 overflow-y-auto">
          {loading ? <Loading /> : null}
          {error ? <MessageAlert description={error} /> : null}
          {files.map(file => (
            <div key={file.path} className="space-y-1">
              <div className="flex items-center justify-between gap-2">
                <code className="min-w-0 truncate text-xs">{file.path}</code>
                <Badge variant="muted">{file.format}</Badge>
              </div>
              <pre className="overflow-x-auto rounded-md bg-muted p-2 text-xs">{file.preview}</pre>
            </div>
          ))}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {i18next.t("general:Close")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function RestoreProviderButton({agent, busy, clearBinding, onRestore}: {
  agent: Agent;
  busy: boolean;
  clearBinding?: boolean;
  onRestore: () => void;
}) {
  return (
    <ConfirmDialog
      title={i18next.t("agent:Restore the configuration of {agent}?").replace("{agent}", agent.name)}
      description={i18next.t(clearBinding ? "agent:Clear binding and restore hint" : "agent:Restore confirm hint")}
      confirmText={i18next.t("agent:Restore configuration")}
      onConfirm={onRestore}
    >
      <Button variant="outline" disabled={busy}>
        <RotateCcw />
        {i18next.t("agent:Restore configuration")}
      </Button>
    </ConfirmDialog>
  );
}

/**
 * Which upstream an agent's requests go to, which upstreams take over when it
 * cannot answer, and how that choice reaches the agent: through the local proxy,
 * or written into the agent's own configuration file.
 */
export function ProviderCard({
  agent,
  providers,
  health,
  busy,
  onRouting,
  onWrite,
}: {
  agent: Agent;
  providers: Provider[];
  health: ProviderHealth[];
  busy: boolean;
  onRouting: (routing: Partial<AgentBackend.AgentRouting>) => void;
  onWrite: (restore: boolean) => void;
}) {
  const [preview, setPreview] = React.useState(false);
  const [recoverViaGateway, setRecoverViaGateway] = React.useState(false);

  const bound = providers.find(provider => providerIdOf(provider) === agent.provider);
  const mode = agent.mode || gatewayMode;
  const missing = agent.provider !== "" && bound === undefined;
  const replacementMode = recoverViaGateway ? gatewayMode : mode;
  const fallbacks = agent.fallbacks ?? [];
  // An older backend does not report the provider state at all, and the card
  // still has to render for the parts that do not depend on it.
  const providerConfig = agent.providerConfig ?? {
    supported: false,
    protocol: "",
    applied: false,
    provider: "",
    mode: "",
    baseUrl: "",
    time: "",
    files: [],
    detail: "",
    builtin: "",
    current: "",
    envConflicts: [],
  };
  const noteKey = agentSetupNoteKey(agent.agentId);
  const healthOf = (id: string) => health.find(item => item.provider === id);
  const boundHealth = healthOf(agent.provider);

  // The agent's client speaks one wire format. Through the gateway that does not
  // narrow the choice, since the proxy translates between the APIs; a direct
  // binding writes the provider's own URL into the agent config, and there the
  // two have to speak the same one. A backend that does not report the format
  // yet filters nothing out.
  const protocol = providerConfig.protocol ?? "";
  const speaksAgentApi = (provider: Provider, selectedMode = mode) => {
    const api = providerWireApi(provider);
    return selectedMode !== directMode || protocol === "" || (api === "responses" ? "openai" : api) === protocol;
  };
  // A provider bound before the mode was switched stays in the list, so the
  // select shows what the agent is on rather than an empty box.
  const options = missing
    ? providers.filter(provider =>
      provider.status === "enabled" && speaksAgentApi(provider, replacementMode) &&
      (replacementMode !== directMode || agentGatewayOnlyKey(agent, provider) === ""),
    )
    : providers.filter(provider => speaksAgentApi(provider) || providerIdOf(provider) === agent.provider);
  const mismatched = bound !== undefined && !speaksAgentApi(bound);

  const candidates = providers.filter(
    provider => providerIdOf(provider) !== agent.provider && speaksAgentApi(provider),
  );

  // An agent whose API no provider here serves is out of reach directly: Codex
  // reads nothing but the Responses API, the Gemini CLI nothing but Google's.
  // Gateway serves both by translating, so the choice is not offered rather
  // than failing on the write.
  const remembered = providers.find(provider => providerIdOf(provider) === agent.directProvider);
  const canReturn = remembered?.status === "enabled" && speaksAgentApi(remembered, directMode) &&
    !agentGatewayOnlyKey(agent, remembered);
  const directTarget = canReturn ? remembered : bound;
  const gatewayOnlyKey = agentGatewayOnlyKey(agent, directTarget);
  const canGoDirect = directTarget?.status === "enabled" && speaksAgentApi(directTarget, directMode) && !gatewayOnlyKey;
  const returnHint = canGoDirect && directTarget
    ? i18next.t("agent:Return to direct provider").replace("{provider}", directTarget.displayName || directTarget.name)
    : i18next.t(gatewayOnlyKey || "agent:Choose a direct-compatible provider");
  const goDirect = () => {
    if (canGoDirect && directTarget) {
      onRouting({mode: directMode, provider: providerIdOf(directTarget)});
    }
  };

  const pointsAtGateway = (providerConfig.current ?? "").includes("/v1/agents/");
  const canRetryRestore = agent.provider === "" &&
    (pointsAtGateway || providerConfig.applied || Boolean(providerConfig.provider));
  const hasEnabledFallback = mode === gatewayMode && providers.some(provider =>
    provider.status === "enabled" && fallbacks.includes(providerIdOf(provider)),
  );

  const chooseProvider = (value: string) => {
    if (value === builtinProvider) {
      onRouting({provider: "", fallbacks: []});
    } else if (missing) {
      const available = new Set(providers.filter(provider => speaksAgentApi(provider, replacementMode)).map(providerIdOf));
      onRouting({
        provider: value,
        mode: replacementMode,
        fallbacks: fallbacks.filter(id => id !== value && available.has(id)),
      });
    } else {
      onRouting({provider: value, fallbacks: []});
    }
  };

  const strategy = agent.strategy || "order";

  return (
    <Card>
      <CardHeader className="p-4 pb-2">
        <CardTitle className="text-base">{i18next.t("agent:Provider")}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3 p-4 pt-0">
        <Select
          value={missing ? "" : agent.provider || builtinProvider}
          disabled={busy}
          onValueChange={chooseProvider}
        >
          <SelectTrigger aria-label={i18next.t("agent:Provider")}>
            <SelectValue placeholder={i18next.t("agent:Choose a replacement provider")} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={builtinProvider}>
              <AgentIcon agent={agent.agentId || agent.name} size={16} fallback={<Bot className="size-4" />} />
              {agentBuiltin(agent)}
              <span className="ml-2 text-xs text-muted-foreground">
                {i18next.t("agent:Built-in")}
              </span>
            </SelectItem>
            {options.map(provider => (
              <SelectItem key={providerIdOf(provider)} value={providerIdOf(provider)}>
                <ProviderIcon icon={provider.icon} baseUrl={provider.baseUrl} size={16} />
                {provider.displayName || provider.name}
                {/* The type is the wire format the provider serves. */}
                <span className="ml-2 text-xs text-muted-foreground">{provider.type}</span>
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <EnvWarningBanner
          conflicts={providerConfig.envConflicts}
          applied={providerConfig.applied}
        />

        {mismatched ? (
          <p className="text-sm text-warning">
            {i18next.t("agent:Provider speaks another API").replace("{protocol}", protocol)}
          </p>
        ) : null}

        {boundHealth && !boundHealth.healthy ? (
          <p className="text-sm text-warning">
            {`${i18next.t("agent:Bound provider cooling down")}: ${boundHealth.lastError}`}
          </p>
        ) : null}

        {canRetryRestore ? (
          <div className="space-y-2 rounded-md border p-3">
            <p className="text-sm text-warning">
              {i18next.t(pointsAtGateway ? "agent:Still calling Gateway" : "agent:Retry restoring provider settings")}
            </p>
            {providerConfig.current ? <code className="block truncate text-xs">{providerConfig.current}</code> : null}
            {!providerConfig.applied && providerConfig.detail ? (
              <p className="text-sm text-muted-foreground">{providerConfig.detail}</p>
            ) : null}
            <RestoreProviderButton agent={agent} busy={busy} onRestore={() => onWrite(true)} />
          </div>
        ) : agent.provider === "" ? (
          <p className="text-sm text-muted-foreground">{i18next.t("agent:Built-in hint")}</p>
        ) : bound === undefined ? (
          <MessageAlert
            variant="warning"
            title={i18next.t("agent:Missing provider {provider}").replace("{provider}", agent.provider)}
            description={i18next.t(hasEnabledFallback ? "agent:Missing provider with fallbacks" : "agent:Missing provider recovery hint")}
            action={
              <div className="space-y-3">
                {mode === directMode ? (
                  <label className="flex items-center gap-2 text-sm">
                    <Switch checked={recoverViaGateway} onCheckedChange={setRecoverViaGateway} disabled={busy} />
                    {i18next.t("agent:Use Gateway for replacement")}
                  </label>
                ) : null}
                {mode === gatewayMode && canReturn ? (
                  <Button variant="outline" disabled={busy} onClick={goDirect}>{returnHint}</Button>
                ) : null}
                <RestoreProviderButton
                  agent={agent}
                  busy={busy}
                  clearBinding
                  onRestore={() => chooseProvider(builtinProvider)}
                />
              </div>
            }
          />
        ) : (
          <>
            <FallbackQueue
              fallbacks={fallbacks}
              providers={providers}
              candidates={candidates}
              health={health}
              throughGateway={mode === gatewayMode}
              busy={busy}
              onChange={next => onRouting({fallbacks: next})}
            />

            {/* With fallbacks the providers are a pool, and which of them a
                request starts on is a choice of its own. */}
            {fallbacks.length > 0 ? (
              <div className="space-y-2">
                <p className="text-sm font-medium">{i18next.t("agent:Pool strategy")}</p>
                <SimpleSelect
                  className="max-w-xs"
                  value={strategy}
                  disabled={busy}
                  onChange={value => onRouting({strategy: value})}
                  options={[
                    {label: i18next.t("agent:Pool order"), value: "order"},
                    {label: i18next.t("agent:Pool smart"), value: "smart"},
                    {label: i18next.t("agent:Pool rotate"), value: "rotate"},
                    {label: i18next.t("agent:Pool usage"), value: "usage"},
                  ]}
                />
                <p className="text-sm text-muted-foreground">{i18next.t(`agent:Pool ${strategy} hint`)}</p>
                <p className="text-sm text-muted-foreground">{i18next.t("agent:Pool sticky hint")}</p>
              </div>
            ) : null}

            <label className="flex items-start gap-2 text-sm">
              <Switch
                className="mt-0.5"
                checked={mode === gatewayMode}
                disabled={busy || (mode === gatewayMode && !canGoDirect)}
                onCheckedChange={checked => checked ? onRouting({mode: gatewayMode}) : goDirect()}
              />
              <span>
                {i18next.t("agent:Route through Gateway")}
                <span className="block text-muted-foreground">
                  {mode === gatewayMode ? returnHint : i18next.t("agent:Direct mode hint")}
                </span>
              </span>
            </label>

            {providerConfig.supported ? (
              <div className="space-y-2 rounded-md border p-3">
                <div className="flex flex-wrap items-center gap-2">
                  <FileCog className="h-4 w-4 text-muted-foreground" />
                  <span className="text-sm font-medium">
                    {i18next.t("agent:Agent configuration")}
                  </span>
                  {providerConfig.applied ? (
                    <Badge variant="success">{i18next.t("agent:Written by Gateway")}</Badge>
                  ) : (
                    <Badge variant="secondary">{i18next.t("agent:Not written")}</Badge>
                  )}
                </div>

                {providerConfig.detail ? (
                  <p className="text-sm text-muted-foreground">{providerConfig.detail}</p>
                ) : null}
                {(providerConfig.files ?? []).map(path => (
                  <code key={path} className="block truncate text-xs">
                    {path}
                  </code>
                ))}

                {/* Once the file belongs to Gateway, picking another provider
                    above rewrites it on the spot, so the button stops being the
                    thing to click and only stays for a manual rewrite. */}
                {providerConfig.applied ? (
                  <p className="text-sm text-muted-foreground">{i18next.t("agent:Switch takes effect at once")}</p>
                ) : null}

                <div className="flex flex-wrap gap-2">
                  <ConfirmDialog
                    title={i18next.t("agent:Write the configuration of {agent}?").replace("{agent}", agent.name)}
                    description={i18next.t("agent:Write confirm hint")}
                    confirmText={i18next.t("agent:Write configuration")}
                    onConfirm={() => onWrite(false)}
                  >
                    <Button
                      variant={providerConfig.applied ? "outline" : "default"}
                      disabled={busy || mismatched}
                    >
                      {providerConfig.applied
                        ? i18next.t("agent:Rewrite configuration")
                        : i18next.t("agent:Write configuration")}
                    </Button>
                  </ConfirmDialog>

                  <Button variant="outline" disabled={busy || mismatched} onClick={() => setPreview(true)}>
                    {i18next.t("agent:Preview")}
                  </Button>

                  {providerConfig.applied || providerConfig.provider !== "" ? (
                    <RestoreProviderButton agent={agent} busy={busy} onRestore={() => onWrite(true)} />
                  ) : null}
                </div>
              </div>
            ) : (
              <>
                <p className="text-sm text-muted-foreground">{i18next.t("agent:Base URL hint")}</p>
                <EnvSnippet
                  protocol={providerProtocol(bound.type)}
                  baseUrl={agent.proxyBaseUrl || agentProxyBaseUrl(agent.agentId)}
                  defaultShell={shellForPath(agent.path)}
                  includeToken={!usesClientAuth(bound)}
                />
                <p className="text-sm text-muted-foreground">
                  {usesClientAuth(bound) ? i18next.t("agent:Client auth token hint") : i18next.t("agent:Token hint")}
                </p>
                {noteKey === "" ? null : (
                  <p className="text-sm text-muted-foreground">{i18next.t(noteKey)}</p>
                )}
              </>
            )}
          </>
        )}

        {bound?.status === "enabled" ? (
          <ProviderStreamCheck key={`${agent.agentId}/${agent.provider}/${mode}`} agentId={agent.agentId} provider={bound} disabled={busy} />
        ) : null}

        <PreviewDialog agent={agent} open={preview} onOpenChange={setPreview} />
      </CardContent>
    </Card>
  );
}
