package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type unavailableClaudeProxyRepo struct{ ProxyRepository }

func (*unavailableClaudeProxyRepo) GetByID(context.Context, int64) (*Proxy, error) {
	return nil, errors.New("proxy lookup failed")
}

func TestClaudeOAuthProxyLookupFailsClosed(t *testing.T) {
	// An embedded nil client panics if any network method is reached.
	svc := NewOAuthService(&unavailableClaudeProxyRepo{}, nil)
	defer svc.Stop()
	id := int64(42)
	account := &Account{
		Platform: PlatformAnthropic, Type: AccountTypeOAuth, ProxyID: &id,
		Proxy:       &Proxy{ID: id, Protocol: "http", Host: "proxy.invalid", Port: 8080},
		Credentials: map[string]any{"refresh_token": "test-refresh"},
	}
	_, err := svc.GenerateAuthURL(context.Background(), &id)
	require.ErrorIs(t, err, ErrAccountProxyUnavailable)
	_, err = svc.CookieAuth(context.Background(), &CookieAuthInput{ProxyID: &id})
	require.ErrorIs(t, err, ErrAccountProxyUnavailable)
	_, err = svc.RefreshAccountToken(context.Background(), account)
	require.ErrorIs(t, err, ErrAccountProxyUnavailable)
	session, err := svc.GenerateAuthURL(context.Background(), nil)
	require.NoError(t, err)
	_, err = svc.ExchangeCode(context.Background(), &ExchangeCodeInput{SessionID: session.SessionID, ProxyID: &id, Code: "test-code"})
	require.ErrorIs(t, err, ErrAccountProxyUnavailable)
	_, exists := svc.sessionStore.Get(session.SessionID)
	require.True(t, exists, "proxy outage must not consume the authorization session")
}

func TestClaudeAccountProxyGuard(t *testing.T) {
	id := int64(42)
	for _, tc := range []struct {
		name  string
		proxy *Proxy
	}{
		{"missing", nil},
		{"wrong_id", &Proxy{ID: 43, Protocol: "http", Host: "proxy.invalid", Port: 8080}},
		{"invalid_url", &Proxy{ID: id, Protocol: "invalid", Host: "proxy.invalid", Port: 8080, Password: "private-password"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := &Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth, ProxyID: &id, Proxy: tc.proxy}
			_, _, err := (&GatewayService{}).GetAccessToken(context.Background(), account)
			require.ErrorIs(t, err, ErrAccountProxyUnavailable)
			require.NotContains(t, err.Error(), "private-password")
			_, err = (&ClaudeTokenProvider{}).GetAccessToken(context.Background(), account)
			require.ErrorIs(t, err, ErrAccountProxyUnavailable)
			account.Type = AccountTypeSetupToken
			_, err = validatedAccountProxyURL(account)
			require.ErrorIs(t, err, ErrAccountProxyUnavailable)
		})
	}
	account := &Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth}
	proxy, err := validatedAccountProxyURL(account)
	require.NoError(t, err)
	require.Empty(t, proxy, "explicit no-proxy remains direct")
	account.ProxyID = &id
	account.Platform = PlatformOpenAI
	_, err = validatedAccountProxyURL(account)
	require.NoError(t, err, "other providers retain their existing behavior")
}

type claudeProxyOutbound struct{ host, proxy, relayProxy string }
type claudeProxyRecordingUpstream struct{ calls chan claudeProxyOutbound }

func (u *claudeProxyRecordingUpstream) Do(r *http.Request, p string, id int64, c int) (*http.Response, error) {
	return u.DoWithTLS(r, p, id, c, nil)
}
func (u *claudeProxyRecordingUpstream) DoWithTLS(r *http.Request, p string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	u.calls <- claudeProxyOutbound{r.URL.Hostname(), p, r.URL.Query().Get("proxy")}
	return nil, errors.New("test: stopped at outbound boundary")
}

func TestClaudeProxyForwardAndCompanions(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, relay := range []bool{false, true} {
			for _, hydrated := range []bool{false, true} {
				t.Run(fmt.Sprintf("stream=%t/relay=%t/hydrated=%t", stream, relay, hydrated), func(t *testing.T) {
					cfg := &config.Config{}
					cfg.Gateway.ClaudeCodeMimicry = config.GatewayClaudeCodeMimicryConfig{
						Enabled: true,
						SyntheticCompanion: config.GatewayClaudeCodeSyntheticCompanionConfig{
							Enabled: true, Mode: config.ClaudeCodeSyntheticCompanionModeAuxOnly,
							TimeoutSeconds: 2, MinIntervalSeconds: 1,
						},
					}
					up := &claudeProxyRecordingUpstream{calls: make(chan claudeProxyOutbound, 16)}
					svc := &GatewayService{cfg: cfg, httpUpstream: up, claudeCodeCompanionProbe: NewClaudeCodeCompanionProbeService(up)}
					id := int64(42)
					account := &Account{
						ID: 7, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Concurrency: 1,
						ProxyID: &id, Credentials: map[string]any{"access_token": "test-token"},
						Extra: map[string]any{"custom_base_url_enabled": relay, "custom_base_url": "https://relay.invalid"},
					}
					if hydrated {
						account.Proxy = &Proxy{ID: id, Protocol: "http", Host: "proxy.invalid", Port: 8080}
					}
					c, _ := gin.CreateTestContext(httptest.NewRecorder())
					c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
					body := []byte(fmt.Sprintf(`{"model":"claude-haiku-4-5","stream":%t,"messages":[{"role":"user","content":"hello"}]}`, stream))
					parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
					require.NoError(t, err)
					_, err = svc.Forward(context.Background(), c, account, parsed)
					if !hydrated {
						require.ErrorIs(t, err, ErrAccountProxyUnavailable)
						require.Empty(t, up.calls, "no main or companion request may be sent without the configured proxy")
						return
					}
					official, relayCalls := 0, 0
					for i := 0; i < 8; i++ {
						select {
						case call := <-up.calls:
							if call.host == "relay.invalid" {
								relayCalls++
								require.Empty(t, call.proxy)
								require.Equal(t, account.Proxy.URL(), call.relayProxy)
							} else {
								official++
								require.Equal(t, "api.anthropic.com", call.host)
								require.Equal(t, account.Proxy.URL(), call.proxy)
							}
						case <-time.After(3 * time.Second):
							t.Fatalf("only received %d of 8 calls", i)
						}
					}
					if relay {
						require.Equal(t, 1, relayCalls)
						require.Equal(t, 7, official)
					} else {
						require.Equal(t, 8, official)
					}
				})
			}
		}
	}
}

type proxyGuardAccountRepo struct {
	AccountRepository
	account *Account
}

func (r *proxyGuardAccountRepo) GetByID(context.Context, int64) (*Account, error) {
	return r.account, nil
}

func TestClaudeProxyGuardAdminPaths(t *testing.T) {
	id := int64(42)
	account := &Account{ID: 7, Platform: PlatformAnthropic, Type: AccountTypeOAuth, ProxyID: &id, Credentials: map[string]any{"access_token": "test"}}
	// Nil network dependencies ensure these paths stop before contacting upstream.
	_, err := (&AccountUsageService{}).fetchOAuthUsageRaw(context.Background(), account)
	require.ErrorIs(t, err, ErrAccountProxyUnavailable)
	svc := &AccountTestService{accountRepo: &proxyGuardAccountRepo{account: account}}
	_, err = svc.FetchUpstreamSupportedModels(context.Background(), account)
	require.ErrorIs(t, err, ErrAccountProxyUnavailable)
	_, err = svc.SyncUpstreamModelCatalog(context.Background(), account)
	require.ErrorIs(t, err, ErrAccountProxyUnavailable)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/test", nil)
	err = svc.TestAccountConnection(c, account.ID, "claude-haiku-4-5", "", "")
	require.ErrorContains(t, err, ErrAccountProxyUnavailable.Error())
	require.Contains(t, rec.Body.String(), ErrAccountProxyUnavailable.Error())
}

type proxyGuardSuccessUpstream struct{ proxy string }

func (u *proxyGuardSuccessUpstream) Do(r *http.Request, p string, id int64, c int) (*http.Response, error) {
	return u.DoWithTLS(r, p, id, c, nil)
}
func (u *proxyGuardSuccessUpstream) DoWithTLS(_ *http.Request, p string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	u.proxy = p
	sse := strings.Replace(anthropicMinimalSSEResponse+"\n", `"input_tokens":12,"output_tokens":0`, `"input_tokens":12,"output_tokens":0,"cache_read_input_tokens":100,"cache_creation_input_tokens":30`, 1)
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(sse))}, nil
}

func TestClaudeProxyGuardProtocolRoundTrips(t *testing.T) {
	for _, protocol := range []string{"messages", "chat", "responses", "count_tokens"} {
		for _, stream := range []bool{false, true} {
			for _, hydrated := range []bool{false, true} {
				if protocol == "count_tokens" && (stream || hydrated) {
					continue
				}
				t.Run(fmt.Sprintf("%s/stream=%t/hydrated=%t", protocol, stream, hydrated), func(t *testing.T) {
					body := []byte(fmt.Sprintf(`{"model":"claude-haiku-4-5","max_tokens":32,"stream":%t,"messages":[{"role":"user","content":"hello"}],"input":"hello"}`, stream))
					parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
					require.NoError(t, err)
					rec := httptest.NewRecorder()
					c, _ := gin.CreateTestContext(rec)
					c.Request = httptest.NewRequest(http.MethodPost, "/v1/"+protocol, nil)
					id := int64(42)
					account := &Account{ID: 7, Platform: PlatformAnthropic, Type: AccountTypeOAuth, ProxyID: &id,
						Credentials: map[string]any{"access_token": "test-token"}, Extra: map[string]any{"force_stream_upstream": true}}
					if hydrated {
						account.Proxy = &Proxy{ID: id, Protocol: "socks5", Host: "proxy.invalid", Port: 1080}
					}
					up := &proxyGuardSuccessUpstream{}
					cfg := &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}
					svc := &GatewayService{cfg: cfg, httpUpstream: up, responseHeaderFilter: compileResponseHeaderFilter(cfg), rateLimitService: &RateLimitService{}, deferredService: &DeferredService{}}
					var result *ForwardResult
					switch protocol {
					case "messages":
						result, err = svc.Forward(context.Background(), c, account, parsed)
					case "chat":
						result, err = svc.ForwardAsChatCompletions(context.Background(), c, account, body, parsed)
					case "responses":
						result, err = svc.ForwardAsResponses(context.Background(), c, account, body, parsed)
					case "count_tokens":
						err = svc.ForwardCountTokens(context.Background(), c, account, parsed)
					}
					if !hydrated {
						require.ErrorIs(t, err, ErrAccountProxyUnavailable)
						require.Empty(t, up.proxy)
						return
					}
					require.NoError(t, err)
					require.NotNil(t, result)
					require.Equal(t, "socks5h://proxy.invalid:1080", up.proxy)
					require.Equal(t, 7, result.Usage.OutputTokens)
					require.Equal(t, 100, result.Usage.CacheReadInputTokens)
					require.Equal(t, 30, result.Usage.CacheCreationInputTokens)
					if stream {
						require.Contains(t, rec.Body.String(), "ok")
						terminal := map[string]string{"messages": "event: message_stop", "chat": "[DONE]", "responses": "response.completed"}[protocol]
						require.Contains(t, rec.Body.String(), terminal)
					} else {
						require.True(t, gjson.ValidBytes(rec.Body.Bytes()))
						textPath := map[string]string{"messages": "content.0.text", "chat": "choices.0.message.content", "responses": "output.0.content.0.text"}[protocol]
						require.Equal(t, "ok", gjson.GetBytes(rec.Body.Bytes(), textPath).String())
					}
				})
			}
		}
	}
}

func BenchmarkClaudeAccountProxyGuard(b *testing.B) {
	id := int64(42)
	account := &Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth, ProxyID: &id, Proxy: &Proxy{ID: id, Protocol: "http", Host: "proxy.invalid", Port: 8080}}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := validatedAccountProxyURL(account); err != nil {
			b.Fatal(err)
		}
	}
}

func TestClaudeProxyGuardNativePassthrough(t *testing.T) {
	for _, hydrated := range []bool{false, true} {
		t.Run(fmt.Sprintf("hydrated=%t", hydrated), func(t *testing.T) {
			id := int64(42)
			account := newAnthropicOAuthNativeAccountForTest()
			account.ProxyID = &id
			if hydrated {
				account.Proxy = &Proxy{ID: id, Protocol: "http", Host: "proxy.invalid", Port: 8080}
			}
			cfg := &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}
			up := &proxyGuardSuccessUpstream{}
			svc := &GatewayService{cfg: cfg, httpUpstream: up, responseHeaderFilter: compileResponseHeaderFilter(cfg)}
			for _, countTokens := range []bool{false, true} {
				if countTokens && hydrated {
					continue
				}
				c, rec := newNativePassthroughTestContext(t, "/v1/messages")
				parsed, err := ParseGatewayRequest(NewRequestBodyRef([]byte(`{"model":"claude-haiku-4-5","stream":true,"max_tokens":32,"messages":[{"role":"user","content":"hello"}]}`)), PlatformAnthropic)
				require.NoError(t, err)
				ctx := SetClaudeCodeClient(context.Background(), true)
				if countTokens {
					err = svc.ForwardCountTokens(ctx, c, account, parsed)
				} else {
					_, err = svc.Forward(ctx, c, account, parsed)
				}
				if !hydrated {
					require.ErrorIs(t, err, ErrAccountProxyUnavailable)
					require.Empty(t, up.proxy)
					continue
				}
				require.NoError(t, err)
				require.Equal(t, account.Proxy.URL(), up.proxy)
				require.Contains(t, rec.Body.String(), "ok")
				require.Contains(t, rec.Body.String(), "event: message_stop")
			}
		})
	}
}
