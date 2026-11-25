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

import {useState} from "react";
import i18next from "i18next";

import * as AgentBackend from "@/backend/AgentBackend";
import {MessageAlert} from "@/components/ui/alert";
import {Button} from "@/components/ui/button";
import type {Agent} from "@/types";

export function NativeProviderImport({agent, onSaved}: {agent: Agent; onSaved: () => void}) {
  const [preview, setPreview] = useState<AgentBackend.NativeProviderPreview | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState("");
  const target = {agentId: agent.agentId, owner: agent.owner, path: agent.path};

  const inspect = async () => {
    setBusy(true);
    setError("");
    setSaved("");
    setPreview(null);
    try {
      const res = await AgentBackend.previewNativeProvider(target);
      if (res.status !== "ok" || !res.data?.revision) {
        throw new Error(res.msg || i18next.t("agent:Could not preview native provider"));
      }
      setPreview(res.data);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  const save = async () => {
    if (!preview) {
      return;
    }
    setBusy(true);
    setError("");
    try {
      const res = await AgentBackend.saveNativeProvider(target, preview.revision);
      if (res.status !== "ok" || !res.data) {
        throw new Error(res.msg || i18next.t("agent:Could not save native provider"));
      }
      setSaved(res.data);
      onSaved();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setPreview(null);
      setBusy(false);
    }
  };

  return (
    <section className="space-y-2 rounded-lg border p-4">
      <h3 className="text-sm font-medium">{i18next.t("agent:Import current CLI provider")}</h3>
      <p className="text-sm text-muted-foreground">{i18next.t("agent:Native provider import hint")}</p>
      {preview ? (
        <div className="space-y-2">
          <p className="break-all text-sm">{preview.provider.displayName} · {preview.provider.protocol}</p>
          <p className="break-all font-mono text-xs">{preview.provider.baseUrl}</p>
          {preview.provider.model && <p className="break-all text-xs">{preview.provider.model}</p>}
          <div className="flex gap-2">
            <Button size="sm" onClick={save} loading={busy}>{i18next.t("agent:Save as provider")}</Button>
            <Button size="sm" variant="ghost" onClick={() => setPreview(null)} disabled={busy}>{i18next.t("general:Cancel")}</Button>
          </div>
        </div>
      ) : (
        <Button size="sm" variant="outline" onClick={inspect} loading={busy}>{i18next.t("agent:Preview current provider")}</Button>
      )}
      {error && <MessageAlert title={error} />}
      {saved && <p role="status" className="text-sm">{i18next.t("agent:Native provider ready").replace("{id}", saved)}</p>}
    </section>
  );
}
