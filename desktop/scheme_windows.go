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

//go:build windows

package main

import (
	"errors"
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// schemeRoot is where Windows records what a URL scheme opens with, under the
// account's own class registrations: a per-user handler needs no elevation and
// leaves other accounts on the machine alone.
const schemeRoot = `Software\Classes\` + importScheme

const schemeCommandKey = schemeRoot + `\shell\open\command`

// registerScheme makes this launcher what a "ccswitch://" link opens.
//
// A registration that is already ours is rewritten, because it records a path
// and an update or a move would leave links opening a Gateway that is no longer
// there. One that is somebody else's is taken only the first time, and what it
// replaced is kept for unregisterScheme to put back.
func registerScheme() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}

	registered, err := readSchemeCommand()
	if err != nil {
		return err
	}
	claiming := false
	switch {
	case schemeIsOurs(registered):
	case claimedScheme():
		// Ours once, and not ours now: the scheme was given back on purpose.
		return nil
	default:
		claiming = true
	}

	key, _, err := registry.CreateKey(registry.CURRENT_USER, schemeRoot, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	if err := key.SetStringValue("", "URL:"+shortcutName); err != nil {
		return err
	}
	// An empty "URL Protocol" value is what marks a class as a scheme handler;
	// without it the shell will not open links with this key at all.
	if err := key.SetStringValue("URL Protocol", ""); err != nil {
		return err
	}

	command, _, err := registry.CreateKey(registry.CURRENT_USER, schemeCommandKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer command.Close()
	if err := command.SetStringValue("", schemeCommand(executable)); err != nil {
		return err
	}

	// Recorded once the scheme is actually ours, so a registration that failed
	// halfway is tried again on the next start rather than counting as the one
	// claim Gateway makes.
	if claiming {
		return os.WriteFile(schemeMarker(), []byte(registered), 0o644)
	}
	return nil
}

// unregisterScheme gives the scheme back to whatever held it before Gateway,
// and takes it off the account where nothing did. A registration that is not
// ours is left exactly as it is.
func unregisterScheme() error {
	registered, err := readSchemeCommand()
	if err != nil {
		return err
	}
	if !schemeIsOurs(registered) {
		return nil
	}

	replaced, _ := os.ReadFile(schemeMarker())
	if previous := strings.TrimSpace(string(replaced)); previous != "" {
		key, _, err := registry.CreateKey(registry.CURRENT_USER, schemeCommandKey, registry.SET_VALUE)
		if err != nil {
			return err
		}
		defer key.Close()
		if err := key.SetStringValue("", previous); err != nil {
			return err
		}
		return os.Remove(schemeMarker())
	}

	for _, path := range []string{schemeCommandKey, schemeRoot + `\shell\open`, schemeRoot + `\shell`, schemeRoot} {
		if err := registry.DeleteKey(registry.CURRENT_USER, path); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return err
		}
	}
	if err := os.Remove(schemeMarker()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func readSchemeCommand() (string, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, schemeCommandKey, registry.QUERY_VALUE)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	defer key.Close()

	value, _, err := key.GetStringValue("")
	if errors.Is(err, registry.ErrNotExist) {
		return "", nil
	}
	return value, err
}

func schemeIsOurs(command string) bool {
	return strings.Contains(strings.ToLower(command), openLinkSubcommand)
}

func schemeCommand(executable string) string {
	return `"` + executable + `" ` + openLinkSubcommand + ` "%1"`
}
