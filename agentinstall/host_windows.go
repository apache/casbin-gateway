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

package agentinstall

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// uninstallerDriver runs the uninstaller the app registered with Windows. Every
// installer records one, so this covers the apps that arrived as a setup
// program rather than from a package manager.
func uninstallerDriver(plan Plan, spec target) (Plan, bool) {
	if spec.action != ActionUninstall || spec.installation.Path == "" {
		return plan, false
	}
	entry, ok := uninstallEntryFor(spec.installation.Path)
	if !ok {
		return plan, false
	}

	program, args := resolveCommandLine(entry.command)
	if program == "" {
		return plan, false
	}
	if !filepath.IsAbs(program) {
		resolved := lookup(program)
		if resolved == "" {
			spec.note("this app registered " + program + " as its uninstaller, which is not on Gateway's PATH")
			return plan, false
		}
		program = resolved
	}

	// A machine-wide uninstaller has to run as an administrator, which a plain
	// start cannot become: Windows refuses it outright. Going through
	// Start-Process raises the consent prompt instead, and waits for the answer.
	if entry.machineWide {
		shell := lookup("powershell")
		if shell == "" {
			spec.note("removing a machine-wide install needs PowerShell, which is not on Gateway's PATH")
			return plan, false
		}
		elevated := fill(plan, ManagerUninstaller, shell, elevatedArgs(program, args)...)
		elevated.Command = entry.command
		elevated.Interactive = true
		elevated.Warning = "this app was installed for every account, so Windows will ask for administrator approval"
		return elevated, true
	}

	built := fill(plan, ManagerUninstaller, program, args...)
	built.Command = entry.command
	// Without a silent switch the uninstaller opens a window of its own, and
	// hiding it would leave the job waiting on a dialog nobody can answer.
	built.Interactive = !entry.quiet
	if built.Interactive {
		built.Warning = "this uninstaller has no silent mode, so it opens its own window to finish in"
	}
	return built, true
}

// elevatedArgs run one program through PowerShell's own launcher, which is what
// raises the Windows consent prompt.
func elevatedArgs(program string, args []string) []string {
	command := "Start-Process -FilePath " + powershellQuote(program)
	if len(args) > 0 {
		quoted := make([]string, 0, len(args))
		for _, arg := range args {
			quoted = append(quoted, powershellQuote(arg))
		}
		command += " -ArgumentList " + strings.Join(quoted, ",")
	}
	command += " -Verb RunAs -Wait"
	return []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", command}
}

// appxDriver removes a Store app, which lives under a directory no account may
// write and is only ever removed through the packaging service.
func appxDriver(plan Plan, spec target) (Plan, bool) {
	family := spec.packages.MsixFamily
	if spec.action != ActionUninstall || family == "" {
		return plan, false
	}
	shell := lookup("powershell")
	if shell == "" {
		spec.note("removing a Microsoft Store app needs PowerShell, which is not on Gateway's PATH")
		return plan, false
	}

	command := "Get-AppxPackage -PackageFamilyName " + powershellQuote(family) + " | Remove-AppxPackage"
	built := fill(plan, ManagerAppx, shell, "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", command)
	built.Command = command
	return built, true
}

// removeCommand deletes a path, through the shell so that what it did is in the
// job's own output rather than only in an error.
func removeCommand(path string, whole bool) (string, []string) {
	shell := lookup("powershell")
	if shell == "" {
		return "", nil
	}
	command := "Remove-Item -LiteralPath " + powershellQuote(path) + " -Force"
	if whole {
		command += " -Recurse"
	}
	return shell, []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", command}
}

func removeDisplay(path string, whole bool) string {
	command := "Remove-Item " + powershellQuote(path) + " -Force"
	if whole {
		command += " -Recurse"
	}
	return command
}

func powershellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

// uninstallEntry is one program's entry in the list Windows keeps for Add or
// remove programs, reduced to what running its uninstaller needs.
type uninstallEntry struct {
	command string
	quiet   bool
	// machineWide marks an entry under the machine's own hive, which only an
	// administrator can remove.
	machineWide bool
}

type uninstallHive struct {
	key         registry.Key
	path        string
	access      uint32
	machineWide bool
}

var uninstallHives = []uninstallHive{
	{registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`, registry.WOW64_64KEY, false},
	{registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`, registry.WOW64_32KEY, false},
	{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`, registry.WOW64_64KEY, true},
	{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`, registry.WOW64_32KEY, true},
}

// uninstallEntryFor finds the entry whose own directory holds this program. An
// entry names where it installed to, so matching on that is what ties a
// discovered launcher to the installer that put it there; the most specific
// directory wins, which keeps one app's entry off another's files.
func uninstallEntryFor(path string) (uninstallEntry, bool) {
	var best uninstallEntry
	bestDepth := -1

	for _, hive := range uninstallHives {
		root, err := registry.OpenKey(hive.key, hive.path, registry.ENUMERATE_SUB_KEYS|hive.access)
		if err != nil {
			continue
		}
		names, err := root.ReadSubKeyNames(-1)
		if err != nil {
			root.Close()
			continue
		}
		for _, name := range names {
			entry, directory, ok := readUninstallEntry(root, name, hive.access, hive.machineWide)
			if !ok || !within(directory, path) {
				continue
			}
			if depth := len(splitPath(directory)); depth > bestDepth {
				best, bestDepth = entry, depth
			}
		}
		root.Close()
	}
	return best, bestDepth >= 0
}

func readUninstallEntry(root registry.Key, name string, access uint32, machineWide bool) (uninstallEntry, string, bool) {
	key, err := registry.OpenKey(root, name, registry.QUERY_VALUE|access)
	if err != nil {
		return uninstallEntry{}, "", false
	}
	defer key.Close()

	// A component Windows installed for something else is not an app anyone
	// uninstalls on its own.
	if system, _, err := key.GetIntegerValue("SystemComponent"); err == nil && system == 1 {
		return uninstallEntry{}, "", false
	}

	quiet, _, _ := key.GetStringValue("QuietUninstallString")
	command, _, _ := key.GetStringValue("UninstallString")
	if quiet != "" {
		command = quiet
	}
	if strings.TrimSpace(command) == "" {
		return uninstallEntry{}, "", false
	}

	location, _, _ := key.GetStringValue("InstallLocation")
	icon, _, _ := key.GetStringValue("DisplayIcon")
	directory := entryDirectory(location, icon, command)
	if directory == "" {
		return uninstallEntry{}, "", false
	}
	return uninstallEntry{command: command, quiet: quiet != "", machineWide: machineWide}, directory, true
}

// entryDirectory is where an entry says its app lives. Not every installer
// records that, so the icon it registered and the uninstaller it left behind
// stand in for it - both sit in the directory the app was installed into.
//
// A drive root, and the few directories every app is installed under, are
// refused whatever they came from: an entry that resolves to one of those would
// claim every installation on the host as its own.
func entryDirectory(location string, icon string, command string) string {
	for _, directory := range []string{
		strings.Trim(strings.TrimSpace(location), `"`),
		iconDirectory(icon),
		programDirectory(command),
	} {
		if directory == "" {
			continue
		}
		if cleaned := filepath.Clean(directory); !isSharedRoot(cleaned) {
			return cleaned
		}
	}
	return ""
}

// isSharedRoot reports a directory that holds programs rather than being one:
// a drive root, or one of the roots a scan looks under.
func isSharedRoot(directory string) bool {
	if len(splitPath(directory)) < 2 {
		return true
	}
	roots := []string{
		os.Getenv("ProgramFiles"),
		os.Getenv("ProgramFiles(x86)"),
		os.Getenv("ProgramW6432"),
		os.Getenv("LOCALAPPDATA"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs"),
		os.Getenv("APPDATA"),
		os.Getenv("USERPROFILE"),
		os.Getenv("SystemRoot"),
	}
	for _, root := range roots {
		if root != "" && strings.EqualFold(filepath.Clean(root), directory) {
			return true
		}
	}
	return false
}

// iconDirectory reads the icon an entry registered, which is recorded as
// "file,index" and sits in the directory the app was installed into.
func iconDirectory(icon string) string {
	path := strings.Trim(strings.TrimSpace(icon), `"`)
	if comma := strings.LastIndex(path, ","); comma > 1 {
		path = path[:comma]
	}
	if !filepath.IsAbs(path) {
		return ""
	}
	return filepath.Dir(path)
}

func programDirectory(command string) string {
	program, _ := resolveCommandLine(command)
	if !filepath.IsAbs(program) {
		return ""
	}
	return filepath.Dir(program)
}

func splitPath(path string) []string {
	parts := strings.Split(filepath.Clean(path), string(filepath.Separator))
	kept := parts[:0]
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	return kept
}

// resolveCommandLine reads a recorded command line the way Windows does. An
// installer that recorded its uninstaller unquoted leaves the space in
// "Program Files" ambiguous, and Windows resolves that by trying each prefix
// until one names a file that is there - so this does too, rather than reading
// "C:\Program" as the program and a drive root as its directory.
func resolveCommandLine(line string) (string, []string) {
	fields := splitCommandLine(line)
	if len(fields) == 0 {
		return "", nil
	}
	if strings.HasPrefix(strings.TrimSpace(line), `"`) {
		return fields[0], fields[1:]
	}

	for taken := 1; taken <= len(fields); taken++ {
		candidate := strings.Join(fields[:taken], " ")
		if isProgram(candidate) || isProgram(candidate+".exe") {
			return candidate, fields[taken:]
		}
	}
	return fields[0], fields[1:]
}

func isProgram(path string) bool {
	if !filepath.IsAbs(path) {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// splitCommandLine breaks a command line on the spaces outside its quotes.
func splitCommandLine(line string) []string {
	var fields []string
	var current strings.Builder
	quoted, started := false, false

	flush := func() {
		if started {
			fields = append(fields, current.String())
			current.Reset()
			started = false
		}
	}
	for i := 0; i < len(line); i++ {
		switch c := line[i]; {
		case c == '"':
			quoted = !quoted
			started = true
		case (c == ' ' || c == '\t') && !quoted:
			flush()
		default:
			current.WriteByte(c)
			started = true
		}
	}
	flush()
	return fields
}
