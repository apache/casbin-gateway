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

package agentsession

import (
	"bufio"
	"strings"
)

// parseText takes whatever the agent printed as the answer. Lines are sent on
// as they arrive rather than gathered up, so a long answer appears while it is
// being written instead of all at once at the end.
func parseText(scanner *bufio.Scanner, emit func(Event)) error {
	said := false
	for scanner.Scan() {
		line := scanner.Text()
		// The leading blank lines are the agent clearing its own progress
		// output, and are not part of what it said.
		if !said && strings.TrimSpace(line) == "" {
			continue
		}
		said = true
		emit(textEvent(EventText, line+"\n"))
	}
	return scanner.Err()
}
