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
import {Link} from "react-router-dom";
import {ArrowDownToLine} from "lucide-react";
import i18next from "i18next";

import * as CcSwitchBackend from "@/backend/CcSwitchBackend";
import {MessageAlert} from "@/components/ui/alert";
import {Button} from "@/components/ui/button";
import type {CcSwitchImport} from "@/types";

const dismissedStorageKey = "ccSwitchBannerDismissed";

function readDismissed(): boolean {
  try {
    return localStorage.getItem(dismissedStorageKey) === "true";
  } catch {
    return false;
  }
}

function writeDismissed() {
  try {
    localStorage.setItem(dismissedStorageKey, "true");
  } catch {
    // Private-mode storage failures must not take the page down.
  }
}

/**
 * What was found, counted, as "3 providers · 1 MCP server" and so on. Each
 * count is a phrase of its own rather than a number glued to a noun: English
 * needs the singular for one of them, and Chinese needs a measure word and no
 * space around the digit.
 */
function summary(found: CcSwitchImport) {
  const counts: [number, string, string][] = [
    [found.providers.length, "link:{count} provider", "link:{count} providers"],
    [found.mcps.length, "link:{count} MCP server", "link:{count} MCP servers"],
    [found.prompts.length, "link:{count} set of instructions", "link:{count} sets of instructions"],
    [found.skills.length, "link:{count} skill source", "link:{count} skill sources"],
  ];

  return counts
    .filter(([count]) => count > 0)
    .map(([count, one, many]) => i18next.t(count === 1 ? one : many).replace("{count}", `${count}`))
    .join(" · ");
}

/**
 * The one line somebody arriving from CC Switch needs on the home screen: it is
 * installed on this machine, and everything in it can be brought over. It shows
 * only while there is something to bring, so it disappears on its own once the
 * import is done, and can be dismissed by anyone keeping both tools.
 */
export function CcSwitchBanner() {
  const [found, setFound] = React.useState<CcSwitchImport | null>(null);
  const [dismissed, setDismissed] = React.useState(readDismissed);

  React.useEffect(() => {
    if (dismissed) {
      return;
    }
    // A machine without CC Switch is the common case and answers "not found",
    // so a failure here is nothing to report: the banner simply stays away.
    CcSwitchBackend.getCcSwitchImport()
      .then(res => setFound(res.status === "ok" ? res.data : null))
      .catch(() => setFound(null));
  }, [dismissed]);

  const total =
    found === null
      ? 0
      : found.providers.length + found.mcps.length + found.prompts.length + found.skills.length;

  if (dismissed || found === null || !found.found || total === 0) {
    return null;
  }

  return (
    <MessageAlert
      variant="info"
      title={i18next.t("link:CC Switch is installed here")}
      // Two lines rather than one sentence: the counts read as a list, and
      // joining them onto the hint would need a separator that is a space in
      // English and nothing at all in Chinese.
      description={
        <>
          <span className="block">{i18next.t("link:CC Switch is installed here hint")}</span>
          <span className="block font-medium">{summary(found)}</span>
        </>
      }
      action={
        <div className="flex flex-wrap gap-2">
          <Button asChild size="sm">
            <Link to="/import">
              <ArrowDownToLine />
              {i18next.t("provider:Bring it all over")}
            </Link>
          </Button>
          <Button
            type="button"
            size="sm"
            variant="ghost"
            onClick={() => {
              writeDismissed();
              setDismissed(true);
            }}
          >
            {i18next.t("general:Dismiss")}
          </Button>
        </div>
      }
    />
  );
}
