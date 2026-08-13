// Copyright 2025 The casbin Authors. All Rights Reserved.
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

package agent

import (
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// executableVersion reads a Windows executable's version resource.
func executableVersion(path string) string {
	size, err := windows.GetFileVersionInfoSize(path, nil)
	if err != nil || size == 0 {
		return ""
	}
	block := make([]byte, size)
	if windows.GetFileVersionInfo(path, 0, size, unsafe.Pointer(&block[0])) != nil {
		return ""
	}

	for _, language := range versionLanguages(block) {
		for _, name := range []string{"ProductVersion", "FileVersion"} {
			if value := versionString(block, language, name); value != "" {
				return value
			}
		}
	}
	return ""
}

func versionLanguages(block []byte) []string {
	var translations *[2]uint16
	var size uint32
	err := windows.VerQueryValue(unsafe.Pointer(&block[0]), `\VarFileInfo\Translation`, unsafe.Pointer(&translations), &size)
	if err != nil || translations == nil || size < 4 {
		return []string{"040904b0"}
	}

	count := int(size / 4)
	pairs := unsafe.Slice(translations, count)
	languages := make([]string, 0, count)
	for _, pair := range pairs {
		languages = append(languages, fmt.Sprintf("%04x%04x", pair[0], pair[1]))
	}
	return languages
}

func versionString(block []byte, language, name string) string {
	var value *uint16
	var size uint32
	subBlock := `\StringFileInfo\` + language + `\` + name
	if windows.VerQueryValue(unsafe.Pointer(&block[0]), subBlock, unsafe.Pointer(&value), &size) != nil || size == 0 {
		return ""
	}
	return strings.TrimSpace(windows.UTF16ToString(unsafe.Slice(value, size)))
}
