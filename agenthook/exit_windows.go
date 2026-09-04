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

package agenthook

import (
	"os"
	"syscall"
)

// terminateSelf ends this process without unwinding it. os.Exit hands over to
// the Windows loader, which a thread stuck there holds up for good: that is
// how a hook whose event never arrives outlives its deadline by hours instead
// of by nothing. There is nothing here to unwind - the hook writes no file and
// holds no lock - so the deadline takes the process down outright.
func terminateSelf() {
	if handle, err := syscall.GetCurrentProcess(); err == nil {
		_ = syscall.TerminateProcess(handle, 0)
	}
	os.Exit(0)
}
