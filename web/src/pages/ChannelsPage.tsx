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
import {MessagesSquare, Pencil, Plus, QrCode, Trash2} from "lucide-react";
import i18next from "i18next";
import {QRCodeSVG} from "qrcode.react";

import * as DriveBackend from "@/backend/DriveBackend";
import * as ImChannelBackend from "@/backend/ImChannelBackend";
import * as Setting from "@/Setting";
import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {EmptyState} from "@/components/shared/empty-state";
import {Field, FormDialog} from "@/components/shared/form-dialog";
import {PageContainer, PageHeader} from "@/components/shared/page-header";
import {SimpleSelect} from "@/components/shared/simple-select";
import {UnauthorizedResult} from "@/components/shared/misc";
import {chatPlatformOptions} from "@/components/shared/brand-options";
import {agentOption} from "@/components/shared/brand-options";
import {MessageAlert} from "@/components/ui/alert";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import {Switch} from "@/components/ui/switch";
import {TagsInput} from "@/components/ui/tags-input";
import type {Account, DrivableAgent, ImChannel, WeixinQrcode} from "@/types";

// scanPollInterval paces the wait for a scan.
const scanPollInterval = 1500;

const PLATFORM_TELEGRAM = "telegram";
const PLATFORM_WEIXIN = "weixin";

function emptyChannel(): ImChannel {
  return {
    name: "",
    platform: PLATFORM_TELEGRAM,
    enabled: true,
    token: "",
    agentId: "",
    agentPath: "",
    agentUser: "",
    workDir: "",
    model: "",
    allowedUsers: [],
  };
}

/** The one line that says whether a channel is actually listening. */
function StatusBadge({channel}: {channel: ImChannel}) {
  if (!channel.enabled) {
    return <Badge variant="outline">{i18next.t("channel:Off")}</Badge>;
  }
  if (channel.status?.error) {
    return <Badge variant="destructive">{channel.status.error}</Badge>;
  }
  if (channel.status?.running) {
    return <Badge variant="secondary">{i18next.t("channel:Listening")}</Badge>;
  }
  return <Badge variant="outline">{i18next.t("channel:Not listening")}</Badge>;
}

/**
 * The chat platforms Gateway listens on. Each one polls out from this machine,
 * so a bot works from a laptop behind any network without a public address.
 */
export default function ChannelsPage({account}: {account: Account}) {
  const isAdmin = Setting.isAdminUser(account);
  const [channels, setChannels] = React.useState<ImChannel[]>([]);
  const [agents, setAgents] = React.useState<DrivableAgent[]>([]);
  const [editing, setEditing] = React.useState<ImChannel | null>(null);
  const [isNew, setIsNew] = React.useState(false);
  const [qrcode, setQrcode] = React.useState<WeixinQrcode | null>(null);
  const [scanState, setScanState] = React.useState("");
  const [error, setError] = React.useState("");
  const [loading, setLoading] = React.useState(true);
  const [saving, setSaving] = React.useState(false);
  // The code being waited on. Clearing it is what stops the wait, so closing
  // the form does not leave a poll running against WeChat forever.
  const waitingFor = React.useRef("");

  const load = React.useCallback(() => {
    return ImChannelBackend.getImChannels()
      .then(res => {
        if (res.status === "ok") {
          setChannels(res.data ?? []);
          setError("");
        } else {
          setError(res.msg || i18next.t("general:Failed to get data"));
        }
      })
      .catch(failure => setError(failure.message || String(failure)));
  }, []);

  React.useEffect(() => {
    if (!isAdmin) {
      setLoading(false);
      return;
    }
    Promise.all([
      load(),
      DriveBackend.getDrivableAgents().then(res => setAgents(res.status === "ok" ? (res.data ?? []) : [])),
    ]).then(() => setLoading(false));
  }, [isAdmin, load]);

  // A listener that has just been switched on reports what the platform said
  // only once it has answered, so the list is read again shortly after a save.
  React.useEffect(() => {
    if (!isAdmin) {
      return;
    }
    const timer = window.setInterval(load, 5000);
    return () => window.clearInterval(timer);
  }, [isAdmin, load]);

  const closeForm = () => {
    waitingFor.current = "";
    setEditing(null);
    setQrcode(null);
    setScanState("");
  };

  const save = () => {
    if (!editing) {
      return;
    }
    setSaving(true);
    ImChannelBackend.updateImChannel(editing)
      .then(res => {
        if (res.status === "ok") {
          closeForm();
          return load();
        }
        setError(res.msg || i18next.t("general:Failed to save"));
      })
      .catch(failure => setError(failure.message || String(failure)))
      .then(() => setSaving(false));
  };

  // Signing WeChat in needs the channel to exist first: the token it hands back
  // is written straight into that row rather than carried through the browser.
  const scan = () => {
    if (!editing) {
      return;
    }
    setScanState(i18next.t("channel:Asking WeChat for a code"));
    ImChannelBackend.updateImChannel(editing)
      .then(() => ImChannelBackend.startWeixinLogin())
      .then(res => {
        if (res.status !== "ok" || !res.data) {
          setScanState(res.msg || i18next.t("general:Failed to get data"));
          return;
        }
        setQrcode(res.data);
        waitingFor.current = res.data.qrcode;
        setScanState(i18next.t("channel:Scan this with the WeChat account that will carry the bot"));
        poll(res.data.qrcode, editing.name);
      })
      .catch(failure => setScanState(failure.message || String(failure)));
  };

  const poll = (code: string, channel: string) => {
    if (waitingFor.current !== code) {
      return;
    }
    ImChannelBackend.getWeixinLoginStatus(code, channel)
      .then(res => {
        if (waitingFor.current !== code) {
          return;
        }
        if (res.status !== "ok") {
          setScanState(res.msg || "");
          return;
        }
        if (res.data?.status === "confirmed") {
          waitingFor.current = "";
          setQrcode(null);
          setScanState(i18next.t("channel:Signed in"));
          load();
          return;
        }
        // The endpoint answers "wait" immediately rather than holding the
        // request open, so asking again is on a timer.
        window.setTimeout(() => poll(code, channel), scanPollInterval);
      })
      .catch(failure => setScanState(failure.message || String(failure)));
  };

  const remove = (name: string) => {
    return ImChannelBackend.deleteImChannel(name)
      .then(() => load())
      .catch(failure => setError(failure.message || String(failure)));
  };

  if (!isAdmin) {
    return <UnauthorizedResult />;
  }

  const agentValue = editing ? editing.agentId + "\n" + editing.agentPath : "";

  return (
    <PageContainer>
      <PageHeader
        title={i18next.t("channel:Chat channels")}
        description={i18next.t("channel:Talk to an agent on this machine from a chat app")}
        actions={
          <Button
            onClick={() => {
              setEditing(emptyChannel());
              setIsNew(true);
              setQrcode(null);
              setScanState("");
            }}
          >
            <Plus className="size-4" />
            {i18next.t("channel:Add channel")}
          </Button>
        }
      />

      {error ? <MessageAlert variant="destructive" title={error} /> : null}

      {channels.length === 0 ? (
        <EmptyState
          icon={MessagesSquare}
          title={loading ? i18next.t("general:Loading") : i18next.t("channel:No channel yet")}
          description={i18next.t("channel:A channel polls out from this machine, so no public address is needed")}
        />
      ) : (
        <div className="flex flex-col gap-2">
          {channels.map(channel => (
            <div key={channel.name} className="flex flex-wrap items-center gap-3 rounded-lg border px-3 py-2.5">
              <span className="font-medium">{channel.name}</span>
              <Badge variant="outline">{channel.platform === PLATFORM_WEIXIN ? "WeChat" : "Telegram"}</Badge>
              <StatusBadge channel={channel} />
              <span className="text-muted-foreground min-w-0 flex-1 truncate text-xs">
                {channel.agentId || i18next.t("channel:No agent bound")}
                {channel.workDir ? " · " + channel.workDir : ""}
              </span>
              <Button
                size="sm"
                variant="ghost"
                onClick={() => {
                  setEditing({...channel, token: ""});
                  setIsNew(false);
                  setQrcode(null);
                  setScanState("");
                }}
              >
                <Pencil className="size-4" />
              </Button>
              <ConfirmDialog
                title={i18next.t("channel:Remove this channel?")}
                description={channel.name}
                onConfirm={() => remove(channel.name)}
              >
                <Button size="sm" variant="ghost">
                  <Trash2 className="size-4" />
                </Button>
              </ConfirmDialog>
            </div>
          ))}
        </div>
      )}

      <FormDialog
        open={editing !== null}
        onOpenChange={open => (open ? null : closeForm())}
        title={isNew ? i18next.t("channel:Add channel") : i18next.t("channel:Edit channel")}
        onSubmit={save}
        submitting={saving}
        submitDisabled={!editing?.name}
        size="lg"
      >
        {editing ? (
          <>
            <Field label={i18next.t("channel:Name")} required>
              <Input
                value={editing.name}
                disabled={!isNew}
                onChange={event => setEditing({...editing, name: event.target.value})}
              />
            </Field>

            <Field label={i18next.t("channel:Platform")}>
              <SimpleSelect
                value={editing.platform}
                onChange={value => setEditing({...editing, platform: value})}
                options={chatPlatformOptions}
                disabled={!isNew}
              />
            </Field>

            {editing.platform === PLATFORM_TELEGRAM ? (
              <Field
                label={i18next.t("channel:Bot token")}
                hint={i18next.t("channel:From @BotFather. Leave empty to keep the stored one")}
              >
                <Input
                  value={editing.token}
                  placeholder={isNew ? "" : "***"}
                  onChange={event => setEditing({...editing, token: event.target.value})}
                />
              </Field>
            ) : (
              <Field label={i18next.t("channel:WeChat sign-in")} hint={scanState}>
                {qrcode?.url ? (
                  // WeChat hands back the link its scanner opens rather than a
                  // picture, so the code itself is drawn here.
                  <div className="w-fit rounded border bg-white p-3">
                    <QRCodeSVG value={qrcode.url} size={168} />
                  </div>
                ) : (
                  <Button type="button" variant="outline" onClick={scan} disabled={!editing.name}>
                    <QrCode className="size-4" />
                    {i18next.t("channel:Show the code")}
                  </Button>
                )}
              </Field>
            )}

            <Field label={i18next.t("channel:Agent")}>
              <SimpleSelect
                value={agentValue}
                onChange={value => {
                  const [agentId, path] = value.split("\n");
                  const found = agents.find(candidate => candidate.agentId === agentId && candidate.path === path);
                  setEditing({
                    ...editing,
                    agentId: agentId ?? "",
                    agentPath: path ?? "",
                    agentUser: found?.owner ?? "",
                  });
                }}
                placeholder={i18next.t("chat:Pick an agent")}
                options={agents.map(candidate => ({
                  ...agentOption(candidate.agentId, candidate.name),
                  value: candidate.agentId + "\n" + candidate.path,
                }))}
              />
            </Field>

            <Field label={i18next.t("chat:Working directory")}>
              <Input
                value={editing.workDir}
                onChange={event => setEditing({...editing, workDir: event.target.value})}
              />
            </Field>

            <Field
              label={i18next.t("channel:Allowed users")}
              hint={i18next.t("channel:Empty lets anybody who finds the bot drive the agent")}
            >
              <TagsInput
                value={editing.allowedUsers ?? []}
                onChange={value => setEditing({...editing, allowedUsers: value})}
              />
            </Field>

            <Field label={i18next.t("channel:Listening")}>
              <Switch
                checked={editing.enabled}
                onCheckedChange={value => setEditing({...editing, enabled: value})}
              />
            </Field>
          </>
        ) : null}
      </FormDialog>
    </PageContainer>
  );
}
