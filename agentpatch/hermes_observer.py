"""Casbin Gateway's behaviour observer for Hermes Agent.

This module intentionally never serializes prompts, responses, reasoning,
tool results, or subagent text. Tool arguments are redacted and bounded before
hook callbacks enqueue records without waiting for the Gateway HTTP endpoint.
"""

from __future__ import annotations

import atexit
import json
import logging
import queue
import re
import threading
import urllib.request
from pathlib import Path
from typing import Any


_LOGGER = logging.getLogger(__name__)
_CONFIG_PATH = Path(__file__).with_name("gateway.json")
_QUEUE: queue.Queue[dict[str, Any]] = queue.Queue(maxsize=1024)
_STOP = threading.Event()
_SENDER: threading.Thread | None = None
_REGISTER_LOCK = threading.Lock()
_MAX_OBJECT_BYTES = 64 * 1024

_CREDENTIAL_PATTERN = re.compile(
    r"\b(?:sk-(?:ant-|proj-)?[a-z0-9_-]{12,}|gh[pousr]_[a-z0-9]{20,}|"
    r"github_pat_[a-z0-9_]{20,}|AKIA[0-9A-Z]{16}|AIza[0-9a-z_-]{30,}|"
    r"xox[baprs]-[0-9a-z-]{12,}|eyJ[a-z0-9_-]{10,}\.[a-z0-9_-]{10,}\."
    r"[a-z0-9_-]{10,})\b",
    re.IGNORECASE,
)
_BEARER_PATTERN = re.compile(r"(bearer\s+)[a-z0-9._~+/=-]{12,}", re.IGNORECASE)
_PRIVATE_KEY_PATTERN = re.compile(
    r"-----BEGIN [^-\n]*PRIVATE KEY-----.*?-----END [^-\n]*PRIVATE KEY-----",
    re.DOTALL,
)
_SENSITIVE_KEY_MARKERS = (
    "secret",
    "token",
    "password",
    "passwd",
    "credential",
    "privatekey",
    "apikey",
    "accesskey",
    "authorization",
    "cookie",
)

_TOKEN_COUNT_KEYS = {
    "maxtokens",
    "maxcompletiontokens",
    "maxtokenstosample",
    "maxoutputtokens",
    "prompttokens",
    "completiontokens",
    "totaltokens",
    "reasoningtokens",
    "inputtokens",
    "outputtokens",
    "tokencount",
    "numtokens",
    "tokens",
    "cachecreationinputtokens",
    "cachereadinputtokens",
    "cachedtokens",
}

_USAGE_KEYS = {
    "input_tokens",
    "output_tokens",
    "total_tokens",
    "prompt_tokens",
    "completion_tokens",
    "cache_read_input_tokens",
    "cache_creation_input_tokens",
    "reasoning_tokens",
    "inputTokens",
    "outputTokens",
    "totalTokens",
    "promptTokens",
    "completionTokens",
    "cacheReadInputTokens",
    "cacheCreationInputTokens",
    "reasoningTokens",
}


def _load_config() -> dict[str, Any]:
    try:
        value = json.loads(_CONFIG_PATH.read_text(encoding="utf-8"))
        return value if isinstance(value, dict) else {}
    except Exception:
        return {}


_CONFIG = _load_config()
_RECORDS_URL = str(_CONFIG.get("recordsUrl") or "")
_AGENT_PATH = str(_CONFIG.get("agentPath") or "")
_USER = str(_CONFIG.get("user") or "")
_INGEST_TOKEN = str(_CONFIG.get("ingestToken") or "")
_INGEST_TOKEN_HEADER = str(_CONFIG.get("ingestTokenHeader") or "")


def _text(value: Any, limit: int = 512) -> str:
    if value is None:
        return ""
    if isinstance(value, (str, int, float, bool)):
        return str(value)[:limit]
    return ""


def _integer(value: Any) -> int | None:
    if isinstance(value, bool):
        return None
    if isinstance(value, (int, float)):
        return int(value)
    return None


def _duration_ms(value: Any, *, seconds: bool = False) -> int:
    try:
        number = float(value)
    except (TypeError, ValueError):
        return 0
    if seconds:
        number *= 1000
    return max(0, int(number))


def _length(value: Any) -> int:
    if value is None:
        return 0
    try:
        return len(value)
    except (TypeError, ValueError):
        return len(str(value))


def _count(value: Any) -> int:
    if isinstance(value, (list, tuple, set, dict)):
        return len(value)
    return 0


def _metadata(**values: Any) -> dict[str, Any]:
    return {key: value for key, value in values.items() if value not in ("", None)}


def _sanitize_value(key: str, value: Any) -> Any:
    normalized_key = key.lower().replace("_", "").replace("-", "").replace(".", "")
    if normalized_key not in _TOKEN_COUNT_KEYS and any(
        marker in normalized_key for marker in _SENSITIVE_KEY_MARKERS
    ):
        return "[REDACTED]"
    if isinstance(value, dict):
        return {child_key: _sanitize_value(child_key, child) for child_key, child in value.items()}
    if isinstance(value, list):
        return [_sanitize_value("", child) for child in value]
    if isinstance(value, str):
        value = _PRIVATE_KEY_PATTERN.sub("[REDACTED PRIVATE KEY]", value)
        value = _BEARER_PATTERN.sub(r"\1[REDACTED]", value)
        return _CREDENTIAL_PATTERN.sub("[REDACTED]", value)
    return value


def _sensitive_path(file_path: str) -> bool:
    normalized = file_path.lower().replace("\\", "/")
    base = normalized.rsplit("/", 1)[-1]
    if base.startswith(".env") and base not in {".env.example", ".env.sample", ".env.template"}:
        return True
    if normalized.startswith(".ssh/") or "/.ssh/" in normalized:
        return True
    if normalized == ".aws/credentials" or normalized.endswith("/.aws/credentials"):
        return True
    if base in {".npmrc", ".pypirc", "credentials", "id_rsa", "id_ed25519"}:
        return True
    return base.endswith((".pem", ".key"))


def _sanitize_tool_arguments(tool_name: str, arguments: dict[str, Any]) -> dict[str, Any]:
    sanitized = _sanitize_value("", arguments)
    normalized_tool = tool_name.lower()
    if (normalized_tool == "patch" or normalized_tool.endswith("__patch")) and "patch" in sanitized:
        sanitized["patch"] = "[OMITTED: patch content]"
    sensitive_write = normalized_tool in {"write", "edit", "write_file", "edit_file", "patch"} or normalized_tool.endswith(
        ("__write_file", "__edit_file", "__patch")
    )
    file_path = arguments.get("path", "") or arguments.get("file_path", "")
    if not sensitive_write or not _sensitive_path(file_path):
        return sanitized
    for key in ("content", "old_string", "new_string"):
        if key in sanitized:
            sanitized[key] = "[REDACTED: sensitive file content]"
    return sanitized


def _usage(value: Any) -> dict[str, int]:
    if not isinstance(value, dict):
        return {}
    result: dict[str, int] = {}
    for key in _USAGE_KEYS:
        number = _integer(value.get(key))
        if number is not None:
            result[key] = number
    return result


def _mcp_identity(tool_name: str) -> tuple[str, str]:
    parts = tool_name.split("__", 2)
    if len(parts) == 3 and parts[0].lower() == "mcp" and parts[1] and parts[2]:
        return parts[1], parts[2]
    return "", ""


def _base_record(
    event_type: str,
    action: str,
    outcome: str,
    values: dict[str, Any],
    metadata: dict[str, Any] | None = None,
) -> dict[str, Any]:
    record: dict[str, Any] = {
        "agent": "hermes-agent",
        "agentPath": _AGENT_PATH,
        "user": _USER,
        "eventType": event_type,
        "action": action,
        "sessionKey": _text(
            values.get("session_id")
            or values.get("parent_session_id")
            or values.get("child_session_id")
        ),
        "model": _text(values.get("model")),
        "promptId": _text(
            values.get("api_request_id")
            or values.get("turn_id")
            or values.get("task_id")
        ),
    }
    if outcome:
        record["outcome"] = outcome
    if metadata:
        encoded = json.dumps(
            metadata, ensure_ascii=False, separators=(",", ":"), sort_keys=True
        )
        encoded_bytes = encoded.encode("utf-8")
        if len(encoded_bytes) > _MAX_OBJECT_BYTES:
            encoded = json.dumps(
                {
                    "truncated": True,
                    "originalBytes": len(encoded_bytes),
                    "preview": encoded_bytes[: _MAX_OBJECT_BYTES // 3].decode(
                        "utf-8", errors="ignore"
                    ),
                },
                ensure_ascii=False,
                separators=(",", ":"),
                sort_keys=True,
            )
        record["object"] = encoded
    return {key: value for key, value in record.items() if value not in ("", None)}


def _enqueue(record: dict[str, Any]) -> None:
    if not _RECORDS_URL:
        return
    try:
        _QUEUE.put_nowait(record)
    except queue.Full:
        _LOGGER.debug("Gateway observer queue is full; dropping one audit record")


def _send(record: dict[str, Any]) -> None:
    body = json.dumps(record, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
    headers = {"Content-Type": "application/json"}
    if _INGEST_TOKEN_HEADER and _INGEST_TOKEN:
        headers[_INGEST_TOKEN_HEADER] = _INGEST_TOKEN
    request = urllib.request.Request(
        _RECORDS_URL,
        data=body,
        headers=headers,
        method="POST",
    )
    with urllib.request.urlopen(request, timeout=2.0) as response:
        response.read(1)


def _sender_loop() -> None:
    while not _STOP.is_set() or not _QUEUE.empty():
        try:
            record = _QUEUE.get(timeout=0.2)
        except queue.Empty:
            continue
        try:
            _send(record)
        except Exception as exc:
            _LOGGER.debug("Gateway record delivery failed: %s", type(exc).__name__)
        finally:
            _QUEUE.task_done()


def _start_sender() -> None:
    global _SENDER
    if not _RECORDS_URL:
        return
    with _REGISTER_LOCK:
        if _SENDER is not None and _SENDER.is_alive():
            return
        _SENDER = threading.Thread(
            target=_sender_loop,
            name="casbin-gateway-agent-monitor",
            daemon=True,
        )
        _SENDER.start()


def _shutdown() -> None:
    _STOP.set()
    sender = _SENDER
    if sender is not None and sender.is_alive():
        sender.join(timeout=1.0)


atexit.register(_shutdown)


def _on_pre_api_request(**values: Any) -> None:
    data = _metadata(
        provider=_text(values.get("provider")),
        apiMode=_text(values.get("api_mode")),
        apiCallCount=_integer(values.get("api_call_count")),
        retryCount=_integer(values.get("retry_count")),
        messageCount=_integer(values.get("message_count")),
        toolCount=_integer(values.get("tool_count")),
        approxInputTokens=_integer(values.get("approx_input_tokens")),
        requestCharCount=_integer(values.get("request_char_count")),
        maxTokens=_integer(values.get("max_tokens")),
    )
    _enqueue(_base_record("llm", "request", "attempted", values, data))


def _on_post_api_request(**values: Any) -> None:
    data = _metadata(
        provider=_text(values.get("provider")),
        apiMode=_text(values.get("api_mode")),
        apiCallCount=_integer(values.get("api_call_count")),
        retryCount=_integer(values.get("retry_count")),
        finishReason=_text(values.get("finish_reason")),
        responseModel=_text(values.get("response_model")),
        messageCount=_integer(values.get("message_count")),
        assistantContentChars=_integer(values.get("assistant_content_chars")),
        assistantToolCallCount=_integer(values.get("assistant_tool_call_count")),
        usage=_usage(values.get("usage")),
    )
    record = _base_record("llm", "response", "success", values, data)
    record["durationMs"] = _duration_ms(values.get("api_duration"), seconds=True)
    _enqueue(record)


def _error_identity(values: dict[str, Any]) -> tuple[str, Any]:
    """Split Hermes' error report into a type name and the message itself.

    ``api_request_error`` reports a structured ``error = {"type", "message"}``;
    the tool hooks report flat ``error_type`` / ``error_message`` instead. Only
    the type name and the message's length ever leave this module.
    """
    error = values.get("error")
    error_type = _text(values.get("error_type"))
    message = values.get("error_message")
    if isinstance(error, dict):
        error_type = error_type or _text(error.get("type"))
        if message is None:
            message = error.get("message")
    elif message is None and error is not None:
        message = error
    if not error_type and message is not None:
        error_type = type(message).__name__
    return error_type, message


def _on_api_request_error(**values: Any) -> None:
    error_type, error_message = _error_identity(values)
    data = _metadata(
        provider=_text(values.get("provider")),
        apiMode=_text(values.get("api_mode")),
        apiCallCount=_integer(values.get("api_call_count")),
        retryCount=_integer(values.get("retry_count")),
        maxRetries=_integer(values.get("max_retries")),
        retryable=values.get("retryable") if isinstance(values.get("retryable"), bool) else None,
        statusCode=_integer(values.get("status_code")),
        reason=_text(values.get("reason")),
        errorType=error_type,
        errorMessageLength=_length(error_message) if error_message is not None else None,
    )
    record = _base_record("llm", "error", "failure", values, data)
    if values.get("api_duration") is not None:
        record["durationMs"] = _duration_ms(values.get("api_duration"), seconds=True)
    _enqueue(record)


def _tool_record(action: str, outcome: str, values: dict[str, Any]) -> dict[str, Any]:
    tool_name = _text(values.get("tool_name"))
    server, tool = _mcp_identity(tool_name)
    event_type = "mcp" if server else "tool"
    error_type, error_message = _error_identity(values)
    data = _metadata(
        taskId=_text(values.get("task_id")),
        turnId=_text(values.get("turn_id")),
        errorType=error_type,
        errorMessageLength=_length(error_message) if error_message is not None else None,
        middlewareStageCount=_count(values.get("middleware_trace")),
    )
    if action == "call":
        data["arguments"] = _sanitize_tool_arguments(tool_name, values["args"])
    record = _base_record(event_type, action, outcome, values, data)
    record["toolName"] = tool_name
    record["toolUseId"] = _text(values.get("tool_call_id"))
    if server:
        record["mcpServer"] = server
        record["mcpTool"] = tool
    duration = _duration_ms(values.get("duration_ms"))
    if duration:
        record["durationMs"] = duration
    return {key: value for key, value in record.items() if value not in ("", None)}


def _on_pre_tool_call(**values: Any) -> None:
    _enqueue(_tool_record("call", "attempted", values))


def _on_post_tool_call(**values: Any) -> None:
    status = _text(values.get("status")).lower()
    outcome = {"ok": "success", "blocked": "denied", "error": "failure"}.get(
        status, "failure"
    )
    _enqueue(_tool_record("result", outcome, values))


def _on_session_start(**values: Any) -> None:
    data = _metadata(taskId=_text(values.get("task_id")))
    _enqueue(_base_record("session", "start", "", values, data))


def _on_session_reset(**values: Any) -> None:
    normalized = dict(values)
    normalized["session_id"] = values.get("new_session_id") or values.get("session_id")
    data = _metadata(
        oldSessionId=_text(values.get("old_session_id")),
        reason=_text(values.get("reason")),
    )
    _enqueue(_base_record("session", "reset", "success", normalized, data))


def _on_session_end(**values: Any) -> None:
    failed = bool(values.get("failed"))
    interrupted = bool(values.get("interrupted"))
    outcome = "failure" if failed or interrupted else "success"
    data = _metadata(
        taskId=_text(values.get("task_id")),
        turnId=_text(values.get("turn_id")),
        completed=bool(values.get("completed")),
        failed=failed,
        interrupted=interrupted,
        exitReason=_text(values.get("turn_exit_reason")),
    )
    _enqueue(_base_record("session", "end", outcome, values, data))


def _on_subagent_start(**values: Any) -> None:
    normalized = dict(values)
    normalized["session_id"] = values.get("parent_session_id") or values.get("session_id")
    normalized["turn_id"] = values.get("parent_turn_id") or values.get("turn_id")
    data = _metadata(
        subagentId=_text(values.get("child_subagent_id") or values.get("subagent_id")),
        childSessionId=_text(values.get("child_session_id")),
        childRole=_text(values.get("child_role")),
        goalLength=_length(values.get("child_goal")),
    )
    _enqueue(_base_record("subagent", "start", "attempted", normalized, data))


def _on_subagent_stop(**values: Any) -> None:
    normalized = dict(values)
    normalized["session_id"] = values.get("parent_session_id") or values.get("session_id")
    normalized["turn_id"] = values.get("parent_turn_id") or values.get("turn_id")
    status = _text(values.get("child_status") or values.get("status")).lower()
    outcome = "success" if status in {"ok", "success", "completed"} else "failure"
    data = _metadata(
        subagentId=_text(values.get("child_subagent_id") or values.get("subagent_id")),
        childSessionId=_text(values.get("child_session_id")),
        childRole=_text(values.get("child_role")),
        status=status,
        summaryLength=_length(values.get("child_summary")),
        toolCallCount=_count(values.get("tool_call_history")),
    )
    # "stop", not "end": the rest of Gateway's record vocabulary already spells
    # the end of a subagent that way (see agenthook's SubagentStop).
    record = _base_record("subagent", "stop", outcome, normalized, data)
    duration = values.get("duration_ms")
    if duration is None and values.get("duration") is not None:
        record["durationMs"] = _duration_ms(values.get("duration"), seconds=True)
    elif duration is not None:
        record["durationMs"] = _duration_ms(duration)
    _enqueue(record)


_HOOKS = {
    "pre_api_request": _on_pre_api_request,
    "post_api_request": _on_post_api_request,
    "api_request_error": _on_api_request_error,
    "pre_tool_call": _on_pre_tool_call,
    "post_tool_call": _on_post_tool_call,
    "on_session_start": _on_session_start,
    "on_session_end": _on_session_end,
    "on_session_reset": _on_session_reset,
    "subagent_start": _on_subagent_start,
    "subagent_stop": _on_subagent_stop,
}


def register(ctx: Any) -> None:
    """Register observational hooks. Return values are deliberately ignored."""
    _start_sender()
    for name, callback in _HOOKS.items():
        ctx.register_hook(name, callback)
