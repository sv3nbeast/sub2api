package repository

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type proxyGuardRedirectTransport struct {
	next   http.RoundTripper
	target string
}

func (r proxyGuardRedirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme == "https" {
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{r.target}}, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
	}
	return r.next.RoundTrip(req)
}

func TestFingerprintProxyGuardRejectsDowngrade(t *testing.T) {
	var targetHits, proxyHits atomic.Int64
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { targetHits.Add(1); w.WriteHeader(200) }))
	defer target.Close()
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { proxyHits.Add(1); w.WriteHeader(502) }))
	defer proxy.Close()
	svc := &httpUpstreamService{cfg: &config.Config{}, clients: make(map[string]*upstreamClientEntry)}
	entry, err := svc.getClientEntryWithTLS(proxy.URL, 42, 1, &tlsfingerprint.Profile{ForceHTTP1WithProxy: true}, service.HTTPUpstreamProfileDefault, false, false)
	require.NoError(t, err)
	entry.client.Transport = proxyGuardRedirectTransport{entry.client.Transport, target.URL}
	req, err := http.NewRequest(http.MethodGet, "https://test.invalid/start", nil)
	require.NoError(t, err)
	_, err = entry.client.Do(req)
	require.ErrorContains(t, err, "HTTPS downgrade")
	require.Zero(t, targetHits.Load())
	require.Zero(t, proxyHits.Load())
}

func TestFingerprintProxyGuardExplicitHTTPUsesProxy(t *testing.T) {
	var targetHits, proxyHits atomic.Int64
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { targetHits.Add(1); w.WriteHeader(200) }))
	defer target.Close()
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { proxyHits.Add(1); w.WriteHeader(502) }))
	defer proxy.Close()
	svc := &httpUpstreamService{cfg: &config.Config{}, clients: make(map[string]*upstreamClientEntry)}
	entry, err := svc.getClientEntryWithTLS(proxy.URL, 42, 1, &tlsfingerprint.Profile{ForceHTTP1WithProxy: true}, service.HTTPUpstreamProfileDefault, false, false)
	require.NoError(t, err)
	defer entry.client.CloseIdleConnections()
	req, err := http.NewRequest(http.MethodGet, target.URL, nil)
	require.NoError(t, err)
	resp, err := entry.client.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusBadGateway, resp.StatusCode)
	require.Zero(t, targetHits.Load(), "proxy failure must not fall back to a direct connection")
	require.EqualValues(t, 1, proxyHits.Load())
}

func TestClaudeOAuthClientExplicitProxyOverridesEnvironment(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://environment.invalid:8080")
	t.Setenv("NO_PROXY", "*")
	unconfigured, err := createReqClient("")
	require.NoError(t, err)
	require.NotNil(t, unconfigured.GetTransport().Proxy, "unconfigured accounts retain environment-proxy support")
	proxied, err := createReqClient("http://configured.invalid:8080")
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", nil)
	require.NoError(t, err)
	proxy, err := proxied.GetTransport().Proxy(req)
	require.NoError(t, err)
	require.Equal(t, "http://configured.invalid:8080", proxy.String(), "NO_PROXY cannot override an account proxy")
}
