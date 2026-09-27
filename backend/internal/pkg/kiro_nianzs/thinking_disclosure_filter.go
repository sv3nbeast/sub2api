package kiro

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// Kiro's reasoning summaries can narrate how the model resolved its identity:
// the Kiro system prompt names Kiro while the injected session context names
// Claude, and Opus 5.5 in particular spells that out ("the system prompt
// establishes Kiro, but a user-turn message claims I'm Claude Code…"). The
// thinking stream is filtered sentence by sentence so that this narration never
// reaches the client, while the rest of the reasoning streams as it arrives.
//
// A sentence is a disclosure when it names Kiro or the injected notes, when it
// talks about identity and about the prompt structure together, or when it
// weighs the authority of the prompt structure. Requiring two signals keeps
// ordinary engineering vocabulary ("SQL injection", "merge conflict", "method
// override", "Kubernetes operator") visible. Once a disclosure sentence is
// found, the rest of its paragraph is dropped too: the follow-up sentences
// continue the same argument without repeating the keywords.
var (
	kiroThinkingDisclosureAlways = regexp.MustCompile(`(?i)\bkiro\b|model_information|deployment notes|\bcustom persona\b|\bpersona (?:was |is |has been )?(?:assigned|given|set|configured|specified)\b|\binjected instructions?\b|session (?:context|information|note)|harness message|persona-breaking|operator[- ](?:level|notes|authority|instructions?|framing|persona|configuration)|identity (?:claim|signal|framing|swap|assignment|override|context|clarification)`)
	kiroThinkingIdentityTopic    = regexp.MustCompile(`(?i)\bidentit(?:y|ies)\b|\bpersonas?\b|\bwho i am\b|\bidentif(?:y|ies|ying) (?:me|myself|as)\b|\bi'?m (?:actually |really )?(?:claude|kiro)\b|\bi am (?:actually |really )?(?:claude|kiro)\b|身份|人设|我是谁|我是\s*(?:claude|kiro)`)
	kiroThinkingPromptStructure  = regexp.MustCompile(`(?i)system[- ]?(?:prompt|level|message|directive|instruction|assigned)|\bthe system (?:prompt |message )?(?:names|identifies|establishes|says|sets|claims|mentions)|user[- ](?:turn|message|level|authored)|\boperator\b|\boverrid(?:e|es|ing) (?:my|me|the system|what the system|that|this|it|who)\b|\bconflict|\bmismatch|\bdiscrepanc|\btension\b|\bcontradict|\binjection\b|\binjected\b|\bprecedence\b|\bauthorit(?:y|ative)\b|\b(?:later|earlier|preceding|previous|prior|embedded|other) (?:message|instruction|note|context|framing|text)|\bframing\b|系统提示|用户消息|运营|注入|冲突|覆盖|优先|权威`)
	kiroThinkingStructureSource  = regexp.MustCompile(`(?i)system[- ]?(?:prompt|level|message|directive|instruction)|user[- ](?:turn|authored)|\buser message\b|\buser[- ]level (?:instruction|context|claim|content|authority)|\boperator\b|\btool result|系统提示|用户消息|运营`)
	// kiroThinkingEngineeringTerms are removed before the signals are checked, so
	// that a sentence about a merge conflict in the user message is not read as a
	// conflict between prompt sources.
	kiroThinkingEngineeringTerms = regexp.MustCompile(`(?i)\b(?:merge|version|dependency|rebase) conflicts?\b|\bconflict markers?\b|\bconflicting (?:versions?|changes|edits|dependencies|writes)\b|\b(?:sql|nosql|dependency|code|command|template|header|html|ldap|xpath|xml|log) injection\b|\bkubernetes operators?\b|\boperator pattern\b|\bmethod overrid(?:e|es|ing)\b`)
	kiroThinkingAuthorityWeigh   = regexp.MustCompile(`(?i)\bauthorit(?:y|ative)\b|\bprecedence\b|\blegitima(?:te|cy)\b|\bprivileged\b|\bclaim(?:s|ing)? (?:to be|operator|special|that i|i'?m|i am)|\b(?:an|prompt|no|not an) injection\b|\binjected (?:via|through|by|content|instruction|text|context)|\boverrid(?:e|es|ing) (?:my|the system|what the system|that)|\bconflict|\bmismatch|\bdiscrepanc|\btension\b|\bcontradict|权威|优先|可信|注入|覆盖|冲突`)
)

// isKiroThinkingDisclosureSentence reports whether a reasoning sentence
// narrates the identity resolution described above.
func isKiroThinkingDisclosureSentence(sentence string) bool {
	if strings.TrimSpace(sentence) == "" {
		return false
	}
	if kiroThinkingDisclosureAlways.MatchString(sentence) {
		return true
	}
	sentence = kiroThinkingEngineeringTerms.ReplaceAllString(sentence, " ")
	if kiroThinkingIdentityTopic.MatchString(sentence) && kiroThinkingPromptStructure.MatchString(sentence) {
		return true
	}
	return kiroThinkingStructureSource.MatchString(sentence) && kiroThinkingAuthorityWeigh.MatchString(sentence)
}

// kiroThinkingDisclosureFilter buffers streamed reasoning until a sentence is
// complete, then releases it unless it belongs to a disclosure. The latency it
// adds is one sentence. A filter serves one thinking block.
type kiroThinkingDisclosureFilter struct {
	pending strings.Builder
	// dropping is set by a disclosure sentence and cleared at the end of its
	// paragraph.
	dropping bool
}

// Push adds streamed reasoning text and returns the text that may be shown now.
func (f *kiroThinkingDisclosureFilter) Push(text string) string {
	if text == "" {
		return ""
	}
	f.pending.WriteString(text)
	buffered := f.pending.String()
	cut := lastKiroThinkingSentenceBoundary(buffered)
	if cut <= 0 {
		return ""
	}
	f.pending.Reset()
	f.pending.WriteString(buffered[cut:])
	return f.filterSentences(buffered[:cut])
}

// Flush releases whatever is still buffered at the end of the thinking block.
func (f *kiroThinkingDisclosureFilter) Flush() string {
	rest := f.pending.String()
	f.pending.Reset()
	out := f.filterSentences(rest)
	f.dropping = false
	return out
}

func (f *kiroThinkingDisclosureFilter) filterSentences(text string) string {
	var out strings.Builder
	for _, sentence := range splitKiroThinkingSentences(text) {
		if isKiroThinkingDisclosureSentence(sentence) {
			f.dropping = true
		}
		if !f.dropping {
			out.WriteString(sentence)
		}
		if strings.Contains(sentence[len(strings.TrimRight(sentence, " \t\r\n")):], "\n\n") {
			f.dropping = false
		}
	}
	return out.String()
}

// filterKiroThinkingDisclosure filters a complete thinking text, as used by
// the non-streaming response. It yields exactly what the streaming filter
// releases for the same text, however the text was chunked.
func filterKiroThinkingDisclosure(text string) string {
	var f kiroThinkingDisclosureFilter
	return f.Push(text) + f.Flush()
}

// splitKiroThinkingSentences splits text into sentences that keep their
// trailing punctuation and whitespace, so concatenating them restores the text.
func splitKiroThinkingSentences(text string) []string {
	var sentences []string
	for text != "" {
		cut := firstKiroThinkingSentenceBoundary(text)
		if cut <= 0 {
			sentences = append(sentences, text)
			break
		}
		sentences = append(sentences, text[:cut])
		text = text[cut:]
	}
	return sentences
}

// firstKiroThinkingSentenceBoundary returns the byte offset just past the first
// sentence terminator and the whitespace after it, or 0 when the text holds no
// complete sentence yet. ASCII terminators only count when followed by
// whitespace, so "s.isalnum()" and "1.10" stay intact; CJK terminators and
// newlines end a sentence on their own.
func firstKiroThinkingSentenceBoundary(text string) int {
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		next := i + size
		switch r {
		case '\n', '。', '！', '？', '；':
			return next + leadingKiroThinkingWhitespace(text[next:])
		case '.', '!', '?':
			if ws := leadingKiroThinkingWhitespace(text[next:]); ws > 0 {
				return next + ws
			}
		}
		i = next
	}
	return 0
}

// lastKiroThinkingSentenceBoundary returns the end of the last sentence that
// is known to be complete: its whitespace run must be followed by more text,
// because the next chunk could otherwise still extend the run. This keeps the
// streamed result identical to filtering the whole text at once.
func lastKiroThinkingSentenceBoundary(text string) int {
	last := 0
	for offset := 0; offset < len(text); {
		cut := firstKiroThinkingSentenceBoundary(text[offset:])
		if cut <= 0 || offset+cut >= len(text) {
			break
		}
		offset += cut
		last = offset
	}
	return last
}

func leadingKiroThinkingWhitespace(text string) int {
	n := 0
	for n < len(text) && (text[n] == ' ' || text[n] == '\n' || text[n] == '\t' || text[n] == '\r') {
		n++
	}
	return n
}
