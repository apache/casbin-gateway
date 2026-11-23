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
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
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
	root     *systray.MenuItem
	status   *systray.MenuItem
	switches chan providerChoice
	drawn    string
	// agents holds only the submenu of each agent: removing one removes the
	// provider items under it, and removing those again is what Windows
	// answers with "invalid menu handle".
	agents []*systray.MenuItem
}

type providerChoice struct {
	agentId      string
	agentName    string
	providerId   string
	providerName string
}

func newProviderMenu(root *systray.MenuItem, status *systray.MenuItem) *providerMenu {
	return &providerMenu{root: root, status: status, switches: make(chan providerChoice, 1)}
}

func (menu *providerMenu) watch() {
	ticker := time.NewTicker(menuInterval)
	defer ticker.Stop()
	menu.draw()
	lastAgent := ""
	for {
		select {
		case choice := <-menu.switches:
			if menu.root.Disabled() {
				continue
			}
			lastAgent = choice.agentId
			menu.root.Disable()
			menu.switchProvider(choice)
			menu.root.Enable()
			menu.draw()
		case <-menu.status.ClickedCh:
			if lastAgent != "" {
				if err := openBrowser(gatewayUrl() + "/agents/" + url.PathEscape(lastAgent)); err != nil {
					fmt.Fprintln(os.Stderr, "casbin-gateway-desktop:", err)
				}
			}
		case <-ticker.C:
			menu.draw()
		}
	}
}

func (menu *providerMenu) switchProvider(choice providerChoice) {
	menu.status.SetTitle("Switching provider for " + choice.agentName + "...")
	menu.status.Show()
	err := setAgentProvider(choice.agentId, choice.providerId)
	if err != nil {
		fmt.Fprintln(os.Stderr, "casbin-gateway-desktop:", err)
	}
	menu.status.SetTitle(providerSwitchMessage(choice, err))
}

func providerSwitchMessage(choice providerChoice, err error) string {
	if err == nil {
		return choice.agentName + ": " + choice.providerName + " selected"
	}
	var failure *apiError
	if !errors.As(err, &failure) {
		return choice.agentName + ": could not confirm provider switch"
	}
	var outcome struct {
		RoutingSaved *bool `json:"routingSaved"`
	}
	if json.Unmarshal(failure.data, &outcome) != nil || outcome.RoutingSaved == nil {
		return choice.agentName + ": provider switch failed"
	}
	if *outcome.RoutingSaved {
		if choice.providerId == "" {
			return choice.agentName + ": routing cleared, configuration restore failed"
		}
		return choice.agentName + ": routing saved, agent update failed"
	}
	return choice.agentName + ": provider switch rejected"
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

		builtin := providerChoice{agentId: entry.AgentId, agentName: entry.Name, providerName: builtinTitle}
		menu.add(submenu, builtin, "Unbind "+entry.Name+" and put its own settings back", entry.Provider == "", "")
		for _, provider := range fetched.Providers {
			choice := providerChoice{
				agentId:      entry.AgentId,
				agentName:    entry.Name,
				providerId:   provider.Id,
				providerName: provider.Name,
			}
			menu.add(submenu, choice, "Send "+entry.Name+" to "+provider.Id, provider.Id == entry.Provider, unavailableProvider(entry, provider))
		}
	}
	menu.drawn = fingerprint
}

func unavailableProvider(agent trayAgent, provider trayProvider) string {
	if provider.Disabled {
		return "disabled"
	}
	if agent.Mode == "direct" && agent.Protocol != "" && provider.Protocol != "" && agent.Protocol != provider.Protocol {
		return "requires gateway mode"
	}
	return ""
}

func (menu *providerMenu) add(submenu *systray.MenuItem, choice providerChoice, tooltip string, checked bool, unavailable string) {
	if unavailable != "" {
		item := submenu.AddSubMenuItemCheckbox(choice.providerName+" ("+unavailable+")", tooltip, checked)
		item.Disable()
		return
	}
	item := submenu.AddSubMenuItemCheckbox(choice.providerName, tooltip, checked)

	// Remove() closes the channel, so the goroutine of an item the next redraw
	// drops ends with it.
	go func() {
		for range item.ClickedCh {
			select {
			case menu.switches <- choice:
			default:
			}
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

// fingerprintOf is what the submenus are drawn from, so an unchanged menu is
// left standing.
func fingerprintOf(fetched *trayMenu) string {
	parts := []string{}
	for _, entry := range fetched.Agents {
		parts = append(parts, strings.Join([]string{entry.AgentId, entry.Name, entry.Provider, entry.Mode, entry.Protocol}, "\x00"))
	}
	for _, provider := range fetched.Providers {
		parts = append(parts, strings.Join([]string{provider.Id, provider.Name, provider.Protocol, fmt.Sprint(provider.Disabled)}, "\x00"))
	}
	return strings.Join(parts, "\x01")
}
