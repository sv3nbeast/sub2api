package service

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
)

// The OpenAI conversions use typed requests; mirror the Messages normalization
// without serializing a potentially large conversation a second time.
func normalizeAnthropicSonnet55Request(req *apicompat.AnthropicRequest) {
	if req == nil || !claude.IsSonnet55Model(req.Model) {
		return
	}
	req.Temperature, req.TopP = nil, nil
	if req.Thinking == nil {
		return
	}
	switch strings.ToLower(strings.TrimSpace(req.Thinking.Type)) {
	case "enabled", "adaptive":
		req.Thinking.Type = "adaptive"
		req.Thinking.BudgetTokens = 0
	case "disabled", "between_tools":
		req.Thinking = &apicompat.AnthropicThinking{Type: "between_tools"}
		if req.OutputConfig != nil && (req.OutputConfig.Effort == "xhigh" || req.OutputConfig.Effort == "max") {
			req.OutputConfig.Effort = "high"
		}
	}
}
