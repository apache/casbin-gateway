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
	"path/filepath"
	"strings"
)

// importScheme is the URL scheme a vendor's "add this to my agent manager"
// button opens. It is CC Switch's, because that is the one vendors already
// publish buttons for, and holding it is what makes those buttons work here.
const importScheme = "ccswitch"

// openLinkSubcommand is how this executable is invoked as the handler of one
// such link.
const openLinkSubcommand = "open-link"

// importPagePath is the page that shows what a link carries and asks whether
// to keep it.
const importPagePath = "/import"

// maxLinkLength mirrors object.MaxImportLinkLength on the server: what arrives
// here is a URL a browser handed to a command line, and nothing longer.
const maxLinkLength = 8192

func isImportLink(argument string) bool {
	return strings.HasPrefix(strings.ToLower(argument), importScheme+"://")
}

// importLinkArg is the link this process was started to open, if it was. The
// Windows registration names the subcommand before the URL; a Linux desktop
// entry started with %u passes the URL on its own, so both shapes arrive here.
func importLinkArg(args []string) string {
	if len(args) > 2 && args[1] == openLinkSubcommand {
		return args[2]
	}
	if len(args) > 1 && isImportLink(args[1]) {
		return args[1]
	}
	return ""
}

// openImportLink hands one clicked link to the Gateway server and opens the
// page that asks what to do with it.
//
// The link travels in the body of an API call rather than in the address of
// that page: a provider link carries an API key, and an address is kept in
// history and handed on to wherever the page navigates next.
func openImportLink(link string) error {
	if !isImportLink(link) || len(link) > maxLinkLength {
		return fmt.Errorf("%q is not a link Gateway can import", link)
	}

	if _, err := startServer(httpPort()); err != nil {
		return err
	}
	if err := openImportLinkOnServer(link); err != nil {
		return err
	}
	return showImportPage()
}

// showImportPage opens a window on the import page, and the default browser
// where no window could be created. It is a window of its own: the browser
// started this process, which has no hold on the one the tray may already have
// open.
func showImportPage() error {
	address := gatewayUrl() + importPagePath

	executable, err := os.Executable()
	if err == nil {
		cmd := exec.Command(executable, "window", address)
		cmd.Dir = gatewayHome()
		hideConsole(cmd)
		if err = cmd.Run(); err == nil {
			return nil
		}
	}
	return openBrowser(address)
}

// schemeMarker records that Gateway has claimed the scheme once, and holds the
// command it took it from. CC Switch hands out these same links and may already
// hold the scheme here; taking it is a thing to do once, on the way in, and to
// undo on the way out - not something to redo behind someone who has pointed
// the scheme back at whatever they prefer.
func schemeMarker() string {
	return filepath.Join(gatewayHome(), ".scheme-"+importScheme)
}

func claimedScheme() bool {
	_, err := os.Stat(schemeMarker())
	return err == nil
}
