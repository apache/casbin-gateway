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

//go:build !windows && !linux

package main

// On macOS a URL scheme belongs to an application bundle, and LaunchServices
// delivers the link as an Apple Event rather than as an argument — which this
// launcher has no event loop to receive. Declaring the scheme would therefore
// take links away from whatever can open them and drop them. So nothing is
// claimed here, and a link is imported by pasting it into the import page.

func registerScheme() error { return nil }

func unregisterScheme() error { return nil }
