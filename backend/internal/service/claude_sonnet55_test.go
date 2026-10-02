package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

var sonnet55Aliases = []string{"claude-sonnet-5-5", "claude-sonnet-5-5-thinking", "claude-sonnet-5.5", "claude-sonnet-5.5-thinking"}

func TestSonnet55PricingAndAliases(t *testing.T) {
	data, err := os.ReadFile("../../resources/model-pricing/model_prices_and_context_window.json")
	require.NoError(t, err)
	catalog := &PricingService{}
	catalog.pricingData, err = catalog.parsePricingData(data)
	require.NoError(t, err)
	for _, source := range []*PricingService{nil, catalog, {pricingData: map[string]*LiteLLMModelPricing{}}} {
		svc := NewBillingService(&config.Config{}, nil)
		svc.pricingService = source
		for _, model := range sonnet55Aliases {
			require.Equal(t, "claude-sonnet-5-5", normalizeAnthropicModelIDForUpstream(model))
			price, err := svc.GetModelPricing(model)
			require.NoError(t, err, model)
			require.InDelta(t, 2e-6, price.InputPricePerToken, 1e-14, model)
			require.InDelta(t, 10e-6, price.OutputPricePerToken, 1e-14, model)
			require.InDelta(t, .2e-6, price.CacheReadPricePerToken, 1e-14, model)
			require.InDelta(t, 2.5e-6, price.CacheCreation5mPrice, 1e-14, model)
			require.InDelta(t, 4e-6, price.CacheCreation1hPrice, 1e-14, model)
			require.Zero(t, price.LongContextInputThreshold)
		}
	}
}

func TestSonnet55TypedNormalizationAndSignedHistory(t *testing.T) {
	req := convertOpenAIChatForAnthropic(t, "claude-sonnet-5-5", "claude-sonnet-5-5", "high")
	require.Equal(t, "adaptive", req.Thinking.Type)
	require.Zero(t, req.Thinking.BudgetTokens)
	req.Thinking = &apicompat.AnthropicThinking{Type: "disabled", BudgetTokens: 4000, Display: "summarized"}
	req.OutputConfig = &apicompat.AnthropicOutputConfig{Effort: "max"}
	normalizeAnthropicSonnet55Request(req)
	require.Equal(t, &apicompat.AnthropicThinking{Type: "between_tools"}, req.Thinking)
	require.Equal(t, "high", req.OutputConfig.Effort)
	body := []byte(`{"model":"claude-sonnet-5-5","thinking":{"type":"between_tools"},"messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"progress","signature":"valid-provider-signature"},{"type":"text","text":"next"}]},{"role":"user","content":"continue"}]}`)
	out := FilterThinkingBlocks(body, "claude-sonnet-5-5")
	require.JSONEq(t, string(body), string(out), "between_tools progress blocks must survive normal history filtering")
	require.Equal(t, string(body), string(PrepareSharedAnthropicThinkingHistory(body, &Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey})))
}

// Exercise the real Forward entrypoints: normal and passthrough Messages,
// Chat, Responses and count_tokens all construct the same model contract.
func TestSonnet55DirectForwardEntryPoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, protocol := range []string{"messages", "passthrough", "chat", "responses", "count"} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%v", protocol, stream), func(t *testing.T) {
				response := `{"id":"msg_55","type":"message","role":"assistant","model":"claude-sonnet-5-5","content":[{"type":"text","text":"OK"}],"stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":2}}`
				if stream || (protocol != "passthrough" && protocol != "count") {
					response = "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_55\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-sonnet-5-5\",\"content\":[],\"stop_reason\":null,\"usage\":{\"input_tokens\":10,\"output_tokens\":0}}}\n\nevent: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\nevent: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"OK\"}}\n\nevent: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\nevent: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":2}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
				}
				if protocol == "count" {
					response = `{"input_tokens":10}`
				}
				u := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(response))}}
				svc := &GatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}, httpUpstream: u, tlsFPProfileService: &TLSFingerprintProfileService{}, rateLimitService: &RateLimitService{}}
				a := newAnthropicSoft429APIKeyAccount(protocol == "passthrough" || protocol == "count")
				var body []byte
				switch protocol {
				case "chat":
					body = []byte(fmt.Sprintf(`{"model":"claude-sonnet-5.5","stream":%v,"temperature":0.3,"reasoning_effort":"high","messages":[{"role":"user","content":"hello"}]}`, stream))
				case "responses":
					body = []byte(fmt.Sprintf(`{"model":"claude-sonnet-5.5","stream":%v,"temperature":0.3,"reasoning":{"effort":"high"},"input":"hello"}`, stream))
				default:
					body = []byte(fmt.Sprintf(`{"model":"claude-sonnet-5.5","stream":%v,"temperature":0.3,"thinking":{"type":"enabled","budget_tokens":4000},"messages":[{"role":"user","content":"hello"}]}`, stream))
				}
				c, rec := newAnthropicSoft429GinContext()
				parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
				require.NoError(t, err)
				switch protocol {
				case "chat":
					_, err = svc.ForwardAsChatCompletions(context.Background(), c, a, body, parsed)
				case "responses":
					_, err = svc.ForwardAsResponses(context.Background(), c, a, body, parsed)
				case "count":
					err = svc.ForwardCountTokens(context.Background(), c, a, parsed)
				default:
					_, err = svc.Forward(context.Background(), c, a, parsed)
				}
				require.NoError(t, err)
				require.Len(t, u.requests, 1)
				require.Equal(t, "claude-sonnet-5-5", gjson.GetBytes(u.lastBody, "model").String())
				require.Equal(t, "adaptive", gjson.GetBytes(u.lastBody, "thinking.type").String())
				require.False(t, gjson.GetBytes(u.lastBody, "thinking.budget_tokens").Exists())
				require.False(t, gjson.GetBytes(u.lastBody, "temperature").Exists())
				if protocol != "count" {
					require.Contains(t, rec.Body.String(), "OK")
				}
			})
		}
	}
}

// Keep a new model inside the existing provider-family malformed-turn guard.
// The synthetic future model verifies that this remains a capability policy.
func TestSonnet55KiroProtocolLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, model := range []string{"claude-sonnet-5-5", "claude-sonnet-9-1"} {
		for _, protocol := range []string{"messages", "chat", "responses"} {
			for _, stream := range []bool{false, true} {
				for _, scenario := range []string{"retry", "short", "exhausted"} {
					t.Run(fmt.Sprintf("%s/%s/%v/%s", model, protocol, stream, scenario), func(t *testing.T) {
						text := "I'll inspect the repository before continuing.\n\ncall"
						svc, u, a := newKiroNativeGPTTestRuntime(t, "")
						enableKiroNativeGPTEnforceMode(svc)
						a.Credentials["model_mapping"] = map[string]any{model: strings.Replace(model, "-5-5", "-5.5", 1)}
						u.resp = nil
						switch scenario {
						case "retry":
							u.responses = []*http.Response{kiroNativeGPTPreludeResponse(t, text), kiroCustomToolEventStreamResponse(t, "t55", "read", `{"path":"README.md"}`)}
						case "short":
							u.responses = []*http.Response{kiroEventStreamResponse(t, "OK", 11, 2)}
						case "exhausted":
							u.responses = []*http.Response{kiroNativeGPTPreludeResponse(t, text), kiroNativeGPTPreludeResponse(t, text)}
						}
						var body []byte
						switch protocol {
						case "messages":
							body = []byte(fmt.Sprintf(`{"model":%q,"stream":%v,"max_tokens":2048,"messages":[{"role":"user","content":"inspect"}],"tools":[{"name":"read","input_schema":{"type":"object","properties":{"path":{"type":"string"}}}}]}`, model, stream))
						case "chat":
							body = []byte(fmt.Sprintf(`{"model":%q,"stream":%v,"messages":[{"role":"user","content":"inspect"}],"tools":[{"type":"function","function":{"name":"read","parameters":{"type":"object","properties":{"path":{"type":"string"}}}}}]}`, model, stream))
						case "responses":
							body = []byte(fmt.Sprintf(`{"model":%q,"stream":%v,"input":"inspect","tools":[{"type":"function","name":"read","parameters":{"type":"object","properties":{"path":{"type":"string"}}}}]}`, model, stream))
						}
						c, rec := newAnthropicSoft429GinContext()
						c.Request = httptest.NewRequest("POST", "/v1/"+protocol, bytes.NewReader(body))
						p := &ParsedRequest{Body: NewRequestBodyRef(body), Model: model, Stream: stream}
						var err error
						switch protocol {
						case "messages":
							p, err = ParseGatewayRequest(NewRequestBodyRef(body), PlatformKiro)
							require.NoError(t, err)
							_, err = svc.Forward(context.Background(), c, a, p)
						case "chat":
							_, err = svc.ForwardAsChatCompletions(context.Background(), c, a, body, p)
						case "responses":
							resetKiroResponsesHistoryStoreForTest()
							_, err = svc.ForwardAsResponses(context.Background(), c, a, body, p)
						}
						require.NotContains(t, rec.Body.String(), "I'll inspect")
						require.NotContains(t, rec.Body.String(), "\n\ncall")
						if scenario == "exhausted" {
							require.Error(t, err)
							require.Len(t, u.requests, 2)
							require.Empty(t, rec.Body.String())
							return
						}
						require.NoError(t, err)
						if scenario == "short" {
							require.Len(t, u.requests, 1)
							require.Contains(t, rec.Body.String(), "OK")
						} else {
							require.Len(t, u.requests, 2)
							require.Contains(t, rec.Body.String(), "read")
						}
						if stream {
							terminal := "event: message_stop"
							if protocol == "chat" {
								terminal = "data: [DONE]"
							}
							if protocol == "responses" {
								terminal = "event: response.completed"
							}
							require.Equal(t, 1, strings.Count(rec.Body.String(), terminal))
						}
					})
				}
			}
		}
	}
}

func TestSonnet55CacheMinimumAndContinuity(t *testing.T) {
	for _, model := range sonnet55Aliases {
		require.Equal(t, 512, kiroMinimumCacheableTokens(model))
		require.Equal(t, 512, nianzsKiroMinimumCacheableTokens(model))
	}
	for _, ttl := range []bool{false, true} {
		resetNianzsKiroCacheTracker()
		svc := &GatewayService{}
		a := &Account{ID: 55, Platform: PlatformKiro}
		g := nianzsTestKiroCacheGroup(1)
		body := bytes.ReplaceAll(nianzsTestKiroCacheRequestBody("sonnet55", ttl), []byte("claude-sonnet-4-6"), []byte("claude-sonnet-5-5"))
		first := svc.buildKiroCacheEmulationUsageNianzs(context.Background(), a, g, body, "claude-sonnet-5-5", 700)
		require.NotNil(t, first)
		require.Greater(t, first.CacheCreationInputTokens, 0)
		second := svc.buildKiroCacheEmulationUsageNianzs(context.Background(), a, g, body, "claude-sonnet-5-5", 700)
		require.NotNil(t, second)
		require.Greater(t, second.CacheReadInputTokens, 0)
		require.Zero(t, second.CacheCreationInputTokens)
	}
}

func BenchmarkSonnet55Normalization(b *testing.B) {
	body, _ := json.Marshal(map[string]any{"model": "claude-sonnet-5-5", "thinking": map[string]string{"type": "adaptive"}, "messages": []map[string]string{{"role": "user", "content": strings.Repeat("context ", 25000)}}})
	b.ReportAllocs()
	b.SetBytes(int64(len(body)))
	for i := 0; i < b.N; i++ {
		claude.NormalizeSonnet55Request(body, "claude-sonnet-5-5")
	}
}
