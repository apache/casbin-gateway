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

// One number and one letter for a report that is otherwise six separate
// findings. The score is a weighted average of the cases that could be
// measured, and nothing else: a case that could not be asked lowers no score,
// and the report still carries every fact the letter was drawn from.

package object

import "math"

// The grades, from an upstream that answered every case the way the vendor's
// own API documents to one that failed the questions that need no argument.
const (
	ProbeGradeA = "A"
	ProbeGradeB = "B"
	ProbeGradeC = "C"
	ProbeGradeD = "D"
	ProbeGradeF = "F"
	// ProbeGradeUnknown is a run that measured nothing, which is not a finding
	// about the provider.
	ProbeGradeUnknown = "unknown"
)

// The score each level is worth. A warning is half credit rather than none: it
// is what a check reaches when the answer is explainable as well as suspicious.
const (
	probeScoreOk    = 1.0
	probeScoreWarn  = 0.5
	probeScoreAlert = 0.0
)

// The floors of each grade, on the 0-100 scale.
const (
	probeGradeFloorA = 90.0
	probeGradeFloorB = 75.0
	probeGradeFloorC = 60.0
	probeGradeFloorD = 40.0
)

// probeDefaultWeights carry a probe stored before the suite had weights, so an
// old report grades on the same scale as a new one rather than not at all.
var probeDefaultWeights = map[string]int{
	ProbeIdentity: probeWeightIdentity,
	ProbeCache:    probeWeightCache,
	ProbeBilling:  probeWeightBilling,
	ProbeTools:    probeWeightTools,
	ProbeStream:   probeWeightStream,
	ProbeVendor:   probeWeightVendor,
}

func probeCheckWeight(check ProbeCheck) int {
	if check.Weight > 0 {
		return check.Weight
	}
	return probeDefaultWeights[check.Key]
}

// scoreProviderProbe fills in the score and the grade from the checks the run
// produced. A run that never reached the upstream is graded unknown: not being
// able to ask is a fact about the request, not about the provider.
func scoreProviderProbe(probe *ProviderProbe) {
	probe.Score, probe.Grade = probeScoreOf(probe)
}

func probeScoreOf(probe *ProviderProbe) (float64, string) {
	if probe == nil || !probe.Ok {
		return 0, ProbeGradeUnknown
	}

	earned, possible := 0.0, 0.0
	for _, check := range probe.Checks {
		weight := float64(probeCheckWeight(check))
		if weight <= 0 || check.Level == LlmAuditUnknown {
			continue
		}
		possible += weight
		switch check.Level {
		case LlmAuditOk:
			earned += weight * probeScoreOk
		case LlmAuditWarn:
			earned += weight * probeScoreWarn
		default:
			earned += weight * probeScoreAlert
		}
	}
	if possible == 0 {
		return 0, ProbeGradeUnknown
	}

	score := math.Round(earned / possible * 1000) / 10
	return score, probeGradeOf(score)
}

func probeGradeOf(score float64) string {
	switch {
	case score >= probeGradeFloorA:
		return ProbeGradeA
	case score >= probeGradeFloorB:
		return ProbeGradeB
	case score >= probeGradeFloorC:
		return ProbeGradeC
	case score >= probeGradeFloorD:
		return ProbeGradeD
	default:
		return ProbeGradeF
	}
}

// ensureProbeScore grades a row that was stored before this existed, so that
// history read back from the database is on one scale.
func ensureProbeScore(probe *ProviderProbe) *ProviderProbe {
	if probe != nil && probe.Grade == "" {
		scoreProviderProbe(probe)
	}
	return probe
}
