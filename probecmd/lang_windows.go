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

package probecmd

import "syscall"

// langChinese is the primary language id Windows gives every Chinese locale.
const langChinese = 0x04

// systemLanguage reads the display language, since Windows sets no LANG.
func systemLanguage() string {
	if lang := languageFromEnv(); lang != "" {
		return lang
	}
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultUILanguage")
	if proc.Find() != nil {
		return ""
	}
	id, _, _ := proc.Call()
	if id&0x3ff == langChinese {
		return "zh"
	}
	return "en"
}
