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

package agentmonitor

import (
	"bufio"
	"errors"
	"io"
)

// readCompleteBoundedLine reads one newline-terminated line without allocating
// more than maximum bytes for it. An incomplete line stays unconsumed so the
// next poll can read it again after its writer finishes it.
func readCompleteBoundedLine(reader *bufio.Reader, maximum int) ([]byte, int64, bool, error) {
	var line []byte
	var size int64
	skipped := false
	for {
		fragment, err := reader.ReadSlice('\n')
		size += int64(len(fragment))
		if !skipped {
			if len(line)+len(fragment) <= maximum {
				line = append(line, fragment...)
			} else {
				line = nil
				skipped = true
			}
		}
		switch {
		case err == nil:
			return line, size, true, nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		case errors.Is(err, io.EOF):
			return nil, 0, false, nil
		default:
			return nil, 0, false, err
		}
	}
}
