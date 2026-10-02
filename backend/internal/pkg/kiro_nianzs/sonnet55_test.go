package kiro

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestSonnet55PayloadContract(t *testing.T) {
	for _, alias := range []string{"claude-sonnet-5-5", "claude-sonnet-5.5", "claude-sonnet-5-5-thinking", "claude-sonnet-5.5-thinking"} {
		require.Equal(t, "claude-sonnet-5.5", MapModel(alias))
		require.Equal(t, 128000, MaxOutputTokensForModel(alias))
		require.Equal(t, 1000000, ContextWindowTokensForModel(alias))
		for _, mode := range []string{"adaptive", "enabled", "disabled", "between_tools"} {
			body := []byte(`{"model":"claude-sonnet-5-5","max_tokens":128000,"temperature":0.1,"top_p":0.9,"thinking":{"type":"` + mode + `","budget_tokens":5000,"display":"summarized"},"output_config":{"effort":"high"},"messages":[{"role":"user","content":"hello"}],"tools":[{"name":"read","input_schema":{"type":"object"}}]}`)
			result, err := BuildKiroPayloadWithContext(body, MapModel(alias), "", "AI_EDITOR", nil)
			require.NoError(t, err)
			want := "adaptive"
			if mode == "disabled" || mode == "between_tools" {
				want = "between_tools"
			}
			require.Equal(t, want, gjson.GetBytes(result.Payload, "additionalModelRequestFields.thinking.type").String())
			require.False(t, gjson.GetBytes(result.Payload, "additionalModelRequestFields.thinking.budget_tokens").Exists())
			if want == "between_tools" {
				require.False(t, gjson.GetBytes(result.Payload, "additionalModelRequestFields.thinking.display").Exists())
			}
			require.False(t, gjson.GetBytes(result.Payload, "inferenceConfig.temperature").Exists())
			require.False(t, gjson.GetBytes(result.Payload, "inferenceConfig.topP").Exists())
			require.EqualValues(t, 128000, gjson.GetBytes(result.Payload, "inferenceConfig.maxTokens").Int())
			require.Equal(t, "claude-sonnet-5.5", gjson.GetBytes(result.Payload, "conversationState.currentMessage.userInputMessage.modelId").String())
			require.True(t, result.Context.ThinkingEnabled)
			require.True(t, gjson.GetBytes(result.Payload, "conversationState.currentMessage.userInputMessage.userInputMessageContext.tools").Exists())
		}
	}
	require.Equal(t, "claude-sonnet-5", MapModel("claude-sonnet-5"))
	require.Equal(t, "claude-sonnet-5.9", MapModel("claude-sonnet-5-9"))
	for _, effort := range []string{"low", "medium", "high", "xhigh", "max"} {
		b := []byte(`{"model":"claude-sonnet-5-5","output_config":{"effort":"` + effort + `"},"messages":[{"role":"user","content":"hello"}]}`)
		built, err := BuildKiroPayloadWithContext(b, MapModel("claude-sonnet-5-5"), "", "AI_EDITOR", nil)
		require.NoError(t, err)
		require.Equal(t, effort, gjson.GetBytes(built.Payload, "additionalModelRequestFields.output_config.effort").String())
	}
}
