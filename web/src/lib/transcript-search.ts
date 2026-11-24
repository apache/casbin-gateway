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

import type {AgentMessage} from "@/types";

export interface TranscriptMatch {
  start: number;
  end: number;
  index: number;
}

interface TranscriptMatches {
  byMessage: Map<number, Map<number, TranscriptMatch[]>>;
  messageIndices: number[];
  limited: boolean;
}

// A short query can match thousands of times in one tool result alone.
const matchLimit = 1000;

export function findTranscriptMatches(messages: AgentMessage[], search: string): TranscriptMatches {
  const byMessage: TranscriptMatches["byMessage"] = new Map();
  const query = search.trim();
  const messageIndices: number[] = [];
  if (!query) {
    return {byMessage, messageIndices, limited: false};
  }

  let pattern: RegExp | undefined;
  for (const [messageIndex, message] of messages.entries()) {
    const blocks = new Map<number, TranscriptMatch[]>();
    for (const [blockIndex, block] of message.blocks.entries()) {
      if (!block.text || block.kind === "image" || block.text.length < query.length) {
        continue;
      }

      pattern ??= new RegExp(query.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"), "giu");
      const matches: TranscriptMatch[] = [];
      for (const match of block.text.matchAll(pattern)) {
        const index = messageIndices.length;
        if (index === matchLimit) {
          return {byMessage, messageIndices, limited: true};
        }
        if (matches.length === 0) {
          blocks.set(blockIndex, matches);
          byMessage.set(messageIndex, blocks);
        }
        matches.push({start: match.index, end: match.index + match[0].length, index});
        messageIndices.push(messageIndex);
      }
    }
  }
  return {byMessage, messageIndices, limited: false};
}
