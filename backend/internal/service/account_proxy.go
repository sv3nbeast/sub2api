package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/pkg/proxyurl"
)

var ErrAccountProxyUnavailable = errors.New("configured account proxy is unavailable")

// validatedAccountProxyURL never interprets an incomplete Claude OAuth proxy
// relationship as an intentional direct connection. It performs no I/O on the
// forwarding path. Other providers retain their existing proxy semantics.
func validatedAccountProxyURL(account *Account) (string, error) {
	if account == nil {
		return "", ErrAccountProxyUnavailable
	}
	if account.ProxyID == nil {
		return "", nil
	}
	if account.IsAnthropicOAuthOrSetupToken() {
		return checkedAccountProxyURL(*account.ProxyID, account.Proxy)
	}
	if account.Proxy != nil {
		return account.Proxy.URL(), nil
	}
	return "", nil
}

func checkedAccountProxyURL(id int64, proxy *Proxy) (string, error) {
	if id <= 0 || proxy == nil || proxy.ID != id {
		return "", fmt.Errorf("%w: proxy_id=%d", ErrAccountProxyUnavailable, id)
	}
	normalized, parsed, err := proxyurl.Parse(proxy.URL())
	if err != nil || parsed == nil {
		// Do not expose proxy credentials in client-visible errors.
		return "", fmt.Errorf("%w: invalid proxy_id=%d", ErrAccountProxyUnavailable, id)
	}
	return normalized, nil
}

func (s *OAuthService) resolveProxyURL(ctx context.Context, proxyID *int64) (string, error) {
	if proxyID == nil {
		return "", nil
	}
	if s.proxyRepo == nil {
		return "", ErrAccountProxyUnavailable
	}
	proxy, err := s.proxyRepo.GetByID(ctx, *proxyID)
	if err != nil {
		return "", fmt.Errorf("%w: proxy_id=%d lookup failed", ErrAccountProxyUnavailable, *proxyID)
	}
	return checkedAccountProxyURL(*proxyID, proxy)
}
