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

import i18next from "i18next";

import type {AgentMessageBlock, AgentTranscript} from "@/types";

/** Transcript words are quoted, never laid out, so no message can take a heading of its own. */
function oneLine(text = "") {
  return text.replace(/\s+/g, " ").trim();
}

function backtickRun(text: string) {
  let longest = 0;
  for (const [run] of text.matchAll(/`+/g)) {
    longest = Math.max(longest, run.length);
  }
  return longest;
}

/** A fence has to outlast every run of backticks inside it, otherwise the text
 *  closes its own block and the rest of it is read as markdown. */
function code(text: string) {
  const mark = "`".repeat(Math.max(3, backtickRun(text) + 1));
  return `${mark}\n${text}${text.endsWith("\n") ? "" : "\n"}${mark}`;
}

function span(text: string) {
  const value = oneLine(text);
  if (value === "") {
    return "";
  }
  const mark = "`".repeat(backtickRun(value) + 1);
  const pad = value.startsWith("`") || value.endsWith("`") ? " " : "";
  return `${mark}${pad}${value}${pad}${mark}`;
}

function blockName(block: AgentMessageBlock) {
  switch (block.kind) {
  case "thinking":
    return i18next.t("agent:Thinking");
  case "toolUse":
    return i18next.t("agent:Tool call");
  case "toolResult":
    return block.isError ? i18next.t("agent:Tool error") : i18next.t("agent:Tool result");
  default:
    return "";
  }
}

function blockChunks(block: AgentMessageBlock) {
  if (block.kind === "image") {
    // What the transcript records is that an image sat here, never its bytes.
    return [`**[${i18next.t("agent:Image")}]**`];
  }

  const name = blockName(block);
  if (name === "") {
    return block.text ? [code(block.text)] : [];
  }

  const tool = block.kind === "toolUse" ? span(block.tool ?? "") : "";
  const label = `**${name}**${tool === "" ? "" : `: ${tool}`}`;
  if (!block.text) {
    // A call with no arguments, a result with no body: the file still says which.
    return [block.kind === "thinking" ? `${label}: *${i18next.t("agent:Not recorded")}*` : label];
  }
  return [label, code(block.text)];
}

/** The file a session download lands as, built from the transcript already in hand. */
export function transcriptMarkdown(transcript: AgentTranscript): string {
  const {session, messages} = transcript;
  const title = session.title || session.sessionKey || i18next.t("agent:Agent Session");
  const chunks = [
    `# ${span(title)}`,
    `${i18next.t("agent:Agent")}: ${span(session.agent)} · ${i18next.t("agent:Session")}: ${span(session.sessionKey)}`,
  ];
  if (session.archived) {
    chunks.push(i18next.t("agent:Archived"));
  }

  if (transcript.truncated) {
    // The file on disk holds more than the server sent, so what is missing is said.
    chunks.push(`> ${i18next.t("agent:Transcript truncated")}`);
  }

  for (const [index, message] of messages.entries()) {
    const time = span(message.time ?? "");
    const role = span(message.role) || i18next.t("agent:Unknown");
    chunks.push(`## ${index + 1}. ${role}${time === "" ? "" : ` — ${time}`}`);

    const blocks = message.blocks.flatMap(blockChunks);
    chunks.push(...(blocks.length === 0 ? [`*${i18next.t("agent:Empty message")}*`] : blocks));
  }

  return `${chunks.join("\n\n")}\n`;
}

export function transcriptFileName({session}: AgentTranscript): string {
  const title = session.title || session.sessionKey || session.agent;
  const stem = title.normalize().replace(/[^\p{L}\p{N}_-]+/gu, "-").replace(/^-+|-+$/g, "");
  return `session-${Array.from(stem).slice(0, 50).join("") || "transcript"}.md`;
}
