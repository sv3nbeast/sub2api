package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type rankingsHandlerRepo struct {
	calls atomic.Int32
	err   error
}

func (r *rankingsHandlerRepo) Aggregate(ctx context.Context, b service.RankingsRange) ([]service.RankingsBucket, error) {
	r.calls.Add(1)
	return []service.RankingsBucket{{Bucket: b.Start, Model: "claude-opus-5", InputTokens: 30, OutputTokens: 10, CacheReadTokens: 20, Requests: 2}}, r.err
}

func TestRankingsHandler_PublicEnvelopeDefaultPeriodAndValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &rankingsHandlerRepo{}
	h := NewRankingsHandler(service.NewRankingsService(repo))
	r := gin.New()
	r.GET("/api/v1/rankings", h.Get)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/rankings", nil))
	require.Equal(t, http.StatusOK, w.Code)
	var envelope struct {
		Code int                      `json:"code"`
		Data service.RankingsSnapshot `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Zero(t, envelope.Code)
	require.Equal(t, "week", envelope.Data.Period)
	require.Equal(t, int64(60), envelope.Data.TotalTokens)
	require.Equal(t, "anthropic", envelope.Data.Models[0].VendorID)
	require.NotContains(t, w.Body.String(), "user_id")
	require.NotContains(t, w.Body.String(), "account_id")
	for _, period := range []string{"year%27", "TODAY", "week%27"} {
		w = httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/rankings?period="+period, nil))
		require.Equal(t, http.StatusBadRequest, w.Code)
	}
	require.Equal(t, int32(1), repo.calls.Load(), "invalid periods must never query database")
}

func TestRankingsHandler_FailureIsSanitizedAndRetryable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewRankingsHandler(service.NewRankingsService(&rankingsHandlerRepo{err: errors.New("secret db host/key")}))
	r.GET("/api/v1/rankings", h.Get)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/rankings?period=quarter", nil))
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.Equal(t, "5", w.Header().Get("Retry-After"))
	require.NotContains(t, w.Body.String(), "secret")
}
