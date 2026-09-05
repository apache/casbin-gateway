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

package connector

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// placeholder is "${field}" in any template value.
var placeholder = regexp.MustCompile(`\$\{([A-Za-z0-9_.-]+)\}`)

// placeholders is every field name a server template refers to, sorted so a
// validation failure names them in a stable order.
func placeholders(server ServerSpec) []string {
	seen := map[string]bool{}
	for _, value := range templateValues(server) {
		for _, match := range placeholder.FindAllStringSubmatch(value, -1) {
			seen[match[1]] = true
		}
	}

	found := make([]string, 0, len(seen))
	for name := range seen {
		found = append(found, name)
	}
	sort.Strings(found)
	return found
}

func templateValues(server ServerSpec) []string {
	values := []string{server.Command, server.Url}
	values = append(values, server.Args...)
	for _, value := range server.Env {
		values = append(values, value)
	}
	for _, value := range server.Headers {
		values = append(values, value)
	}
	return values
}

// Rendered is one connector's server with its credentials filled in, in the
// shape agentconfig writes into an agent's own configuration.
type Rendered struct {
	Name      string
	Transport string
	Command   string
	Args      []string
	Env       map[string]string
	Url       string
	Headers   map[string]string
}

// Render fills a connector's server template from credentials. A missing
// required credential is an error rather than an empty substitution, so a
// half-configured connection is never written into an agent.
func (c Connector) Render(credentials map[string]string) (Rendered, error) {
	for _, field := range c.Auth.Fields {
		if field.Required && strings.TrimSpace(credentials[field.Key]) == "" {
			return Rendered{}, fmt.Errorf("%s needs a value", field.Label)
		}
	}
	// Saying what is missing is worth a case of its own: "no value for
	// ${accessToken}" names a template, not the thing the operator has to do.
	if c.Auth.Kind == AuthOauth2 && strings.TrimSpace(credentials[KeyAccessToken]) == "" {
		return Rendered{}, fmt.Errorf("this connection has not been authorized yet")
	}

	fill := func(value string) (string, error) {
		var missing string
		filled := placeholder.ReplaceAllStringFunc(value, func(match string) string {
			name := placeholder.FindStringSubmatch(match)[1]
			found, ok := credentials[name]
			if !ok || found == "" {
				missing = name
				return ""
			}
			return found
		})
		if missing != "" {
			return "", fmt.Errorf("no value for ${%s}", missing)
		}
		return filled, nil
	}

	rendered := Rendered{Name: c.Server.Name, Transport: c.Server.Transport}
	var err error
	if rendered.Command, err = fill(c.Server.Command); err != nil {
		return Rendered{}, err
	}
	if rendered.Url, err = fill(c.Server.Url); err != nil {
		return Rendered{}, err
	}
	for _, arg := range c.Server.Args {
		filled, err := fill(arg)
		if err != nil {
			return Rendered{}, err
		}
		rendered.Args = append(rendered.Args, filled)
	}
	if rendered.Env, err = fillMap(c.Server.Env, fill); err != nil {
		return Rendered{}, err
	}
	if rendered.Headers, err = fillMap(c.Server.Headers, fill); err != nil {
		return Rendered{}, err
	}
	return rendered, nil
}

func fillMap(values map[string]string, fill func(string) (string, error)) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	filled := make(map[string]string, len(values))
	for key, value := range values {
		found, err := fill(value)
		if err != nil {
			return nil, err
		}
		filled[key] = found
	}
	return filled, nil
}
