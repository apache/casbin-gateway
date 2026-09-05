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
import {ExternalLink, KeyRound, LogIn, RefreshCw, Save, Trash2, UserCheck, UserRound} from "lucide-react";
import i18next from "i18next";

import * as Setting from "@/Setting";
import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {DataTable, type Column} from "@/components/shared/data-table";
import {CodeText} from "@/components/shared/misc";
import {Alert, AlertDescription} from "@/components/ui/alert";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {Input} from "@/components/ui/input";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {AiDots} from "@/components/shared/loading";
import {
  accountKindLabel,
  accountName,
  accountsOf,
  agentKey,
  useAgentAccounts,
  type AgentAccountControls,
} from "@/lib/agents";
import type {Agent, SavedAccount} from "@/types";

/** The mark for one kind of sign-in, so a key is told from an account at a glance. */
function KindIcon({kind, className}: {kind: string; className?: string}) {
  return kind === "apikey" ? (
    <KeyRound className={className} />
  ) : (
    <UserRound className={className} />
  );
}

/** Editable label of one stored account, so a second account of the same plan
 *  is told from the first without reading its email. */
function AccountName({
  account,
  onRename,
}: {
  account: SavedAccount;
  onRename: (account: SavedAccount, displayName: string) => void;
}) {
  const [value, setValue] = React.useState(account.displayName);

  React.useEffect(() => setValue(account.displayName), [account.displayName]);

  return (
    <Input
      value={value}
      placeholder={account.email || account.name}
      aria-label={i18next.t("general:Name")}
      className="hover:border-input h-7 border-transparent px-1 font-medium shadow-none"
      onChange={event => setValue(event.target.value)}
      onBlur={() => {
        const named = value.trim();
        if (named !== account.displayName) {
          onRename(account, named);
        }
      }}
      onKeyDown={event => {
        if (event.key === "Enter") {
          event.currentTarget.blur();
        } else if (event.key === "Escape") {
          setValue(account.displayName);
        }
      }}
    />
  );
}

/** Stores an API key as an account of its own, so the key and a subscription
 *  account are swapped the same way. */
function AddApiKeyDialog({
  agent,
  controls,
}: {
  agent: Agent;
  controls: AgentAccountControls;
}) {
  const [open, setOpen] = React.useState(false);
  const [apiKey, setApiKey] = React.useState("");
  const [displayName, setDisplayName] = React.useState("");

  React.useEffect(() => {
    if (open) {
      setApiKey("");
      setDisplayName("");
    }
  }, [open]);

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button variant="outline" size="sm">
          <KeyRound />
          {i18next.t("agent:Add API key")}
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{i18next.t("agent:Add API key")}</DialogTitle>
          <DialogDescription>{i18next.t("agent:Add API key hint")}</DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <Input
            value={apiKey}
            type="password"
            autoComplete="off"
            placeholder="sk-..."
            aria-label={i18next.t("agent:API key")}
            onChange={event => setApiKey(event.target.value)}
          />
          <Input
            value={displayName}
            placeholder={i18next.t("general:Name")}
            aria-label={i18next.t("general:Name")}
            onChange={event => setDisplayName(event.target.value)}
          />
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => setOpen(false)}>
            {i18next.t("general:Cancel")}
          </Button>
          <Button
            disabled={apiKey.trim() === ""}
            loading={controls.busyKey === `${agentKey(agent)}:add`}
            onClick={() => controls.add(agent, apiKey, displayName).then(() => setOpen(false))}
          >
            {i18next.t("general:Save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/**
 * The browser sign-in, while it waits. The agent's own login is run against a
 * directory of its own, so the account it brings back is stored without
 * touching the one the agent is using; the swap is a separate click.
 *
 * One of these is rendered per page, since one sign-in runs at a time.
 */
export function AgentSigninDialog({controls}: {controls: AgentAccountControls}) {
  const session = controls.session;
  if (!session) {
    return null;
  }

  const finished = !session.running;
  return (
    <Dialog open={true} onOpenChange={() => controls.closeSession(session.ok)}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{i18next.t("agent:Sign in")}</DialogTitle>
          <DialogDescription>
            {session.running
              ? i18next.t("agent:Finish the sign-in in the browser")
              : session.ok
                ? `${i18next.t("agent:Account saved")}: ${session.account ?? ""}`
                : i18next.t("agent:The sign-in did not finish")}
          </DialogDescription>
        </DialogHeader>

        {session.running ? (
          <div className="space-y-3">
            <AiDots />
            {session.url ? (
              <div className="space-y-2">
                <CodeText copyable>{session.url}</CodeText>
                <Button variant="outline" size="sm" asChild>
                  <a href={session.url} target="_blank" rel="noreferrer">
                    <ExternalLink />
                    {i18next.t("agent:Open the sign-in page")}
                  </a>
                </Button>
              </div>
            ) : null}
          </div>
        ) : null}

        {finished && session.error ? (
          <Alert variant="destructive">
            <AlertDescription>{session.error}</AlertDescription>
          </Alert>
        ) : null}

        <DialogFooter>
          <Button
            variant={finished ? "default" : "outline"}
            onClick={() => controls.closeSession(session.ok)}
          >
            {i18next.t(finished ? "general:Close" : "general:Cancel")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/**
 * The sign-ins Gateway keeps for one agent. Codex writes one credential file
 * and its own sign-in overwrites whatever was there, so a subscription account
 * and an API key are only held at once by keeping copies here - and swapping
 * between them is then a click rather than another sign-in.
 */
export function AgentAccounts({agent, enabled = true}: {agent: Agent; enabled?: boolean}) {
  // The card owns one agent, so it owns one hook; the home page hands the same
  // controls to a row of cards instead.
  const agents = React.useMemo(() => [agent], [agent]);
  const controls = useAgentAccounts(agents, enabled && agent.supportsAccounts === true);

  if (!agent.supportsAccounts) {
    return null;
  }

  const listing = accountsOf(controls, agent);
  const columns: Column<SavedAccount>[] = [
    {
      title: i18next.t("general:Name"),
      key: "displayName",
      dataIndex: "displayName",
      render: (_value, record) => (
        <AccountName
          account={record}
          onRename={(account, displayName) => controls.rename(account, displayName)}
        />
      ),
    },
    {
      title: i18next.t("agent:Account"),
      key: "email",
      render: (_value, record) => (
        <div className="flex flex-col">
          <span className="flex items-center gap-2">
            <Badge variant="muted">
              <KindIcon kind={record.kind} className="size-3" />
              {accountKindLabel(record.kind)}
            </Badge>
            {record.plan ? <span className="capitalize">{record.plan}</span> : null}
          </span>
          {record.email ? (
            <code className="text-muted-foreground text-xs">{record.email}</code>
          ) : null}
        </div>
      ),
    },
    {
      title: i18next.t("agent:Status"),
      key: "current",
      render: (_value, record) =>
        record.current ? (
          <Badge variant="success">{i18next.t("agent:In use")}</Badge>
        ) : (
          <span className="text-muted-foreground text-xs">
            {Setting.getFormattedDate(record.lastUsedTime) || i18next.t("agent:Never used")}
          </span>
        ),
    },
    {
      title: i18next.t("general:Action"),
      key: "action",
      render: (_value, record) => (
        <div className="flex items-center gap-2">
          <Button
            size="sm"
            variant="outline"
            disabled={record.current === true}
            loading={controls.busyKey === record.name}
            onClick={() => controls.switchTo(agent, record)}
          >
            <UserCheck />
            {i18next.t("agent:Use")}
          </Button>
          <ConfirmDialog
            title={`${i18next.t("agent:Remove account")}: ${accountName(record)}?`}
            description={i18next.t("agent:Remove account hint")}
            confirmText={i18next.t("agent:Remove account")}
            variant="destructive"
            onConfirm={() => controls.remove(record)}
          >
            <Button size="icon" variant="ghost" className="size-6">
              <Trash2 />
            </Button>
          </ConfirmDialog>
        </div>
      ),
    },
  ];

  // A sign-in in place that Gateway has no copy of is the one a swap would lose,
  // so it is offered for saving before anything else is switched to.
  const unsaved = listing.current != null && !listing.stored;

  return (
    <div className="space-y-3">
      {unsaved ? (
        <Alert variant="warning">
          <AlertDescription className="flex flex-wrap items-center justify-between gap-2">
            <span>
              {i18next
                .t("agent:Unsaved sign-in hint")
                .replace(
                  "{account}",
                  listing.current?.email || accountKindLabel(listing.current?.kind ?? ""),
                )}
            </span>
            <Button
              size="sm"
              loading={controls.busyKey === `${agentKey(agent)}:save`}
              onClick={() => controls.saveCurrent(agent)}
            >
              <Save />
              {i18next.t("agent:Save the sign-in in use")}
            </Button>
          </AlertDescription>
        </Alert>
      ) : null}

      <DataTable
        title={i18next.t("agent:Accounts")}
        description={i18next.t("agent:Accounts hint")}
        columns={columns}
        dataSource={listing.accounts}
        rowKey={account => account.name}
        loading={controls.loading}
        pageSize={0}
        emptyText={i18next.t("agent:No accounts saved yet")}
        toolbar={
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => controls.reload()}
              loading={controls.loading}
            >
              <RefreshCw />
              {i18next.t("general:Refresh")}
            </Button>
            <AddApiKeyDialog agent={agent} controls={controls} />
            <Button
              size="sm"
              loading={controls.busyKey === `${agentKey(agent)}:signin`}
              onClick={() => controls.signIn(agent)}
            >
              <LogIn />
              {i18next.t("agent:Sign in")}
            </Button>
          </div>
        }
      />

      <AgentSigninDialog controls={controls} />
    </div>
  );
}

/** The value the picker carries for a sign-in Gateway holds no copy of. */
const unsavedAccount = "-";

/**
 * The account of one agent as a grid card shows it: the sign-in it is on, and
 * the picker that moves it to another one Gateway holds. The card has no room
 * for the rest, so saving, renaming and removing live on the detail page.
 */
export function AgentCardAccounts({
  agent,
  controls,
}: {
  agent: Agent;
  controls: AgentAccountControls;
}) {
  if (!agent.supportsAccounts) {
    return null;
  }

  const listing = accountsOf(controls, agent);
  // Nothing is signed in and nothing is stored: there is no account to show,
  // and the sign-in that would make one is on the detail page.
  if (listing.current == null && listing.accounts.length === 0) {
    return null;
  }

  const current = listing.accounts.find(account => account.current);
  const busy = controls.busyKey !== "";

  return (
    <div className="space-y-1.5">
      <div className="flex min-w-0 items-center justify-between gap-2">
        <span className="text-muted-foreground shrink-0 text-[11px]">
          {i18next.t("agent:Account")}
        </span>
        {listing.current ? (
          <span className="text-muted-foreground truncate text-[11px]">
            {accountKindLabel(listing.current.kind)}
          </span>
        ) : null}
      </div>

      <div className="flex min-w-0 items-center gap-1.5">
        <Select
          value={current ? current.name : unsavedAccount}
          disabled={busy}
          onValueChange={value => {
            const picked = listing.accounts.find(account => account.name === value);
            if (picked && !picked.current) {
              controls.switchTo(agent, picked);
            }
          }}
        >
          {/* The box shows the picked item's own children, so it is told to fit
              them: the note beside each name is dropped, since on a card this
              narrow it would leave the email nothing to be read in. */}
          <SelectTrigger
            size="sm"
            className="min-w-0 flex-1 text-xs *:data-[slot=select-value]:min-w-0 *:data-[slot=select-value]:flex-1 *:data-[slot=select-value]:overflow-hidden [&_[data-note]]:hidden"
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {/* The one in place, when Gateway has no copy of it: it cannot be
                switched back to, so it is shown and left unselectable. */}
            {current ? null : (
              <SelectItem value={unsavedAccount}>
                <KindIcon kind={listing.current?.kind ?? ""} className="size-4" />
                <span className="min-w-0 flex-1 truncate">
                  {listing.current?.email || accountKindLabel(listing.current?.kind ?? "")}
                </span>
                <span data-note className="text-warning ml-2 shrink-0 text-xs">
                  {i18next.t("agent:Not saved")}
                </span>
              </SelectItem>
            )}
            {listing.accounts.map(account => (
              <SelectItem key={account.name} value={account.name}>
                <KindIcon kind={account.kind} className="size-4" />
                <span className="min-w-0 flex-1 truncate">{accountName(account)}</span>
                <span data-note className="text-muted-foreground ml-2 shrink-0 text-xs capitalize">
                  {account.plan || accountKindLabel(account.kind)}
                </span>
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <SimpleTooltip title={i18next.t("agent:Sign in")}>
          <span>
            <Button
              size="icon"
              variant="ghost"
              className="size-7 shrink-0"
              aria-label={i18next.t("agent:Sign in")}
              loading={controls.busyKey === `${agentKey(agent)}:signin`}
              onClick={() => controls.signIn(agent)}
            >
              <LogIn />
            </Button>
          </span>
        </SimpleTooltip>
      </div>

      {current ? null : (
        <button
          type="button"
          className="text-warning hover:underline text-[11px]"
          disabled={busy}
          onClick={() => controls.saveCurrent(agent)}
        >
          {i18next.t("agent:Save the sign-in in use")}
        </button>
      )}
    </div>
  );
}
