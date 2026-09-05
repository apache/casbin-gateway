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

// Package protocol translates LLM requests and answers between the wire formats
// the gateway speaks. Every format is read into one canonical form and written
// back out of it, so a client speaking any of them can be served by a provider
// speaking any other.
package protocol

import (
	"encoding/json"
	"fmt"
	"io"
)

// The wire formats. A client speaks one of them to the gateway, a provider's
// upstream serves one of them.
const (
	OpenAi    = "openai"
	Anthropic = "anthropic"
	// Responses is the OpenAI Responses API, which recent Codex versions speak
	// and which OpenAI itself and some relays serve upstream.
	Responses = "responses"
	// Gemini is Google's generateContent API, which the Gemini CLI speaks. Like
	// Responses it is a client format only: a Gemini provider is reached through
	// its OpenAI-compatible endpoint.
	Gemini = "gemini"
)

// Codec is a format a client can call the gateway in: it reads the request and
// writes the answer back.
type Codec interface {
	Name() string
	// DecodeRequest reads a client request into the canonical form.
	DecodeRequest(raw []byte) (*Request, error)
	// EncodeResponse writes a whole answer in this format.
	EncodeResponse(response *Response) ([]byte, error)
	// NewStreamWriter writes an answer out as the event stream this format
	// defines. flush may be nil.
	NewStreamWriter(writer io.Writer, flush func(), model string) StreamWriter
	// EncodeError writes an error body in the shape this format reports errors
	// in, which is what a client parses a failure out of.
	EncodeError(kind string, message string) []byte
}

// Upstream is a format a provider's own API serves, so a request can also be
// written in it and an answer read out of it.
type Upstream interface {
	Codec
	// Endpoint is the path under the provider base URL that answers.
	Endpoint() string
	// EncodeRequest writes the canonical request out for such an upstream.
	EncodeRequest(request *Request) ([]byte, error)
	// DecodeResponse reads a whole upstream answer.
	DecodeResponse(raw []byte) (*Response, error)
	// DecodeStream reads a streamed upstream answer, calling fn on every event
	// it carries until fn returns false.
	DecodeStream(reader io.Reader, fn func(Event) bool) error
}

// StreamWriter writes canonical events out as one event stream. Open and Close
// carry whatever the format wraps the events in.
type StreamWriter interface {
	Open()
	Write(event Event)
	Close()
}

var (
	codecs    = map[string]Codec{}
	upstreams = map[string]Upstream{}
)

func register(codec Codec) {
	codecs[codec.Name()] = codec
	if upstream, ok := codec.(Upstream); ok {
		upstreams[codec.Name()] = upstream
	}
}

// Of is the codec for a client format, nil for one the gateway does not speak.
func Of(name string) Codec {
	return codecs[name]
}

// IsUpstream reports whether a provider can be talked to in this format.
func IsUpstream(name string) bool {
	_, ok := upstreams[name]
	return ok
}

// UpstreamOf is the codec for a provider format.
func UpstreamOf(name string) (Upstream, error) {
	upstream, ok := upstreams[name]
	if !ok {
		return nil, fmt.Errorf("the gateway cannot speak the %s API to a provider", name)
	}
	return upstream, nil
}

// errorBody is how both formats report a failure: OpenAI sends the detail bare,
// Anthropic wraps it in a typed envelope, and the detail itself is the same.
type errorBody struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// ReadError is the failure an upstream error body describes, so it can be
// written back out in the format the client reads errors in. An unreadable body
// keeps the fallback message.
func ReadError(raw []byte, fallback string) (kind string, message string) {
	var body errorBody
	if err := json.Unmarshal(raw, &body); err != nil || body.Error.Message == "" {
		return "server_error", fallback
	}
	kind = body.Error.Type
	if kind == "" {
		kind = "server_error"
	}
	return kind, body.Error.Message
}
