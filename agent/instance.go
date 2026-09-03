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

package agent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/apache/casbin-gateway/agenthome"
)

// instancesDir is the directory, under a home, holding the private state of
// every extra copy of an agent. One place for all of them is what lets an
// installation's own status tell its processes from theirs.
var instancesDir = filepath.Join(".casbin-gateway", "agent-instances")

// isolation is how one agent is given a state directory of its own.
type isolation struct {
	arg string
	env string
}

// isolationOf reports how a second copy of this agent is kept apart from the
// first. The zero value means Gateway knows no way, so the agent runs once.
func isolationOf(agentId string) isolation {
	for i := range fingerprints {
		if fingerprints[i].ID == agentId {
			return isolation{arg: fingerprints[i].InstanceArg, env: fingerprints[i].InstanceEnv}
		}
	}
	return isolation{}
}

func (i isolation) supported() bool {
	return i.arg != "" || i.env != ""
}

// SupportsInstances reports whether Gateway can run more than one copy of this
// agent at a time, each signed in to its own account.
func SupportsInstances(agentId string) bool {
	return isolationOf(agentId).supported()
}

// LinkSchemeOf is the URL scheme an agent registers for its own links, which is
// what a browser hands a finished sign-in back through. Empty for an agent that
// has none, or none Gateway knows about.
func LinkSchemeOf(agentId string) string {
	for i := range fingerprints {
		if fingerprints[i].ID == agentId {
			return fingerprints[i].LinkScheme
		}
	}
	return ""
}

// InstancesRoot is where every instance of every agent keeps its state: the
// home of the account Gateway runs as, which is also the account an instance is
// started under. An installation owned by another account - a machine-wide one
// is owned by the system - is still run by this one.
func InstancesRoot() (string, error) {
	return agenthome.Resolve("")
}

// InstanceDir is the state directory of one named instance. The name is what
// the caller chose, so it is checked here rather than trusted into a path.
func InstanceDir(agentId string, name string) (string, error) {
	if err := CheckInstanceName(name); err != nil {
		return "", err
	}
	home, err := InstancesRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, instancesDir, agentId, name), nil
}

// CheckInstanceName keeps an instance name to what is safe as one path segment
// and readable as a label.
func CheckInstanceName(name string) error {
	if name == "" {
		return errors.New("the instance name is empty")
	}
	if len(name) > 64 {
		return errors.New("the instance name is longer than 64 characters")
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-', r == '_':
		default:
			return errors.New("an instance name may hold only letters, digits, - and _")
		}
	}
	return nil
}

// InstanceLaunchOf resolves what to run for one instance of an installation,
// without touching the disk. An agent that takes its state directory on the
// command line is simply given the flag; one that reads it from the environment
// is started through a launcher script inside that directory, because the
// console a CLI opens in would not otherwise carry the variable - and nothing
// on the command line would then say which instance a process belongs to.
func InstanceLaunchOf(installation Installation, dataDir string) (Launch, error) {
	launch := LaunchOf(installation)
	if launch.Executable == "" {
		return Launch{}, errors.New("no launcher was found for this installation")
	}

	rules := isolationOf(installation.AgentId)
	switch {
	case rules.arg != "":
		launch.Args = append(append([]string{}, launch.Args...), rules.arg+"="+dataDir)
		return launch, nil
	case rules.env != "":
		launch.Executable, launch.Args = instanceScriptPath(dataDir), nil
		return launch, nil
	}
	return Launch{}, fmt.Errorf("gateway cannot run a second copy of %s", installation.AgentId)
}

// PrepareInstance creates the state directory of one instance and, for an agent
// isolated by an environment variable, the launcher script that sets it. The
// script is rewritten on every start, so an upgraded agent is never launched
// through a stale path.
//
// An installation with no launcher - one found by the state directory it left
// behind - still gets its directory: what cannot be started is reported by the
// start, not by the instance being registered.
func PrepareInstance(installation Installation, dataDir string) error {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	rules := isolationOf(installation.AgentId)
	launch := LaunchOf(installation)
	if rules.env == "" || launch.Executable == "" {
		return nil
	}
	return writeInstanceScript(dataDir, rules.env, launch)
}

func instanceScriptPath(dataDir string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(dataDir, "launch.cmd")
	}
	return filepath.Join(dataDir, "launch.sh")
}

// writeInstanceScript writes the launcher of one instance beside its state.
func writeInstanceScript(dataDir string, variable string, launch Launch) error {
	lines := []string{}
	if runtime.GOOS == "windows" {
		lines = append(lines, "@echo off", `set "`+variable+"="+dataDir+`"`, windowsCommand(launch), "")
		return os.WriteFile(instanceScriptPath(dataDir), []byte(strings.Join(lines, "\r\n")), 0o700)
	}

	lines = append(lines, "#!/bin/sh", "export "+variable+"="+shellQuote(dataDir), "exec "+shellCommand(launch), "")
	return os.WriteFile(instanceScriptPath(dataDir), []byte(strings.Join(lines, "\n")), 0o700)
}

func windowsCommand(launch Launch) string {
	command := `"` + launch.Executable + `"`
	for _, argument := range launch.Args {
		command += ` "` + argument + `"`
	}
	return command
}

func shellCommand(launch Launch) string {
	command := shellQuote(launch.Executable)
	for _, argument := range launch.Args {
		command += " " + shellQuote(argument)
	}
	return command
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
