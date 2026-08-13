// Copyright 2025 The casbin Authors. All Rights Reserved.
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

// Package localserver identifies programs on configured loopback ports.
package localserver

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	dialTimeout     = 300 * time.Millisecond
	requestTimeout  = 2 * time.Second
	maxResponseBody = 64 * 1024
)

// probeHosts restricts probes to the local machine.
var probeHosts = []string{"127.0.0.1", "::1"}

// Server describes how to confirm and query a local HTTP service.
type Server struct {
	Ports        []int    `json:"ports,omitempty"`
	ProbePath    string   `json:"probePath,omitempty"`
	ProbeMarkers []string `json:"probeMarkers,omitempty"`

	VersionPath   string   `json:"versionPath,omitempty"`
	VersionFields []string `json:"versionFields,omitempty"`
}

// Process is a process holding a listening TCP port.
type Process struct {
	Pid   int
	Path  string
	Owner string
}

// Confirm verifies the configured service on a loopback port.
func (s *Server) Confirm(ctx context.Context, port int) (base string, ok bool) {
	if s == nil || s.ProbePath == "" || len(s.ProbeMarkers) == 0 {
		return "", false
	}
	for _, host := range probeHosts {
		if ctx.Err() != nil {
			return "", false
		}
		address := net.JoinHostPort(host, strconv.Itoa(port))
		dialer := net.Dialer{Timeout: dialTimeout}
		conn, err := dialer.DialContext(ctx, "tcp", address)
		if err != nil {
			continue
		}
		conn.Close()
		base = "http://" + address
		if s.probe(ctx, base) {
			return base, true
		}
	}
	return "", false
}

func (s *Server) probe(ctx context.Context, base string) bool {
	answer, ok := get(ctx, base+s.ProbePath)
	if !ok {
		return false
	}

	haystack := strings.ToLower(answer.header + string(answer.body))
	for _, marker := range s.ProbeMarkers {
		if marker != "" && strings.Contains(haystack, strings.ToLower(marker)) {
			return true
		}
	}
	return false
}

// Version reads the configured JSON version field from a confirmed service.
func (s *Server) Version(ctx context.Context, base string) string {
	if s == nil || s.VersionPath == "" || len(s.VersionFields) == 0 {
		return ""
	}
	answer, ok := get(ctx, base+s.VersionPath)
	if !ok {
		return ""
	}

	var payload any
	if json.Unmarshal(answer.body, &payload) != nil {
		return ""
	}
	for _, field := range s.VersionFields {
		object, ok := payload.(map[string]any)
		if !ok {
			return ""
		}
		payload = object[field]
	}
	version, _ := payload.(string)
	return version
}

type answer struct {
	header string
	body   []byte
}

func get(ctx context.Context, url string) (answer, bool) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return answer{}, false
	}
	client := &http.Client{
		Timeout:       requestTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	response, err := client.Do(request)
	if err != nil {
		return answer{}, false
	}
	defer response.Body.Close()

	var header strings.Builder
	_ = response.Header.Write(&header)
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBody))
	if err != nil {
		return answer{}, false
	}
	return answer{header: header.String(), body: body}, true
}
