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

package agentmonitor

import (
	"time"

	"github.com/apache/casbin-gateway/auditutil"
	"github.com/google/uuid"
)

// Record is one observed agent behaviour. It is deliberately independent from
// Gateway's HTTP traffic record and is retained only in the live monitor store.
type Record struct {
	Id          string `json:"id"`
	CreatedTime string `json:"createdTime"`

	Agent     string `json:"agent"`
	AgentPath string `json:"agentPath,omitempty"`
	ClientIp  string `json:"clientIp,omitempty"`
	User      string `json:"user,omitempty"`

	EventType  string `json:"eventType"`
	Action     string `json:"action"`
	Outcome    string `json:"outcome,omitempty"`
	SessionKey string `json:"sessionKey,omitempty"`
	Title      string `json:"title,omitempty"`

	PromptId   string `json:"promptId,omitempty"`
	ToolUseId  string `json:"toolUseId,omitempty"`
	ToolName   string `json:"toolName,omitempty"`
	McpServer  string `json:"mcpServer,omitempty"`
	McpTool    string `json:"mcpTool,omitempty"`
	Model      string `json:"model,omitempty"`
	DurationMs int64  `json:"durationMs,omitempty"`

	Object string `json:"object,omitempty"`
	Detail string `json:"detail,omitempty"`
}

func normalizeRecord(record *Record) {
	if record.Id == "" {
		record.Id = uuid.NewString()
	}
	if record.CreatedTime == "" {
		record.CreatedTime = time.Now().Format(time.RFC3339Nano)
	}
	record.Title = auditutil.BoundString(auditutil.SanitizeString(record.Title), 512)
	record.Detail = auditutil.BoundString(auditutil.SanitizeString(record.Detail), auditutil.MaxPayloadBytes)
	if record.Object != "" {
		record.Object = auditutil.SanitizeJSON(record.Object, auditutil.MaxPayloadBytes)
	}
	if record.DurationMs < 0 {
		record.DurationMs = 0
	}
}
