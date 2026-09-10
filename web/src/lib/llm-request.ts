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

/**
 * Reads a recorded request body into the shape the inspector renders. The two
 * APIs describe the same conversation differently, so both are normalized here
 * and the inspector renders one shape.
 */

export type BlockKind = "text" | "thinking" | "image" | "toolUse" | "toolResult" | "other";

export interface RequestBlock {
  kind: BlockKind;
  text?: string;
  toolName?: string;
  toolUseId?: string;
  /** A tool call's arguments, or the raw block when nothing else fits. */
  data?: unknown;
  isError?: boolean;
  mediaType?: string;
  /** The block carries a cache_control marker. */
  cached?: boolean;
  chars: number;
}

export interface RequestMessage {
  index: number;
  role: string;
  blocks: RequestBlock[];
  chars: number;
}

export interface ToolParameter {
  name: string;
  type: string;
  required: boolean;
  description: string;
}

export interface ToolSchema {
  name: string;
  description: string;
  parameters: ToolParameter[];
  schema: unknown;
  cached: boolean;
  chars: number;
}

export interface ParsedRequest {
  system: RequestBlock[];
  systemChars: number;
  messages: RequestMessage[];
  tools: ToolSchema[];
  /** Everything else the body carried, e.g. max_tokens or tool_choice. */
  params: {key: string; value: string}[];
  raw: string;
  /** The body was not JSON, so only the raw tab has anything to show. */
  invalid: boolean;
  /** Too large even after shrinking, so only a preview is left. */
  previewOnly: boolean;
}

const SYSTEM_ROLES = ["system", "developer"];

/** The keys the inspector renders in tabs of their own, not as parameters. */
const OWN_TABS = ["system", "messages", "tools"];

function textOf(value: unknown): string {
  if (typeof value === "string") {
    return value;
  }
  if (Array.isArray(value)) {
    return value.map(textOf).filter(Boolean).join("\n\n");
  }
  if (value && typeof value === "object") {
    const text = (value as Record<string, unknown>).text;
    return typeof text === "string" ? text : "";
  }
  return "";
}

function isCached(block: unknown): boolean {
  return Boolean(block && typeof block === "object" && (block as Record<string, unknown>).cache_control);
}

function textBlock(text: string, kind: BlockKind = "text", cached = false): RequestBlock {
  return {kind: kind, text: text, cached: cached, chars: text.length};
}

/** One content block, in either API's spelling. */
function readBlock(value: unknown): RequestBlock {
  if (typeof value === "string") {
    return textBlock(value);
  }
  if (!value || typeof value !== "object") {
    return {kind: "other", data: value, chars: 0};
  }

  const block = value as Record<string, any>;
  const cached = isCached(block);
  switch (block.type) {
  case "text":
  case "input_text":
  case "output_text":
    return textBlock(String(block.text ?? ""), "text", cached);
  case "thinking":
  case "reasoning":
    return textBlock(String(block.thinking ?? block.text ?? ""), "thinking", cached);
  case "redacted_thinking":
    return {kind: "thinking", text: "", cached: cached, chars: String(block.data ?? "").length};
  case "image":
  case "image_url":
  case "input_image":
    return {
      kind: "image",
      mediaType: String(block.source?.media_type ?? block.source?.type ?? "image"),
      cached: cached,
      chars: String(block.source?.data ?? block.image_url?.url ?? "").length,
    };
  case "tool_use":
  case "server_tool_use":
    return {
      kind: "toolUse",
      toolName: String(block.name ?? ""),
      toolUseId: String(block.id ?? ""),
      data: block.input,
      cached: cached,
      chars: JSON.stringify(block.input ?? "").length,
    };
  case "tool_result": {
    const text = textOf(block.content);
    return {
      kind: "toolResult",
      toolUseId: String(block.tool_use_id ?? ""),
      text: text,
      data: text === "" ? block.content : undefined,
      isError: Boolean(block.is_error),
      cached: cached,
      chars: text.length,
    };
  }
  default: {
    const text = textOf(block);
    if (text !== "") {
      return textBlock(text, "text", cached);
    }
    return {kind: "other", data: block, cached: cached, chars: JSON.stringify(block ?? "").length};
  }
  }
}

function readContent(content: unknown): RequestBlock[] {
  if (content === undefined || content === null || content === "") {
    return [];
  }
  return (Array.isArray(content) ? content : [content]).map(readBlock);
}

/** An OpenAI assistant message asks for its tools in a field of its own. */
function readToolCalls(message: Record<string, any>): RequestBlock[] {
  if (!Array.isArray(message.tool_calls)) {
    return [];
  }
  return message.tool_calls.map((call: Record<string, any>) => {
    const args = call.function?.arguments;
    let parsed: unknown = args;
    try {
      parsed = typeof args === "string" ? JSON.parse(args) : args;
    } catch {
      // Cut short by the capture limit, so the text is what there is to show.
    }
    return {
      kind: "toolUse" as const,
      toolName: String(call.function?.name ?? call.name ?? ""),
      toolUseId: String(call.id ?? ""),
      data: parsed,
      chars: String(args ?? "").length,
    };
  });
}

function readTool(value: unknown): ToolSchema | null {
  if (!value || typeof value !== "object") {
    return null;
  }
  const tool = value as Record<string, any>;
  // OpenAI wraps the tool in a function envelope, Anthropic does not.
  const inner = tool.function && typeof tool.function === "object" ? tool.function : tool;
  const schema = inner.input_schema ?? inner.parameters ?? inner.schema;
  const properties = (schema?.properties ?? {}) as Record<string, any>;
  const required: string[] = Array.isArray(schema?.required) ? schema.required : [];

  return {
    name: String(inner.name ?? tool.type ?? ""),
    description: String(inner.description ?? ""),
    parameters: Object.keys(properties).map(name => ({
      name: name,
      type: typeOfProperty(properties[name]),
      required: required.includes(name),
      description: String(properties[name]?.description ?? ""),
    })),
    schema: schema ?? tool,
    cached: isCached(tool),
    chars: JSON.stringify(tool ?? "").length,
  };
}

function typeOfProperty(property: any): string {
  if (!property || typeof property !== "object") {
    return "";
  }
  if (typeof property.type === "string") {
    return property.type;
  }
  const alternatives = property.anyOf ?? property.oneOf;
  if (Array.isArray(alternatives)) {
    return alternatives.map((one: any) => one?.type).filter(Boolean).join(" | ");
  }
  return property.enum ? "enum" : "";
}

export interface ParsedResponse {
  message: RequestMessage | null;
  stopReason: string;
  raw: string;
  invalid: boolean;
}

/** Reads a recorded answer, which is stored as an Anthropic message whatever the upstream spoke. */
export function parseResponse(payload: string): ParsedResponse {
  const empty: ParsedResponse = {message: null, stopReason: "", raw: payload, invalid: false};
  if (!payload) {
    return empty;
  }

  let body: Record<string, any>;
  try {
    body = JSON.parse(payload);
  } catch {
    return {...empty, invalid: true};
  }
  if (!body || typeof body !== "object" || !Array.isArray(body.content)) {
    return {...empty, raw: JSON.stringify(body, null, 2), invalid: true};
  }

  const blocks = readContent(body.content);
  return {
    message: {
      index: 0,
      role: String(body.role ?? "assistant"),
      blocks: blocks,
      chars: blocks.reduce((total, block) => total + block.chars, 0),
    },
    stopReason: String(body.stop_reason ?? ""),
    raw: JSON.stringify(body, null, 2),
    invalid: false,
  };
}

export function parseRequest(payload: string): ParsedRequest {
  const empty: ParsedRequest = {
    system: [],
    systemChars: 0,
    messages: [],
    tools: [],
    params: [],
    raw: payload,
    invalid: false,
    previewOnly: false,
  };
  if (!payload) {
    return empty;
  }

  let body: Record<string, any>;
  try {
    body = JSON.parse(payload);
  } catch {
    return {...empty, invalid: true};
  }
  if (!body || typeof body !== "object") {
    return {...empty, invalid: true};
  }

  const raw = JSON.stringify(body, null, 2);
  if (body.truncated === true && typeof body.preview === "string") {
    return {...empty, raw: raw, previewOnly: true};
  }

  const system = readContent(body.system);
  const messages: RequestMessage[] = [];
  const rawMessages: unknown[] = Array.isArray(body.messages) ? body.messages : [];
  rawMessages.forEach((item, index) => {
    if (!item || typeof item !== "object") {
      return;
    }
    const message = item as Record<string, any>;
    const role = String(message.role ?? "");
    const blocks = [...readContent(message.content), ...readToolCalls(message)];

    // Lifted out so the System tab means the same for both APIs.
    if (SYSTEM_ROLES.includes(role) && messages.length === 0) {
      system.push(...blocks);
      return;
    }
    // An OpenAI tool reply is a message of its own rather than a block.
    if (role === "tool" && message.tool_call_id) {
      blocks.forEach(block => {
        block.kind = "toolResult";
        block.toolUseId = String(message.tool_call_id);
        block.toolName = String(message.name ?? "");
      });
    }

    messages.push({
      index: index,
      role: role,
      blocks: blocks,
      chars: blocks.reduce((total, block) => total + block.chars, 0),
    });
  });

  const tools = (Array.isArray(body.tools) ? body.tools : [])
    .map(readTool)
    .filter((tool): tool is ToolSchema => tool !== null);

  const params = Object.keys(body)
    .filter(key => !OWN_TABS.includes(key))
    .map(key => ({
      key: key,
      value: typeof body[key] === "object" ? JSON.stringify(body[key]) : String(body[key]),
    }));

  return {
    system: system,
    systemChars: system.reduce((total, block) => total + block.chars, 0),
    messages: messages,
    tools: tools,
    params: params,
    raw: raw,
    invalid: false,
    previewOnly: false,
  };
}
