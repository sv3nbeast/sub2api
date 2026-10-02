package claude

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestSonnet55NormalizationPreservesHistoryAndToolContract(t *testing.T) {
	for _, model := range []string{"claude-sonnet-5-5", "claude-sonnet-5.5", "claude-sonnet-5-5-thinking", "claude-sonnet-5.5-thinking"} {
		body := []byte(`{"thinking":{"type":"disabled","budget_tokens":4096,"display":"summarized"},"temperature":0.2,"top_p":0.8,"top_k":10,"output_config":{"effort":"max"},"system":[{"type":"text","text":"stable","cache_control":{"type":"ephemeral","ttl":"1h"}}],"tool_choice":{"type":"tool","name":"read"},"messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"note","signature":"provider-signature"},{"type":"tool_use","id":"t1","name":"read","input":{}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"done"}]}]}`)
		out := NormalizeSonnet55Request(body, model)
		require.JSONEq(t, `{"type":"between_tools"}`, gjson.GetBytes(out, "thinking").Raw)
		require.Equal(t, "high", gjson.GetBytes(out, "output_config.effort").String())
		for _, field := range []string{"messages", "system", "tool_choice"} {
			require.Equal(t, gjson.GetBytes(body, field).Raw, gjson.GetBytes(out, field).Raw, field)
		}
		for _, field := range []string{"temperature", "top_p", "top_k"} {
			require.False(t, gjson.GetBytes(out, field).Exists(), field)
		}
		require.Equal(t, string(out), string(NormalizeSonnet55Request(out, model)), "idempotent")
	}
	body := []byte(`{"thinking":{"type":"enabled","budget_tokens":4096},"temperature":0.3}`)
	require.Equal(t, string(body), string(NormalizeSonnet55Request(body, "claude-sonnet-5")))
	require.Equal(t, "adaptive", gjson.GetBytes(NormalizeSonnet55Request(body, "claude-sonnet-5-5"), "thinking.type").String())
	require.Contains(t, DefaultModelIDs(), "claude-sonnet-5-5")
}
