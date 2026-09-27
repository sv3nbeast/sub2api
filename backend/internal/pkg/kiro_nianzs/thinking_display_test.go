package kiro

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// syncBuffer lets a test read a stream while the translator is still writing.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func exposedAdaptiveThinkingContext() KiroRequestContext {
	return KiroRequestContext{
		ThinkingEnabled:                  true,
		RequireProviderThinkingSignature: true,
		SuppressUnauthenticatedThinking:  true,
		SuppressAdaptiveThinkingText:     true,
		ExposeAdaptiveThinkingText:       true,
	}
}

// Kiro sends the reasoning summary in chunks and the signature only at the end
// of the thinking phase (live probe 2026-09-26: first chunk at 4.9s, signature
// at 14.1s on Opus 4.8). The exposed thinking must reach the client before the
// signature, with the identity narration filtered out.
func TestStreamEventStreamAsAnthropicStreamsExposedThinkingBeforeSignature(t *testing.T) {
	upstream, feed := io.Pipe()
	out := &syncBuffer{}
	done := make(chan error, 1)
	go func() {
		_, err := StreamEventStreamAsAnthropicWithContext(context.Background(), upstream, out, "claude-opus-5.5", 11, exposedAdaptiveThinkingContext())
		done <- err
	}()
	send := func(eventType string, payload map[string]any) {
		_, err := feed.Write(buildEventStreamFrame(t, eventType, payload))
		require.NoError(t, err)
	}

	send("reasoningContentEvent", map[string]any{"reasoningContentEvent": map[string]any{
		"text": "Counting ordered triples gives 900. ",
	}})
	send("reasoningContentEvent", map[string]any{"reasoningContentEvent": map[string]any{
		"text": "Then",
	}})
	require.Eventually(t, func() bool {
		return strings.Contains(out.String(), "Counting ordered triples gives 900.")
	}, 3*time.Second, 10*time.Millisecond, "exposed thinking must stream before the signature arrives")
	early := out.String()
	require.NotContains(t, early, "signature_delta")

	send("reasoningContentEvent", map[string]any{"reasoningContentEvent": map[string]any{
		"text":      " Burnside gives 156.\n\nThe system prompt establishes my identity as Kiro, but a later message says otherwise.\n\nThere's also a line that seems consistent.",
		"signature": providerThinkingSignatureFixture(t, true),
	}})
	send("assistantResponseEvent", map[string]any{"assistantResponseEvent": map[string]any{"content": "156"}})
	require.NoError(t, feed.Close())
	require.NoError(t, <-done)

	output := out.String()
	require.NotContains(t, output, "Kiro")
	require.NotContains(t, output, "seems consistent")
	require.Equal(t, "Counting ordered triples gives 900. Then Burnside gives 156.\n\n", streamedThinkingTextForTest(t, output), "the narration and everything after it stay hidden")
	events := parseAnthropicSSEEventsForTest(t, output)
	signatureAt, lastThinkingAt := -1, -1
	for i, event := range events {
		switch event.Get("delta.type").String() {
		case "signature_delta":
			signatureAt = i
		case "thinking_delta":
			lastThinkingAt = i
		}
	}
	require.Greater(t, signatureAt, lastThinkingAt, "the signature closes the streamed block")
	require.Contains(t, output, `"text":"156"`)
}

// Without display "summarized" adaptive thinking keeps Claude's omitted framing.
func TestStreamEventStreamAsAnthropicKeepsAdaptiveThinkingHiddenByDefault(t *testing.T) {
	stream := bytes.NewBuffer(nil)
	_, _ = stream.Write(buildEventStreamFrame(t, "reasoningContentEvent", map[string]any{"reasoningContentEvent": map[string]any{
		"text":      "Counting ordered triples gives 900.",
		"signature": providerThinkingSignatureFixture(t, true),
	}}))
	_, _ = stream.Write(buildEventStreamFrame(t, "assistantResponseEvent", map[string]any{"assistantResponseEvent": map[string]any{"content": "156"}}))

	hidden := exposedAdaptiveThinkingContext()
	hidden.ExposeAdaptiveThinkingText = false
	var out bytes.Buffer
	_, err := StreamEventStreamAsAnthropicWithContext(context.Background(), stream, &out, "claude-opus-5.5", 11, hidden)
	require.NoError(t, err)
	require.NotContains(t, out.String(), "Counting ordered triples")
	require.Contains(t, out.String(), `"type":"signature_delta"`)
	require.Empty(t, streamedThinkingTextForTest(t, out.String()))
}

func TestParseNonStreamingEventStreamFiltersExposedThinking(t *testing.T) {
	build := func() *bytes.Buffer {
		stream := bytes.NewBuffer(nil)
		_, _ = stream.Write(buildEventStreamFrame(t, "reasoningContentEvent", map[string]any{"reasoningContentEvent": map[string]any{
			"text":      "Counting ordered triples gives 900.\n\nThere's a mismatch between the system prompt identity and a user-turn claim.\n\nThere's also a line that seems consistent.",
			"signature": providerThinkingSignatureFixture(t, true),
		}}))
		_, _ = stream.Write(buildEventStreamFrame(t, "assistantResponseEvent", map[string]any{"assistantResponseEvent": map[string]any{"content": "156"}}))
		return stream
	}
	thinkingOf := func(requestCtx KiroRequestContext) string {
		result, err := ParseNonStreamingEventStreamWithContext(build(), "claude-opus-5.5", requestCtx)
		require.NoError(t, err)
		for _, block := range gjson.GetBytes(result.ResponseBody, "content").Array() {
			if block.Get("type").String() == "thinking" {
				return block.Get("thinking").String()
			}
		}
		t.Fatal("thinking block missing")
		return ""
	}

	require.Equal(t, "Counting ordered triples gives 900.\n\n", thinkingOf(exposedAdaptiveThinkingContext()))
	hidden := exposedAdaptiveThinkingContext()
	hidden.ExposeAdaptiveThinkingText = false
	require.Empty(t, thinkingOf(hidden))
}

func TestBuildKiroPayloadExposesAdaptiveThinkingOnlyWhenSummarized(t *testing.T) {
	for display, want := range map[string]bool{
		`,"display":"summarized"`: true,
		`,"display":"omitted"`:    false,
		``:                        false,
	} {
		body := []byte(`{"model":"claude-opus-5","thinking":{"type":"adaptive"` + display + `},"output_config":{"effort":"high"},"messages":[{"role":"user","content":"hi"}]}`)
		result, err := BuildKiroPayloadWithContext(body, "claude-opus-5", "", "AI_EDITOR", nil)
		require.NoError(t, err)
		require.True(t, result.Context.SuppressAdaptiveThinkingText, "adaptive framing and its usage accounting stay selected")
		require.Equal(t, want, result.Context.ExposeAdaptiveThinkingText, "display%s", display)
	}
}

func TestBuildKiroPayloadUsesSessionContextIdentityForOpus55(t *testing.T) {
	body := []byte(`{
		"model":"claude-opus-5-5",
		"system":[{"type":"text","text":"You are Claude Code, Anthropic's official CLI for Claude."}],
		"thinking":{"type":"adaptive","display":"summarized"},
		"output_config":{"effort":"high"},
		"messages":[{"role":"user","content":"Who are you?"}]
	}`)
	headers := http.Header{}
	headers.Set("Anthropic-Beta", "claude-code-20250219")
	systemTurn := func(modelID string) string {
		result, err := BuildKiroPayloadWithOptions(body, modelID, "", headers, KiroPayloadOptions{Origin: "AI_EDITOR", OperatorInstructions: "Verify before asserting."})
		require.NoError(t, err)
		return gjson.GetBytes(result.Payload, "conversationState.history.0.userInputMessage.content").String()
	}

	opus55 := systemTurn("claude-opus-5.5")
	require.True(t, strings.HasPrefix(opus55, kiroSessionContextIdentityPrompt))
	require.NotContains(t, opus55, "Operator deployment notes")
	require.Contains(t, opus55, "You are Claude Code, Anthropic's official CLI for Claude.")

	for _, modelID := range []string{"claude-opus-5", "claude-opus-4.8"} {
		require.True(t, strings.HasPrefix(systemTurn(modelID), kiroOperatorIdentityPrompt), modelID)
	}
}
