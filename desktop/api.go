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

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// localTokenPath mirrors service.LocalTokenPath, resolved against the same
// directory the server runs in. It is what the tray sends instead of a session:
// the server writes it where only this account can read it.
const localTokenPath = "tmp/local-token"

const localTokenHeader = "X-Casbin-Gateway-Local-Token"

const apiTimeout = 10 * time.Second

// trayMenu is what the server says the provider submenus should contain.
type trayMenu struct {
	Agents []struct {
		AgentId  string `json:"agentId"`
		Name     string `json:"name"`
		Provider string `json:"provider"`
	} `json:"agents"`
	Providers []struct {
		Id       string `json:"id"`
		Name     string `json:"name"`
		Disabled bool   `json:"disabled"`
	} `json:"providers"`
}

// serverUrl is the server's own address. It is not gatewayUrl(): that one may
// be pointed at the frontend dev server, which has no API of its own.
func serverUrl() string {
	return fmt.Sprintf("http://127.0.0.1:%d", httpPort())
}

func fetchTrayMenu() (*trayMenu, error) {
	menu := &trayMenu{}
	if err := callApi(http.MethodGet, "/api/get-tray-menu", nil, menu); err != nil {
		return nil, err
	}
	return menu, nil
}

func setAgentProvider(agentId string, providerId string) error {
	body, err := json.Marshal(map[string]string{"agentId": agentId, "provider": providerId})
	if err != nil {
		return err
	}
	return callApi(http.MethodPost, "/api/set-agent-provider", body, nil)
}

// callApi unwraps the {status, msg, data} envelope every Gateway API answers
// with, so a rejected call is an error here rather than an empty menu.
func callApi(method string, path string, body []byte, out interface{}) error {
	token, err := os.ReadFile(filepath.Join(gatewayHome(), filepath.FromSlash(localTokenPath)))
	if err != nil {
		return fmt.Errorf("the Gateway server has not left a local token: %v", err)
	}

	var payload *bytes.Reader
	if body == nil {
		payload = bytes.NewReader(nil)
	} else {
		payload = bytes.NewReader(body)
	}
	request, err := http.NewRequest(method, serverUrl()+path, payload)
	if err != nil {
		return err
	}
	request.Header.Set(localTokenHeader, strings.TrimSpace(string(token)))
	request.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: apiTimeout}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	envelope := struct {
		Status string          `json:"status"`
		Msg    string          `json:"msg"`
		Data   json.RawMessage `json:"data"`
	}{}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("%s answered %s: %v", path, response.Status, err)
	}
	if envelope.Status != "ok" {
		return errors.New(envelope.Msg)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(envelope.Data, out)
}
