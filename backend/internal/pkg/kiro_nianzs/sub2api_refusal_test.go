package kiro

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const testKiroRefusalExplanation = "The selected model cannot continue this conversation."

// kiroRefusalTestFrames builds a wire-shaped (unwrapped payload) Kiro stream.
func kiroRefusalTestFrames(t *testing.T, withMetadata bool) *bytes.Buffer {
	t.Helper()
	stream := bytes.NewBuffer(nil)
	_, _ = stream.Write(buildEventStreamFrame(t, "reasoningContentEvent", map[string]any{"text": "Weighing the request. "}))
	_, _ = stream.Write(buildEventStreamFrame(t, "reasoningContentEvent", map[string]any{"text": "Checking the evidence."}))
	if withMetadata {
		_, _ = stream.Write(buildEventStreamFrame(t, "metadataEvent", map[string]any{
			"stopDetails": map[string]any{"refusal": map[string]any{
				"category":    "CYBER",
				"explanation": testKiroRefusalExplanation,
			}},
			"stopReason": "CONTENT_FILTERED",
		}))
	}
	_, _ = stream.Write(buildEventStreamFrame(t, "contextUsageEvent", map[string]any{"contextUsagePercentage": 21.7}))
	return stream
}

// sseEventData returns the data payload of the only SSE event of the given type.
func sseEventData(t *testing.T, sse, event string) string {
	t.Helper()
	var found []string
	for _, block := range strings.Split(sse, "\n\n") {
		if !strings.HasPrefix(block, "event: "+event+"\n") {
			continue
		}
		for _, line := range strings.Split(block, "\n") {
			if data, ok := strings.CutPrefix(line, "data: "); ok {
				found = append(found, data)
			}
		}
	}
	require.Len(t, found, 1, "event %s", event)
	return found[0]
}

func TestStreamReportsKRSContentFilteredAsRefusal(t *testing.T) {
	var out bytes.Buffer
	result, err := StreamEventStreamAsAnthropicWithContext(context.Background(), kiroRefusalTestFrames(t, true), &out, "claude-opus-5-5", 200_000, KiroRequestContext{
		RequireTerminalEvent: true,
		ThinkingEnabled:      true,
	})

	require.NoError(t, err)
	require.Equal(t, "refusal", result.StopReason)
	require.Equal(t, &Refusal{Category: "cyber", Explanation: testKiroRefusalExplanation}, result.Refusal)
	require.Contains(t, out.String(), "Checking the evidence.")
	delta := sseEventData(t, out.String(), "message_delta")
	require.Equal(t, "refusal", gjson.Get(delta, "delta.stop_reason").String())
	require.Equal(t, "refusal", gjson.Get(delta, "delta.stop_details.type").String())
	require.Equal(t, "cyber", gjson.Get(delta, "delta.stop_details.category").String())
	require.Equal(t, testKiroRefusalExplanation, gjson.Get(delta, "delta.stop_details.explanation").String())
	require.Equal(t, 1, strings.Count(out.String(), "event: message_stop"))
}

func TestParseNonStreamingReportsKRSContentFilteredAsRefusal(t *testing.T) {
	for _, claudeCode := range []bool{false, true} {
		result, err := ParseNonStreamingEventStreamWithContext(kiroRefusalTestFrames(t, true), "claude-opus-5-5", KiroRequestContext{
			RequireTerminalEvent: true,
			ThinkingEnabled:      true,
			EmitProtocolPing:     claudeCode,
		})

		require.NoError(t, err)
		require.Equal(t, "refusal", result.StopReason)
		require.Equal(t, "cyber", result.Refusal.Category)
		body := string(result.ResponseBody)
		require.Equal(t, "refusal", gjson.Get(body, "stop_reason").String())
		require.Equal(t, "refusal", gjson.Get(body, "stop_details.type").String())
		require.Equal(t, "cyber", gjson.Get(body, "stop_details.category").String())
		require.Equal(t, testKiroRefusalExplanation, gjson.Get(body, "stop_details.explanation").String())
	}
}

// The Q endpoint stops the same turn without the metadataEvent.
func TestReasoningOnlySemanticTailIsInferredRefusal(t *testing.T) {
	t.Run("stream", func(t *testing.T) {
		var out bytes.Buffer
		var diagnostics []KiroEventDiagnostic
		result, err := StreamEventStreamAsAnthropicWithContext(context.Background(), kiroRefusalTestFrames(t, false), &out, "claude-opus-5-5", 200_000, KiroRequestContext{
			RequireTerminalEvent:  true,
			AcceptSemanticTailEOF: true,
			ThinkingEnabled:       true,
			EventDiagnosticSink: func(event KiroEventDiagnostic) {
				diagnostics = append(diagnostics, event)
			},
		})

		require.NoError(t, err)
		require.Equal(t, "refusal", result.StopReason)
		require.True(t, result.Refusal.Inferred)
		delta := sseEventData(t, out.String(), "message_delta")
		require.Equal(t, "refusal", gjson.Get(delta, "delta.stop_reason").String())
		require.Equal(t, "cyber", gjson.Get(delta, "delta.stop_details.category").String())
		require.Equal(t, kiroInferredRefusalExplanation, gjson.Get(delta, "delta.stop_details.explanation").String())
		require.Equal(t, "reasoning_tail_refusal", diagnostics[len(diagnostics)-1].DecodeStatus)
	})

	t.Run("non_stream", func(t *testing.T) {
		result, err := ParseNonStreamingEventStreamWithContext(kiroRefusalTestFrames(t, false), "claude-opus-5-5", KiroRequestContext{
			RequireTerminalEvent:  true,
			AcceptSemanticTailEOF: true,
			ThinkingEnabled:       true,
		})

		require.NoError(t, err)
		require.Equal(t, "refusal", gjson.GetBytes(result.ResponseBody, "stop_reason").String())
		require.Equal(t, kiroInferredRefusalExplanation, gjson.GetBytes(result.ResponseBody, "stop_details.explanation").String())
	})

	t.Run("thinking_disabled", func(t *testing.T) {
		result, err := ParseNonStreamingEventStreamWithContext(kiroRefusalTestFrames(t, false), "claude-opus-5-5", KiroRequestContext{
			RequireTerminalEvent:  true,
			AcceptSemanticTailEOF: true,
		})

		require.NoError(t, err)
		require.Equal(t, "refusal", result.StopReason)
	})

	t.Run("tail_fallback_disabled", func(t *testing.T) {
		result, err := ParseNonStreamingEventStreamWithContext(kiroRefusalTestFrames(t, false), "claude-opus-5-5", KiroRequestContext{
			RequireTerminalEvent: true,
			ThinkingEnabled:      true,
		})

		require.Nil(t, result)
		require.ErrorContains(t, err, "missing completion evidence")
	})
}

// A refusal can end the reasoning before its signature arrives. Strict
// signature handling must drop the unsigned block, not fail the refused turn.
func TestRefusalDropsUnsignedHiddenThinking(t *testing.T) {
	requestCtx := KiroRequestContext{
		RequireTerminalEvent:             true,
		AcceptSemanticTailEOF:            true,
		ThinkingEnabled:                  true,
		SuppressAdaptiveThinkingText:     true,
		RequireProviderThinkingSignature: true,
	}

	var out bytes.Buffer
	result, err := StreamEventStreamAsAnthropicWithContext(context.Background(), kiroRefusalTestFrames(t, false), &out, "claude-opus-5-5", 200_000, requestCtx)
	require.NoError(t, err)
	require.Equal(t, "refusal", result.StopReason)
	require.NotContains(t, out.String(), "Checking the evidence.")
	require.Equal(t, 1, strings.Count(out.String(), "event: message_stop"))

	parsed, err := ParseNonStreamingEventStreamWithContext(kiroRefusalTestFrames(t, true), "claude-opus-5-5", requestCtx)
	require.NoError(t, err)
	require.Equal(t, "refusal", parsed.StopReason)
	require.NotContains(t, string(parsed.ResponseBody), "Checking the evidence.")
}
