package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type RankingsHandler struct{ service *service.RankingsService }

func NewRankingsHandler(rankings *service.RankingsService) *RankingsHandler {
	return &RankingsHandler{service: rankings}
}

// Get publishes aggregate usage only. Database/network details stay in logs.
func (h *RankingsHandler) Get(c *gin.Context) {
	period := c.DefaultQuery("period", "week")
	snapshot, err := h.service.Get(c.Request.Context(), period)
	if errors.Is(err, service.ErrInvalidRankingsPeriod) {
		response.BadRequest(c, service.ErrInvalidRankingsPeriod.Error())
		return
	}
	if errors.Is(err, service.ErrRankingsYearUnavailable) {
		response.Error(c, http.StatusNotFound, "Year rankings are not available yet.")
		return
	}
	if err != nil {
		slog.Warn("rankings.aggregate_unavailable", "error", err)
		c.Header("Retry-After", "5")
		response.Error(c, http.StatusServiceUnavailable, "Rankings are temporarily unavailable. Please try again shortly.")
		return
	}
	// The response is aggregate public data, so browsers and a front proxy can
	// reuse it while the service-level snapshot cache avoids repeated database
	// scans. Keep the edge TTL aligned with the server cache for each period.
	c.Header("Cache-Control", rankingsCacheControl(period))
	response.Success(c, snapshot)
}

func rankingsCacheControl(period string) string {
	switch period {
	case "year":
		return "public, max-age=900, stale-while-revalidate=1800"
	case "month":
		return "public, max-age=300, stale-while-revalidate=600"
	default:
		return "public, max-age=60, stale-while-revalidate=120"
	}
}
