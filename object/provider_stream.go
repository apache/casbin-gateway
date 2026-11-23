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

package object

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/apache/casbin-gateway/auditutil"
	"github.com/apache/casbin-gateway/protocol"
	"github.com/apache/casbin-gateway/proxy"
)

type ProviderStreamResult struct {
	Success     bool   `json:"success"`
	Protocol    string `json:"protocol"`
	Model       string `json:"model"`
	StatusCode  int    `json:"statusCode,omitempty"`
	FirstTextMs *int64 `json:"firstTextMs,omitempty"`
	Message     string `json:"message,omitempty"`
}

// CheckProviderStream asks the first configured model for a short streamed answer.
func CheckProviderStream(ctx context.Context, provider *Provider, name string) (result ProviderStreamResult) {
	result.Protocol = name
	defer func() {
		if provider.ApiKey != "" {
			result.Message = strings.ReplaceAll(result.Message, provider.ApiKey, "[redacted]")
		}
		result.Message = auditutil.BoundString(result.Message, probeErrorChars)
	}()
	if err := checkProviderTarget(provider); err != nil {
		result.Message = err.Error()
		return
	}
	if ServesAnyModel(provider) || strings.TrimSpace(provider.ApiKey) == "" {
		result.Message = "the stream check needs an API key saved on the provider"
		return
	}
	for _, model := range provider.Models {
		if result.Model = strings.TrimSpace(model); result.Model != "" {
			break
		}
	}
	if result.Model == "" {
		result.Message = "the stream check needs a model configured on the provider"
		return
	}

	upstream, err := protocol.UpstreamOf(name)
	if err != nil {
		result.Message = err.Error()
		return
	}
	target, err := BuildProviderUrl(provider.BaseUrl, name, upstream.Endpoint())
	if err != nil {
		result.Message = err.Error()
		return
	}
	maxTokens := 64
	body, err := upstream.EncodeRequest(&protocol.Request{
		Model: result.Model, Stream: true, MaxTokens: &maxTokens,
		Messages: []protocol.Message{{Role: "user", Content: []protocol.Content{{Kind: protocol.KindText, Text: probeStreamPrompt}}}},
	})
	if err != nil {
		result.Message = err.Error()
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	result = runProviderStreamCheck(ctx, provider, upstream, target, body, result)
	if name == ProtocolOpenAi && result.StatusCode == http.StatusBadRequest && strings.Contains(strings.ToLower(result.Message), "max_tokens") {
		body = bytes.Replace(body, []byte(`"max_tokens":`), []byte(`"max_completion_tokens":`), 1)
		result = runProviderStreamCheck(ctx, provider, upstream, target, body, ProviderStreamResult{Protocol: name, Model: result.Model})
	}
	return
}

func runProviderStreamCheck(ctx context.Context, provider *Provider, upstream protocol.Upstream, target string, body []byte, result ProviderStreamResult) ProviderStreamResult {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		result.Message = err.Error()
		return result
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	if upstream.Name() == ProtocolAnthropic {
		request.Header.Set("Anthropic-Version", AnthropicVersion)
	}
	SetProviderAuth(request.Header, provider)
	client := &http.Client{
		Transport:     proxy.Transport(),
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	start := time.Now()
	response, err := client.Do(request)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		result.Message = err.Error()
		return result
	}
	defer response.Body.Close()
	result.StatusCode = response.StatusCode

	mediaType, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if response.StatusCode < 200 || response.StatusCode >= 300 || mediaType != "text/event-stream" {
		fallback := response.Status
		if response.StatusCode >= 200 && response.StatusCode < 300 {
			fallback = "the upstream did not return an event stream"
		}
		raw, _ := io.ReadAll(io.LimitReader(response.Body, maxProbeBody))
		_, result.Message = protocol.ReadError(raw, fallback)
		return result
	}

	bounded := &io.LimitedReader{R: response.Body, N: maxProbeBody + 1}
	err = upstream.DecodeStream(bounded, func(event protocol.Event) bool {
		if event.Kind == protocol.EventFailure && event.Failure != nil {
			result.Message = event.Failure.Message
			if result.Message == "" {
				result.Message = "the upstream stream failed"
			}
			return false
		}
		if result.FirstTextMs == nil && event.Kind == protocol.EventText && strings.TrimSpace(event.Text) != "" {
			elapsed := time.Since(start).Milliseconds()
			result.FirstTextMs = &elapsed
		}
		return true
	})
	if result.Message != "" {
		return result
	}
	switch {
	case err != nil:
		result.Message = err.Error()
	case bounded.N == 0:
		result.Message = "the upstream stream exceeded the check's size limit"
	case result.FirstTextMs == nil:
		result.Message = "the upstream stream returned no text"
	default:
		result.Success = true
	}
	return result
}
