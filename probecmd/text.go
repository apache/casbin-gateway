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
	"os"
	"strings"

	"github.com/apache/casbin-gateway/object"
)

type messages struct {
	starting        string
	models          string
	couldNotProbe   string
	nothingMeasured string
	grade           string
	spent           string
	costUnknown     string
	evidenceHint    string
	cardSaved       string
	pass            string
	warn            string
	fail            string
	skip            string
	gradeMeanings   map[string]string
	caseTitles      map[string]string
}

func (text *messages) gradeMeaning(grade string) string {
	return text.gradeMeanings[grade]
}

// caseTitle translates a shipped case by its name. A case the suite does not
// ship keeps the title it ran under.
func (text *messages) caseTitle(check object.ProbeCheck) string {
	if title, ok := text.caseTitles[check.Case]; ok {
		return title
	}
	if check.Title != "" {
		return check.Title
	}
	return check.Key
}

var english = &messages{
	starting:        "Probing %s (%s). This sends about twenty small requests on your key and can take a minute.\n",
	models:          "Asked for %s, answered by %s",
	couldNotProbe:   "Could not probe: ",
	nothingMeasured: "no check could be measured",
	grade:           "Grade %s · %.1f/100 · %s",
	spent:           "%d requests · %s · %.1fs",
	costUnknown:     "cost unknown",
	evidenceHint:    "Every request and answer is in --json. Gateway re-runs this daily for the providers you add: https://github.com/apache/casbin-gateway",
	cardSaved:       "Card saved to %s\n",
	pass:            "PASS",
	warn:            "WARN",
	fail:            "FAIL",
	skip:            "SKIP",
	gradeMeanings: map[string]string{
		object.ProbeGradeA: "Answers as documented",
		object.ProbeGradeB: "Mostly as documented",
		object.ProbeGradeC: "Several answers do not match",
		object.ProbeGradeD: "Largely does not match",
		object.ProbeGradeF: "Does not answer as this API",
	},
	caseTitles: map[string]string{},
}

var chinese = &messages{
	starting:        "正在检测%s（%s）。会用你的Key发送约20个小请求，最多需要一分钟。\n",
	models:          "请求的模型：%s，实际作答：%s",
	couldNotProbe:   "无法检测：",
	nothingMeasured: "没有任何一项能测出结果",
	grade:           "等级%s · %.1f/100 · %s",
	spent:           "%d个请求 · %s · %.1f秒",
	costUnknown:     "费用未知",
	evidenceHint:    "每个请求和返回都在--json里。把中转站加进Gateway，它每天自动复测：https://github.com/apache/casbin-gateway",
	cardSaved:       "卡片已保存到%s\n",
	pass:            "通过",
	warn:            "存疑",
	fail:            "不符",
	skip:            "跳过",
	gradeMeanings: map[string]string{
		object.ProbeGradeA: "与官方文档一致",
		object.ProbeGradeB: "基本一致",
		object.ProbeGradeC: "多项答案对不上",
		object.ProbeGradeD: "大部分对不上",
		object.ProbeGradeF: "不像这套API",
	},
	caseTitles: map[string]string{
		"identity-model-name":          "模型身份",
		"selfid-vendor":                "自述来历",
		"hidden-system-prompt":         "隐藏指令",
		"repeat-same-backend":          "单一后端还是号池",
		"tools-nested-schema":          "嵌套工具参数",
		"stream-events-anthropic":      "流式事件（Anthropic）",
		"stream-events-openai":         "流式事件（OpenAI）",
		"cache-identical-prefix":       "提示词缓存",
		"billing-identical-requests":   "Token计费",
		"vendor-headers":               "厂商响应头",
		"knowledge-letter-count":       "数字母",
		"knowledge-decimal-order":      "比较小数大小",
		"knowledge-arithmetic":         "不借助工具的算术",
		"knowledge-reverse-string":     "按原序复述",
		"knowledge-atomic-number":      "长尾知识",
		"knowledge-day-of-week":        "推算星期",
		"knowledge-chinese-author":     "中文长尾知识",
		"knowledge-instruction-exact":  "严格遵循指令",
		"feature-logprobs":             "Token概率",
		"feature-choice-count":         "一次返回两个答案",
		"feature-stop-openai":          "停止序列（OpenAI）",
		"feature-stop-anthropic":       "停止序列（Anthropic）",
		"feature-openai-fingerprint":   "后端指纹",
		"feature-openai-completion-id": "补全ID格式",
	},
}

func textFor(lang string) *messages {
	if lang == "" {
		lang = systemLanguage()
	}
	if strings.HasPrefix(strings.ToLower(lang), "zh") {
		return chinese
	}
	return english
}

func languageFromEnv() string {
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if value := os.Getenv(name); value != "" {
			return value
		}
	}
	return ""
}
