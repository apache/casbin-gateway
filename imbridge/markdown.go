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

package imbridge

import (
	"strings"
)

// markdownStyle is what one platform makes of the Markdown an agent writes. The
// reading of it is the same everywhere; only what comes out differs, so a
// platform with rich text and one without share a parser.
type markdownStyle struct {
	escape func(string) string
	bold   func(string) string
	italic func(string) string
	strike func(string) string
	code   func(string) string
	block  func(language, body string) string
	quote  func(lines []string) string
	link   func(label, url string) string
	rule   string
}

// renderMarkdown walks the block shapes: a fenced block, a run of quoted lines,
// or a single line of its own.
func renderMarkdown(text string, style markdownStyle) string {
	out := &strings.Builder{}
	lines := strings.Split(text, "\n")

	for index := 0; index < len(lines); index++ {
		line := lines[index]

		if language, ok := fenceStart(line); ok {
			body := []string{}
			for index++; index < len(lines) && !isFence(lines[index]); index++ {
				body = append(body, lines[index])
			}
			out.WriteString(style.block(language, strings.Join(body, "\n")) + "\n")
			continue
		}

		if isQuote(line) {
			quoted := []string{}
			for ; index < len(lines) && isQuote(lines[index]); index++ {
				quoted = append(quoted, inlineMarkdown(trimQuote(lines[index]), style))
			}
			index--
			out.WriteString(style.quote(quoted) + "\n")
			continue
		}

		out.WriteString(blockLine(line, style) + "\n")
	}
	return strings.TrimRight(out.String(), "\n")
}

// telegramHtml renders into the small HTML subset Telegram parses. HTML is
// chosen over MarkdownV2 because only three characters ever need escaping and
// the tags are written here rather than by the agent, so a marker left open
// halfway through a streamed answer comes out as literal text instead of a
// refused message.
func telegramHtml(text string) string {
	return renderMarkdown(text, markdownStyle{
		escape: escapeHtml,
		bold:   func(inner string) string { return "<b>" + inner + "</b>" },
		italic: func(inner string) string { return "<i>" + inner + "</i>" },
		strike: func(inner string) string { return "<s>" + inner + "</s>" },
		code:   func(inner string) string { return "<code>" + escapeHtml(inner) + "</code>" },
		block: func(language, body string) string {
			if !isPlainWord(language) {
				return "<pre>" + escapeHtml(body) + "</pre>"
			}
			return `<pre><code class="language-` + language + `">` + escapeHtml(body) + "</code></pre>"
		},
		quote: func(lines []string) string { return "<blockquote>" + strings.Join(lines, "\n") + "</blockquote>" },
		link:  func(label, url string) string { return `<a href="` + escapeHtml(url) + `">` + label + "</a>" },
		rule:  "──────────",
	})
}

// plainText drops the markup for a platform that has none, so a chat is not
// shown the asterisks around a word that was meant to be bold. A link keeps its
// address, since there is nothing else left to carry it.
func plainText(text string) string {
	keep := func(inner string) string { return inner }
	return renderMarkdown(text, markdownStyle{
		escape: keep,
		bold:   keep,
		italic: keep,
		strike: keep,
		code:   keep,
		block:  func(language, body string) string { return body },
		quote:  func(lines []string) string { return strings.Join(lines, "\n") },
		link: func(label, url string) string {
			if label == "" || label == url {
				return url
			}
			return label + " " + url
		},
		rule: "──────────",
	})
}

func fenceStart(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "```") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(trimmed, "```")), true
}

func isFence(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "```")
}

func isQuote(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), ">")
}

func trimQuote(line string) string {
	return strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(line), ">"), " ")
}

// blockLine renders the one-line shapes: a heading becomes bold, a list marker
// becomes a bullet, and a rule becomes a line somebody can actually see.
func blockLine(line string, style markdownStyle) string {
	if trimmed := strings.TrimSpace(line); isRule(trimmed) {
		return style.rule
	}

	indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
	rest := line[len(indent):]

	if hashes := len(rest) - len(strings.TrimLeft(rest, "#")); hashes > 0 && hashes <= 6 && strings.HasPrefix(rest[hashes:], " ") {
		return indent + style.bold(inlineMarkdown(strings.TrimSpace(rest[hashes:]), style))
	}

	for _, marker := range []string{"- ", "* ", "+ "} {
		if strings.HasPrefix(rest, marker) {
			return indent + "• " + inlineMarkdown(rest[len(marker):], style)
		}
	}
	return indent + inlineMarkdown(rest, style)
}

func isRule(line string) bool {
	if len(line) < 3 {
		return false
	}
	return strings.Trim(line, "-") == "" || strings.Trim(line, "*") == "" || strings.Trim(line, "_") == ""
}

// inlineMarkdown walks one line and hands each marker it recognises to the
// style. A marker with nothing closing it is written out as the character it is.
func inlineMarkdown(text string, style markdownStyle) string {
	out := &strings.Builder{}
	runes := []rune(text)

	for index := 0; index < len(runes); {
		switch {
		case runes[index] == '`':
			width := runLength(runes, index, '`')
			if end := findRun(runes, index+width, '`', width); end >= 0 {
				out.WriteString(style.code(strings.Trim(string(runes[index+width:end]), " ")))
				index = end + width
				continue
			}

		case hasMarker(runes, index, "**"), hasMarker(runes, index, "__"):
			marker := string(runes[index : index+2])
			if end := findMarker(runes, index+2, marker); end >= 0 {
				out.WriteString(style.bold(inlineMarkdown(string(runes[index+2:end]), style)))
				index = end + 2
				continue
			}

		case hasMarker(runes, index, "~~"):
			if end := findMarker(runes, index+2, "~~"); end >= 0 {
				out.WriteString(style.strike(inlineMarkdown(string(runes[index+2:end]), style)))
				index = end + 2
				continue
			}

		case opensEmphasis(runes, index):
			if end := findEmphasis(runes, index+1, runes[index]); end >= 0 {
				out.WriteString(style.italic(inlineMarkdown(string(runes[index+1:end]), style)))
				index = end + 1
				continue
			}

		case runes[index] == '[':
			if label, url, width := linkAt(runes, index); width > 0 {
				out.WriteString(style.link(inlineMarkdown(label, style), url))
				index += width
				continue
			}
		}

		out.WriteString(style.escape(string(runes[index])))
		index++
	}
	return out.String()
}

func hasMarker(runes []rune, index int, marker string) bool {
	return index+len(marker) <= len(runes) && string(runes[index:index+len(marker)]) == marker
}

func findMarker(runes []rune, from int, marker string) int {
	for index := from; index+len(marker) <= len(runes); index++ {
		if hasMarker(runes, index, marker) && index > from {
			return index
		}
	}
	return -1
}

func runLength(runes []rune, index int, char rune) int {
	width := 0
	for index+width < len(runes) && runes[index+width] == char {
		width++
	}
	return width
}

func findRun(runes []rune, from int, char rune, width int) int {
	for index := from; index < len(runes); index++ {
		if runes[index] == char && runLength(runes, index, char) == width {
			return index
		}
	}
	return -1
}

// opensEmphasis keeps a single marker from opening on a word it is part of, so
// snake_case names and a lone asterisk stay as they were written.
func opensEmphasis(runes []rune, index int) bool {
	char := runes[index]
	if char != '*' && char != '_' {
		return false
	}
	if index+1 >= len(runes) || isSpace(runes[index+1]) {
		return false
	}
	if char == '_' && index > 0 && isWord(runes[index-1]) {
		return false
	}
	return true
}

func findEmphasis(runes []rune, from int, char rune) int {
	for index := from; index < len(runes); index++ {
		if runes[index] != char || isSpace(runes[index-1]) {
			continue
		}
		if char == '_' && index+1 < len(runes) && isWord(runes[index+1]) {
			continue
		}
		return index
	}
	return -1
}

// linkAt reads a [label](url) starting at index and reports how wide it was.
func linkAt(runes []rune, index int) (string, string, int) {
	label := -1
	for scan := index + 1; scan < len(runes); scan++ {
		if runes[scan] == ']' {
			label = scan
			break
		}
	}
	if label < 0 || label+1 >= len(runes) || runes[label+1] != '(' {
		return "", "", 0
	}

	end := -1
	for scan := label + 2; scan < len(runes); scan++ {
		if isSpace(runes[scan]) {
			return "", "", 0
		}
		if runes[scan] == ')' {
			end = scan
			break
		}
	}
	if end < 0 {
		return "", "", 0
	}

	url := string(runes[label+2 : end])
	if !isSafeUrl(url) {
		return "", "", 0
	}
	return string(runes[index+1 : label]), url, end - index + 1
}

// isSafeUrl keeps a link to the schemes a chat should follow, so a javascript:
// href never reaches somebody's client.
func isSafeUrl(url string) bool {
	lowered := strings.ToLower(url)
	for _, scheme := range []string{"http://", "https://", "mailto:", "tg://"} {
		if strings.HasPrefix(lowered, scheme) {
			return true
		}
	}
	return false
}

func isPlainWord(text string) bool {
	for _, char := range text {
		if !isWord(char) && char != '-' && char != '+' && char != '#' {
			return false
		}
	}
	return text != ""
}

func isWord(char rune) bool {
	return char == '_' || char >= '0' && char <= '9' || char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char > 127
}

func isSpace(char rune) bool {
	return char == ' ' || char == '\t' || char == '\n'
}

func escapeHtml(text string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(text)
}
