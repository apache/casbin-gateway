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

package agentconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ParseMcpServer reads one MCP server written the way every host writes one: a
// block with a command or a URL in it. The block a tool hands over is sometimes
// wrapped in the "mcpServers" object of a whole configuration file, so a wrapper
// naming exactly one server is unwrapped and its key taken as the name.
func ParseMcpServer(raw string) (*McpRequest, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return nil, errors.New("this MCP server carries no configuration")
	}

	block := map[string]any{}
	if err := json.Unmarshal([]byte(text), &block); err != nil {
		return nil, fmt.Errorf("this is not an MCP server: %w", err)
	}

	name := ""
	if wrapped, key, ok := unwrapMcpServers(block); ok {
		block, name = wrapped, key
	}

	request := &McpRequest{
		Name:    name,
		Command: stringField(block, "command"),
		Args:    textList(block["args"]),
		Env:     textPairs(block["env"]),
		Url:     stringField(block, "url", "serverUrl", "endpoint"),
		Headers: textPairs(block["headers"]),
	}

	// A block can also declare "sse" or "streamable-http", which are both a URL
	// reached over HTTP, so what it carries decides rather than what it says.
	switch {
	case request.Command != "" && transportOf(block) != TransportHttp:
		request.Transport = TransportStdio
	case request.Url != "":
		request.Transport = TransportHttp
	default:
		return nil, errors.New("this MCP server names neither a command to run nor a URL")
	}
	return request, nil
}

// unwrapMcpServers takes the single server out of a configuration file, and
// says so. More than one is left wrapped: which of them was meant is not this
// function's to guess.
func unwrapMcpServers(block map[string]any) (map[string]any, string, bool) {
	for _, key := range []string{"mcpServers", "servers", "mcp_servers"} {
		servers, ok := block[key].(map[string]any)
		if !ok || len(servers) != 1 {
			continue
		}
		for name, server := range servers {
			if entry, ok := server.(map[string]any); ok {
				return entry, name, true
			}
		}
	}
	return block, "", false
}

func textList(value any) []string {
	values, _ := value.([]any)
	list := make([]string, 0, len(values))
	for _, held := range values {
		if text, ok := held.(string); ok {
			list = append(list, text)
		}
	}
	return list
}

// textPairs keeps the settings that are plain strings.
func textPairs(value any) map[string]string {
	block, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	pairs := map[string]string{}
	for key, held := range block {
		if text, ok := held.(string); ok {
			pairs[key] = text
		}
	}
	return pairs
}
