package kiro

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func historyReasoningBody(t *testing.T) []byte {
	t.Helper()
	return []byte(`{"model":"claude-opus-5-5","max_tokens":8192,"thinking":{"type":"adaptive"},"messages":[` +
		`{"role":"user","content":"q1"},` +
		`{"role":"assistant","content":[{"type":"thinking","thinking":"weighing it","signature":"SIG-REAL-123"},{"type":"text","text":"391"}]},` +
		`{"role":"user","content":"q2"}]}`)
}

func TestBuildAssistantMessageStructEmitsReasoningContentForSignedThinking(t *testing.T) {
	msg := gjson.Parse(`{"role":"assistant","content":[{"type":"thinking","thinking":"my reasoning","signature":"SIGABC"},{"type":"text","text":"answer"}]}`)

	on := buildAssistantMessageStruct(msg, &KiroRequestContext{EmitHistoryReasoningContent: true})
	require.NotNil(t, on.ReasoningContent)
	require.Equal(t, "my reasoning", on.ReasoningContent.ReasoningText.Text)
	require.Equal(t, "SIGABC", on.ReasoningContent.ReasoningText.Signature)
	require.Equal(t, "answer", on.Content)
	require.NotContains(t, on.Content, "<thinking>")

	off := buildAssistantMessageStruct(msg, &KiroRequestContext{EmitHistoryReasoningContent: false})
	require.Nil(t, off.ReasoningContent)
	require.Contains(t, off.Content, "<thinking>my reasoning</thinking>")
	require.Contains(t, off.Content, "answer")
}

func TestBuildAssistantMessageStructKeepsUnsignedThinkingInline(t *testing.T) {
	msg := gjson.Parse(`{"role":"assistant","content":[{"type":"thinking","thinking":"unsigned"},{"type":"text","text":"answer"}]}`)
	got := buildAssistantMessageStruct(msg, &KiroRequestContext{EmitHistoryReasoningContent: true})
	require.Nil(t, got.ReasoningContent, "no provider signature means no reasoningContent")
	require.Contains(t, got.Content, "<thinking>unsigned</thinking>")
}

func TestBuildAssistantMessageStructReasoningOnlyTurnKeepsEmptyContent(t *testing.T) {
	// Claude Code stores omitted thinking as an empty text with a signature.
	msg := gjson.Parse(`{"role":"assistant","content":[{"type":"thinking","thinking":"","signature":"SIGONLY"}]}`)
	got := buildAssistantMessageStruct(msg, &KiroRequestContext{EmitHistoryReasoningContent: true})
	require.NotNil(t, got.ReasoningContent)
	require.Equal(t, "SIGONLY", got.ReasoningContent.ReasoningText.Signature)
	require.Equal(t, "", got.ReasoningContent.ReasoningText.Text)
	require.Equal(t, "", got.Content, "reasoningContent alone is a valid turn; no placeholder space")
}

func TestMergeKiroAssistantMessagesKeepsReasoningContent(t *testing.T) {
	dst := &KiroAssistantResponseMessage{Content: "first"}
	src := &KiroAssistantResponseMessage{Content: "second", ReasoningContent: &KiroReasoningContent{ReasoningText: KiroReasoningText{Text: "r", Signature: "S"}}}
	mergeKiroAssistantMessages(dst, src)
	require.NotNil(t, dst.ReasoningContent)
	require.Equal(t, "S", dst.ReasoningContent.ReasoningText.Signature)
}

// historyAssistantTurn returns the single historical assistant turn carrying
// the replayed thinking (index shifts with the prepended system turn).
func historyAssistantTurn(t *testing.T, payload []byte) gjson.Result {
	t.Helper()
	var found gjson.Result
	matches := 0
	gjson.GetBytes(payload, "conversationState.history").ForEach(func(_, h gjson.Result) bool {
		if am := h.Get("assistantResponseMessage"); am.Exists() && am.Get("content").String() != "I will follow these instructions." {
			found = am
			matches++
		}
		return true
	})
	require.Equal(t, 1, matches, "one historical assistant answer turn")
	return found
}

func TestBuildKiroPayloadEmitsReasoningContentInHistory(t *testing.T) {
	built, err := BuildKiroPayloadWithOptions(historyReasoningBody(t), MapModel("claude-opus-5-5"), "", http.Header{}, KiroPayloadOptions{
		Origin:                      "AI_EDITOR",
		EmitHistoryReasoningContent: true,
	})
	require.NoError(t, err)
	am := historyAssistantTurn(t, built.Payload)
	require.Equal(t, "SIG-REAL-123", am.Get("reasoningContent.reasoningText.signature").String())
	require.Equal(t, "weighing it", am.Get("reasoningContent.reasoningText.text").String())
	require.Equal(t, "391", am.Get("content").String())
	require.NotContains(t, am.Get("content").String(), "<thinking>")
}

func TestBuildKiroPayloadInlinesHistoryThinkingWhenDisabled(t *testing.T) {
	built, err := BuildKiroPayloadWithOptions(historyReasoningBody(t), MapModel("claude-opus-5-5"), "", http.Header{}, KiroPayloadOptions{
		Origin:                      "AI_EDITOR",
		EmitHistoryReasoningContent: false,
	})
	require.NoError(t, err)
	am := historyAssistantTurn(t, built.Payload)
	require.False(t, am.Get("reasoningContent").Exists())
	require.Contains(t, am.Get("content").String(), "<thinking>weighing it</thinking>")
}
