package kiro

import "strings"

// Kiro reports a safety-classifier stop on the KRS endpoint as a metadataEvent
// with stopReason CONTENT_FILTERED and stopDetails.refusal. The Amazon Q
// endpoint ends the same turn after reasoning with only a contextUsageEvent and
// a clean EOF, carrying no reason at all.

// kiroInferredRefusalExplanation is reported when the stream shape, not the
// provider, identifies the refusal.
const kiroInferredRefusalExplanation = "The upstream model ended this turn while reasoning, without a response or a stated reason. This matches a cyber content-filter stop. Send a follow-up message to continue, or rephrase the request."

// kiroInferredRefusalCategory is the category of every reasoning-only stop
// observed on the Q endpoint: KRS reports the same requests as CYBER.
const kiroInferredRefusalCategory = "cyber"

// Refusal is an upstream content-filter stop, reported to Anthropic clients as
// stop_reason "refusal" with stop_details.
type Refusal struct {
	// Category is the lowercase Anthropic refusal category, empty when unknown.
	Category string
	// Explanation is the human-readable reason.
	Explanation string
	// Inferred marks a refusal recognized from the stream shape because the
	// endpoint reported no reason.
	Inferred bool
}

// kiroRefusalFromEvent returns the refusal a Kiro event reports, or nil.
func kiroRefusalFromEvent(eventType string, event map[string]any) *Refusal {
	if event == nil {
		return nil
	}
	for _, candidate := range []map[string]any{event, nestedEvent(event, eventType)} {
		if !strings.EqualFold(strings.TrimSpace(readStopReason(candidate)), "CONTENT_FILTERED") {
			continue
		}
		refusal := &Refusal{}
		details, _ := candidate["stopDetails"].(map[string]any)
		if info, ok := details["refusal"].(map[string]any); ok {
			refusal.Category = strings.ToLower(strings.TrimSpace(getString(info, "category")))
			refusal.Explanation = strings.TrimSpace(getString(info, "explanation"))
		}
		return refusal
	}
	return nil
}

// inferredKiroReasoningRefusal describes a turn the provider ended during
// reasoning without saying why.
func inferredKiroReasoningRefusal() *Refusal {
	return &Refusal{Category: kiroInferredRefusalCategory, Explanation: kiroInferredRefusalExplanation, Inferred: true}
}

// stopDetails renders the refusal in the Anthropic Messages stop_details shape,
// or nil when there is no refusal.
func (r *Refusal) stopDetails() any {
	if r == nil {
		return nil
	}
	return map[string]any{
		"type":        "refusal",
		"category":    nullableKiroString(r.Category),
		"explanation": nullableKiroString(r.Explanation),
	}
}

func nullableKiroString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
