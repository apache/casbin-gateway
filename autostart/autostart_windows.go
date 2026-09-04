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

package autostart

import (
	"errors"
	"os"

	"golang.org/x/sys/windows/registry"
)

// The per-user Run key, holding the value the launcher's tray checkbox writes:
// it needs no elevation, and Task Manager's Startup tab lists it.
const (
	runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	runKeyName = "CasbinGateway"
)

func enabled() (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	defer key.Close()

	value, _, err := key.GetStringValue(runKeyName)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return value != "", nil
}

func set(launcher string, enable bool) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	if !enable {
		err := key.DeleteValue(runKeyName)
		if errors.Is(err, registry.ErrNotExist) || errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	return key.SetStringValue(runKeyName, `"`+launcher+`"`)
}
