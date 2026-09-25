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

package probecmd

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/apache/casbin-gateway/object"
)

const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiDim    = "\x1b[2m"
	ansiRed    = "\x1b[31m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
)

// useColor is off for a pipe, for NO_COLOR, and for a Windows console that is
// not Windows Terminal, which prints the escapes instead of reading them.
func useColor(out io.Writer) bool {
	file, ok := out.(*os.File)
	if !ok || os.Getenv("NO_COLOR") != "" {
		return false
	}
	info, err := file.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	return runtime.GOOS != "windows" || os.Getenv("WT_SESSION") != ""
}

func printReport(out io.Writer, probe *object.ProviderProbe, text *messages) {
	color := useColor(out)
	paint := func(code string, value string) string {
		if !color {
			return value
		}
		return code + value + ansiReset
	}

	if !probe.Ok || probe.Grade == object.ProbeGradeUnknown {
		reason := probe.Error
		if reason == "" {
			reason = text.nothingMeasured
		}
		fmt.Fprintln(out, paint(ansiRed, text.couldNotProbe+reason))
		return
	}

	answered := probe.UpstreamModel
	if answered == "" {
		answered = "-"
	}
	fmt.Fprintf(out, text.models+"\n\n", probe.Model, answered)

	width := 0
	for _, check := range probe.Checks {
		width = max(width, textWidth(text.caseTitle(check)))
	}
	for _, check := range probe.Checks {
		title := text.caseTitle(check)
		padding := strings.Repeat(" ", width-textWidth(title))
		fmt.Fprintf(out, "  %s  %s%s  %s\n", levelTag(check.Level, text, paint), title, padding,
			paint(ansiDim, detailOf(check, text)))
	}

	gradeColor := ansiGreen
	switch probe.Grade {
	case object.ProbeGradeC:
		gradeColor = ansiYellow
	case object.ProbeGradeD, object.ProbeGradeF:
		gradeColor = ansiRed
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, paint(ansiBold+gradeColor, fmt.Sprintf(text.grade, probe.Grade, probe.Score, text.gradeMeaning(probe.Grade))))

	cost := text.costUnknown
	if probe.Priced {
		cost = fmt.Sprintf("$%.4f", probe.Cost)
	}
	fmt.Fprintf(out, text.spent+"\n", probe.Requests, cost, float64(probe.DurationMs)/1000)
	fmt.Fprintln(out, paint(ansiDim, text.evidenceHint))
}

func levelTag(level string, text *messages, paint func(string, string) string) string {
	switch level {
	case object.LlmAuditOk:
		return paint(ansiGreen, text.pass)
	case object.LlmAuditWarn:
		return paint(ansiYellow, text.warn)
	case object.LlmAuditAlert:
		return paint(ansiRed, text.fail)
	default:
		return paint(ansiDim, text.skip)
	}
}

// textWidth counts a CJK character as two columns, which is how a terminal
// draws it.
func textWidth(value string) int {
	width := 0
	for _, r := range value {
		if r >= 0x1100 && (r <= 0x115f || (r >= 0x2e80 && r <= 0xa4cf) || (r >= 0xac00 && r <= 0xd7a3) ||
			(r >= 0xf900 && r <= 0xfaff) || (r >= 0xff00 && r <= 0xff60) || (r >= 0xffe0 && r <= 0xffe6)) {
			width += 2
		} else {
			width++
		}
	}
	return width
}
