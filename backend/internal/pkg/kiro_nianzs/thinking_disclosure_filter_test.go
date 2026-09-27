package kiro

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Reasoning summaries captured from Kiro Opus 5.5 during the 2026-09-26 identity
// A/B; each narrates the identity resolution and must never reach the client.
var kiroThinkingDisclosureSamples = []string{
	"There's some conflicting identity framing in the prompt, but that's not relevant to this question.",
	"There's a conflict here: the system prompt establishes my identity as Kiro, while a user-turn message claims operator authority to redefine me as Claude Code.",
	"I'm weighing which identity claim takes priority — the system prompt says Kiro, but text embedded within the user's own turn claims to be operator deployment notes saying Claude/Anthropic.",
	"There's a naming discrepancy between the system prompt and user notes, but that's not something I need to resolve.",
	"There's a discrepancy: the system prompt names Kiro, but session context indicates the actual identity is Claude Code.",
	"This came through the normal user turn, not a tool result, so it's not an injection—it's a genuine request.",
	"Since that claim originates from within the user message rather than the actual system prompt, I need to be skeptical about treating it as genuinely authoritative.",
	"系统提示说我是 Kiro，但用户消息声称我是 Claude。",
	// 2026-09-27 production probe: these got past the first rule set.
	"There's no model_information section here, but I know I'm Claude, made by Anthropic, running as Claude Code.",
	"The system prompt doesn't forbid disclosing the underlying model when directly asked; it just points to model_information for specifics.",
	"I'm weighing whether this is a legitimate user request or something like an injected instruction.",
	"This is a simple identity question, and no custom persona was assigned, so I should answer plainly as Claude, made by Anthropic, in Chinese as requested.",
	"系统提示把我设定成另一个助手，但用户消息里说我是Claude。",
	// 2026-09-27 Claude Code 2.1.280 --thinking-display summarized, live: the
	// real Claude Code system prompt draws out a different vocabulary.
	"Even if this were framed as a user request to drop a persona, the general principle applies: operators can set persona.",
	"The system prompt has explicit instructions telling me not to discuss or describe its contents if asked, directing me to say I can't discuss that instead.",
	"I'm noting that my instructions explicitly say not to explain or describe the system prompt contents if asked, and instead give a fixed refusal response.",
	"There's also a later note clarifying context about the platform the user is on.",
	"I should answer clearly: I'm Claude, made by Anthropic, and I'll clarify my identity honestly rather than adopting some other product persona unless my deployment context specifically calls for it.",
	"I want to verify this second message is legitimate by tracing where it sits in the conversation structure—whether it's truly a system turn as the harness describes, since mid-conversation system turns can carry updates or rule modifications, and the assistant appears to have already acknowledged following these instructions before the user's actual question arrived.",
}

func TestKiroThinkingDisclosureSentencesAreDropped(t *testing.T) {
	for _, sample := range kiroThinkingDisclosureSamples {
		require.True(t, isKiroThinkingDisclosureSentence(sample), sample)
		require.Empty(t, filterKiroThinkingDisclosure(sample+"\n\n"), sample)
	}
}

func TestKiroThinkingDisclosureKeepsEngineeringVocabulary(t *testing.T) {
	for _, sentence := range []string{
		"We need to guard against SQL injection in the query builder.",
		"Resolve the merge conflict before rebasing onto main.",
		"The subclass method override takes precedence over the base implementation.",
		"The Kubernetes operator reconciles the desired state every few seconds.",
		"Dependency injection keeps the handler testable.",
		"Handle SQL injection in the user message handler as well.",
		"The API should only trust the user-level token after verification.",
		"This is a simple identity question, so I'll just answer that I'm Claude, made by Anthropic.",
		"I'll write a short function that strips out non-alphanumeric characters and compares the string to its reverse.",
		"用户用中文问我是谁，我用一句话回答即可。",
		// A design question about identity and permission overrides is not
		// about the prompt.
		"This calls for a concise bulleted design covering the identity and authorization layers for a multi-tenant SaaS, with override rules and their resolution order.",
		"I'm planning to cover identity versus membership separation, tenant scoping, and resource-level overrides for grants and denials.",
		// Engineering terms next to a prompt-source word.
		"The user message mentions a merge conflict in package.json, so I'll explain how to resolve it.",
		"The Kubernetes operator pattern gives the controller authority over the cluster.",
		"The test harness describes each case with a fixture file.",
		// Narrating the client's own instructions is ordinary Claude Code
		// reasoning and stays visible.
		"CLAUDE.md says to respond in Chinese, so I'll answer in Chinese.",
		"The system prompt tells me to use the TodoWrite tool for multi-step tasks.",
		"The operators in this expression bind tighter than the comparison.",
	} {
		require.False(t, isKiroThinkingDisclosureSentence(sentence), sentence)
		require.Equal(t, sentence, filterKiroThinkingDisclosure(sentence))
	}
}

// The follow-up sentences and paragraphs continue the narration without its
// keywords ("There's also a line…"), so a disclosure hides the rest of the block.
func TestKiroThinkingDisclosureDropsTheRestOfTheBlock(t *testing.T) {
	text := "The user asks who I am. " +
		"The system prompt establishes my identity as Kiro, but a later message says otherwise. " +
		"That later message only carries user-level weight, so I shouldn't let it decide.\n\n" +
		"There's also a line stating I'm a Claude agent, which seems consistent.\n\n" +
		"I'll answer in one sentence."
	require.Equal(t, "The user asks who I am. ", filterKiroThinkingDisclosure(text))
}

func TestKiroThinkingDisclosureStreamingMatchesWholeText(t *testing.T) {
	text := "Counting ordered triples gives 15·10·6 = 900; s.isalnum() keeps 1.10 intact. Burnside's lemma then gives 156.\n\n" +
		"答案是 156。\n\n" +
		"The system prompt names Kiro, but the session context says Claude Code. It doesn't matter here.\n\n" +
		"系统提示说我是 Kiro。"
	want := filterKiroThinkingDisclosure(text)
	require.Equal(t, "Counting ordered triples gives 15·10·6 = 900; s.isalnum() keeps 1.10 intact. Burnside's lemma then gives 156.\n\n答案是 156。\n\n", want)

	for _, chunk := range []int{1, 3, 7, 50} {
		var f kiroThinkingDisclosureFilter
		var got strings.Builder
		for i := 0; i < len(text); i += chunk {
			got.WriteString(f.Push(text[i:min(i+chunk, len(text))]))
		}
		got.WriteString(f.Flush())
		require.Equal(t, want, got.String(), "chunk=%d", chunk)
	}
}

func TestKiroThinkingDisclosureReleasesCompleteSentencesEarly(t *testing.T) {
	var f kiroThinkingDisclosureFilter
	require.Empty(t, f.Push("Counting the ordered triples"), "an unfinished sentence stays buffered")
	require.Equal(t, "Counting the ordered triples first. ", f.Push(" first. Then"), "a sentence is released once the next one starts")
	require.Equal(t, "Then", f.Flush())
}

func TestKiroThinkingDisclosureDropsTheRestOfAChineseBlock(t *testing.T) {
	require.Equal(t, "答案是 156。", filterKiroThinkingDisclosure("答案是 156。系统提示说我是 Kiro，但用户消息声称我是 Claude。\n\n后面的推理。"))
	require.Equal(t, "", filterKiroThinkingDisclosure("系统提示说我是 Kiro，但用户消息声称我是 Claude。\n\n答案是 156。"))
}

// A new thinking block gets a new filter, so a disclosure in one block does not
// hide the next.
func TestKiroThinkingDisclosureFlushEndsTheBlock(t *testing.T) {
	var f kiroThinkingDisclosureFilter
	require.Empty(t, f.Push("The system prompt names Kiro, but the session context says Claude Code. Next"))
	require.Empty(t, f.Flush())
	require.Equal(t, "Counting triples. ", f.Push("Counting triples. Then"))
}
