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

// The suite Gateway ships. Every case here is a question the vendor's own API
// documents an answer to, which is what makes a finding something to send to
// the provider rather than something to believe.

package object

import "encoding/json"

// The weights the shipped suite is balanced on. Identity is worth the most
// because a name that does not match is the one finding that needs no argument;
// vendor headers are worth the least because a tidy relay strips them.
const (
	probeWeightIdentity = 30
	probeWeightCache    = 20
	probeWeightBilling  = 20
	probeWeightTools    = 15
	probeWeightStream   = 10
	probeWeightVendor   = 5
)

func builtInProbeCases() []*ProbeCase {
	schema, err := json.MarshalIndent(probeToolSchema(), "", "  ")
	if err != nil {
		schema = []byte("{}")
	}

	return []*ProbeCase{
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
				"what the vendor says it will be. Any other mismatch is decisive; a matching name " +
				"proves little, since echoing one back is the easiest thing in the world to do.",
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
				"tidy, so this case never reaches an alert on its own. Leave the header list empty to " +
				"use the vendor's own, or name headers here to ask for those instead.",
			Params: ProbeCaseParams{MinHeaders: 2},
		},
	}
}
