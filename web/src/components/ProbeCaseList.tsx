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
import {ChevronDown, FlaskConical, Pencil, Plus, RotateCcw, Trash2} from "lucide-react";
import i18next from "i18next";

import * as ProbeCaseBackend from "@/backend/ProbeCaseBackend";
import * as Setting from "@/Setting";
import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {EmptyState} from "@/components/shared/empty-state";
import {Field, FormDialog} from "@/components/shared/form-dialog";
import {Loading} from "@/components/shared/loading";
import {NumberInput} from "@/components/shared/number-input";
import {SimpleSelect} from "@/components/shared/simple-select";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Card, CardContent} from "@/components/ui/card";
import {Input} from "@/components/ui/input";
import {Switch} from "@/components/ui/switch";
import {TagsInput} from "@/components/ui/tags-input";
import {Textarea} from "@/components/ui/textarea";
import {cn} from "@/lib/utils";
import {caseMethod, caseQuestion, caseTitle} from "@/lib/authenticity";
import type {ProbeCase, ProbeKey} from "@/types";

const checkKeys: ProbeKey[] = [
  "identity",
  "selfid",
  "hidden",
  "knowledge",
  "feature",
  "repeat",
  "tools",
  "stream",
  "cache",
  "billing",
  "vendor",
];

/** The engines that send a question and judge what was written back. */
const askingChecks: ProbeKey[] = ["knowledge", "selfid", "hidden", "feature", "repeat"];

function asks(check: ProbeKey) {
  return askingChecks.includes(check);
}

function checkLabel(key: ProbeKey) {
  switch (key) {
  case "identity":
    return i18next.t("audit:Model identity");
  case "cache":
    return i18next.t("audit:Prompt cache");
  case "billing":
    return i18next.t("audit:Token billing");
  case "stream":
    return i18next.t("audit:Stream shape");
  case "tools":
    return i18next.t("audit:Tool schema");
  case "knowledge":
    return i18next.t("audit:Test bank");
  case "selfid":
    return i18next.t("audit:Self-reported maker");
  case "hidden":
    return i18next.t("audit:Hidden instructions");
  case "feature":
    return i18next.t("audit:Documented parameter");
  case "repeat":
    return i18next.t("audit:Repeated request");
  default:
    return i18next.t("audit:Vendor headers");
  }
}

function matchLabel(match: ProbeCase["params"]["match"]) {
  if (match === "exact") {
    return i18next.t("audit:Match exact");
  }
  if (match === "regex") {
    return i18next.t("audit:Match regex");
  }
  return i18next.t("audit:Match contains");
}

function protocolLabel(protocol: ProbeCase["protocol"]) {
  if (protocol === "anthropic") {
    return "Anthropic";
  }
  if (protocol === "openai") {
    return "OpenAI";
  }
  return i18next.t("audit:Both APIs");
}

/**
 * A case as this page needs it. A list a case never filled comes back as null
 * from an older build, and every list here is read as one.
 */
function withLists(probeCase: ProbeCase): ProbeCase {
  const params = probeCase.params;
  return {
    ...probeCase,
    params: {
      ...params,
      events: params.events ?? [],
      headers: params.headers ?? [],
      expect: params.expect ?? [],
      forbid: params.forbid ?? [],
      require: params.require ?? [],
      vendors: params.vendors ?? [],
    },
  };
}

/** A case someone starts from: nothing asked yet, and nothing weighted yet. */
function emptyCase(): ProbeCase {
  return {
    name: "",
    createdTime: "",
    updatedTime: "",
    displayName: "",
    check: "identity",
    protocol: "",
    enabled: true,
    weight: 10,
    sort: 100,
    builtIn: false,
    question: "",
    method: "",
    edited: false,
    params: {
      system: "",
      prompt: "",
      maxTokens: 0,
      toolName: "",
      schema: "",
      events: [],
      fillerChars: 0,
      gapMs: 0,
      headers: [],
      minHeaders: 0,
      driftTolerance: 0,
      warnHigh: 0,
      alertHigh: 0,
      warnLow: 0,
      alertLow: 0,
      expect: [],
      forbid: [],
      match: "",
      extra: "",
      require: [],
      samples: 0,
      vendors: [],
    },
  };
}

/** One labelled line of "what is actually sent", for the request the case makes. */
function ParamRow({label, value}: {label: string; value: React.ReactNode}) {
  return (
    <div className="grid grid-cols-[9rem_1fr] gap-2 py-1">
      <span className="text-muted-foreground text-xs">{label}</span>
      <span className="text-xs break-words whitespace-pre-wrap">{value}</span>
    </div>
  );
}

/**
 * What this case sends and what it compares against, in the values that are
 * actually used. A case that leaves a field empty is running on the shipped
 * default, and the row says so rather than showing a blank.
 */
function CaseParams({probeCase}: {probeCase: ProbeCase}) {
  const fallback = i18next.t("audit:Built-in default");
  const params = probeCase.params;
  const rows: {label: string; value: React.ReactNode}[] = [];

  if (probeCase.check === "tools") {
    rows.push({label: i18next.t("audit:System prompt"), value: params.system || fallback});
    rows.push({label: i18next.t("audit:User prompt"), value: params.prompt || fallback});
    rows.push({label: i18next.t("audit:Tool name"), value: params.toolName || fallback});
    rows.push({
      label: i18next.t("audit:Tool schema"),
      value: params.schema ? (
        <code className="bg-muted block max-h-56 overflow-auto rounded p-2 font-mono text-[11px]">
          {params.schema}
        </code>
      ) : (
        fallback
      ),
    });
  }
  if (probeCase.check === "stream") {
    rows.push({label: i18next.t("audit:User prompt"), value: params.prompt || fallback});
    rows.push({
      label: i18next.t("audit:Expected events"),
      value: params.events.length > 0 ? params.events.join(", ") : fallback,
    });
  }
  if (probeCase.check === "cache") {
    rows.push({label: i18next.t("audit:User prompt"), value: params.prompt || fallback});
    rows.push({
      label: i18next.t("audit:Cached prefix"),
      value: params.fillerChars > 0 ? `${params.fillerChars} chars` : fallback,
    });
    rows.push({
      label: i18next.t("audit:Second request after"),
      value: params.gapMs > 0 ? `${params.gapMs} ms` : fallback,
    });
  }
  if (probeCase.check === "billing") {
    rows.push({
      label: i18next.t("audit:Allowed drift"),
      value: params.driftTolerance > 0 ? `${(params.driftTolerance * 100).toFixed(1)}%` : fallback,
    });
    rows.push({
      label: i18next.t("audit:Warn outside"),
      value:
        params.warnLow > 0 && params.warnHigh > 0 ? `${params.warnLow}x – ${params.warnHigh}x` : fallback,
    });
    rows.push({
      label: i18next.t("audit:Alert outside"),
      value:
        params.alertLow > 0 && params.alertHigh > 0
          ? `${params.alertLow}x – ${params.alertHigh}x`
          : fallback,
    });
  }
  if (probeCase.check === "vendor") {
    rows.push({
      label: i18next.t("audit:Headers looked for"),
      value: params.headers.length > 0 ? params.headers.join(", ") : fallback,
    });
    rows.push({
      label: i18next.t("audit:Headers required"),
      value: params.minHeaders > 0 ? String(params.minHeaders) : fallback,
    });
  }
  if (probeCase.check === "identity") {
    rows.push({label: i18next.t("audit:Reads"), value: i18next.t("audit:Identity reads")});
  }
  if (asks(probeCase.check)) {
    rows.push({label: i18next.t("audit:User prompt"), value: params.prompt || fallback});
    if (params.system) {
      rows.push({label: i18next.t("audit:System prompt"), value: params.system});
    }
    if (params.expect.length > 0) {
      rows.push({label: i18next.t("audit:Accepted answers"), value: params.expect.join(" / ")});
    }
    if (params.forbid.length > 0) {
      rows.push({label: i18next.t("audit:Rejected answers"), value: params.forbid.join(" / ")});
    }
    if (params.expect.length > 0 || params.forbid.length > 0) {
      rows.push({label: i18next.t("audit:Compared as"), value: matchLabel(params.match)});
    }
    if (params.extra) {
      rows.push({
        label: i18next.t("audit:Extra request fields"),
        value: (
          <code className="bg-muted block max-h-40 overflow-auto rounded p-2 font-mono text-[11px]">
            {params.extra}
          </code>
        ),
      });
    }
    if (params.require.length > 0) {
      rows.push({label: i18next.t("audit:Required answer fields"), value: params.require.join(", ")});
    }
    if (probeCase.check === "repeat") {
      rows.push({
        label: i18next.t("audit:Requests sent"),
        value: params.samples > 0 ? String(params.samples) : fallback,
      });
    }
  }
  if (params.vendors.length > 0) {
    rows.push({label: i18next.t("audit:Only these vendors"), value: params.vendors.join(", ")});
  }
  if (probeCase.check !== "identity" && probeCase.check !== "vendor" && probeCase.check !== "billing") {
    rows.push({
      label: i18next.t("audit:Answer length"),
      value: params.maxTokens > 0 ? `${params.maxTokens} tokens` : fallback,
    });
  }

  return <div className="divide-border/60 divide-y">{rows.map(row => <ParamRow key={row.label} {...row} />)}</div>;
}

function CaseCard({
  probeCase,
  share,
  onEdit,
  onDelete,
  onToggle,
  busy,
}: {
  probeCase: ProbeCase;
  /** What this case is worth of everything weighted, as a percentage. */
  share: number;
  onEdit: () => void;
  onDelete: () => void;
  onToggle: (enabled: boolean) => void;
  busy: boolean;
}) {
  const [open, setOpen] = React.useState(false);

  return (
    <Card className={cn(!probeCase.enabled && "opacity-60")}>
      <CardContent className="flex flex-col gap-3 p-4">
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-sm font-medium">{caseTitle(probeCase)}</span>
          <Badge variant="muted">{checkLabel(probeCase.check)}</Badge>
          <Badge variant="outline">{protocolLabel(probeCase.protocol)}</Badge>
          {probeCase.builtIn ? (
            <Badge variant="info">{i18next.t("audit:Built-in case")}</Badge>
          ) : (
            <Badge variant="secondary">{i18next.t("audit:Custom case")}</Badge>
          )}
          <Badge variant={probeCase.weight > 0 ? "success" : "muted"}>
            {`${i18next.t("audit:Weight")} ${probeCase.weight}${
              probeCase.enabled && share > 0 ? ` · ${Math.round(share)}%` : ""
            }`}
          </Badge>

          <div className="ml-auto flex items-center gap-2">
            <Switch
              checked={probeCase.enabled}
              disabled={busy}
              onCheckedChange={onToggle}
              aria-label={i18next.t("audit:Run this case")}
            />
            <Button size="xs" variant="outline" onClick={onEdit}>
              <Pencil />
              {i18next.t("general:Edit")}
            </Button>
            <ConfirmDialog
              title={i18next.t("audit:Delete this test case?")}
              description={
                probeCase.builtIn
                  ? i18next.t("audit:Delete built-in case detail")
                  : i18next.t("audit:Delete custom case detail")
              }
              onConfirm={onDelete}
            >
              <Button size="xs" variant="outline">
                <Trash2 />
                {i18next.t("general:Delete")}
              </Button>
            </ConfirmDialog>
          </div>
        </div>

        <div className="flex flex-col gap-1">
          <p className="text-sm">{caseQuestion(probeCase)}</p>
          <p className="text-muted-foreground text-xs leading-relaxed">{caseMethod(probeCase)}</p>
        </div>

        <div>
          <Button size="xs" variant="ghost" onClick={() => setOpen(current => !current)}>
            <ChevronDown className={cn("transition-transform", open && "rotate-180")} />
            {i18next.t("audit:What this sends")}
          </Button>
          {open ? (
            <div className="mt-2 rounded-lg border p-3">
              <CaseParams probeCase={probeCase} />
              <p className="text-muted-foreground mt-2 font-mono text-[11px]">{probeCase.name}</p>
            </div>
          ) : null}
        </div>
      </CardContent>
    </Card>
  );
}

/** The dialog that writes a case, with only the fields its engine reads. */
function CaseDialog({
  open,
  draft,
  creating,
  submitting,
  onChange,
  onOpenChange,
  onSubmit,
}: {
  open: boolean;
  draft: ProbeCase;
  creating: boolean;
  submitting: boolean;
  onChange: (next: ProbeCase) => void;
  onOpenChange: (open: boolean) => void;
  onSubmit: () => void;
}) {
  const set = (patch: Partial<ProbeCase>) => onChange({...draft, ...patch});
  const setParam = (patch: Partial<ProbeCase["params"]>) =>
    onChange({...draft, params: {...draft.params, ...patch}});

  return (
    <FormDialog
      open={open}
      onOpenChange={onOpenChange}
      size="lg"
      title={creating ? i18next.t("audit:New test case") : i18next.t("audit:Edit test case")}
      description={i18next.t("audit:Case dialog detail")}
      submitting={submitting}
      submitDisabled={creating && draft.name.trim() === ""}
      onSubmit={onSubmit}
    >
      {creating ? (
        <Field label={i18next.t("general:Name")} required hint={i18next.t("audit:Case name hint")}>
          <Input
            value={draft.name}
            onChange={event => set({name: event.target.value})}
            placeholder="identity-second-model"
          />
        </Field>
      ) : null}

      <Field label={i18next.t("general:Display name")}>
        <Input value={draft.displayName} onChange={event => set({displayName: event.target.value})} />
      </Field>

      <div className="grid gap-4 sm:grid-cols-2">
        <Field label={i18next.t("audit:Check")} hint={i18next.t("audit:Check hint")}>
          <SimpleSelect
            value={draft.check}
            onChange={value => set({check: value as ProbeKey})}
            options={checkKeys.map(key => ({label: checkLabel(key), value: key}))}
          />
        </Field>
        <Field label={i18next.t("audit:Upstream API")}>
          <SimpleSelect
            value={draft.protocol}
            onChange={value => set({protocol: value as ProbeCase["protocol"]})}
            options={[
              {label: i18next.t("audit:Both APIs"), value: ""},
              {label: "Anthropic", value: "anthropic"},
              {label: "OpenAI", value: "openai"},
            ]}
          />
        </Field>
        <Field label={i18next.t("audit:Only these vendors")} hint={i18next.t("audit:Vendors hint")}>
          <TagsInput value={draft.params.vendors} onChange={value => setParam({vendors: value})} />
        </Field>
        <Field label={i18next.t("audit:Weight")} hint={i18next.t("audit:Weight hint")}>
          <NumberInput min={0} max={1000} value={draft.weight} onChange={value => set({weight: value})} />
        </Field>
        <Field label={i18next.t("audit:Order")} hint={i18next.t("audit:Order hint")}>
          <NumberInput min={0} max={100000} value={draft.sort} onChange={value => set({sort: value})} />
        </Field>
      </div>

      <Field label={i18next.t("audit:Run this case")}>
        <Switch checked={draft.enabled} onCheckedChange={value => set({enabled: value})} />
      </Field>

      <Field label={i18next.t("audit:Question")} hint={i18next.t("audit:Question hint")}>
        <Textarea rows={2} value={draft.question} onChange={event => set({question: event.target.value})} />
      </Field>
      <Field label={i18next.t("audit:Method")} hint={i18next.t("audit:Method hint")}>
        <Textarea rows={4} value={draft.method} onChange={event => set({method: event.target.value})} />
      </Field>

      {draft.check === "identity" ? (
        <p className="text-muted-foreground text-xs">{i18next.t("audit:Identity has no request")}</p>
      ) : null}

      {asks(draft.check) ? (
        <>
          <Field label={i18next.t("audit:User prompt")} hint={i18next.t("audit:Question prompt hint")}>
            <Textarea rows={3} value={draft.params.prompt} onChange={event => setParam({prompt: event.target.value})} />
          </Field>
          <Field label={i18next.t("audit:System prompt")}>
            <Textarea rows={2} value={draft.params.system} onChange={event => setParam({system: event.target.value})} />
          </Field>
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label={i18next.t("audit:Accepted answers")} hint={i18next.t("audit:Accepted answers hint")}>
              <TagsInput value={draft.params.expect} onChange={value => setParam({expect: value})} />
            </Field>
            <Field label={i18next.t("audit:Rejected answers")} hint={i18next.t("audit:Rejected answers hint")}>
              <TagsInput value={draft.params.forbid} onChange={value => setParam({forbid: value})} />
            </Field>
          </div>
          <Field label={i18next.t("audit:Compared as")} hint={i18next.t("audit:Compared as hint")}>
            <SimpleSelect
              value={draft.params.match || "contains"}
              onChange={value => setParam({match: value as ProbeCase["params"]["match"]})}
              options={[
                {label: i18next.t("audit:Match contains"), value: "contains"},
                {label: i18next.t("audit:Match exact"), value: "exact"},
                {label: i18next.t("audit:Match regex"), value: "regex"},
              ]}
            />
          </Field>
          <Field label={i18next.t("audit:Extra request fields")} hint={i18next.t("audit:Extra hint")}>
            <Textarea
              rows={3}
              className="font-mono text-xs"
              value={draft.params.extra}
              onChange={event => setParam({extra: event.target.value})}
              placeholder={"{\"logprobs\": true}"}
            />
          </Field>
          {draft.check === "feature" ? (
            <Field label={i18next.t("audit:Required answer fields")} hint={i18next.t("audit:Required hint")}>
              <TagsInput value={draft.params.require} onChange={value => setParam({require: value})} />
            </Field>
          ) : null}
          {draft.check === "repeat" ? (
            <Field label={i18next.t("audit:Requests sent")} hint={i18next.t("audit:Requests sent hint")}>
              <NumberInput min={0} max={6} value={draft.params.samples} onChange={value => setParam({samples: value})} />
            </Field>
          ) : null}
        </>
      ) : null}

      {draft.check === "tools" ? (
        <>
          <Field label={i18next.t("audit:System prompt")}>
            <Textarea rows={2} value={draft.params.system} onChange={event => setParam({system: event.target.value})} />
          </Field>
          <Field label={i18next.t("audit:User prompt")}>
            <Textarea rows={2} value={draft.params.prompt} onChange={event => setParam({prompt: event.target.value})} />
          </Field>
          <Field label={i18next.t("audit:Tool name")}>
            <Input value={draft.params.toolName} onChange={event => setParam({toolName: event.target.value})} />
          </Field>
          <Field label={i18next.t("audit:Tool schema")} hint={i18next.t("audit:Tool schema hint")}>
            <Textarea
              rows={8}
              className="font-mono text-xs"
              value={draft.params.schema}
              onChange={event => setParam({schema: event.target.value})}
            />
          </Field>
        </>
      ) : null}

      {draft.check === "stream" ? (
        <>
          <Field label={i18next.t("audit:User prompt")}>
            <Textarea rows={2} value={draft.params.prompt} onChange={event => setParam({prompt: event.target.value})} />
          </Field>
          <Field label={i18next.t("audit:Expected events")} hint={i18next.t("audit:Expected events hint")}>
            <TagsInput value={draft.params.events} onChange={value => setParam({events: value})} />
          </Field>
        </>
      ) : null}

      {draft.check === "cache" ? (
        <>
          <Field label={i18next.t("audit:User prompt")}>
            <Textarea rows={2} value={draft.params.prompt} onChange={event => setParam({prompt: event.target.value})} />
          </Field>
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label={i18next.t("audit:Cached prefix")} hint={i18next.t("audit:Cached prefix hint")}>
              <NumberInput
                min={0}
                max={200000}
                value={draft.params.fillerChars}
                onChange={value => setParam({fillerChars: value})}
                addonAfter="chars"
              />
            </Field>
            <Field label={i18next.t("audit:Second request after")}>
              <NumberInput
                min={0}
                max={60000}
                value={draft.params.gapMs}
                onChange={value => setParam({gapMs: value})}
                addonAfter="ms"
              />
            </Field>
          </div>
        </>
      ) : null}

      {draft.check === "billing" ? (
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={i18next.t("audit:Allowed drift")} hint={i18next.t("audit:Allowed drift hint")}>
            <Input
              value={String(draft.params.driftTolerance)}
              onChange={event => setParam({driftTolerance: Number(event.target.value) || 0})}
            />
          </Field>
          <Field label={i18next.t("audit:Warn outside")}>
            <div className="flex gap-2">
              <Input
                value={String(draft.params.warnLow)}
                onChange={event => setParam({warnLow: Number(event.target.value) || 0})}
              />
              <Input
                value={String(draft.params.warnHigh)}
                onChange={event => setParam({warnHigh: Number(event.target.value) || 0})}
              />
            </div>
          </Field>
          <Field label={i18next.t("audit:Alert outside")}>
            <div className="flex gap-2">
              <Input
                value={String(draft.params.alertLow)}
                onChange={event => setParam({alertLow: Number(event.target.value) || 0})}
              />
              <Input
                value={String(draft.params.alertHigh)}
                onChange={event => setParam({alertHigh: Number(event.target.value) || 0})}
              />
            </div>
          </Field>
        </div>
      ) : null}

      {draft.check === "vendor" ? (
        <div className="grid gap-4">
          <Field label={i18next.t("audit:Headers looked for")} hint={i18next.t("audit:Headers hint")}>
            <TagsInput value={draft.params.headers} onChange={value => setParam({headers: value})} />
          </Field>
          <Field label={i18next.t("audit:Headers required")}>
            <NumberInput
              min={0}
              max={20}
              value={draft.params.minHeaders}
              onChange={value => setParam({minHeaders: value})}
            />
          </Field>
        </div>
      ) : null}

      {draft.check !== "identity" && draft.check !== "vendor" && draft.check !== "billing" ? (
        <Field label={i18next.t("audit:Answer length")} hint={i18next.t("audit:Answer length hint")}>
          <NumberInput
            min={0}
            max={4096}
            value={draft.params.maxTokens}
            onChange={value => setParam({maxTokens: value})}
            addonAfter="tokens"
          />
        </Field>
      ) : null}
    </FormDialog>
  );
}

/**
 * The suite, published. Every case says what it asks the upstream and how the
 * answer is judged, because a score nobody can check is not evidence — and
 * every case can be reweighted, turned off, rewritten or added to, because the
 * questions worth asking of a reseller are not the same everywhere.
 */
export function ProbeCaseList({onChanged}: {onChanged?: () => void}) {
  const [cases, setCases] = React.useState<ProbeCase[] | null>(null);
  const [draft, setDraft] = React.useState<ProbeCase | null>(null);
  const [creating, setCreating] = React.useState(false);
  const [submitting, setSubmitting] = React.useState(false);
  const [busy, setBusy] = React.useState("");

  const load = React.useCallback(() => {
    ProbeCaseBackend.getProbeCases()
      .then(res => {
        if (res.status === "ok") {
          setCases((res.data ?? []).map(withLists));
        } else {
          Setting.showMessage("error", res.msg || i18next.t("general:Failed to get data"));
        }
      })
      .catch(failure => Setting.showMessage("error", `${failure}`));
  }, []);

  React.useEffect(() => {
    load();
  }, [load]);

  const done = () => {
    load();
    onChanged?.();
  };

  const save = () => {
    if (!draft) {
      return;
    }
    setSubmitting(true);
    const write = creating
      ? ProbeCaseBackend.addProbeCase(draft)
      : ProbeCaseBackend.updateProbeCase(draft.name, draft);
    write
      .then(res => {
        if (res.status !== "ok") {
          Setting.showMessage("error", res.msg || i18next.t("audit:The test case could not be saved"));
          return;
        }
        Setting.showMessage("success", i18next.t("audit:Test case saved"));
        setDraft(null);
        done();
      })
      .catch(failure => Setting.showMessage("error", `${failure}`))
      .then(() => setSubmitting(false));
  };

  const remove = (probeCase: ProbeCase) => {
    setBusy(probeCase.name);
    return ProbeCaseBackend.deleteProbeCase(probeCase.name)
      .then(res => {
        if (res.status !== "ok") {
          Setting.showMessage("error", res.msg || i18next.t("audit:The test case could not be saved"));
          return;
        }
        done();
      })
      .catch(failure => Setting.showMessage("error", `${failure}`))
      .then(() => setBusy(""));
  };

  const toggle = (probeCase: ProbeCase, enabled: boolean) => {
    setBusy(probeCase.name);
    ProbeCaseBackend.updateProbeCase(probeCase.name, {...probeCase, enabled: enabled})
      .then(res => {
        if (res.status !== "ok") {
          Setting.showMessage("error", res.msg || i18next.t("audit:The test case could not be saved"));
          return;
        }
        done();
      })
      .catch(failure => Setting.showMessage("error", `${failure}`))
      .then(() => setBusy(""));
  };

  const restore = () => {
    return ProbeCaseBackend.resetProbeCases()
      .then(res => {
        if (res.status !== "ok") {
          Setting.showMessage("error", res.msg || i18next.t("audit:The test case could not be saved"));
          return;
        }
        Setting.showMessage("success", i18next.t("audit:Defaults restored"));
        done();
      })
      .catch(failure => Setting.showMessage("error", `${failure}`));
  };

  if (cases === null) {
    return <Loading />;
  }

  const weighted = cases
    .filter(probeCase => probeCase.enabled)
    .reduce((total, probeCase) => total + probeCase.weight, 0);

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <p className="text-muted-foreground max-w-3xl text-sm">{i18next.t("audit:Cases description")}</p>
        <div className="flex shrink-0 gap-2">
          <ConfirmDialog
            title={i18next.t("audit:Restore the shipped suite?")}
            description={i18next.t("audit:Restore detail")}
            variant="default"
            onConfirm={restore}
          >
            <Button variant="outline">
              <RotateCcw />
              {i18next.t("audit:Restore defaults")}
            </Button>
          </ConfirmDialog>
          <Button
            onClick={() => {
              setCreating(true);
              setDraft(emptyCase());
            }}
          >
            <Plus />
            {i18next.t("audit:New test case")}
          </Button>
        </div>
      </div>

      {cases.length === 0 ? (
        <Card>
          <EmptyState
            icon={FlaskConical}
            title={i18next.t("audit:No test case")}
            description={i18next.t("audit:No test case detail")}
          />
        </Card>
      ) : (
        <div className="space-y-3">
          {cases.map(probeCase => (
            <CaseCard
              key={probeCase.name}
              probeCase={probeCase}
              share={weighted === 0 ? 0 : (probeCase.weight / weighted) * 100}
              busy={busy === probeCase.name}
              onEdit={() => {
                setCreating(false);
                setDraft(probeCase);
              }}
              onDelete={() => remove(probeCase)}
              onToggle={enabled => toggle(probeCase, enabled)}
            />
          ))}
        </div>
      )}

      {draft ? (
        <CaseDialog
          open
          draft={draft}
          creating={creating}
          submitting={submitting}
          onChange={setDraft}
          onOpenChange={open => (open ? undefined : setDraft(null))}
          onSubmit={save}
        />
      ) : null}
    </div>
  );
}
