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

package service

import (
	"fmt"
	"strings"

	"github.com/apache/casbin-gateway/conf"
	"github.com/apache/casbin-gateway/embedsupport"
	"github.com/apache/casbin-gateway/object"
	"github.com/apache/casbin-gateway/webui"
)

type summaryRow struct {
	key   string
	value string
}

// PrintStartupSummary prints one table describing what this process is actually
// going to do: which port it serves, whether the database answers, and whether
// Casdoor is wired up. Every one of these has been a silent failure for someone
// starting Gateway for the first time.
func PrintStartupSummary() {
	rows := []summaryRow{
		{"Management UI", describeManagementUrl()},
		{"Settings", describeConf()},
		{"Web UI files", describeWebBuild()},
		{"Database", describeDatabase()},
		{"Sign-in", describeSignin()},
		{"Relay auth", describeRelayAuth()},
	}

	printSummaryTable("Casbin Gateway", rows)
}

// describeManagementUrl says both where the UI is and who can reach it, since
// the bind address decides whether the local admin is signed in automatically.
func describeManagementUrl() string {
	url := fmt.Sprintf("http://localhost:%d", conf.GetHttpPort())
	if !conf.IsHttpAddrLoopback() {
		return fmt.Sprintf("%s, bound to %s", url, conf.GetHttpAddr())
	}
	if lanAccessNeeded() {
		if host, err := LanHost(); err == nil {
			return fmt.Sprintf("%s, and %s:%d for an agent that runs in a sandbox", url, host, conf.GetHttpPort())
		}
	}

	return url + " (this machine only)"
}

// lanAccessNeeded mirrors what SyncLanAccess is about to do, so the table says
// what this process will serve rather than what httpaddr alone would suggest.
func lanAccessNeeded() bool {
	needed, err := sandboxedAgentIsBound()
	return err == nil && needed
}

// describeRelayAuth explains what an agent has to send to /v1. A request off
// this machine needs the token, which is also what Gateway writes into the
// configuration of an agent it switches.
func describeRelayAuth() string {
	if conf.IsHttpAddrLoopback() && !lanAccessNeeded() {
		return "this machine only, no token needed"
	}

	return "token required off this machine, see Settings"
}

// describeConf names where the settings were seeded from. They live in the
// database from the first start onwards, so this is only the origin of the
// values the Settings page now owns.
func describeConf() string {
	if embedsupport.IsEmbeddedConf() {
		return "Settings page, seeded from the conf/app.conf embedded in this binary"
	}

	return "Settings page, seeded from conf/app.conf"
}

func describeWebBuild() string {
	if buildDir := webui.GetBuildDir(); buildDir != "" {
		return buildDir
	}

	if embedsupport.HasWeb() {
		return "embedded in the binary"
	}

	return fmt.Sprintf("%s is missing, run \"yarn install && yarn build\" in web/", webui.BuildDir)
}

func describeDatabase() string {
	// The connection string holds the database password, so only the driver and
	// the database name are ever printed. A SQLite path carries no credentials,
	// so that one is printed in full.
	driverName := conf.GetConfigDriverName()
	target := fmt.Sprintf("%s, database \"%s\"", unquote(driverName), unquote(conf.GetConfigString("dbName")))
	if conf.IsSqliteDriver(driverName) {
		target = fmt.Sprintf("%s, file \"%s\"", driverName, conf.GetConfigDataSourceName())
	}

	err := object.PingDatabase()
	if err != nil {
		return fmt.Sprintf("%s (unreachable: %s)", target, sanitizeDbError(err))
	}

	return fmt.Sprintf("%s (connected)", target)
}

func describeSignin() string {
	if conf.IsCasdoorAvailable() {
		return fmt.Sprintf("Casdoor SSO at %s", unquote(conf.GetConfigString("casdoorEndpoint")))
	}

	return "built-in user table, Casdoor is not configured"
}

// unquote drops the quotes beego keeps around values such as dbName = "casbin_gateway".
func unquote(value string) string {
	return strings.Trim(value, `"' `)
}

// sanitizeDbError keeps the password inside "dataSourceName" out of the console,
// since driver errors sometimes quote the whole connection string back.
func sanitizeDbError(err error) string {
	msg := err.Error()

	dataSourceName := conf.GetConfigDataSourceName()
	if dataSourceName != "" {
		msg = strings.ReplaceAll(msg, dataSourceName, "<dataSourceName>")
		if i := strings.Index(dataSourceName, "@"); i > 0 {
			msg = strings.ReplaceAll(msg, dataSourceName[:i], "<credentials>")
		}
	}

	return msg
}

func printSummaryTable(title string, rows []summaryRow) {
	keyWidth, valueWidth := 0, len(title)
	for _, row := range rows {
		if len(row.key) > keyWidth {
			keyWidth = len(row.key)
		}
		if len(row.value) > valueWidth {
			valueWidth = len(row.value)
		}
	}

	// The title spans both columns, so the separator has to cover them plus the
	// " | " that joins them.
	border := "+" + strings.Repeat("-", keyWidth+valueWidth+5) + "+"

	fmt.Println()
	fmt.Println(border)
	fmt.Printf("| %-*s |\n", keyWidth+valueWidth+3, title)
	fmt.Println(border)
	for _, row := range rows {
		fmt.Printf("| %-*s | %-*s |\n", keyWidth, row.key, valueWidth, row.value)
	}
	fmt.Println(border)
	fmt.Println()
}
