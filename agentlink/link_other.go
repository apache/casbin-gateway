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

//go:build !windows

package agentlink

// A URL scheme is claimed here by the application bundle or the desktop entry
// that owns it, neither of which Gateway can hand one link to and give back, so
// nothing is captured on these hosts.

func schemesSupported() bool { return false }

func readHandler(string) (string, error) { return "", ErrUnsupported }

func writeHandler(string, string) error { return ErrUnsupported }

func handlerCommand() (string, error) { return "", ErrUnsupported }

func start(string, []string) error { return ErrUnsupported }

func openWith(string, string) error { return ErrUnsupported }

func detachConsole() {}
