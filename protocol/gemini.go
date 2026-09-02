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

// This file speaks the Gemini generateContent API, the only wire format the
// Gemini CLI knows. It is a client format only: the gateway reads requests in
// it and writes answers back in it, while a Gemini provider is reached through
// its OpenAI-compatible endpoint, so a request arriving here is always
// translated for whichever upstream answers it.
//
// This API names its model in the URL rather than in the body, so the model of
// a decoded request is filled in by the caller that read the path.

package protocol

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
)

type geminiCodec struct{}

func init() {
	register(geminiCodec{})
}

func (geminiCodec) Name() string { return Gemini }

// ---------------------------------------------------------------------------
// The wire types
// ---------------------------------------------------------------------------

type geminiRequest struct {
	Contents          []geminiContent         `json:"contents"`
	SystemInstruction *geminiContent          `json:"systemInstruction"`
	Tools             []geminiTool            `json:"tools"`
	ToolConfig        *geminiToolConfig       `json:"toolConfig"`
	GenerationConfig  *geminiGenerationConfig `json:"generationConfig"`
}

// geminiContent is one turn: this API spells the assistant "model", and carries
// tool calls and their results as parts of a turn rather than as turns of their
// own.
type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts,omitempty"`
}

// geminiPart is one block of a turn. Only the field its kind names is filled
// in, which is how this API models a union.
type geminiPart struct {
	Text string `json:"text,omitempty"`
	// Thought marks a text part as the model's own reasoning rather than the
	// answer, and ThoughtSignature is what the vendor signed it with.
	Thought          bool                    `json:"thought,omitempty"`
	ThoughtSignature string                  `json:"thoughtSignature,omitempty"`
	InlineData       *geminiBlob             `json:"inlineData,omitempty"`
	FileData         *geminiFileData         `json:"fileData,omitempty"`
	FunctionCall     *geminiFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResponse `json:"functionResponse,omitempty"`
}

type geminiBlob struct {
	MimeType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"`
}

type geminiFileData struct {
	MimeType string `json:"mimeType,omitempty"`
	FileUri  string `json:"fileUri,omitempty"`
}

type geminiFunctionCall struct {
	Id   string          `json:"id,omitempty"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"args,omitempty"`
}

type geminiFunctionResponse struct {
	Id       string          `json:"id,omitempty"`
	Name     string          `json:"name,omitempty"`
	Response json.RawMessage `json:"response,omitempty"`
}

type geminiTool struct {
	FunctionDeclarations []geminiFunctionDeclaration `json:"functionDeclarations,omitempty"`
}

type geminiFunctionDeclaration struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
	// ParametersJsonSchema is the same schema in the spelling the newer clients
	// use: a plain JSON Schema rather than the trimmed dialect above.
	ParametersJsonSchema json.RawMessage `json:"parametersJsonSchema,omitempty"`
}

type geminiToolConfig struct {
	FunctionCallingConfig *struct {
		Mode                 string   `json:"mode"`
		AllowedFunctionNames []string `json:"allowedFunctionNames"`
	} `json:"functionCallingConfig"`
}

type geminiGenerationConfig struct {
	Temperature        *float64        `json:"temperature,omitempty"`
	TopP               *float64        `json:"topP,omitempty"`
	MaxOutputTokens    *int            `json:"maxOutputTokens,omitempty"`
	StopSequences      []string        `json:"stopSequences,omitempty"`
	ResponseMimeType   string          `json:"responseMimeType,omitempty"`
	ResponseSchema     json.RawMessage `json:"responseSchema,omitempty"`
	ResponseJsonSchema json.RawMessage `json:"responseJsonSchema,omitempty"`
	ThinkingConfig     *struct {
		IncludeThoughts bool `json:"includeThoughts"`
		// ThinkingBudget is a token budget, or -1 for as much as the model
		// decides it needs.
		ThinkingBudget *int `json:"thinkingBudget"`
	} `json:"thinkingConfig,omitempty"`
}

type geminiUsage struct {
	PromptTokenCount        int `json:"promptTokenCount,omitempty"`
	CandidatesTokenCount    int `json:"candidatesTokenCount,omitempty"`
	CachedContentTokenCount int `json:"cachedContentTokenCount,omitempty"`
	ThoughtsTokenCount      int `json:"thoughtsTokenCount,omitempty"`
	TotalTokenCount         int `json:"totalTokenCount,omitempty"`
}

// ---------------------------------------------------------------------------
// Request
// ---------------------------------------------------------------------------

func (geminiCodec) DecodeRequest(raw []byte) (*Request, error) {
	var body geminiRequest
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, errors.New("invalid request body")
	}
	if len(body.Contents) == 0 {
		return nil, errors.New("contents is required")
	}

	request := &Request{ToolChoice: geminiToolChoiceOf(body.ToolConfig)}
	if body.SystemInstruction != nil {
		if system := geminiTextOf(body.SystemInstruction.Parts); system != "" {
			request.System = []string{system}
		}
	}
	for _, tool := range body.Tools {
		for _, declaration := range tool.FunctionDeclarations {
			if declaration.Name == "" {
				continue
			}
			parameters := declaration.Parameters
			if len(parameters) == 0 {
				parameters = declaration.ParametersJsonSchema
			}
			request.Tools = append(request.Tools, Tool{
				Name: declaration.Name, Description: declaration.Description, Parameters: parameters,
			})
		}
	}
	for _, content := range body.Contents {
		request.Messages = appendGeminiContent(request.Messages, content)
	}
	if len(request.Messages) == 0 {
		return nil, errors.New("contents is required")
	}

	geminiApplyGenerationConfig(request, body.GenerationConfig)
	return request, nil
}

func appendGeminiContent(messages []Message, content geminiContent) []Message {
	role := RoleUser
	if content.Role == "model" || content.Role == RoleAssistant {
		role = RoleAssistant
	}

	blocks := []Content{}
	for _, part := range content.Parts {
		switch {
		case part.FunctionCall != nil:
			blocks = append(blocks, Content{
				Kind:      KindToolUse,
				Signature: part.ThoughtSignature,
				ToolUse: &ToolUse{
					Id:        part.FunctionCall.Id,
					Name:      part.FunctionCall.Name,
					Arguments: emptyAsObject(string(part.FunctionCall.Args)),
				},
			})
		case part.FunctionResponse != nil:
			blocks = append(blocks, Content{Kind: KindToolResult, ToolResult: &ToolResult{
				Id:   emptyAs(part.FunctionResponse.Id, part.FunctionResponse.Name),
				Text: geminiResponseText(part.FunctionResponse.Response),
			}})
		case part.InlineData != nil:
			blocks = append(blocks, Content{Kind: KindImage, Image: &Image{
				MediaType: part.InlineData.MimeType, Data: part.InlineData.Data,
			}})
		case part.FileData != nil:
			blocks = append(blocks, Content{Kind: KindImage, Image: &Image{
				MediaType: part.FileData.MimeType, Url: part.FileData.FileUri,
			}})
		case part.Text != "" && part.Thought:
			blocks = append(blocks, Content{Kind: KindThinking, Text: part.Text, Signature: part.ThoughtSignature})
		case part.Text != "":
			blocks = append(blocks, Content{Kind: KindText, Text: part.Text})
		}
	}
	if len(blocks) == 0 {
		return messages
	}
	return append(messages, Message{Role: role, Content: blocks})
}

// geminiTextOf is the plain text of a set of parts, which is all a system
// instruction ever holds.
func geminiTextOf(parts []geminiPart) string {
	text := []string{}
	for _, part := range parts {
		if part.Text != "" && !part.Thought {
			text = append(text, part.Text)
		}
	}
	return strings.Join(text, "\n")
}

// geminiResponseText is the text of a tool result. This API wraps one in an
// object of the tool's own making, and the clients put the result under
// "output" or "error"; anything else is passed on as the JSON it is.
func geminiResponseText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}

	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return string(raw)
	}
	for _, key := range []string{"output", "error", "result", "content"} {
		value, ok := wrapper[key]
		if !ok {
			continue
		}
		var inner string
		if err := json.Unmarshal(value, &inner); err == nil {
			return inner
		}
		return string(value)
	}
	return string(raw)
}

func geminiToolChoiceOf(config *geminiToolConfig) *ToolChoice {
	if config == nil || config.FunctionCallingConfig == nil {
		return nil
	}

	calling := config.FunctionCallingConfig
	switch strings.ToUpper(calling.Mode) {
	case "AUTO":
		return &ToolChoice{Mode: ChoiceAuto}
	case "NONE":
		return &ToolChoice{Mode: ChoiceNone}
	case "ANY":
		// One allowed name is the same thing as naming the tool that has to be
		// called, which is how the other formats spell it.
		if len(calling.AllowedFunctionNames) == 1 {
			return &ToolChoice{Mode: ChoiceTool, Name: calling.AllowedFunctionNames[0]}
		}
		return &ToolChoice{Mode: ChoiceRequired}
	}
	return nil
}

func geminiApplyGenerationConfig(request *Request, config *geminiGenerationConfig) {
	if config == nil {
		return
	}

	request.Temperature = config.Temperature
	request.TopP = config.TopP
	request.MaxTokens = config.MaxOutputTokens
	request.StopSequences = config.StopSequences

	schema := config.ResponseSchema
	if len(schema) == 0 {
		schema = config.ResponseJsonSchema
	}
	switch {
	case len(schema) > 0:
		request.Format = &Format{Kind: "json_schema", Schema: schema}
	case config.ResponseMimeType == "application/json":
		request.Format = &Format{Kind: "json_object"}
	}

	thinking := config.ThinkingConfig
	if thinking == nil {
		return
	}
	switch {
	case thinking.ThinkingBudget != nil && *thinking.ThinkingBudget > 0:
		request.Reasoning = &Reasoning{BudgetTokens: *thinking.ThinkingBudget}
	case thinking.IncludeThoughts || (thinking.ThinkingBudget != nil && *thinking.ThinkingBudget < 0):
		// A budget the model decides for itself, which no other format spells,
		// asks for the middle of what they do.
		request.Reasoning = &Reasoning{Effort: EffortMedium}
	}
}

// ---------------------------------------------------------------------------
// Response
// ---------------------------------------------------------------------------

// geminiAnswer builds the answer as this API reports it. The whole-body encoder
// and the stream writer share it, so a client reading events and one reading a
// body are told the same thing.
type geminiAnswer struct {
	id    string
	model string
	// calls holds the tool calls being assembled, keyed by the index of the
	// canonical stream, with callOrder keeping the order they were opened in.
	calls     map[int]*geminiFunctionCall
	callOrder []int
	usage     Usage
	stop      string
	failure   *Failure
}

func newGeminiAnswer(model string) *geminiAnswer {
	return &geminiAnswer{
		id:    "gw-" + newToken(),
		model: model,
		calls: map[int]*geminiFunctionCall{},
	}
}

// add records one event of a streamed answer.
func (answer *geminiAnswer) add(event Event) {
	if event.Model != "" {
		answer.model = event.Model
	}

	switch event.Kind {
	case EventToolUse:
		if event.Tool == nil {
			return
		}
		call, ok := answer.calls[event.Tool.Index]
		if !ok {
			call = &geminiFunctionCall{}
			answer.calls[event.Tool.Index] = call
			answer.callOrder = append(answer.callOrder, event.Tool.Index)
		}
		if event.Tool.Id != "" {
			call.Id = event.Tool.Id
		}
		call.Name += event.Tool.Name
		// The arguments arrive in pieces, and this API carries them as one JSON
		// object, so the call is only written once the stream has closed.
		call.Args = append(call.Args, event.Tool.Arguments...)
	case EventDone:
		if event.StopReason != "" {
			answer.stop = event.StopReason
		}
		if event.Usage != nil {
			answer.usage = *event.Usage
		}
	case EventFailure:
		answer.failure = event.Failure
	}
}

// callParts are the tool calls the answer ended with, with their arguments
// parsed back into the object this API carries them as.
func (answer *geminiAnswer) callParts() []geminiPart {
	parts := []geminiPart{}
	for _, index := range answer.callOrder {
		call := answer.calls[index]
		parts = append(parts, geminiPart{FunctionCall: &geminiFunctionCall{
			Id: call.Id, Name: call.Name, Args: geminiArgs(string(call.Args)),
		}})
	}
	return parts
}

// chunk is one whole answer, or one piece of a streamed one: this API sends
// both in the same shape.
func (answer *geminiAnswer) chunk(parts []geminiPart, finish string) map[string]any {
	candidate := map[string]any{
		"content": map[string]any{"role": "model", "parts": parts},
		"index":   0,
	}
	if finish != "" {
		candidate["finishReason"] = finish
	}

	chunk := map[string]any{
		"candidates":   []any{candidate},
		"modelVersion": answer.model,
		"responseId":   answer.id,
	}
	if usage := geminiUsageOf(answer.usage); usage != nil {
		chunk["usageMetadata"] = usage
	}
	if answer.failure != nil {
		// This API has no error event, so an upstream failing mid-answer is
		// reported beside the last piece of it.
		chunk["error"] = geminiError(emptyAs(answer.failure.Kind, "server_error"), answer.failure.Message)
	}
	return chunk
}

func (geminiCodec) EncodeResponse(response *Response) ([]byte, error) {
	answer := newGeminiAnswer(response.Model)
	answer.usage = response.Usage
	answer.stop = response.StopReason
	answer.failure = response.Failure

	parts := []geminiPart{}
	for _, content := range response.Content {
		switch content.Kind {
		case KindText:
			parts = append(parts, geminiPart{Text: content.Text})
		case KindThinking:
			parts = append(parts, geminiPart{
				Text: content.Text, Thought: true, ThoughtSignature: content.Signature,
			})
		case KindToolUse:
			if content.ToolUse == nil {
				continue
			}
			parts = append(parts, geminiPart{FunctionCall: &geminiFunctionCall{
				Id:   content.ToolUse.Id,
				Name: content.ToolUse.Name,
				Args: geminiArgs(content.ToolUse.Arguments),
			}})
		}
	}
	return json.Marshal(answer.chunk(parts, geminiFinishOfStop(answer.stop)))
}

// ---------------------------------------------------------------------------
// Stream
// ---------------------------------------------------------------------------

// geminiStreamWriter turns canonical events into the chunks this API streams:
// one whole answer object per event, with the tool calls, the finish reason and
// the counters in the last of them.
type geminiStreamWriter struct {
	events *eventWriter
	answer *geminiAnswer
}

func (geminiCodec) NewStreamWriter(writer io.Writer, flush func(), model string) StreamWriter {
	return &geminiStreamWriter{
		events: &eventWriter{writer: writer, flush: flush},
		answer: newGeminiAnswer(model),
	}
}

func (writer *geminiStreamWriter) Open() {}

func (writer *geminiStreamWriter) Write(event Event) {
	writer.answer.add(event)

	switch event.Kind {
	case EventText:
		if event.Text == "" {
			return
		}
		writer.send([]geminiPart{{Text: event.Text}}, "")
	case EventThinking:
		if event.Text == "" && event.Signature == "" {
			return
		}
		writer.send([]geminiPart{{
			Text: event.Text, Thought: true, ThoughtSignature: event.Signature,
		}}, "")
	}
}

func (writer *geminiStreamWriter) Close() {
	// The tool calls go out in the last chunk: their arguments arrive in
	// pieces, and this API has no event to carry a piece of one in.
	writer.send(writer.answer.callParts(), geminiFinishOfStop(writer.answer.stop))
}

func (writer *geminiStreamWriter) send(parts []geminiPart, finish string) {
	writer.events.send("", writer.answer.chunk(parts, finish))
}

func (geminiCodec) EncodeError(kind string, message string) []byte {
	data, err := json.Marshal(map[string]any{"error": geminiError(kind, message)})
	if err != nil {
		return []byte(`{"error":{"code":500,"message":"server error","status":"INTERNAL"}}`)
	}
	return data
}

// ---------------------------------------------------------------------------
// Shared pieces
// ---------------------------------------------------------------------------

// geminiArgs is a tool call's arguments as the object this API carries them in.
// Half an object is what a stream that was cut off leaves behind, and an empty
// one is what a client can still act on.
func geminiArgs(arguments string) json.RawMessage {
	trimmed := strings.TrimSpace(arguments)
	if trimmed == "" {
		return json.RawMessage("{}")
	}
	var value map[string]any
	if err := json.Unmarshal([]byte(trimmed), &value); err != nil {
		return json.RawMessage("{}")
	}
	return json.RawMessage(trimmed)
}

// geminiUsageOf writes the counters back out. This API counts the cached part
// inside the prompt total, which is where the canonical form keeps it too.
func geminiUsageOf(usage Usage) *geminiUsage {
	if usage.IsZero() {
		return nil
	}

	written := &geminiUsage{
		PromptTokenCount:        usage.InputTokens + usage.CacheWriteTokens,
		CandidatesTokenCount:    usage.OutputTokens,
		CachedContentTokenCount: usage.CacheReadTokens,
		ThoughtsTokenCount:      usage.ReasoningTokens,
	}
	written.TotalTokenCount = written.PromptTokenCount + written.CandidatesTokenCount
	return written
}

func geminiFinishOfStop(stop string) string {
	switch stop {
	case StopMaxTokens:
		return "MAX_TOKENS"
	case StopFilter:
		return "SAFETY"
	}
	return "STOP"
}

// geminiError is one failure as Google's APIs report it: the canonical error
// name in their own spelling, beside the status code that name is answered
// with, which is where their clients read the reason off.
func geminiError(kind string, message string) map[string]any {
	code, status := 500, "INTERNAL"
	switch kind {
	case "invalid_request_error":
		code, status = 400, "INVALID_ARGUMENT"
	case "authentication_error":
		code, status = 401, "UNAUTHENTICATED"
	case "permission_error":
		code, status = 403, "PERMISSION_DENIED"
	case "not_found_error":
		code, status = 404, "NOT_FOUND"
	case "rate_limit_error":
		code, status = 429, "RESOURCE_EXHAUSTED"
	}
	return map[string]any{"code": code, "message": message, "status": status}
}
