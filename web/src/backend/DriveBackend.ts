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

import {query, request} from "@/backend/request";
import {ServerUrl} from "@/Setting";
import type {DrivableAgent, DrivenEvent, DrivenSession} from "@/types";

/** The installations that publish a non-interactive mode, which is all Gateway can drive. */
export function getDrivableAgents() {
  return request<DrivableAgent[]>("/api/get-drivable-agents");
}

export function getDrivenSessions() {
  return request<DrivenSession[]>("/api/get-driven-sessions");
}

/** Opens a conversation. Nothing runs until it is asked something. */
export function openDrivenSession(target: {agentId: string; path: string; owner: string}, workDir: string, model = "") {
  return request<DrivenSession>("/api/open-driven-session", "POST", {...target, workDir: workDir, model: model});
}

/** Asks one thing; the answer arrives on the stream, not in this response. */
export function sendDrivenSession(id: string, prompt: string) {
  return request<DrivenSession>("/api/send-driven-session", "POST", {id: id, prompt: prompt});
}

export function interruptDrivenSession(id: string) {
  return request<boolean>("/api/interrupt-driven-session", "POST", {id: id});
}

export function closeDrivenSession(id: string) {
  return request<boolean>("/api/close-driven-session", "POST", {id: id});
}

/**
 * Opens the feed of one conversation; the browser reconnects on its own and the
 * server replays only what was missed. Returns the function that closes it.
 */
export function streamDrivenSession(
  id: string,
  handlers: {onEvent: (event: DrivenEvent) => void; onOpen?: () => void; onError?: () => void},
) {
  const source = new EventSource(`${ServerUrl}/api/stream-driven-session${query({id: id})}`, {
    withCredentials: true,
  });

  source.addEventListener("message", event => {
    try {
      handlers.onEvent(JSON.parse((event as MessageEvent).data) as DrivenEvent);
    } catch {
      // A malformed event is not worth tearing the feed down for.
    }
  });
  source.onopen = () => handlers.onOpen?.();
  source.onerror = () => handlers.onError?.();

  return () => source.close();
}
