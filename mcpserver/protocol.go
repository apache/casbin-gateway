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

package mcpserver

import "encoding/json"

const (
	invalidParams  = -32602
	methodNotFound = -32601
)

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	Id      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	Id      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (r request) reply(result any) response {
	return response{Id: r.Id, Result: result}
}

func (r request) fail(code int, message string) response {
	return response{Id: r.Id, Error: &rpcError{Code: code, Message: message}}
}

func (r request) paramsMap() map[string]any {
	if len(r.Params) == 0 {
		return nil
	}
	var params map[string]any
	if json.Unmarshal(r.Params, &params) != nil {
		return nil
	}
	return params
}
