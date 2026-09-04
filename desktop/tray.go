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
	"os/exec"
	"runtime"
	"sync"
	"time"

	"fyne.io/systray"

	"github.com/apache/casbin-gateway/desktop/internal/assets"
)

// ownsServer records that the launcher is what started the server, which is the
// only case where quitting is allowed to stop it. A server that was already
// running — as a login item, a service, or from a terminal — outlives the tray.
var ownsServer struct {
	sync.Mutex
	value bool
}

func runTray() {
	go func() {
		// An archive that was unpacked by hand has no way in but the executable
		// itself, so the first start is what gives it one.
		ensureShortcuts()

		// Unlike the shortcuts, this is reasserted on every start: the handler
		// records a path, and an update or a move would otherwise leave links
		// opening a Gateway that is no longer there. It follows the shortcuts
		// because on Linux the desktop entry is what carries the registration.
		if err := registerScheme(); err != nil {
			fmt.Fprintln(os.Stderr, "casbin-gateway-desktop: could not claim ccswitch:// links:", err)
		}
	}()

	// The tray menu is drawn by this process too, so it blurs on a scaled
	// display for the same reason the window did.
	enableDpiAwareness()

	// Left click opens the window; right click keeps the menu, which is what
	// leaving the secondary handler unset means on Windows and macOS. This has
	// to be set before Run: Linux decides from it, once, whether the icon is a
	// menu button or a clickable item.
	systray.SetOnTapped(showWindow)
	setSecondaryTapped()

	systray.Run(onTrayReady, func() {})
}

func onTrayReady() {
	setTrayIcon()
	systray.SetTitle("")
	systray.SetTooltip("Casbin Gateway")

	mOpen := systray.AddMenuItem("Open Casbin Gateway", "Show the management window")
	mBrowser := systray.AddMenuItem("Open in Browser", "Show the management UI in the default browser")
	systray.AddSeparator()
	mProviders := systray.AddMenuItem("Switch Provider (unavailable)", "Point an agent at another provider")
	mProviders.Disable()
	providers := newProviderMenu(mProviders)
	systray.AddSeparator()
	mStatus := systray.AddMenuItem("Starting...", "")
	mStatus.Disable()
	systray.AddSeparator()
	mAutostart := systray.AddMenuItemCheckbox("Start at Login", "Run Casbin Gateway when you log in", autostartEnabled())
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Close the window and leave the tray")

	// The Settings page writes the same login entry, so the checkbox follows it
	// rather than showing what was true when the menu was built.
	go watchAutostart(mAutostart)

	port := httpPort()

	// The window is opened from a goroutine so that the tray answers clicks
	// while the server is still starting up.
	go func() {
		started, err := startServer(port)
		if err != nil {
			// The status item keeps the reason: a launcher started from a
			// shortcut has nowhere else to report it.
			mStatus.SetTitle(fmt.Sprintf("Not running: %v", err))
			return
		}
		ownsServer.Lock()
		ownsServer.value = started
		ownsServer.Unlock()

		go watchStatus(mStatus, port)
		go providers.watch()
		showWindow()
	}()

	for {
		select {
		case <-mOpen.ClickedCh:
			showWindow()
		case <-mBrowser.ClickedCh:
			if err := openBrowser(gatewayUrl()); err != nil {
				fmt.Fprintln(os.Stderr, "casbin-gateway-desktop:", err)
			}
		case <-mAutostart.ClickedCh:
			toggleAutostart(mAutostart)
		case <-mQuit.ClickedCh:
			quit()
			return
		}
	}
}

func setTrayIcon() {
	// Windows wants an icon container; the macOS menu bar and the Linux tray
	// want a plain PNG.
	if runtime.GOOS == "windows" {
		systray.SetIcon(assets.AppIcon)
		return
	}
	systray.SetIcon(assets.TrayIcon)
}

func watchStatus(item *systray.MenuItem, port int) {
	for {
		if isServing(port) {
			item.SetTitle(fmt.Sprintf("Running on port %d", port))
		} else {
			item.SetTitle("Not running")
		}
		time.Sleep(probeInterval)
	}
}

func watchAutostart(item *systray.MenuItem) {
	for {
		if autostartEnabled() {
			item.Check()
		} else {
			item.Uncheck()
		}
		time.Sleep(probeInterval)
	}
}

func toggleAutostart(item *systray.MenuItem) {
	if item.Checked() {
		if err := setAutostart(false); err != nil {
			fmt.Fprintln(os.Stderr, "casbin-gateway-desktop:", err)
			return
		}
		item.Uncheck()
		return
	}

	if err := setAutostart(true); err != nil {
		fmt.Fprintln(os.Stderr, "casbin-gateway-desktop:", err)
		return
	}
	item.Check()
}

func quit() {
	closeWindow()
	releaseInstance()

	ownsServer.Lock()
	owned := ownsServer.value
	ownsServer.Unlock()

	if owned {
		if err := stopServer(); err != nil {
			fmt.Fprintln(os.Stderr, "casbin-gateway-desktop: could not stop the Gateway server:", err)
		}
	}
	systray.Quit()
}

// openBrowser is the fallback for hosts where no webview could be created, and
// the "Open in Browser" item for everyone else.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	hideConsole(cmd)
	return cmd.Start()
}
