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

// Package auditutil removes credentials from agent-monitoring payloads.
package auditutil

import (
	"encoding/json"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"
)

// MaxPayloadBytes is the maximum payload retained for one monitored event.
const MaxPayloadBytes = 64 * 1024

var (
	credentialPattern = regexp.MustCompile(`(?i)\b(?:sk-(?:ant-|proj-)?[a-z0-9_-]{12,}|gh[pousr]_[a-z0-9]{20,}|github_pat_[a-z0-9_]{20,}|AKIA[0-9A-Z]{16}|AIza[0-9a-z_-]{30,}|xox[baprs]-[0-9a-z-]{12,}|eyJ[a-z0-9_-]{10,}\.[a-z0-9_-]{10,}\.[a-z0-9_-]{10,})\b`)
	bearerPattern     = regexp.MustCompile(`(?i)(bearer\s+)[a-z0-9._~+/=-]{12,}`)
	privateKeyPattern = regexp.MustCompile(`(?s)-----BEGIN [^-\n]*PRIVATE KEY-----.*?-----END [^-\n]*PRIVATE KEY-----`)
)

// SanitizeValue recursively redacts credential-bearing fields and credentials
// embedded in string values.
func SanitizeValue(key string, value any) any {
	if SensitiveKey(key) {
		return "[REDACTED]"
	}
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for childKey, child := range typed {
			result[childKey] = SanitizeValue(childKey, child)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			result[index] = SanitizeValue("", child)
		}
		return result
	case string:
		return SanitizeString(typed)
	default:
		return value
	}
}

// SensitiveKey recognizes common credential-bearing field names after
// punctuation and case normalization.
func SensitiveKey(key string) bool {
	normalized := strings.ToLower(key)
	normalized = strings.NewReplacer("_", "", "-", "", ".", "").Replace(normalized)
	if normalized == "token" || normalized == "accesstoken" || normalized == "refreshtoken" || normalized == "idtoken" {
		return true
	}
	for _, marker := range []string{"secret", "token", "password", "passwd", "credential", "privatekey", "apikey", "accesskey", "authorization", "cookie"} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

// SanitizeString redacts credential formats that may appear in errors,
// commands or other free-form text.
func SanitizeString(value string) string {
	value = privateKeyPattern.ReplaceAllString(value, "[REDACTED PRIVATE KEY]")
	value = bearerPattern.ReplaceAllString(value, "${1}[REDACTED]")
	return credentialPattern.ReplaceAllString(value, "[REDACTED]")
}

// SanitizeJSON sanitizes a JSON payload before retaining it. Invalid JSON is
// still useful as a diagnostic, so it is treated as text rather than dropped.
func SanitizeJSON(value string, maximum int) string {
	var decoded any
	if json.Unmarshal([]byte(value), &decoded) == nil {
		return EncodeBoundedJSON(decoded, maximum)
	}
	return BoundString(SanitizeString(value), maximum)
}

// ParseMcpTool splits the canonical MCP tool name mcp__<server>__<tool>.
func ParseMcpTool(name, prefix string) (string, string, bool) {
	if prefix == "" || !strings.HasPrefix(name, prefix) {
		return "", "", false
	}
	parts := strings.SplitN(strings.TrimPrefix(name, prefix), "__", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// SanitizeToolInput hides content written to sensitive files while retaining
// enough operation metadata to diagnose the event.
func SanitizeToolInput(toolName string, input any) any {
	sanitized := SanitizeValue("", input)
	if !isSensitiveWrite(toolName) || !hasSensitivePath(input) {
		return sanitized
	}
	inputMap, ok := sanitized.(map[string]any)
	if !ok {
		return sanitized
	}
	for key := range inputMap {
		normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(key))
		switch normalized {
		case "content", "oldstring", "newstring":
			inputMap[key] = "[REDACTED: sensitive file content]"
		}
	}
	return inputMap
}

// EncodeBoundedJSON serializes a sanitized value, replacing oversized values
// with a small metadata envelope rather than retaining the full payload.
func EncodeBoundedJSON(value any, maximum int) string {
	if maximum <= 0 {
		maximum = MaxPayloadBytes
	}
	encoded, err := json.Marshal(SanitizeValue("", value))
	if err != nil {
		return ""
	}
	if len(encoded) <= maximum {
		return string(encoded)
	}
	previewLimit := maximum / 3
	preview := strings.ToValidUTF8(string(encoded[:previewLimit]), "")
	truncated, err := json.Marshal(map[string]any{
		"truncated":     true,
		"originalBytes": len(encoded),
		"preview":       preview,
	})
	if err != nil {
		return ""
	}
	return string(truncated)
}

// BoundString caps text at maximum bytes without splitting UTF-8 runes.
func BoundString(value string, maximum int) string {
	if maximum <= 0 {
		maximum = MaxPayloadBytes
	}
	if len(value) <= maximum {
		return value
	}
	limit := maximum - len("...[truncated]")
	for limit > 0 && !utf8.RuneStart(value[limit]) {
		limit--
	}
	return value[:limit] + "...[truncated]"
}

func isSensitiveWrite(toolName string) bool {
	normalized := strings.ToLower(toolName)
	return normalized == "write" || normalized == "edit" || normalized == "write_file" || normalized == "edit_file" ||
		strings.HasSuffix(normalized, "__write_file") || strings.HasSuffix(normalized, "__edit_file")
}

func hasSensitivePath(input any) bool {
	inputMap, ok := input.(map[string]any)
	if !ok {
		return false
	}
	filePath := stringValue(inputMap["file_path"])
	if filePath == "" {
		filePath = stringValue(inputMap["path"])
	}
	return isSensitivePath(filePath)
}

func isSensitivePath(filePath string) bool {
	if filePath == "" {
		return false
	}
	normalized := strings.ToLower(strings.ReplaceAll(filePath, `\`, "/"))
	base := path.Base(normalized)
	if strings.HasPrefix(base, ".env") && base != ".env.example" && base != ".env.sample" && base != ".env.template" {
		return true
	}
	if strings.HasPrefix(normalized, ".ssh/") || strings.Contains(normalized, "/.ssh/") || normalized == ".aws/credentials" || strings.HasSuffix(normalized, "/.aws/credentials") {
		return true
	}
	if base == ".npmrc" || base == ".pypirc" || base == "credentials" || base == "id_rsa" || base == "id_ed25519" {
		return true
	}
	return strings.HasSuffix(base, ".pem") || strings.HasSuffix(base, ".key")
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}
