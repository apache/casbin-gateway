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

// The suite Gateway ships. Every case here is a question with an answer that
// can be checked against something written down — what the vendor's API
// documents, or a fact that does not change — which is what makes a finding
// something to send to the provider rather than something to believe.

package object

import "encoding/json"

// The weights the shipped suite is balanced on. What an upstream is is worth
// more than how it behaves: a real API in front of a cheaper model answers the
// envelope questions perfectly, because a real API is what is answering them.
const (
	probeWeightIdentity  = 25
	probeWeightSelfId    = 15
	probeWeightRepeat    = 15
	probeWeightCache     = 15
	probeWeightBilling   = 15
	probeWeightTools     = 12
	probeWeightHidden    = 10
	probeWeightFeature   = 10
	probeWeightVendor    = 10
	probeWeightStream    = 8
	probeWeightKnowledge = 5
)

func builtInProbeCases() []*ProbeCase {
	schema, err := json.MarshalIndent(probeToolSchema(), "", "  ")
	if err != nil {
		schema = []byte("{}")
	}

	cases := []*ProbeCase{
		{
			Name:        "identity-model-name",
			DisplayName: "Model identity",
			Check:       ProbeIdentity,
			Enabled:     true,
			Weight:      probeWeightIdentity,
			Sort:        10,
			BuiltIn:     true,
			Question:    "Does the model that answers name itself as the one that was asked for?",
			Method: "The first request names a model. Every completion API answers with the model that " +
				"ran, and a version or date suffix on the same family counts as a match. Where the " +
				"name asked for is one the vendor documents as moving — deepseek-chat, anything " +
				"ending in -latest — the answer only has to be a model of that same vendor, which is " +
				"what the vendor says it will be. A mismatch is decisive. A match is worth much less " +
				"than it looks: that field is written by whatever answered, so on an endpoint that is " +
				"not the vendor's own it is the request being read back to you, and is scored as half " +
				"an answer rather than a whole one.",
		},
		{
			Name:        "selfid-vendor",
			DisplayName: "Self-reported maker",
			Check:       ProbeSelfId,
			Enabled:     true,
			Weight:      probeWeightSelfId,
			Sort:        11,
			BuiltIn:     true,
			Question:    "Asked who made it, does the model name the vendor whose model it was sold as?",
			Method: "The model name in the response is whatever the upstream typed there. What the " +
				"model says about itself is not: it comes out of the weights that are actually running. " +
				"A model sold as one vendor's that names another vendor is the finding this suite " +
				"exists for. Naming neither is a note — a system prompt can silence a model without " +
				"making it lie. This is only asked where the name sold belongs to a vendor whose " +
				"models this build knows.",
			Params: ProbeCaseParams{Prompt: probeSelfIdPrompt, MaxTokens: 256},
		},
		{
			Name:        "hidden-system-prompt",
			DisplayName: "Hidden instructions",
			Check:       ProbeHidden,
			Enabled:     true,
			Weight:      probeWeightHidden,
			Sort:        12,
			BuiltIn:     true,
			Question:    "Is anything put in front of the request that this gateway did not send?",
			Method: "The request carries no system prompt, so there is nothing above it to repeat. An " +
				"upstream that answers to a name that is not its own is usually being told to, by a " +
				"prompt injected in front of every request; asked to repeat what is above, a model " +
				"either shows it or refuses. Neither proves anything alone, so this never reaches an " +
				"alert — it reports what came back, and the answer is on the report to read.",
			Params: ProbeCaseParams{Prompt: probeHiddenPrompt, MaxTokens: 512, Expect: []string{"NONE"}},
		},
		{
			Name:        "repeat-same-backend",
			DisplayName: "One backend or a pool",
			Check:       ProbeRepeat,
			Enabled:     true,
			Weight:      probeWeightRepeat,
			Sort:        13,
			BuiltIn:     true,
			Question:    "Do several identical requests come back from the same model?",
			Method: "The same request is sent three times. A reseller that fills part of its traffic " +
				"from the model it sold and the rest from something cheaper cannot hold two things " +
				"steady across them: the model name on the answer, and the input token count, which " +
				"is a property of the tokenizer and so of the model itself. Either one moving is a " +
				"finding. The answers differing is only a note, since sampling is allowed to differ.",
			Params: ProbeCaseParams{Prompt: probeAskPrompt, Samples: probeRepeatSamples, MaxTokens: 256},
		},
		{
			Name:        "tools-nested-schema",
			DisplayName: "Nested tool schema",
			Check:       ProbeTools,
			Enabled:     true,
			Weight:      probeWeightTools,
			Sort:        20,
			BuiltIn:     true,
			Question:    "Can what answers hold a two-level tool schema when the call is forced?",
			Method: "The tool call is forced, so refusing is not an option, and the schema requires " +
				"fields at two levels including an array of objects. A smaller model standing in for " +
				"the one that was sold usually flattens the object or drops the array.",
			Params: ProbeCaseParams{
				System:    probeToolSystem,
				Prompt:    probeToolPrompt,
				ToolName:  probeToolName,
				Schema:    string(schema),
				MaxTokens: probeToolMaxTokens,
			},
		},
		{
			Name:        "stream-events-anthropic",
			DisplayName: "Stream shape (Anthropic)",
			Check:       ProbeStream,
			Protocol:    ProtocolAnthropic,
			Enabled:     true,
			Weight:      probeWeightStream,
			Sort:        30,
			BuiltIn:     true,
			Question:    "Does the event stream carry every event this API documents, in order?",
			Method: "The same request is sent with streaming on. A backend translating some other API " +
				"into this one gets the envelope wrong long before it gets the text wrong: the opening " +
				"and closing events go missing, or message_start carries no input count because " +
				"nothing has been counted yet.",
			Params: ProbeCaseParams{
				Prompt:    "Reply with the single word: ready.",
				MaxTokens: probeStreamMaxTokens,
				Events:    probeAnthropicEvents,
			},
		},
		{
			Name:        "stream-events-openai",
			DisplayName: "Stream shape (OpenAI)",
			Check:       ProbeStream,
			Protocol:    ProtocolOpenAi,
			Enabled:     true,
			Weight:      probeWeightStream,
			Sort:        31,
			BuiltIn:     true,
			Question:    "Does the event stream carry chunks, a finish reason and the closing marker?",
			Method: "The same request is sent with streaming on and usage asked for. The three parts " +
				"this API documents are the completion chunks, a finish reason on the last one and the " +
				"[DONE] marker. Missing usage is a note rather than a finding: not every " +
				"OpenAI-compatible vendor answers it.",
			Params: ProbeCaseParams{
				Prompt:    "Reply with the single word: ready.",
				MaxTokens: probeStreamMaxTokens,
			},
		},
		{
			Name:        "cache-identical-prefix",
			DisplayName: "Prompt cache",
			Check:       ProbeCache,
			Enabled:     true,
			Weight:      probeWeightCache,
			Sort:        40,
			BuiltIn:     true,
			Question:    "Is the prompt cache real, or is a cached prefix billed as fresh input twice?",
			Method: "One long, byte-identical prompt is sent twice a second and a half apart. A real " +
				"cache writes on the first request and reads back on the second. Nothing coming back " +
				"in either counter means the whole prefix was billed as fresh input both times.",
			Params: ProbeCaseParams{
				Prompt:      probeCachePrompt,
				MaxTokens:   probeCacheMaxTokens,
				FillerChars: probeFillerChars,
				GapMs:       int(probeCacheGap.Milliseconds()),
			},
		},
		{
			Name:        "billing-identical-requests",
			DisplayName: "Token billing",
			Check:       ProbeBilling,
			Enabled:     true,
			Weight:      probeWeightBilling,
			Sort:        50,
			BuiltIn:     true,
			Question:    "Are two byte-identical requests billed the same, and near what was sent?",
			Method: "The two cache requests were the same bytes, so the input counts have to agree; " +
				"counts that drift apart were not counted but invented. The counts are then compared " +
				"against the bytes actually sent, at roughly four characters to the token, so only a " +
				"wide gap is treated as a finding.",
			Params: ProbeCaseParams{
				DriftTolerance: 0.02,
				WarnHigh:       1.5,
				AlertHigh:      2.5,
				WarnLow:        0.7,
				AlertLow:       0.4,
			},
		},
		{
			Name:        "vendor-headers",
			DisplayName: "Vendor headers",
			Check:       ProbeVendor,
			Enabled:     true,
			Weight:      probeWeightVendor,
			Sort:        60,
			BuiltIn:     true,
			Question:    "Do the response headers of the vendor's own API come back?",
			Method: "This is asked only of an endpoint that is a vendor's own, against the headers " +
				"that vendor documents: speaking an OpenAI-compatible API is not a claim to be OpenAI. " +
				"A reseller in front of the real thing usually passes some through and a backend that " +
				"never talked to the vendor has none to pass, but stripping headers is also just being " +
				"tidy, so this case never reaches an alert on its own. An endpoint selling a vendor's " +
				"model names that is not that vendor's own host is a relay by definition, and is " +
				"reported as one: nothing on it can be checked against the vendor it is selling. " +
				"Leave the header list empty to use the vendor's own, or name headers here to ask for " +
				"those instead.",
			Params: ProbeCaseParams{MinHeaders: 2},
		},
	}

	return append(cases, append(builtInProbeKnowledgeCases(), builtInProbeFeatureCases()...)...)
}

// builtInProbeKnowledgeCases are the test bank: questions with one right answer
// that does not move. They are what a substituted model fails and an echoed
// model name cannot cover for — and they are rows, so the bank is meant to be
// added to as the questions that separate one tier of model from another change.
func builtInProbeKnowledgeCases() []*ProbeCase {
	bank := []struct {
		name     string
		title    string
		sort     int
		question string
		method   string
		prompt   string
		expect   []string
		match    string
	}{
		{
			name:     "knowledge-letter-count",
			title:    "Letter counting",
			sort:     70,
			question: "How many times does the letter r appear in the word strawberry?",
			method: "Counting letters inside a word is work a model has to do against its own " +
				"tokenizer rather than recall, which is why it separates the tiers so cleanly. The " +
				"answer is three.",
			prompt: "How many times does the letter r appear in the word strawberry? " +
				"Reply with only the digit.",
			expect: []string{"3"},
			match:  "exact",
		},
		{
			name:     "knowledge-decimal-order",
			title:    "Which decimal is larger",
			sort:     71,
			question: "Which is larger, 9.11 or 9.9?",
			method: "Read as version numbers 9.11 is the later one, and a model that has seen more " +
				"changelogs than arithmetic says so. The answer is 9.9.",
			prompt: "Which number is larger, 9.11 or 9.9? Reply with only that number.",
			expect: []string{"9.9"},
			match:  "exact",
		},
		{
			name:     "knowledge-arithmetic",
			title:    "Arithmetic without a tool",
			sort:     72,
			question: "What is 3947 multiplied by 8261?",
			method: "Multiplying two four-digit numbers in one pass is a capability floor: the answer " +
				"is 32606167, and a model that cannot hold the carries writes a number near it.",
			prompt: "What is 3947 multiplied by 8261? Reply with only the digits of the result.",
			expect: []string{"32606167", "32,606,167"},
		},
		{
			name:     "knowledge-reverse-string",
			title:    "Exact-order recall",
			sort:     73,
			question: "What is the word probe written backwards?",
			method: "Reversing a word needs the letters themselves rather than the token, which is " +
				"the same weakness the counting case reaches from the other side. The answer is eborp.",
			prompt: "Write the word probe with its letters in reverse order. Reply with only that word.",
			expect: []string{"eborp"},
			match:  "exact",
		},
		{
			name:     "knowledge-atomic-number",
			title:    "Long-tail recall",
			sort:     74,
			question: "What is the atomic number of tungsten?",
			method: "A fact from far enough down the tail that a small model guesses at it. The answer " +
				"is 74.",
			prompt: "What is the atomic number of tungsten? Reply with only the number.",
			expect: []string{"74"},
			match:  "exact",
		},
		{
			name:     "knowledge-day-of-week",
			title:    "Dates without a calendar",
			sort:     75,
			question: "If today is Wednesday, what day is it 100 days from now?",
			method: "One hundred days is fourteen weeks and two days, so the answer is Friday. It is " +
				"two steps, and a model standing in for a larger one usually takes only the first.",
			prompt: "If today is Wednesday, what day of the week will it be 100 days from today? " +
				"Reply with only the name of the day.",
			expect: []string{"friday", "星期五", "周五"},
		},
		{
			name:     "knowledge-chinese-author",
			title:    "Chinese long-tail recall",
			sort:     76,
			question: "Who wrote the Preface to the Pavilion of Prince Teng?",
			method: "A model trained mostly in English answers this one with the wrong Tang poet. The " +
				"answer is Wang Bo. Asking in Chinese also measures whether the answer comes back in " +
				"the language it was asked in.",
			prompt: "《滕王阁序》的作者是谁？只回答姓名。",
			expect: []string{"王勃"},
		},
		{
			name:     "knowledge-instruction-exact",
			title:    "Following an exact instruction",
			sort:     77,
			question: "Asked for three characters and nothing else, does anything else come back?",
			method: "The instruction leaves no room: three characters, no punctuation, no sentence " +
				"around them. A model standing in for a larger one adds a greeting or an explanation, " +
				"which is the cheapest instruction-following test there is.",
			prompt: "Reply with exactly these three characters and nothing else: A-1",
			expect: []string{"A-1"},
			match:  "exact",
		},
	}

	cases := []*ProbeCase{}
	for _, entry := range bank {
		cases = append(cases, &ProbeCase{
			Name:        entry.name,
			DisplayName: entry.title,
			Check:       ProbeKnowledge,
			Enabled:     true,
			Weight:      probeWeightKnowledge,
			Sort:        entry.sort,
			BuiltIn:     true,
			Question:    entry.question,
			Method:      entry.method,
			Params: ProbeCaseParams{
				Prompt:    entry.prompt,
				MaxTokens: 512,
				Expect:    entry.expect,
				Match:     entry.match,
			},
		})
	}
	return cases
}

// builtInProbeFeatureCases send a parameter the upstream's own API documents.
// There are three ways that can go and they are not the same finding: honoured
// is the API working, refused is the vendor saying it does not implement it,
// and accepted-then-dropped is a request being translated into some other API
// behind the key.
func builtInProbeFeatureCases() []*ProbeCase {
	return []*ProbeCase{
		{
			Name:        "feature-logprobs",
			DisplayName: "Token probabilities",
			Check:       ProbeFeature,
			Protocol:    ProtocolOpenAi,
			Enabled:     true,
			Weight:      probeWeightFeature,
			Sort:        80,
			BuiltIn:     true,
			Question:    "Asked for token probabilities, does the answer carry them?",
			Method: "logprobs is part of the chat completions API, and the probabilities can only be " +
				"produced by whatever actually generated the tokens. A vendor that does not implement " +
				"it refuses the request, which is measured as such and scores nothing either way; a " +
				"backend translating this call into some other API answers 200 with the field simply " +
				"missing.",
			Params: ProbeCaseParams{
				Prompt:    probeAskPrompt,
				MaxTokens: 32,
				Extra:     `{"logprobs": true, "top_logprobs": 3}`,
				Require:   []string{"choices.0.logprobs.content.0.top_logprobs"},
			},
		},
		{
			Name:        "feature-choice-count",
			DisplayName: "Two answers at once",
			Check:       ProbeFeature,
			Protocol:    ProtocolOpenAi,
			Enabled:     true,
			Weight:      probeWeightFeature,
			Sort:        81,
			BuiltIn:     true,
			Question:    "Asked for two completions, do two come back?",
			Method: "n is part of the chat completions API. Several vendors do not implement it and " +
				"say so, which is measured as a refusal and scores nothing; what this looks for is the " +
				"upstream that accepts n, charges for the request and returns one choice, which is a " +
				"parameter it never passed on.",
			Params: ProbeCaseParams{
				Prompt:    probeAskPrompt,
				MaxTokens: 32,
				Extra:     `{"n": 2}`,
				Require:   []string{"choices.1.message"},
			},
		},
		{
			Name:        "feature-stop-openai",
			DisplayName: "Stop sequence (OpenAI)",
			Check:       ProbeFeature,
			Protocol:    ProtocolOpenAi,
			Enabled:     true,
			Weight:      probeWeightFeature,
			Sort:        82,
			BuiltIn:     true,
			Question:    "Does generation stop where the request said it should?",
			Method: "The model is asked to write A B C and the request stops it at B, so the answer " +
				"has to be A alone. Text past the stop sequence is the parameter having been dropped " +
				"somewhere in the chain. Some reasoning models apply stop sequences loosely, so this " +
				"is a note rather than a finding.",
			Params: ProbeCaseParams{
				Prompt:    "Output exactly this and nothing else: A B C",
				MaxTokens: 64,
				Extra:     `{"stop": ["B"]}`,
				Expect:    []string{"A"},
				Match:     "exact",
			},
		},
		{
			Name:        "feature-stop-anthropic",
			DisplayName: "Stop sequence (Anthropic)",
			Check:       ProbeFeature,
			Protocol:    ProtocolAnthropic,
			Enabled:     true,
			Weight:      probeWeightFeature,
			Sort:        83,
			BuiltIn:     true,
			Question:    "Does generation stop where the request said it should?",
			Method: "The same question in this API's spelling: stop_sequences cuts the answer at B, so " +
				"A is all that may come back. Text past it is the parameter having been dropped " +
				"somewhere in the chain.",
			Params: ProbeCaseParams{
				Prompt:    "Output exactly this and nothing else: A B C",
				MaxTokens: 64,
				Extra:     `{"stop_sequences": ["B"]}`,
				Expect:    []string{"A"},
				Match:     "exact",
			},
		},
		{
			Name:        "feature-openai-fingerprint",
			DisplayName: "Backend fingerprint",
			Check:       ProbeFeature,
			Protocol:    ProtocolOpenAi,
			Enabled:     true,
			Weight:      probeWeightFeature,
			Sort:        84,
			BuiltIn:     true,
			Question:    "Does an OpenAI model answer with the backend fingerprint OpenAI documents?",
			Method: "system_fingerprint is a documented field of OpenAI's own chat completions " +
				"response, and it is the sort of field a backend that is not OpenAI has nothing to " +
				"put in. This is asked only of models from OpenAI's own catalogue, because it is " +
				"OpenAI's own API that documents it.",
			Params: ProbeCaseParams{
				Prompt:    probeAskPrompt,
				MaxTokens: 32,
				Extra:     `{"seed": 7}`,
				Require:   []string{"system_fingerprint"},
				Vendors:   []string{"openai"},
			},
		},
		{
			Name:        "feature-openai-completion-id",
			DisplayName: "Completion id shape",
			Check:       ProbeFeature,
			Protocol:    ProtocolOpenAi,
			Enabled:     false,
			Weight:      probeWeightFeature,
			Sort:        85,
			BuiltIn:     true,
			Question:    "Does an OpenAI completion carry an id of the shape OpenAI's own API returns?",
			Method: "Every chat completion from OpenAI's own API is identified as chatcmpl-…; a relay " +
				"answering out of some other API of its own carries that API's id instead. This is a " +
				"fingerprint rather than something OpenAI documents as a guarantee, which is why it " +
				"ships turned off: turn it on where an id from another API is worth knowing about.",
			Params: ProbeCaseParams{
				Prompt:    probeAskPrompt,
				MaxTokens: 32,
				Require:   []string{"id=^chatcmpl-"},
				Vendors:   []string{"openai"},
			},
		},
	}
}
