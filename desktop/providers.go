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
	"fmt"
	"os"
	"strings"
	"time"

	"fyne.io/systray"
)

// menuInterval is how often the provider submenus are compared against the
// server. A switch made on the pages reaches the menu within one.
const menuInterval = 10 * time.Second

// builtinTitle is the entry that unbinds an agent, which puts back the provider
// settings it had before Gateway first switched it.
const builtinTitle = "Built-in model"

// providerMenu is the tray's provider switcher: one submenu per agent installed
// here, each listing the providers that agent can be pointed at. The submenus
// are redrawn only when what they should show changes, since redrawing one
// closes it under the pointer.
type providerMenu struct {
	root  *systray.MenuItem
	drawn string
	// agents holds only the submenu of each agent: removing one removes the
	// provider items under it, and removing those again is what Windows
	// answers with "invalid menu handle".
	agents []*systray.MenuItem
	// redrawNow carries a switch this menu just made, so the check mark moves
	// without waiting out the interval.
	redrawNow chan struct{}
}

func newProviderMenu(root *systray.MenuItem) *providerMenu {
	return &providerMenu{root: root, redrawNow: make(chan struct{}, 1)}
}

func (menu *providerMenu) watch() {
	for {
		menu.draw()
		select {
		case <-menu.redrawNow:
		case <-time.After(menuInterval):
		}
	}
}

func (menu *providerMenu) draw() {
	fetched, err := fetchTrayMenu()
	if err != nil {
		menu.clear("unavailable")
		return
	}
	if len(fetched.Agents) == 0 {
		menu.clear("no agent is installed")
		return
	}
	if len(fetched.Providers) == 0 {
		menu.clear("no provider is configured")
		return
	}

	fingerprint := fingerprintOf(fetched)
	if fingerprint == menu.drawn {
		return
	}
	menu.remove()

	menu.root.SetTitle("Switch Provider")
	menu.root.Enable()
	for _, entry := range fetched.Agents {
		submenu := menu.root.AddSubMenuItem(entry.Name, "Choose where "+entry.Name+" sends its requests")
		menu.agents = append(menu.agents, submenu)

		menu.add(submenu, entry.AgentId, "", builtinTitle, "Unbind "+entry.Name+" and put its own settings back", entry.Provider == "")
		for _, provider := range fetched.Providers {
			title := provider.Name
			if provider.Disabled {
				title += " (disabled)"
			}
			menu.add(submenu, entry.AgentId, provider.Id, title, "Send "+entry.Name+" to "+provider.Id, provider.Id == entry.Provider)
		}
	}
	menu.drawn = fingerprint
}

func (menu *providerMenu) add(submenu *systray.MenuItem, agentId string, providerId string, title string, tooltip string, checked bool) {
	item := submenu.AddSubMenuItemCheckbox(title, tooltip, checked)

	// Remove() closes the channel, so the goroutine of an item the next redraw
	// drops ends with it.
	go func() {
		for range item.ClickedCh {
			if err := setAgentProvider(agentId, providerId); err != nil {
				fmt.Fprintln(os.Stderr, "casbin-gateway-desktop:", err)
			}
			// Either way the menu is redrawn from what the server now says, so
			// a switch that failed leaves the check mark where it was.
			menu.requestRedraw()
		}
	}()
}

// clear leaves the switcher greyed out, saying why there is nothing under it.
// The reason is what the menu is drawn from while it is empty, so a Gateway
// that stays down is not redrawn every poll.
func (menu *providerMenu) clear(reason string) {
	title := "Switch Provider (" + reason + ")"
	if menu.drawn == title {
		return
	}

	menu.remove()
	menu.root.SetTitle(title)
	menu.root.Disable()
	menu.drawn = title
}

func (menu *providerMenu) remove() {
	for _, item := range menu.agents {
		item.Remove()
	}
	menu.agents = nil
}

func (menu *providerMenu) requestRedraw() {
	select {
	case menu.redrawNow <- struct{}{}:
	default:
	}
}

// fingerprintOf is what the submenus are drawn from, so an unchanged menu is
// left standing.
func fingerprintOf(fetched *trayMenu) string {
	parts := []string{}
	for _, entry := range fetched.Agents {
		parts = append(parts, entry.AgentId+"\x00"+entry.Name+"\x00"+entry.Provider)
	}
	for _, provider := range fetched.Providers {
		parts = append(parts, provider.Id+"\x00"+provider.Name+"\x00"+fmt.Sprint(provider.Disabled))
	}
	return strings.Join(parts, "\x01")
}
