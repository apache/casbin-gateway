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

package imbridge

import (
	"context"
	"io"
	"strings"
	"time"
)

// retryDelay is how long a platform waits after a failed poll. It is short
// because a chat that has stopped answering is what somebody notices first.
const retryDelay = 5 * time.Second

// maxBody caps what is read from one API response.
const maxBody = 8 * 1024 * 1024

func waitBeforeRetry(ctx context.Context) {
	timer := time.NewTimer(retryDelay)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
	}
}

func readAll(reader io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(reader, maxBody))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// splitForChat cuts one answer into messages a platform will accept, breaking at
// a line end where there is one nearby so a paragraph is not sliced mid-word.
func splitForChat(text string, limit int) []string {
	runes := []rune(text)
	if len(runes) <= limit {
		return []string{text}
	}

	chunks := []string{}
	for len(runes) > 0 {
		if len(runes) <= limit {
			chunks = append(chunks, string(runes))
			break
		}
		cut := limit
		// Look back over the last fifth of the chunk for a line end. Further
		// back than that and the messages come out lopsided.
		if index := strings.LastIndex(string(runes[:limit]), "\n"); index > 0 {
			if candidate := len([]rune(string(runes[:limit])[:index])); candidate > limit*4/5 {
				cut = candidate
			}
		}
		chunks = append(chunks, strings.TrimRight(string(runes[:cut]), "\n"))
		runes = runes[cut:]
	}
	return chunks
}
