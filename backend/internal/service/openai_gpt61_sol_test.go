package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/kiro"
	kironianzs "github.com/Wei-Shaw/sub2api/internal/pkg/kiro_nianzs"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGPT61SolModelContract(t *testing.T) {
	model := "gpt-6.1-sol"
	require.Contains(t, openai.DefaultModelIDs(), model)
	require.True(t, isOpenAIOAuthServableModel(model))
	require.True(t, isOpenAIGPT6Model(model))
	require.True(t, isOpenAIGPT61SolModel(model))
	require.False(t, isOpenAIGPT6SolModel(model))
	require.Equal(t, model, normalizeKnownOpenAICodexModel("openai/"+model))
	require.Equal(t, model, normalizeCodexModel(model))
	require.Equal(t, model, normalizeModelNameForPricing(model))
	require.True(t, supportsOpenAIReasoningEffortMax(model))
	require.True(t, configuredCodexSupportsPriorityServiceTier(model))
	require.True(t, isOpenAICodexImageInputModel(model))
	require.Contains(t, openai.CodexBaseInstructionsForModel(model), "GPT-6")
	for _, m := range kiro.DefaultModels {
		require.NotEqual(t, model, m.ID)
	}
	for _, m := range kironianzs.DefaultModels {
		require.NotEqual(t, model, m.ID)
	}

	descriptor := newConfiguredCodexModelDescriptor(model)
	require.Equal(t, "GPT-6.1 Sol", descriptor.DisplayName)
	require.EqualValues(t, 272000, descriptor.ContextWindow)
	require.EqualValues(t, 872000, descriptor.MaxContextWindow)
	require.Equal(t, "xhigh", *descriptor.MultiAgentReasoningEffort)
	require.Equal(t, []configuredCodexReasoningLevel{
		{Effort: "low", Description: "Fast responses with lighter reasoning"},
		{Effort: "medium", Description: "Balanced reasoning for most coding tasks"},
		{Effort: "high", Description: "Greater reasoning depth for coding and agent tasks"},
		{Effort: "xhigh", Description: "Extra-high reasoning depth for difficult tasks"},
		{Effort: "max", Description: "Maximum reasoning depth for complex tasks"},
		{Effort: "ultra", Description: "Maximum reasoning with automatic task delegation"},
	}, descriptor.SupportedReasoningLevels)
}

func TestGPT61SolProtocolForwarders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, protocol := range []string{"responses", "chat", "messages"} {
		for _, stream := range []bool{false, true} {
			t.Run(protocol+"/stream="+map[bool]string{false: "false", true: "true"}[stream], func(t *testing.T) {
				upstream := &httpUpstreamRecorder{resp: &http.Response{
					StatusCode: 200,
					Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
					Body:       io.NopCloser(strings.NewReader(gpt6SolLunaSSE("gpt-6.1-sol"))),
				}}
				svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
				account := &Account{ID: 7, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 1,
					Credentials: map[string]any{"access_token": "test-token", "chatgpt_account_id": "test-account"}}
				payload := map[string]any{"model": "gpt-6.1-sol", "stream": stream, "reasoning": map[string]any{"effort": "max"}}
				parameters := map[string]any{"type": "object", "properties": map[string]any{"value": map[string]any{"type": "string"}}, "required": []string{"value"}}
				function := map[string]any{"name": "probe_echo", "description": "Echo the given value", "parameters": parameters}
				payload["tools"] = []any{map[string]any{"type": "function", "name": "probe_echo", "description": "Echo the given value", "parameters": parameters}}
				path := "/v1/responses"
				if protocol == "responses" {
					payload["input"] = "hello"
				} else {
					payload["messages"] = []any{map[string]any{"role": "user", "content": "hello"}}
					payload["reasoning_effort"] = "max"
					payload["tools"] = []any{map[string]any{"type": "function", "function": function}}
					path = "/v1/chat/completions"
					if protocol == "messages" {
						payload["max_tokens"] = 128
						payload["output_config"] = map[string]any{"effort": "max"}
						payload["tools"] = []any{map[string]any{"name": "probe_echo", "description": "Echo the given value", "input_schema": parameters}}
						path = "/v1/messages"
					}
				}
				body, err := json.Marshal(payload)
				require.NoError(t, err)
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				c.Request = httptest.NewRequest("POST", path, strings.NewReader(string(body)))
				SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)

				var result *OpenAIForwardResult
				switch protocol {
				case "responses":
					result, err = svc.Forward(context.Background(), c, account, body)
				case "chat":
					result, err = svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
				case "messages":
					result, err = svc.ForwardAsAnthropic(context.Background(), c, account, body, "", "")
				}
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, "gpt-6.1-sol", gjson.GetBytes(upstream.lastBody, "model").String())
				require.Equal(t, "max", gjson.GetBytes(upstream.lastBody, "reasoning.effort").String())
				require.Equal(t, "probe_echo", gjson.GetBytes(upstream.lastBody, "tools.0.name").String())
				require.Equal(t, "string", gjson.GetBytes(upstream.lastBody, "tools.0.parameters.properties.value.type").String())
				require.Contains(t, rec.Body.String(), "world")
				require.Contains(t, rec.Body.String(), "probe_echo")
				if stream {
					terminal := "\"type\":\"response.completed\""
					if protocol == "chat" {
						terminal = "data: [DONE]"
					} else if protocol == "messages" {
						terminal = "event: message_stop"
					}
					require.Equal(t, 1, strings.Count(rec.Body.String(), terminal), rec.Body.String())
				}
			})
		}
	}
}

func TestGPT61SolNormalizesUnsupportedEffortsWithoutTouchingConversation(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	for _, effort := range []string{"none", "minimal"} {
		require.Equal(t, "low", normalizeOpenAIReasoningEffortForModel(effort, "gpt-6.1-sol"))
		body := []byte(`{"model":"gpt-6.1-sol","reasoning":{"effort":"` + effort + `"},"input":[{"type":"message","role":"user","content":"hello"}]}`)
		normalized, changed, err := normalizeOpenAIGPT6Request(account, body)
		require.NoError(t, err)
		require.True(t, changed)
		require.Equal(t, "low", gjson.GetBytes(normalized, "reasoning.effort").String())
		require.JSONEq(t, gjson.GetBytes(body, "input").Raw, gjson.GetBytes(normalized, "input").Raw)
	}
	for _, effort := range []string{"low", "medium", "high", "xhigh", "max"} {
		body := []byte(`{"model":"gpt-6.1-sol","reasoning":{"effort":"` + effort + `"},"input":"hello"}`)
		normalized, changed, err := normalizeOpenAIGPT6Request(account, body)
		require.NoError(t, err)
		require.False(t, changed)
		require.Equal(t, body, normalized)
	}
}

func TestGPT61SolPricingFallbackAndCatalog(t *testing.T) {
	body, err := os.ReadFile("../../resources/model-pricing/model_prices_and_context_window.json")
	require.NoError(t, err)
	catalog := &PricingService{}
	catalog.pricingData, err = catalog.parsePricingData(body)
	require.NoError(t, err)

	billing := NewBillingService(&config.Config{}, nil)
	for _, source := range []*PricingService{nil, catalog, {pricingData: map[string]*LiteLLMModelPricing{}}} {
		billing.pricingService = source
		price, err := billing.GetModelPricing("gpt-6.1-sol")
		require.NoError(t, err)
		require.InDelta(t, 2e-6, price.InputPricePerToken, 1e-14)
		require.InDelta(t, 10e-6, price.OutputPricePerToken, 1e-14)
		require.InDelta(t, 2.5e-6, price.CacheCreationPricePerToken, 1e-14)
		require.InDelta(t, 0.1e-6, price.CacheReadPricePerToken, 1e-14)
		require.InDelta(t, 4e-6, price.InputPricePerTokenPriority, 1e-14)
		require.InDelta(t, 20e-6, price.OutputPricePerTokenPriority, 1e-14)
		require.Equal(t, 272000, price.LongContextInputThreshold)
	}
	require.InDelta(t, 2, openAIModelFastPricingRatio("gpt-6.1-sol"), 1e-9)
	require.Contains(t, localChannelPricingModelNamesByProvider("openai"), "gpt-6.1-sol")

	var catalogEntries map[string]map[string]any
	require.NoError(t, json.Unmarshal(body, &catalogEntries))
	entry := catalogEntries["gpt-6.1-sol"]
	require.NotNil(t, entry)
	require.Equal(t, float64(2e-6), entry["input_cost_per_token"])
}
