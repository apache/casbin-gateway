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

package agentpatch

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/apache/casbin-gateway/conf"
)

const defaultHttpPort = "17000"

// recordsURL is the loopback endpoint a patched installation reports to.
func recordsURL() (string, error) {
	return loopbackURL("/api/add-agent-record")
}

// decisionURL is the loopback endpoint a patched installation asks before it
// runs a tool. It is what holds an agent whose model traffic never comes
// through the proxy.
func decisionURL() (string, error) {
	return loopbackURL("/api/check-agent-tool")
}

// loopbackURL builds one of those. A malformed httpport would bake an
// unreachable URL into an agent's config, so it is rejected while the operator
// can still see the error.
func loopbackURL(path string) (string, error) {
	port := strings.TrimSpace(conf.GetConfigString("httpport"))
	if port == "" {
		port = defaultHttpPort
	}
	number, err := strconv.Atoi(port)
	if err != nil || number <= 0 || number > 65535 {
		return "", fmt.Errorf("cannot build the agent endpoint %s: httpport %q is not a valid port", path, port)
	}
	return fmt.Sprintf("http://127.0.0.1:%d%s", number, path), nil
}

// GatewayExecutable is this Gateway's own program, which an agent is given as
// the command to run for a proxied MCP connection.
func GatewayExecutable() (string, error) {
	return gatewayExecutable()
}

// GatewayBaseUrl is the loopback address of the Gateway on this machine, for a
// child process that has to ask it something.
func GatewayBaseUrl() (string, error) {
	return loopbackURL("")
}
