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

// Package agenthome resolves the home directory of an agent installation's
// owner. Both the monitoring patches and the skill/MCP configuration reader
// write into that home, and neither should own the account lookup.
package agenthome

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

// Resolve returns the home directory of owner. Callers write into that home, so
// guessing here would silently modify the wrong account's configuration: an
// unresolvable owner is an error rather than a fallback to whichever account
// happens to run Gateway.
func Resolve(owner string) (string, error) {
	owner = strings.TrimSpace(owner)
	// A machine-wide installation is stamped with the local system account
	// rather than a person, and its configuration belongs to whoever is using it.
	if machineWideOwner(owner) {
		owner = ""
	}
	if owner == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return home, nil
	}

	if account, err := lookupAccount(owner); err == nil {
		return usableHome(owner, account.HomeDir)
	}

	// user.Lookup is unreliable for domain-qualified names on Windows and for
	// directory-backed accounts on Linux, so fall back to the process home only
	// when the owner really is the account Gateway runs as.
	if current, err := user.Current(); err == nil && sameAccount(owner, current.Username) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory of %q: %w", owner, err)
		}
		return home, nil
	}
	return "", fmt.Errorf("cannot resolve the home directory of %q; run Gateway as that user to patch this installation", owner)
}

// usableHome expands a home the operating system reported and refuses one that
// is still relative, which would write an agent's configuration under Gateway's
// working directory and report success.
func usableHome(owner string, home string) (string, error) {
	home = strings.TrimSpace(expandHome(home))
	if home == "" {
		return "", fmt.Errorf("account %q has no home directory", owner)
	}
	if !filepath.IsAbs(home) {
		return "", fmt.Errorf("account %q reports %q as its home directory, which is not an absolute path", owner, home)
	}
	return filepath.Clean(home), nil
}

// lookupAccount resolves an owner name, retrying with the bare account name so
// that DOMAIN\user and user@domain forms also resolve.
func lookupAccount(owner string) (*user.User, error) {
	account, err := user.Lookup(owner)
	if err == nil {
		return account, nil
	}
	if bare := accountName(owner); bare != owner && bare != "" {
		if account, bareErr := user.Lookup(bare); bareErr == nil {
			return account, nil
		}
	}
	return nil, err
}

// SameAccount reports whether two owner names name the same account, ignoring
// any DOMAIN\ or @domain qualifier one of them carries.
func SameAccount(left, right string) bool {
	return sameAccount(left, right)
}

func sameAccount(left, right string) bool {
	return strings.EqualFold(accountName(left), accountName(right))
}

// accountName strips any domain qualifier from DOMAIN\user or user@domain.
func accountName(value string) string {
	value = strings.TrimSpace(value)
	if index := strings.LastIndexAny(value, `\/`); index >= 0 {
		value = value[index+1:]
	}
	if index := strings.Index(value, "@"); index > 0 {
		value = value[:index]
	}
	return filepath.Base(value)
}
