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
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// legacyFile is config.json, which CC Switch kept its whole provider list in
// before the SQLite database. Its first version held one app's providers at the
// top level; the second moved them under the app they belong to.
type legacyFile struct {
	Providers map[string]legacyProvider `json:"providers"`
	Current   string                    `json:"current"`
	Apps      map[string]legacyApp      `json:"apps"`
}

type legacyApp struct {
	Providers map[string]legacyProvider `json:"providers"`
	Current   string                    `json:"current"`
}

type legacyProvider struct {
	Id             string          `json:"id"`
	Name           string          `json:"name"`
	WebsiteUrl     string          `json:"websiteUrl"`
	SettingsConfig json.RawMessage `json:"settingsConfig"`
}

// readLegacy fills the store from config.json. A file that is not there, or
// carries nothing this understands, leaves the store as it was.
func readLegacy(path string, store *Store) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	file := &legacyFile{}
	if err := json.Unmarshal(data, file); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	apps := map[string]legacyApp{}
	// The one app the first version knew was Claude Code.
	if len(file.Providers) > 0 {
		apps["claude"] = legacyApp{Providers: file.Providers, Current: file.Current}
	}
	for app, held := range file.Apps {
		apps[app] = held
	}

	names := make([]string, 0, len(apps))
	for app := range apps {
		names = append(names, app)
	}
	sort.Strings(names)

	for _, app := range names {
		held := apps[app]
		ids := make([]string, 0, len(held.Providers))
		for id := range held.Providers {
			ids = append(ids, id)
		}
		sort.Strings(ids)

		for _, id := range ids {
			entry := held.Providers[id]
			if entry.Id == "" {
				entry.Id = id
			}
			store.Providers = append(store.Providers, &Provider{
				Id:       entry.Id,
				App:      app,
				Name:     entry.Name,
				Website:  entry.WebsiteUrl,
				Current:  entry.Id == held.Current,
				Settings: string(entry.SettingsConfig),
			})
		}
	}

	store.Legacy = len(store.Providers) > 0
	return nil
}
