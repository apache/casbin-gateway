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
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

// ResolveHome returns the home directory for a discovered agent owner.
func ResolveHome(target Target) (string, error) {
	return homeOf(target)
}

// homeOf resolves the home directory of the installation owner. Patching writes
// into that home, so guessing here would silently modify the wrong account's
// configuration: an unresolvable owner is an error rather than a fallback to
// whichever account happens to run Gateway.
func homeOf(target Target) (string, error) {
	owner := strings.TrimSpace(target.Owner)
	if owner == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return home, nil
	}

	if account, err := lookupAccount(owner); err == nil {
		if account.HomeDir == "" {
			return "", fmt.Errorf("account %q has no home directory", owner)
		}
		return account.HomeDir, nil
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
