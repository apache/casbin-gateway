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
import {ArrowDown, ArrowUp, Plus, Trash2} from "lucide-react";
import i18next from "i18next";

import {Field, FormDialog} from "@/components/shared/form-dialog";
import {SearchSelect, type SelectOption} from "@/components/shared/simple-select";
import {ProviderIcon} from "@/components/ProviderIcon";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import {Label} from "@/components/ui/label";
import {Switch} from "@/components/ui/switch";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {providerIdOf} from "@/lib/providers";
import type {Agent, ModelRoute, Provider, RouteTarget} from "@/types";

const maxTargets = 8;

function emptyRoute(): ModelRoute {
  return {
    name: "",
    createdTime: "",
    updatedTime: "",
    displayName: "",
    match: "",
    agent: "",
    targets: [{provider: "", model: ""}],
    sort: 0,
    enabled: true,
  };
}

/** A rule with at least one rung to edit, however it arrived. */
function draftOf(route: ModelRoute | null): ModelRoute {
  if (route === null) {
    return emptyRoute();
  }
  return {...route, targets: route.targets.length > 0 ? [...route.targets] : [{provider: "", model: ""}]};
}

/** One rung of the ladder: which provider answers it, asked for which model. */
function TargetRow({
  target,
  index,
  count,
  providers,
  onChange,
  onMove,
  onRemove,
}: {
  target: RouteTarget;
  index: number;
  count: number;
  providers: SelectOption[];
  onChange: (target: RouteTarget) => void;
  onMove: (offset: number) => void;
  onRemove: () => void;
}) {
  return (
    <div className="flex items-end gap-2">
      <span className="text-muted-foreground w-16 shrink-0 pb-2.5 text-xs">
        {index === 0
          ? i18next.t("llm:First choice")
          : i18next.t("llm:Step {n}").replace("{n}", String(index + 1))}
      </span>
      <div className="grid min-w-0 flex-1 gap-2 sm:grid-cols-2">
        <Input
          className="font-mono"
          placeholder={i18next.t("llm:Keep the requested model")}
          value={target.model}
          onChange={event => onChange({...target, model: event.target.value})}
        />
        <SearchSelect
          allowClear
          value={target.provider}
          onChange={provider => onChange({...target, provider: provider})}
          options={providers}
          placeholder={i18next.t("llm:Any provider serving it")}
        />
      </div>
      <div className="flex shrink-0 items-center">
        <SimpleTooltip title={i18next.t("llm:Move up")}>
          <Button type="button" size="icon" variant="ghost" disabled={index === 0} onClick={() => onMove(-1)}>
            <ArrowUp />
          </Button>
        </SimpleTooltip>
        <SimpleTooltip title={i18next.t("llm:Move down")}>
          <Button
            type="button"
            size="icon"
            variant="ghost"
            disabled={index === count - 1}
            onClick={() => onMove(1)}
          >
            <ArrowDown />
          </Button>
        </SimpleTooltip>
        <SimpleTooltip title={i18next.t("general:Delete")}>
          <Button type="button" size="icon" variant="ghost" disabled={count === 1} onClick={onRemove}>
            <Trash2 />
          </Button>
        </SimpleTooltip>
      </div>
    </div>
  );
}

/**
 * Edits one routing rule. The pattern says which requests it covers and the
 * ladder says where they go, top rung first: everything below it is what the
 * request steps down to when the rung above is rate-limited, out of quota or
 * down.
 */
export function RouteEditDialog({
  open,
  onOpenChange,
  route,
  providers,
  agents,
  onSave,
  saving,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Null adds a rule rather than editing one. */
  route: ModelRoute | null;
  providers: Provider[];
  agents: Agent[];
  onSave: (route: ModelRoute) => void;
  saving?: boolean;
}) {
  const [draft, setDraft] = React.useState<ModelRoute>(() => draftOf(route));

  // The dialog is kept mounted between rows, so the draft is reset whenever it
  // is opened on a different rule rather than on first render only.
  React.useEffect(() => {
    if (open) {
      setDraft(draftOf(route));
    }
  }, [open, route]);

  const adding = route === null;

  const providerOptions: SelectOption[] = providers.map(provider => ({
    value: providerIdOf(provider),
    text: `${provider.displayName} ${provider.name}`,
    label: (
      <span className="flex min-w-0 items-center gap-2">
        <ProviderIcon icon={provider.icon} baseUrl={provider.baseUrl} alt={provider.name} size={16} />
        <span className="truncate">{provider.displayName || provider.name}</span>
      </span>
    ),
  }));

  const agentOptions: SelectOption[] = Array.from(new Set(agents.map(agent => agent.agentId || agent.name)))
    .filter(id => id !== "")
    .map(id => ({value: id, label: id}));

  const setTargets = (targets: RouteTarget[]) => setDraft(current => ({...current, targets: targets}));

  const changeTarget = (index: number, target: RouteTarget) =>
    setTargets(draft.targets.map((current, at) => (at === index ? target : current)));

  const moveTarget = (index: number, offset: number) => {
    const targets = [...draft.targets];
    const [moved] = targets.splice(index, 1);
    targets.splice(index + offset, 0, moved);
    setTargets(targets);
  };

  const named = draft.targets.filter(target => target.provider !== "" || target.model !== "");

  return (
    <FormDialog
      open={open}
      onOpenChange={onOpenChange}
      size="lg"
      title={adding ? i18next.t("llm:Add a routing rule") : i18next.t("llm:Edit routing rule")}
      description={i18next.t("llm:Routing rule dialog description")}
      onSubmit={() => onSave({...draft, targets: named})}
      submitting={saving}
      submitDisabled={draft.name.trim() === "" || draft.match.trim() === "" || named.length === 0}
    >
      <div className="flex flex-col gap-4">
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={i18next.t("general:Name")} htmlFor="route-name" required hint={i18next.t("llm:Rule name hint")}>
            <Input
              id="route-name"
              // The name is the row's key, so an existing one is not editable:
              // changing it would leave the old rule in place beside the new.
              disabled={!adding}
              className="font-mono"
              placeholder="haiku-to-cheap"
              value={draft.name}
              onChange={event => setDraft(current => ({...current, name: event.target.value}))}
            />
          </Field>
          <Field label={i18next.t("general:Display name")} htmlFor="route-display-name">
            <Input
              id="route-display-name"
              placeholder={i18next.t("llm:Background work on the cheap model")}
              value={draft.displayName}
              onChange={event => setDraft(current => ({...current, displayName: event.target.value}))}
            />
          </Field>
        </div>

        <div className="grid gap-4 sm:grid-cols-2">
          <Field
            label={i18next.t("llm:When the client asks for")}
            htmlFor="route-match"
            required
            hint={i18next.t("llm:Model pattern hint")}
          >
            <Input
              id="route-match"
              className="font-mono"
              placeholder="*haiku*"
              value={draft.match}
              onChange={event => setDraft(current => ({...current, match: event.target.value}))}
            />
          </Field>
          <Field label={i18next.t("agent:Agent")} htmlFor="route-agent" hint={i18next.t("llm:Rule agent hint")}>
            <SearchSelect
              id="route-agent"
              allowClear
              allowCustomValue
              value={draft.agent}
              onChange={agent => setDraft(current => ({...current, agent: agent}))}
              options={agentOptions}
              placeholder={i18next.t("llm:Every agent")}
            />
          </Field>
        </div>

        <div className="flex flex-col gap-2">
          <Label>{i18next.t("llm:Send it to")}</Label>
          <p className="text-muted-foreground text-xs">{i18next.t("llm:Ladder hint")}</p>
          <div className="flex flex-col gap-2">
            {draft.targets.map((target, index) => (
              <TargetRow
                key={index}
                target={target}
                index={index}
                count={draft.targets.length}
                providers={providerOptions}
                onChange={next => changeTarget(index, next)}
                onMove={offset => moveTarget(index, offset)}
                onRemove={() => setTargets(draft.targets.filter((_, at) => at !== index))}
              />
            ))}
          </div>
          <div>
            <Button
              type="button"
              size="sm"
              variant="outline"
              disabled={draft.targets.length >= maxTargets}
              onClick={() => setTargets([...draft.targets, {provider: "", model: ""}])}
            >
              <Plus />
              {i18next.t("llm:Add a fallback step")}
            </Button>
          </div>
        </div>

        <div className="flex items-center justify-between gap-4 rounded-md border px-3 py-2">
          <div className="flex flex-col">
            <Label htmlFor="route-enabled">{i18next.t("provider:Enabled")}</Label>
            <span className="text-muted-foreground text-xs">{i18next.t("llm:Rule enabled hint")}</span>
          </div>
          <Switch
            id="route-enabled"
            checked={draft.enabled}
            onCheckedChange={enabled => setDraft(current => ({...current, enabled: enabled}))}
          />
        </div>
      </div>
    </FormDialog>
  );
}
