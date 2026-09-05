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

// This file speaks the OpenAI Responses API, the only wire format recent Codex
// versions know. It works both ways: as a client format, and as the upstream of
// a provider that answers on /responses rather than on chat completions.

package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

type responsesCodec struct{}

func init() {
	register(responsesCodec{})
}

func (responsesCodec) Name() string { return Responses }

func (responsesCodec) Endpoint() string { return "/responses" }

// ---------------------------------------------------------------------------
// Request
// ---------------------------------------------------------------------------

// responsesRequest is the part of a Responses body that has a canonical
// counterpart. The rest (store, include, previous_response_id) describes a
// conversation this API keeps for itself, which no upstream here holds.
type responsesRequest struct {
	Model             string          `json:"model"`
	Stream            bool            `json:"stream"`
	Instructions      string          `json:"instructions"`
	Input             json.RawMessage `json:"input"`
	Tools             []responsesTool `json:"tools"`
	ToolChoice        json.RawMessage `json:"tool_choice"`
	ParallelToolCalls *bool           `json:"parallel_tool_calls"`
	Temperature       *float64        `json:"temperature"`
	TopP              *float64        `json:"top_p"`
	MaxOutputTokens   *int            `json:"max_output_tokens"`
	Text              *struct {
		Format json.RawMessage `json:"format"`
	} `json:"text"`
	Reasoning *struct {
		Effort string `json:"effort"`
	} `json:"reasoning"`
}

type responsesTool struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// responsesInput is one element of the input array. Its fields are a union over
// the item types, which is how the API itself is shaped.
type responsesInput struct {
	Type      string          `json:"type"`
	Role      string          `json:"role"`
	Content   json.RawMessage `json:"content"`
	Name      string          `json:"name"`
	Arguments string          `json:"arguments"`
	CallId    string          `json:"call_id"`
	Output    json.RawMessage `json:"output"`
}

type responsesPart struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	ImageUrl string `json:"image_url"`
}

func (responsesCodec) DecodeRequest(raw []byte) (*Request, error) {
	var body responsesRequest
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, errors.New("invalid request body")
	}

	items, err := responsesItemsOf(body.Input)
	if err != nil {
		return nil, errors.New("invalid input")
	}

	request := &Request{
		Model:             body.Model,
		Stream:            body.Stream,
		ToolChoice:        responsesToolChoiceOf(body.ToolChoice),
		ParallelToolCalls: body.ParallelToolCalls,
		Temperature:       body.Temperature,
		TopP:              body.TopP,
		MaxTokens:         body.MaxOutputTokens,
	}
	if body.Instructions != "" {
		request.System = []string{body.Instructions}
	}
	if body.Reasoning != nil && body.Reasoning.Effort != "" {
		request.Reasoning = &Reasoning{Effort: body.Reasoning.Effort}
	}
	if body.Text != nil {
		request.Format = responsesFormatOf(body.Text.Format)
	}
	for _, tool := range body.Tools {
		// Only plain function tools cross over: the hosted ones (web_search,
		// local_shell) are run by OpenAI itself, and no upstream here has them.
		if tool.Type != "function" || tool.Name == "" {
			continue
		}
		request.Tools = append(request.Tools, Tool{
			Name: tool.Name, Description: tool.Description, Parameters: tool.Parameters,
		})
	}

	for _, item := range items {
		request.Messages = appendResponsesItem(request.Messages, item)
	}
	if len(request.Messages) == 0 {
		return nil, errors.New("input is required")
	}
	return request, nil
}

// responsesItemsOf reads the input field, which is either a bare prompt or the
// conversation so far.
func responsesItemsOf(raw json.RawMessage) ([]responsesInput, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	var prompt string
	if err := json.Unmarshal(raw, &prompt); err == nil {
		return []responsesInput{{Type: "message", Role: RoleUser, Content: rawString(prompt)}}, nil
	}

	var items []responsesInput
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func appendResponsesItem(messages []Message, item responsesInput) []Message {
	switch {
	case item.Type == "function_call":
		use := Content{Kind: KindToolUse, ToolUse: &ToolUse{
			Id: item.CallId, Name: item.Name, Arguments: emptyAsObject(item.Arguments),
		}}
		// Calls the model made in one turn belong to one assistant message,
		// which is how they were sent to the client in the first place.
		if last := len(messages) - 1; last >= 0 && messages[last].Role == RoleAssistant {
			messages[last].Content = append(messages[last].Content, use)
			return messages
		}
		return append(messages, Message{Role: RoleAssistant, Content: []Content{use}})

	case item.Type == "function_call_output":
		return append(messages, Message{Role: RoleUser, Content: []Content{{
			Kind:       KindToolResult,
			ToolResult: &ToolResult{Id: item.CallId, Text: responsesOutputText(item.Output)},
		}}})

	case item.Type == "message" || (item.Type == "" && item.Role != ""):
		role := RoleUser
		if item.Role == RoleAssistant {
			role = RoleAssistant
		}
		content := responsesContentOf(item.Content)
		if len(content) == 0 {
			return messages
		}
		return append(messages, Message{Role: role, Content: content})
	}

	// A developer message is a system prompt sent as an item; anything else,
	// reasoning above all, describes a turn no other API would take back.
	if item.Role == "developer" || item.Role == "system" {
		return append(messages, Message{Role: RoleUser, Content: responsesContentOf(item.Content)})
	}
	return messages
}

func responsesContentOf(raw json.RawMessage) []Content {
	if len(raw) == 0 {
		return nil
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		if text == "" {
			return nil
		}
		return TextContent(text)
	}

	var parts []responsesPart
	if err := json.Unmarshal(raw, &parts); err != nil {
		return nil
	}

	content := []Content{}
	for _, part := range parts {
		switch {
		case part.ImageUrl != "":
			content = append(content, Content{Kind: KindImage, Image: imageOfUrl(part.ImageUrl)})
		case part.Text != "":
			content = append(content, Content{Kind: KindText, Text: part.Text})
		}
	}
	return content
}

// responsesOutputText is the text of a function_call_output, which clients send
// either as a string or as the object their tool runner produced.
func responsesOutputText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}

	var wrapper struct {
		Content json.RawMessage `json:"content"`
		Output  string          `json:"output"`
	}
	if err := json.Unmarshal(raw, &wrapper); err == nil {
		inner := Message{Content: responsesContentOf(wrapper.Content)}
		if value := inner.Text(); value != "" {
			return value
		}
		if wrapper.Output != "" {
			return wrapper.Output
		}
	}
	return string(raw)
}

func responsesToolChoiceOf(raw json.RawMessage) *ToolChoice {
	if len(raw) == 0 {
		return nil
	}

	var mode string
	if err := json.Unmarshal(raw, &mode); err == nil {
		switch mode {
		case ChoiceAuto, ChoiceNone, ChoiceRequired:
			return &ToolChoice{Mode: mode}
		}
		return nil
	}

	var choice struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &choice); err == nil && choice.Type == "function" && choice.Name != "" {
		return &ToolChoice{Mode: ChoiceTool, Name: choice.Name}
	}
	return nil
}

// responsesFormatOf reads text.format, which is the response format one level
// less deep than chat completions spells it.
func responsesFormatOf(raw json.RawMessage) *Format {
	if len(raw) == 0 {
		return nil
	}

	var format struct {
		Type   string          `json:"type"`
		Name   string          `json:"name"`
		Schema json.RawMessage `json:"schema"`
		Strict *bool           `json:"strict"`
	}
	if err := json.Unmarshal(raw, &format); err != nil {
		return nil
	}
	switch format.Type {
	case "json_object", "json_schema":
		return &Format{Kind: format.Type, Name: format.Name, Schema: format.Schema, Strict: format.Strict}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Response
// ---------------------------------------------------------------------------

// responsesItem is one item of the answer this API reports as a list: the
// assistant message, or one function call.
type responsesItem struct {
	id    string
	index int
	// callId, name and arguments are what a function call item carries.
	callId    string
	name      string
	arguments strings.Builder
	text      strings.Builder
}

func (item *responsesItem) message(status string) map[string]any {
	return map[string]any{
		"type": "message", "id": item.id, "status": status, "role": RoleAssistant,
		"content": []any{map[string]any{
			"type": "output_text", "text": item.text.String(), "annotations": []any{},
		}},
	}
}

func (item *responsesItem) call(status string) map[string]any {
	return map[string]any{
		"type": "function_call", "id": item.id, "status": status,
		"call_id": emptyAs(item.callId, item.id), "name": item.name,
		"arguments": emptyAsObject(item.arguments.String()),
	}
}

// responsesAnswer builds the answer as this API reports it. The stream writer
// and the whole-body encoder share it, so a client reading the events and one
// reading the body are told the same thing.
type responsesAnswer struct {
	token   string
	id      string
	model   string
	created int64

	nextIndex int
	message   *responsesItem
	// thinkingId names the reasoning item the thinking deltas belong to. The
	// item itself is not part of the output: no other API signs one, and this
	// one only takes back what it produced.
	thinkingId string
	calls      map[int]*responsesItem
	callOrder  []int

	usage   Usage
	failure *Failure
}

func newResponsesAnswer(model string) *responsesAnswer {
	now := time.Now()
	token := fmt.Sprintf("%d", now.UnixNano())
	return &responsesAnswer{
		token:   token,
		id:      "resp_" + token,
		model:   model,
		created: now.Unix(),
		calls:   map[int]*responsesItem{},
	}
}

// startMessage opens the assistant message item, which every answer carrying
// text has exactly one of.
func (answer *responsesAnswer) startMessage() (*responsesItem, bool) {
	if answer.message != nil {
		return answer.message, false
	}
	answer.message = &responsesItem{
		id:    fmt.Sprintf("msg_%s_%d", answer.token, answer.nextIndex),
		index: answer.nextIndex,
	}
	answer.nextIndex++
	return answer.message, true
}

// startCall opens the item one tool call is assembled in.
func (answer *responsesAnswer) startCall(index int) (*responsesItem, bool) {
	if call, ok := answer.calls[index]; ok {
		return call, false
	}
	call := &responsesItem{
		id:    fmt.Sprintf("fc_%s_%d", answer.token, answer.nextIndex),
		index: answer.nextIndex,
	}
	answer.nextIndex++
	answer.calls[index] = call
	answer.callOrder = append(answer.callOrder, index)
	return call, true
}

func (answer *responsesAnswer) reasoningId() string {
	if answer.thinkingId == "" {
		answer.thinkingId = "rs_" + answer.token
	}
	return answer.thinkingId
}

// output is the item list a finished answer carries, in the order the items
// were started.
func (answer *responsesAnswer) output() []any {
	slots := make([]any, answer.nextIndex)
	if answer.message != nil {
		slots[answer.message.index] = answer.message.message("completed")
	}
	for _, index := range answer.callOrder {
		call := answer.calls[index]
		slots[call.index] = call.call("completed")
	}

	items := []any{}
	for _, item := range slots {
		if item != nil {
			items = append(items, item)
		}
	}
	return items
}

func (answer *responsesAnswer) response(status string, output []any) map[string]any {
	response := map[string]any{
		"id": answer.id, "object": "response", "created_at": answer.created,
		"status": status, "model": answer.model, "output": output,
	}
	if !answer.usage.IsZero() {
		response["usage"] = responsesUsageOf(answer.usage)
	}
	if answer.failure != nil {
		response["error"] = map[string]any{
			"code": emptyAs(answer.failure.Kind, "server_error"), "message": answer.failure.Message,
		}
	}
	return response
}

// add records one event, and reports the item it landed in so a stream writer
// can send the same piece on.
func (answer *responsesAnswer) add(event Event) (*responsesItem, bool) {
	if event.Model != "" {
		answer.model = event.Model
	}

	switch event.Kind {
	case EventText:
		item, opened := answer.startMessage()
		item.text.WriteString(event.Text)
		return item, opened
	case EventToolUse:
		if event.Tool == nil {
			return nil, false
		}
		call, opened := answer.startCall(event.Tool.Index)
		if event.Tool.Id != "" {
			call.callId = event.Tool.Id
		}
		call.name += event.Tool.Name
		call.arguments.WriteString(event.Tool.Arguments)
		return call, opened
	case EventDone:
		if event.Usage != nil {
			answer.usage = *event.Usage
		}
	case EventFailure:
		answer.failure = event.Failure
	}
	return nil, false
}

func (responsesCodec) EncodeResponse(response *Response) ([]byte, error) {
	answer := newResponsesAnswer(response.Model)
	answer.usage = response.Usage
	answer.failure = response.Failure
	WriteStream(&responsesCollector{answer: answer}, response)

	status := "completed"
	if response.Failure != nil {
		status = "failed"
	}
	return json.Marshal(answer.response(status, answer.output()))
}

// responsesCollector fills an answer in without sending anything, for the
// client that asked for a body rather than a stream.
type responsesCollector struct {
	answer *responsesAnswer
}

func (collector *responsesCollector) Open() {}

func (collector *responsesCollector) Write(event Event) { collector.answer.add(event) }

func (collector *responsesCollector) Close() {}

// ---------------------------------------------------------------------------
// Stream
// ---------------------------------------------------------------------------

// responsesWriter turns canonical events into the Responses events a client
// such as Codex expects.
type responsesWriter struct {
	events   *eventWriter
	answer   *responsesAnswer
	sequence int
}

func (responsesCodec) NewStreamWriter(writer io.Writer, flush func(), model string) StreamWriter {
	return &responsesWriter{
		events: &eventWriter{writer: writer, flush: flush},
		answer: newResponsesAnswer(model),
	}
}

func (writer *responsesWriter) emit(name string, payload map[string]any) {
	payload["type"] = name
	payload["sequence_number"] = writer.sequence
	writer.sequence++
	writer.events.send(name, payload)
}

func (writer *responsesWriter) Open() {
	writer.emit("response.created", map[string]any{
		"response": writer.answer.response("in_progress", []any{}),
	})
}

func (writer *responsesWriter) Write(event Event) {
	if event.Kind == EventThinking {
		if event.Text == "" {
			return
		}
		writer.emit("response.reasoning_summary_text.delta", map[string]any{
			"item_id": writer.answer.reasoningId(), "output_index": 0,
			"summary_index": 0, "delta": event.Text,
		})
		return
	}

	item, opened := writer.answer.add(event)
	if item == nil {
		return
	}

	switch event.Kind {
	case EventText:
		if opened {
			writer.emit("response.output_item.added", map[string]any{
				"output_index": item.index,
				"item": map[string]any{
					"type": "message", "id": item.id, "status": "in_progress",
					"role": RoleAssistant, "content": []any{},
				},
			})
		}
		writer.emit("response.output_text.delta", map[string]any{
			"item_id": item.id, "output_index": item.index, "content_index": 0, "delta": event.Text,
		})
	case EventToolUse:
		if opened {
			writer.emit("response.output_item.added", map[string]any{
				"output_index": item.index,
				"item": map[string]any{
					"type": "function_call", "id": item.id, "status": "in_progress",
					"call_id": emptyAs(item.callId, item.id), "name": item.name, "arguments": "",
				},
			})
		}
		if event.Tool.Arguments != "" {
			writer.emit("response.function_call_arguments.delta", map[string]any{
				"item_id": item.id, "output_index": item.index, "delta": event.Tool.Arguments,
			})
		}
	}
}

func (writer *responsesWriter) Close() {
	answer := writer.answer
	if message := answer.message; message != nil {
		writer.emit("response.output_text.done", map[string]any{
			"item_id": message.id, "output_index": message.index,
			"content_index": 0, "text": message.text.String(),
		})
		writer.emit("response.output_item.done", map[string]any{
			"output_index": message.index, "item": message.message("completed"),
		})
	}
	for _, index := range answer.callOrder {
		call := answer.calls[index]
		writer.emit("response.function_call_arguments.done", map[string]any{
			"item_id": call.id, "output_index": call.index,
			"arguments": emptyAsObject(call.arguments.String()),
		})
		writer.emit("response.output_item.done", map[string]any{
			"output_index": call.index, "item": call.call("completed"),
		})
	}

	if answer.failure != nil {
		writer.emit("response.failed", map[string]any{
			"response": answer.response("failed", answer.output()),
		})
		return
	}
	writer.emit("response.completed", map[string]any{
		"response": answer.response("completed", answer.output()),
	})
}

func (responsesCodec) EncodeError(kind string, message string) []byte {
	return openAiCodec{}.EncodeError(kind, message)
}

// ---------------------------------------------------------------------------
// Upstream
// ---------------------------------------------------------------------------

// responsesBody is the request written for an upstream serving this API. store
// is false because the gateway never sends a previous_response_id, so a copy
// kept upstream would never be read.
type responsesBody struct {
	Model             string          `json:"model"`
	Stream            bool            `json:"stream,omitempty"`
	Store             bool            `json:"store"`
	Instructions      string          `json:"instructions,omitempty"`
	Input             []any           `json:"input"`
	Tools             []responsesTool `json:"tools,omitempty"`
	ToolChoice        json.RawMessage `json:"tool_choice,omitempty"`
	ParallelToolCalls *bool           `json:"parallel_tool_calls,omitempty"`
	Temperature       *float64        `json:"temperature,omitempty"`
	TopP              *float64        `json:"top_p,omitempty"`
	MaxOutputTokens   *int            `json:"max_output_tokens,omitempty"`
	Text              json.RawMessage `json:"text,omitempty"`
	Reasoning         json.RawMessage `json:"reasoning,omitempty"`
}

func (responsesCodec) EncodeRequest(request *Request) ([]byte, error) {
	body := responsesBody{
		Model:             request.Model,
		Stream:            request.Stream,
		Instructions:      strings.Join(request.System, "\n\n"),
		Input:             []any{},
		ParallelToolCalls: request.ParallelToolCalls,
		Temperature:       request.Temperature,
		TopP:              request.TopP,
		MaxOutputTokens:   request.MaxTokens,
	}
	for _, message := range request.Messages {
		body.Input = append(body.Input, responsesInputOf(message)...)
	}
	if len(body.Input) == 0 {
		return nil, errors.New("the request carries no message")
	}

	for _, tool := range request.Tools {
		parameters := tool.Parameters
		if len(parameters) == 0 {
			parameters = rawValue(map[string]any{"type": "object", "properties": map[string]any{}})
		}
		body.Tools = append(body.Tools, responsesTool{
			Type: "function", Name: tool.Name, Description: tool.Description, Parameters: parameters,
		})
	}
	if choice := request.ToolChoice; choice != nil {
		switch choice.Mode {
		case ChoiceTool:
			body.ToolChoice = rawValue(map[string]any{"type": "function", "name": choice.Name})
		case ChoiceAuto, ChoiceNone, ChoiceRequired:
			body.ToolChoice = rawString(choice.Mode)
		}
	}
	if format := request.Format; format != nil && format.Kind != "" {
		// This API keeps the name and the schema beside the type rather than
		// one level down, as chat completions does.
		written := map[string]any{"type": format.Kind}
		if format.Kind == "json_schema" {
			written["name"] = emptyAs(format.Name, "response")
			written["schema"] = format.Schema
			if format.Strict != nil {
				written["strict"] = *format.Strict
			}
		}
		body.Text = rawValue(map[string]any{"format": written})
	}
	if effort := request.Reasoning.EffortOf(); effort != "" {
		body.Reasoning = rawValue(map[string]any{"effort": effort})
	}
	return json.Marshal(body)
}

// responsesInputOf writes one canonical message out. A tool call and a tool
// result are items of their own here rather than content of a message, so a
// message carrying either turns into several.
func responsesInputOf(message Message) []any {
	items := []any{}
	parts := []any{}
	partType := "input_text"
	if message.Role == RoleAssistant {
		partType = "output_text"
	}

	flush := func() {
		if len(parts) == 0 {
			return
		}
		items = append(items, map[string]any{
			"type": "message", "role": message.Role, "content": parts,
		})
		parts = []any{}
	}

	for _, content := range message.Content {
		switch content.Kind {
		case KindText:
			if content.Text == "" {
				continue
			}
			parts = append(parts, map[string]any{"type": partType, "text": content.Text})
		case KindImage:
			if content.Image == nil {
				continue
			}
			parts = append(parts, map[string]any{"type": "input_image", "image_url": content.Image.Link()})
		case KindToolUse:
			if content.ToolUse == nil {
				continue
			}
			flush()
			items = append(items, map[string]any{
				"type": "function_call", "call_id": content.ToolUse.Id,
				"name": content.ToolUse.Name, "arguments": emptyAsObject(content.ToolUse.Arguments),
			})
		case KindToolResult:
			if content.ToolResult == nil {
				continue
			}
			flush()
			items = append(items, map[string]any{
				"type": "function_call_output", "call_id": content.ToolResult.Id,
				"output": content.ToolResult.Text,
			})
		}
		// A thinking block is left out: this API only takes back reasoning
		// items of its own, which carry an id no other upstream signed.
	}

	flush()
	return items
}

type responsesResult struct {
	Id                string                `json:"id"`
	Model             string                `json:"model"`
	Status            string                `json:"status"`
	Output            []responsesOutputItem `json:"output"`
	Usage             *responsesUsage       `json:"usage"`
	IncompleteDetails *struct {
		Reason string `json:"reason"`
	} `json:"incomplete_details"`
	Error *responsesFailure `json:"error"`
}

type responsesFailure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (failure *responsesFailure) canonical() *Failure {
	if failure == nil || failure.Message == "" {
		return nil
	}
	return &Failure{Kind: emptyAs(failure.Code, "server_error"), Message: failure.Message}
}

// responsesOutputItem is one item of the output list: the assistant message,
// the reasoning, or one function call.
type responsesOutputItem struct {
	Type      string                `json:"type"`
	Role      string                `json:"role"`
	Content   []responsesOutputPart `json:"content"`
	Summary   []responsesOutputPart `json:"summary"`
	Id        string                `json:"id"`
	CallId    string                `json:"call_id"`
	Name      string                `json:"name"`
	Arguments string                `json:"arguments"`
}

type responsesOutputPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type responsesUsage struct {
	InputTokens        int `json:"input_tokens"`
	OutputTokens       int `json:"output_tokens"`
	TotalTokens        int `json:"total_tokens"`
	InputTokensDetails *struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"input_tokens_details"`
	OutputTokensDetails *struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"output_tokens_details"`
}

func (usage *responsesUsage) canonical() Usage {
	canonical := Usage{InputTokens: usage.InputTokens, OutputTokens: usage.OutputTokens}
	if usage.InputTokensDetails != nil {
		canonical.CacheReadTokens = usage.InputTokensDetails.CachedTokens
	}
	if usage.OutputTokensDetails != nil {
		canonical.ReasoningTokens = usage.OutputTokensDetails.ReasoningTokens
	}
	return canonical
}

// responsesUsageOf writes the counters back out. This format counts the cached
// part inside the input total, which is where the canonical form keeps it too.
func responsesUsageOf(usage Usage) map[string]any {
	input := usage.InputTokens + usage.CacheWriteTokens
	written := map[string]any{
		"input_tokens":  input,
		"output_tokens": usage.OutputTokens,
		"total_tokens":  input + usage.OutputTokens,
	}
	if usage.CacheReadTokens > 0 {
		written["input_tokens_details"] = map[string]any{"cached_tokens": usage.CacheReadTokens}
	}
	if usage.ReasoningTokens > 0 {
		written["output_tokens_details"] = map[string]any{"reasoning_tokens": usage.ReasoningTokens}
	}
	return written
}

func (responsesCodec) DecodeResponse(raw []byte) (*Response, error) {
	var body responsesResult
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, errors.New("the upstream answered with an unreadable body")
	}

	response := &Response{Id: body.Id, Model: body.Model, StopReason: StopEnd}
	if body.Usage != nil {
		response.Usage = body.Usage.canonical()
	}
	response.Failure = body.Error.canonical()

	for _, item := range body.Output {
		switch item.Type {
		case "message":
			for _, part := range item.Content {
				if part.Text != "" {
					response.Content = append(response.Content, Content{Kind: KindText, Text: part.Text})
				}
			}
		case "reasoning":
			// A model that hides its reasoning sends only the summary.
			for _, part := range append(append([]responsesOutputPart{}, item.Content...), item.Summary...) {
				if part.Text != "" {
					response.Content = append(response.Content, Content{Kind: KindThinking, Text: part.Text})
				}
			}
		case "function_call":
			response.StopReason = StopToolUse
			response.Content = append(response.Content, Content{Kind: KindToolUse, ToolUse: &ToolUse{
				Id: emptyAs(item.CallId, item.Id), Name: item.Name, Arguments: emptyAsObject(item.Arguments),
			}})
		}
	}
	if body.Status == "incomplete" && body.IncompleteDetails != nil &&
		body.IncompleteDetails.Reason == "max_output_tokens" {
		response.StopReason = StopMaxTokens
	}
	return response, nil
}

// responsesEvent is one event of this API's stream, which names itself in its
// own payload.
type responsesEvent struct {
	Type        string               `json:"type"`
	Delta       string               `json:"delta"`
	OutputIndex int                  `json:"output_index"`
	Item        *responsesOutputItem `json:"item"`
	Response    *responsesResult     `json:"response"`
	Error       *responsesFailure    `json:"error"`
}

func (responsesCodec) DecodeStream(reader io.Reader, fn func(Event) bool) error {
	stop := ""
	usage := Usage{}
	model := ""
	running := true
	// A call's output index is what its argument deltas name it by, so it is
	// the call's index too.
	opened := map[int]bool{}

	err := forEachEvent(reader, func(data []byte) bool {
		var event responsesEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return true
		}
		if event.Response != nil && event.Response.Model != "" {
			model = event.Response.Model
		}

		switch event.Type {
		case "response.output_text.delta":
			if event.Delta != "" {
				running = fn(Event{Kind: EventText, Text: event.Delta, Model: model})
			}

		case "response.reasoning_summary_text.delta", "response.reasoning_text.delta":
			if event.Delta != "" {
				running = fn(Event{Kind: EventThinking, Text: event.Delta, Model: model})
			}

		case "response.output_item.added":
			// A call's name and id arrive here once; only its arguments stream.
			if event.Item == nil || event.Item.Type != "function_call" {
				return true
			}
			opened[event.OutputIndex] = true
			stop = StopToolUse
			running = fn(Event{Kind: EventToolUse, Model: model, Tool: &ToolDelta{
				Index: event.OutputIndex,
				Id:    emptyAs(event.Item.CallId, event.Item.Id),
				Name:  event.Item.Name,
			}})

		case "response.function_call_arguments.delta":
			if event.Delta == "" {
				return true
			}
			running = fn(Event{Kind: EventToolUse, Model: model, Tool: &ToolDelta{
				Index: event.OutputIndex, Arguments: event.Delta,
			}})

		case "response.output_item.done":
			// An upstream that sent no delta reports the call only here.
			if event.Item == nil || event.Item.Type != "function_call" || opened[event.OutputIndex] {
				return true
			}
			opened[event.OutputIndex] = true
			stop = StopToolUse
			running = fn(Event{Kind: EventToolUse, Model: model, Tool: &ToolDelta{
				Index:     event.OutputIndex,
				Id:        emptyAs(event.Item.CallId, event.Item.Id),
				Name:      event.Item.Name,
				Arguments: event.Item.Arguments,
			}})

		case "response.completed", "response.incomplete":
			if event.Response == nil {
				return true
			}
			if event.Response.Usage != nil {
				usage = event.Response.Usage.canonical()
			}
			if event.Response.IncompleteDetails != nil &&
				event.Response.IncompleteDetails.Reason == "max_output_tokens" {
				stop = StopMaxTokens
			}

		case "response.failed", "error":
			failure := event.Error.canonical()
			if failure == nil && event.Response != nil {
				failure = event.Response.Error.canonical()
			}
			if failure == nil {
				failure = &Failure{Kind: "server_error", Message: "the upstream stream failed"}
			}
			running = fn(Event{Kind: EventFailure, Failure: failure})
		}
		return running
	})

	if running {
		fn(Event{Kind: EventDone, StopReason: emptyAs(stop, StopEnd), Usage: &usage})
	}
	return err
}
