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
	"strings"
	"unicode/utf8"

	"github.com/apache/casbin-gateway/object"
)

// The findings, worded short enough for one line. They follow the web UI's
// reading of each check's facts, see ProviderAuditCard.tsx.
var detailPhrases = map[string][2]string{
	"failed":           {"not asked: %s", "没能发出：%s"},
	"empty":            {"empty answer", "回答为空"},
	"wrong":            {"said \"%s\", the answer is %s", "答了“%s”，正确答案是%s"},
	"forbidden":        {"contains \"%s\", which is ruled out", "含有被排除的“%s”"},
	"right":            {"right", "答对"},
	"selfUndocumented": {"no known vendor sells this name", "这个模型名不属于已知厂商"},
	"selfOther":        {"sold as %[2]s's, says it is %[1]s's", "卖的是%[2]s的模型，自称出自%[1]s"},
	"selfSilent":       {"never names %s", "没说自己出自%s"},
	"selfMatch":        {"says it is %s's", "自称出自%s"},
	"hiddenFound":      {"a system prompt nobody sent", "有一段不是你发的系统提示"},
	"hiddenNone":       {"none", "没有"},
	"paramRejected":    {"refused by the upstream, not scored", "被上游拒绝，不计分"},
	"paramIgnored":     {"accepted, no %s in the answer", "收下了，返回里却没有%s"},
	"paramShape":       {"%s is %s, want %s", "%s为%s，应为%s"},
	"paramDropped":     {"accepted, not applied: \"%s\"", "收下了没照做：“%s”"},
	"paramHonored":     {"honored", "照文档生效"},
	"repeatModels":     {"different models: %s", "模型名不一致：%s"},
	"repeatTokens":     {"same input counted as %s", "同样的输入被计为%s"},
	"repeatAnswers":    {"answers differ, which sampling allows", "回答不同，采样允许"},
	"repeatSame":       {"%s identical answers", "%s次完全一致"},
	"vendorUndoc":      {"not asked: not a vendor's own host", "未发问：不是厂商自己的域名"},
	"notAsked":         {"not asked", "没能发出"},
	"noModel":          {"no model name came back", "没有返回模型名"},
	"identityAlias":    {"%s, a documented alias", "%s，厂商文档写明的别名"},
	"identityEcho":     {"%s, echoed back and not checkable", "%s，原样回显，无法核实"},
	"identityOk":       {"%s", "%s"},
	"identityOther":    {"%s answered instead", "实际作答的是%s"},
	"cacheOk":          {"%s written, %s read back", "写入%s，读回%s"},
	"cacheWritten":     {"%s written, none read back", "写入%s，没有读回"},
	"cacheNone":        {"no cache tokens, billed as fresh input", "没有缓存token，全按新输入计费"},
	"billingDrift":     {"same request billed %s and %s", "同一请求计了%s和%s"},
	"billingEstimate":  {"%[1]s billed for ~%[2]s sent", "发出约%[2]s，计了%[1]s"},
	"streamComplete":   {"every event, in order", "事件齐全且有序"},
	"streamMissing":    {"missing: %s", "缺失：%s"},
	"toolsOk":          {"nested schema filled", "嵌套结构完整"},
	"toolsPartial":     {"nested fields left empty", "嵌套字段没填"},
	"toolsNone":        {"no tool call", "没有调用工具"},
	"vendorRelayed":    {"not %s's own host", "不是%s自己的域名"},
	"vendorNone":       {"no vendor header", "没有厂商响应头"},
	"vendorFound":      {"%s", "%s"},
}

func (text *messages) phrase(key string, values ...any) string {
	index := 0
	if text == chinese {
		index = 1
	}
	return fmt.Sprintf(detailPhrases[key][index], values...)
}

func fact(check object.ProbeCheck, index int) string {
	if index < len(check.Facts) {
		return check.Facts[index]
	}
	return ""
}

func quote(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if utf8.RuneCountInString(value) > 32 {
		return string([]rune(value)[:31]) + "…"
	}
	return value
}

func detailOf(check object.ProbeCheck, text *messages) string {
	switch check.Key {
	case object.ProbeKnowledge, object.ProbeSelfId, object.ProbeHidden, object.ProbeFeature, object.ProbeRepeat:
		return answerDetailOf(check, text)
	}
	if check.Level == object.LlmAuditUnknown && check.Key != object.ProbeIdentity {
		if fact(check, 0) == "undocumented" {
			return text.phrase("vendorUndoc")
		}
		return text.phrase("notAsked")
	}

	switch check.Key {
	case object.ProbeIdentity:
		switch {
		case len(check.Facts) == 0:
			return text.phrase("noModel")
		case fact(check, 1) == "alias":
			return text.phrase("identityAlias", fact(check, 0))
		case fact(check, 1) == "unverified":
			return text.phrase("identityEcho", fact(check, 0))
		case check.Level == object.LlmAuditOk:
			return text.phrase("identityOk", fact(check, 0))
		}
		return text.phrase("identityOther", fact(check, 0))
	case object.ProbeCache:
		written, read := fact(check, 0), fact(check, 1)
		if read != "" && read != "0" {
			return text.phrase("cacheOk", written, read)
		}
		if written != "" && written != "0" {
			return text.phrase("cacheWritten", written)
		}
		return text.phrase("cacheNone")
	case object.ProbeBilling:
		if fact(check, 0) == "drift" {
			return text.phrase("billingDrift", fact(check, 1), fact(check, 2))
		}
		return text.phrase("billingEstimate", fact(check, 1), fact(check, 2))
	case object.ProbeStream:
		if len(check.Facts) == 0 {
			return text.phrase("streamComplete")
		}
		return text.phrase("streamMissing", strings.Join(check.Facts, ", "))
	case object.ProbeTools:
		switch check.Level {
		case object.LlmAuditOk:
			return text.phrase("toolsOk")
		case object.LlmAuditWarn:
			return text.phrase("toolsPartial")
		}
		return text.phrase("toolsNone")
	}

	if fact(check, 0) == "relayed" {
		return text.phrase("vendorRelayed", fact(check, 1))
	}
	if len(check.Facts) == 0 {
		return text.phrase("vendorNone")
	}
	return text.phrase("vendorFound", strings.Join(check.Facts, ", "))
}

func answerDetailOf(check object.ProbeCheck, text *messages) string {
	outcome, answer, extra, wanted := fact(check, 0), fact(check, 1), fact(check, 2), fact(check, 3)
	switch outcome {
	case "failed":
		return text.phrase("failed", quote(answer))
	case "empty":
		return text.phrase("empty")
	}

	switch check.Key {
	case object.ProbeKnowledge:
		switch outcome {
		case "missed":
			return text.phrase("wrong", quote(answer), extra)
		case "forbidden":
			return text.phrase("forbidden", extra)
		}
		return text.phrase("right")
	case object.ProbeSelfId:
		switch outcome {
		case "undocumented":
			return text.phrase("selfUndocumented")
		case "other":
			return text.phrase("selfOther", extra, wanted)
		case "silent":
			return text.phrase("selfSilent", extra)
		}
		return text.phrase("selfMatch", extra)
	case object.ProbeHidden:
		if outcome == "hidden" {
			return text.phrase("hiddenFound")
		}
		return text.phrase("hiddenNone")
	case object.ProbeFeature:
		switch outcome {
		case "rejected":
			return text.phrase("paramRejected")
		case "ignored":
			return text.phrase("paramIgnored", answer)
		case "shape":
			return text.phrase("paramShape", answer, extra, wanted)
		case "dropped":
			return text.phrase("paramDropped", quote(answer))
		}
		return text.phrase("paramHonored")
	}

	switch outcome {
	case "model":
		return text.phrase("repeatModels", strings.Join(check.Facts[1:], ", "))
	case "tokens":
		return text.phrase("repeatTokens", strings.Join(check.Facts[1:], ", "))
	case "answers":
		return text.phrase("repeatAnswers")
	}
	return text.phrase("repeatSame", answer)
}
