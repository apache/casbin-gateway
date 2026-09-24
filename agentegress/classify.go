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

package agentegress

import "strings"

const (
	KindModel     = "model"
	KindStorage   = "storage"
	KindTelemetry = "telemetry"
	// KindLocal is another process on this host, usually a proxy, which hides
	// where the bytes go next.
	KindLocal = "local"
	KindOther = "other"
)

// Model APIs are where an agent is expected to send its context, so volume
// there is not an upload.
var modelSuffixes = []string{
	"anthropic.com", "claude.ai", "openai.com", "chatgpt.com", "bigmodel.cn", "api.z.ai",
	"deepseek.com", "moonshot.cn", "moonshot.ai", "kimi.com", "dashscope.aliyuncs.com",
	"generativelanguage.googleapis.com", "aiplatform.googleapis.com", "x.ai", "mistral.ai",
	"openrouter.ai", "groq.com", "together.xyz", "fireworks.ai", "cohere.com", "cohere.ai",
	"minimaxi.com", "minimax.io", "siliconflow.cn", "volces.com", "hunyuan.cloud.tencent.com",
	"githubcopilot.com", "cursor.sh", "cursor.com", "codeium.com", "windsurf.com",
	"openai.azure.com", "bedrock-runtime.amazonaws.com", "qianfan.baidubce.com",
}

func classify(host string) string {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "" {
		return KindOther
	}
	if isStorage(host) {
		return KindStorage
	}
	if hasSuffix(host, modelSuffixes) {
		return KindModel
	}
	if isTelemetry(host) {
		return KindTelemetry
	}
	return KindOther
}

// isStorage recognises object storage upload endpoints, where a client posts a
// file directly with a credential its vendor signed.
func isStorage(host string) bool {
	switch {
	case strings.HasSuffix(host, ".aliyuncs.com"):
		return strings.HasPrefix(host, "oss-") || strings.Contains(host, ".oss-") || strings.Contains(host, ".oss.")
	case strings.HasSuffix(host, ".amazonaws.com"):
		return strings.HasPrefix(host, "s3.") || strings.HasPrefix(host, "s3-") || strings.Contains(host, ".s3.") || strings.Contains(host, ".s3-")
	case strings.HasSuffix(host, ".myqcloud.com"):
		return strings.Contains(host, ".cos.")
	case strings.HasSuffix(host, ".volces.com"), strings.HasSuffix(host, ".volccdn.com"):
		return strings.HasPrefix(host, "tos-") || strings.Contains(host, ".tos-")
	case strings.HasSuffix(host, ".myhuaweicloud.com"):
		return strings.HasPrefix(host, "obs.") || strings.Contains(host, ".obs.")
	}
	return hasSuffix(host, []string{
		"storage.googleapis.com", "blob.core.windows.net", "r2.cloudflarestorage.com",
		"qiniucs.com", "qbox.me", "bcebos.com", "ksyuncs.com", "backblazeb2.com",
		"digitaloceanspaces.com", "wasabisys.com",
	})
}

func isTelemetry(host string) bool {
	if strings.HasSuffix(host, ".log.aliyuncs.com") {
		return true
	}
	return hasSuffix(host, []string{
		"sentry.io", "mixpanel.com", "segment.io", "segment.com", "amplitude.com",
		"datadoghq.com", "datadoghq.eu", "posthog.com", "statsig.com", "statsigapi.net",
		"honeycomb.io", "googletagmanager.com", "google-analytics.com", "bugsnag.com",
		"newrelic.com", "nr-data.net", "growthbook.io", "launchdarkly.com",
	})
}

func hasSuffix(host string, suffixes []string) bool {
	for _, suffix := range suffixes {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
}
