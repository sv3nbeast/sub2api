package claude

import (
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// IsSonnet55Model matches the documented model and its gateway aliases only.
func IsSonnet55Model(model string) bool {
	model = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(model)), "-thinking")
	return model == "claude-sonnet-5-5" || model == "claude-sonnet-5.5"
}

// NormalizeSonnet55Request keeps append-only history, signatures, cache controls
// and tool_choice untouched. In particular, forced tools must not silently turn
// into auto: the provider's rejection preserves the client's requested contract.
// The compatibility rewrite also precedes Kiro native payload construction.
func NormalizeSonnet55Request(body []byte, model string) []byte {
	if !IsSonnet55Model(model) || !gjson.ValidBytes(body) {
		return body
	}
	out := body
	remove := func(path string) {
		if gjson.GetBytes(out, path).Exists() {
			if next, err := sjson.DeleteBytes(out, path); err == nil {
				out = next
			}
		}
	}
	set := func(path string, value any) {
		if next, err := sjson.SetBytes(out, path, value); err == nil {
			out = next
		}
	}
	for _, path := range []string{"temperature", "top_p", "top_k"} {
		remove(path)
	}
	mode := strings.ToLower(strings.TrimSpace(gjson.GetBytes(out, "thinking.type").String()))
	if mode == "enabled" {
		mode = "adaptive"
		set("thinking.type", mode)
	}
	if mode == "disabled" {
		mode = "between_tools"
		set("thinking.type", mode)
	}
	if mode == "adaptive" {
		remove("thinking.budget_tokens")
	}
	if mode == "between_tools" {
		// This mode takes no other thinking fields. xhigh/max require adaptive;
		// preserve the lower-thinking intent using the highest accepted effort.
		set("thinking", map[string]string{"type": "between_tools"})
		effort := strings.ToLower(gjson.GetBytes(out, "output_config.effort").String())
		if effort == "xhigh" || effort == "max" {
			set("output_config.effort", "high")
		}
	}
	return out
}
