package kiro

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const claudeCodeSystem = "You are a Claude agent, built on Anthropic's Claude Agent SDK.\n\n" +
	"You are an interactive agent that helps users with software engineering tasks."

// claudeCodeClientBody builds a Claude Code shaped request whose system prompt
// uses the wording current clients actually send.
func claudeCodeClientBody(t *testing.T, system string) []byte {
	t.Helper()
	raw, err := json.Marshal(system)
	require.NoError(t, err)
	return []byte(`{"model":"claude-opus-5-5","max_tokens":8192,` +
		`"thinking":{"type":"adaptive"},"output_config":{"effort":"high"},` +
		`"system":[{"type":"text","text":` + string(raw) + `}],` +
		`"messages":[{"role":"user","content":"hi"}]}`)
}

func platformSystemPrompt(t *testing.T, payload []byte) string {
	t.Helper()
	sys := gjson.GetBytes(payload, "conversationState.history.0.userInputMessage.content").String()
	require.NotEmpty(t, sys)
	return sys
}

func header(kv ...string) http.Header {
	h := http.Header{}
	for i := 0; i+1 < len(kv); i += 2 {
		h.Set(kv[i], kv[i+1])
	}
	return h
}

// A Claude Code client is recognized by its protocol marker or user agent, not
// by the literal wording of its system prompt. Effort then reaches Kiro only as
// a field, and the <thinking_mode>/<thinking_effort> text directive is dropped.
func TestClaudeCodeClientSkipsThinkingTextDirective(t *testing.T) {
	for _, tc := range []struct {
		name   string
		header http.Header
	}{
		{"claude-code beta, cli 2.1.259", header("anthropic-beta", "claude-code-20250219", "User-Agent", "claude-cli/2.1.259 (external, cli)")},
		{"claude-code beta, desktop 2.1.271", header("anthropic-beta", "claude-code-20250219,interleaved-thinking-2025-05-14", "User-Agent", "claude-cli/2.1.271 (external, claude-desktop-3p, agent-sdk/0.3.281)")},
		{"cli user agent only", header("User-Agent", "claude-cli/2.1.281 (external, mirasim)")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			built, err := BuildKiroPayloadWithOptions(claudeCodeClientBody(t, claudeCodeSystem), MapModel("claude-opus-5-5"), "", tc.header, KiroPayloadOptions{Origin: "AI_EDITOR"})
			require.NoError(t, err)
			sys := platformSystemPrompt(t, built.Payload)
			require.NotContains(t, sys, "<thinking_mode>")
			require.NotContains(t, sys, "<thinking_effort>")
			require.Equal(t, "high", gjson.GetBytes(built.Payload, "additionalModelRequestFields.output_config.effort").String(),
				"effort still reaches the upstream as a field")
		})
	}
}

// Detection no longer hinges on one exact prompt sentence: both the historical
// and the current wording take the native path when the client marker is present.
func TestClaudeCodeDetectionIgnoresPromptWording(t *testing.T) {
	legacy := `You are Claude Code, Anthropic's official CLI for Claude.`
	for _, tc := range []struct {
		name          string
		system        string
		claudeCode    bool
		expectTextTag bool
	}{
		{"legacy wording, claude code client", legacy, true, false},
		{"current wording, claude code client", claudeCodeSystem, true, false},
		{"current wording, other client", claudeCodeSystem, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := header()
			if tc.claudeCode {
				h.Set("anthropic-beta", "claude-code-20250219")
			}
			built, err := BuildKiroPayloadWithOptions(claudeCodeClientBody(t, tc.system), MapModel("claude-opus-5-5"), "", h, KiroPayloadOptions{Origin: "AI_EDITOR"})
			require.NoError(t, err)
			hasTag := strings.Contains(platformSystemPrompt(t, built.Payload), "<thinking_effort>")
			require.Equal(t, tc.expectTextTag, hasTag)
		})
	}
}

// A non-Claude-Code client keeps the text directive as a fallback.
func TestNonClaudeCodeClientKeepsThinkingTextDirective(t *testing.T) {
	built, err := BuildKiroPayloadWithOptions(claudeCodeClientBody(t, "You are a helpful assistant."), MapModel("claude-opus-5-5"), "", header("User-Agent", "curl/8.7.1"), KiroPayloadOptions{Origin: "AI_EDITOR"})
	require.NoError(t, err)
	require.Contains(t, platformSystemPrompt(t, built.Payload), "<thinking_effort>high</thinking_effort>")
}

// The client's own system prompt still reaches Kiro.
func TestClaudeCodeSystemPromptIsForwarded(t *testing.T) {
	built, err := BuildKiroPayloadWithOptions(claudeCodeClientBody(t, claudeCodeSystem), MapModel("claude-opus-5-5"), "", header("anthropic-beta", "claude-code-20250219"), KiroPayloadOptions{Origin: "AI_EDITOR"})
	require.NoError(t, err)
	require.Contains(t, platformSystemPrompt(t, built.Payload), "interactive agent that helps users")
}
