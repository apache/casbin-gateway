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

package ccswitch

import (
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite" // read another tool's own database
)

// mcpAppPrefix starts the per-app switches on an MCP server. The app is the
// part after it, so a column CC Switch adds for a new app is read without this
// file changing.
const mcpAppPrefix = "enabled_"

// readDatabase fills the store from CC Switch's SQLite database. It is opened
// read-only, and a table this version of CC Switch does not have is read as
// empty rather than as a failure: the schema grows with its releases.
func readDatabase(path string, store *Store) error {
	if _, err := os.Stat(path); err != nil {
		return nil
	}

	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro")
	if err != nil {
		return err
	}
	defer db.Close()

	store.Providers = readProviderRows(db)
	store.Mcps = readMcpRows(db)
	store.Prompts = readPromptRows(db)
	store.SkillRepos = readSkillRepoRows(db)
	return nil
}

func readProviderRows(db *sql.DB) []*Provider {
	providers := []*Provider{}
	for _, row := range rowsOf(db, "providers") {
		providers = append(providers, &Provider{
			Id:       row["id"],
			App:      row["app_type"],
			Name:     row["name"],
			Website:  row["website_url"],
			Notes:    row["notes"],
			Current:  isTrue(row["is_current"]),
			Settings: row["settings_config"],
		})
	}
	return providers
}

func readMcpRows(db *sql.DB) []*McpServer {
	servers := []*McpServer{}
	for _, row := range rowsOf(db, "mcp_servers") {
		apps := []string{}
		for column, value := range row {
			if strings.HasPrefix(column, mcpAppPrefix) && isTrue(value) {
				apps = append(apps, strings.TrimPrefix(column, mcpAppPrefix))
			}
		}
		sort.Strings(apps)

		servers = append(servers, &McpServer{
			Id:     row["id"],
			Name:   row["name"],
			Config: row["server_config"],
			Apps:   apps,
		})
	}
	return servers
}

func readPromptRows(db *sql.DB) []*Prompt {
	prompts := []*Prompt{}
	for _, row := range rowsOf(db, "prompts") {
		prompts = append(prompts, &Prompt{
			Id:      row["id"],
			App:     row["app_type"],
			Name:    row["name"],
			Content: row["content"],
			Enabled: isTrue(row["enabled"]),
		})
	}
	return prompts
}

func readSkillRepoRows(db *sql.DB) []*SkillRepo {
	repos := []*SkillRepo{}
	for _, row := range rowsOf(db, "skill_repos") {
		if !isTrue(row["enabled"]) {
			continue
		}
		repos = append(repos, &SkillRepo{
			Owner:  row["owner"],
			Name:   row["name"],
			Branch: row["branch"],
		})
	}
	return repos
}

// rowsOf reads one table as a row per map of column name to value. The columns
// come from the result rather than from the query because CC Switch has added
// some over its releases, and a store written by any of them should still be
// readable here.
func rowsOf(db *sql.DB, table string) []map[string]string {
	rows, err := db.Query("SELECT * FROM " + table)
	if err != nil {
		return nil
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil
	}

	read := []map[string]string{}
	for rows.Next() {
		cells := make([]any, len(columns))
		for index := range cells {
			cells[index] = &sql.NullString{}
		}
		if err := rows.Scan(cells...); err != nil {
			return read
		}

		row := map[string]string{}
		for index, name := range columns {
			if cell := cells[index].(*sql.NullString); cell.Valid {
				row[name] = cell.String
			}
		}
		read = append(read, row)
	}
	return read
}

// isTrue reads the booleans, which SQLite stores as 0 and 1 and the legacy file
// as JSON.
func isTrue(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true":
		return true
	default:
		return false
	}
}
