package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
)

// GPT-6 模型各自只有一个官方 ID（Astra / Sol / Luna）。推理档位属于请求参数，
// 绝不做成合成的模型名后缀：2026-09-23 实测上游对 gpt-6-terra 及任意其它
// gpt-6-* 拼写一律 400。这里只复用统一的供应商/拼写归一化。

// gpt6UnsupportedSamplingFields 是 GPT-6 全族在 Responses 上游一律拒绝的采样/日志参数。
// 2026-09-23 对 Astra / Sol / Luna 三个模型逐项实测，均返回 400 Unsupported parameter。
var gpt6UnsupportedSamplingFields = []string{"temperature", "top_p", "top_logprobs", "logprobs", "prompt_cache_retention"}

func normalizeOpenAIGPT6LegacyCacheOptions(req map[string]any) bool {
	model, _ := req["model"].(string)
	if !isOpenAIGPT6Model(model) {
		return false
	}
	if _, exists := req["prompt_cache_retention"]; !exists {
		return false
	}
	options, _ := req["prompt_cache_options"].(map[string]any)
	if options == nil {
		options = map[string]any{}
		req["prompt_cache_options"] = options
	}
	if _, exists := options["ttl"]; !exists {
		options["ttl"] = "30m"
	}
	delete(req, "prompt_cache_retention")
	return true
}

// gpt6EffortReplacement 返回该模型需要改写的推理档位替换值，无需改写时返回 ""。
//
// "minimal" 不是 GPT-6 的合法档位，统一改写为 "low"。"none" 仅对官方确认支持
// 它的 GPT-6 Sol / Luna 保留；Astra 与 GPT-6.1 Sol 都必须降级为 "low"。
func gpt6EffortReplacement(model string, effort string) string {
	switch strings.ToLower(strings.TrimSpace(effort)) {
	case "minimal":
		return "low"
	case "none":
		if !isOpenAIGPT6SolModel(model) && !isOpenAIGPT6LunaModel(model) {
			return "low"
		}
	}
	return ""
}

// normalizeOpenAIGPT6Request applies the GPT-6 family's documented wire contract
// at the final OpenAI boundary. It does not alter other providers or conversation
// items (including configuration_update, async tools, and encrypted reasoning).
func normalizeOpenAIGPT6Request(account *Account, body []byte) ([]byte, bool, error) {
	model := gjson.GetBytes(body, "model").String()
	if account == nil || !account.IsOpenAI() || !isOpenAIGPT6Model(model) {
		return body, false, nil
	}
	// Valid native requests are the common path: preserve their original bytes
	// and avoid copying the full (potentially 1M-context) conversation.
	needsNormalization := false
	if account.IsOpenAIOAuthLike() {
		needsNormalization = gjson.GetBytes(body, "prompt_cache_options").Exists()
	}
	for _, key := range gpt6UnsupportedSamplingFields {
		needsNormalization = needsNormalization || gjson.GetBytes(body, key).Exists()
	}
	for _, key := range []string{"reasoning.effort", "reasoning_effort"} {
		effort := gjson.GetBytes(body, key).String()
		needsNormalization = needsNormalization || gpt6EffortReplacement(model, effort) != ""
	}
	for _, value := range gjson.GetBytes(body, "include").Array() {
		needsNormalization = needsNormalization || value.String() == "message.output_text.logprobs"
	}
	if !needsNormalization {
		return body, false, nil
	}
	var req map[string]any
	if err := decodeOpenAIJSONUseNumber(body, &req); err != nil {
		return body, false, err
	}
	changed := false
	if account.IsOpenAIOAuthLike() {
		// Verified Codex OAuth backend rejects the public API cache options.
		for _, key := range []string{"prompt_cache_options", "prompt_cache_retention"} {
			if _, exists := req[key]; exists {
				delete(req, key)
				changed = true
			}
		}
	} else {
		changed = normalizeOpenAIGPT6LegacyCacheOptions(req)
	}
	for _, key := range gpt6UnsupportedSamplingFields {
		if _, ok := req[key]; ok {
			delete(req, key)
			changed = true
		}
	}
	for _, key := range []string{"reasoning_effort", "effort"} {
		target := req
		if key == "effort" {
			target, _ = req["reasoning"].(map[string]any)
		}
		if value, ok := target[key].(string); ok {
			if replacement := gpt6EffortReplacement(model, value); replacement != "" {
				target[key] = replacement
				changed = true
			}
		}
	}
	if include, ok := req["include"].([]any); ok {
		filtered := make([]any, 0, len(include))
		for _, value := range include {
			if value == "message.output_text.logprobs" {
				changed = true
				continue
			}
			filtered = append(filtered, value)
		}
		if len(filtered) != len(include) {
			req["include"] = filtered
		}
	}
	if !changed {
		return body, false, nil
	}
	result, err := json.Marshal(req)
	if err != nil {
		return body, false, fmt.Errorf("normalize GPT-6 request: %w", err)
	}
	return result, true, nil
}
