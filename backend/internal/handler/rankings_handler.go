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
	snapshot, err := h.service.Get(c.Request.Context(), c.DefaultQuery("period", "week"))
	if errors.Is(err, service.ErrInvalidRankingsPeriod) {
		response.BadRequest(c, service.ErrInvalidRankingsPeriod.Error())
		return
	}
	if err != nil {
		slog.Warn("rankings.aggregate_unavailable", "error", err)
		c.Header("Retry-After", "5")
		response.Error(c, http.StatusServiceUnavailable, "Rankings are temporarily unavailable. Please try again shortly.")
		return
	}
	// Application snapshots have their own period-dependent TTL. Avoid caching
	// yesterday's page across the configured timezone's midnight boundary.
	c.Header("Cache-Control", "no-cache")
	response.Success(c, snapshot)
}
