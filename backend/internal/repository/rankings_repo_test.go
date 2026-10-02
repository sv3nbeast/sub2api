package repository

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestRankingsRepository_PublicIndexedBoundsCanonicalTokensAndRequestedModel(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, loc)
	bounds, err := service.RankingsPeriodRange("week", now, loc)
	require.NoError(t, err)
	cols := []string{"bucket", "model_name", "creator_model", "previous", "input", "output", "cache_read", "cache_creation", "requests"}
	mock.ExpectQuery(regexp.QuoteMeta(RankingsAggregateSQL)).WithArgs(bounds.Start, bounds.End, bounds.ComparisonStart, bounds.ComparisonEnd, "Asia/Shanghai", "day").
		WillReturnRows(sqlmock.NewRows(cols).AddRow(time.Date(2026, 10, 2, 0, 0, 0, 0, loc).UTC(), "display-alias", "claude-opus-5", false, int64(100), int64(20), int64(200), int64(80), int64(3)))
	repo := NewRankingsRepository(db)
	rows, err := repo.Aggregate(context.Background(), bounds)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "display-alias", rows[0].Model)
	require.Equal(t, "claude-opus-5", rows[0].CreatorModel)
	require.Equal(t, loc, rows[0].Bucket.Location())
	require.Equal(t, 0, rows[0].Bucket.Hour())
	require.Equal(t, int64(200), rows[0].CacheReadTokens)
	require.Equal(t, int64(80), rows[0].CacheCreationTokens)
	require.NoError(t, mock.ExpectationsWereMet())
	query := strings.Join(strings.Fields(RankingsAggregateSQL), " ")
	require.Contains(t, query, "g.is_exclusive = FALSE")
	require.Contains(t, query, "g.deleted_at IS NULL")
	require.Contains(t, query, "g.status = 'active'")
	require.Contains(t, query, "ul.created_at >= $3 AND ul.created_at < $2")
	require.Contains(t, query, "ul.created_at >= $3 AND ul.created_at < $4")
	require.Contains(t, query, "COALESCE(NULLIF(BTRIM(ul.requested_model), ''), ul.model)")
	require.Contains(t, query, "CASE WHEN $6 = 'hour' THEN DATE_BIN('1 hour'::interval, ul.created_at, $1::timestamptz)")
	for _, field := range []string{"input_tokens", "output_tokens", "cache_read_tokens", "cache_creation_tokens"} {
		require.Equal(t, 1, strings.Count(query, "ul."+field), "each canonical token bucket counted once")
	}
	for _, private := range []string{"JOIN accounts", "JOIN users", "JOIN api_keys", "user_id", "account_id", "api_key_id", "cache_creation_5m_tokens", "cache_creation_1h_tokens"} {
		require.NotContains(t, query, private)
	}
}

func TestRankingsRepository_DatabaseFailureAndBadScan(t *testing.T) {
	for _, badScan := range []bool{false, true} {
		t.Run(map[bool]string{false: "query", true: "scan"}[badScan], func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			bounds, _ := service.RankingsPeriodRange("today", time.Now(), time.UTC)
			expect := mock.ExpectQuery(regexp.QuoteMeta(RankingsAggregateSQL))
			if badScan {
				expect.WillReturnRows(sqlmock.NewRows([]string{"bad"}).AddRow("broken"))
			} else {
				expect.WillReturnError(errors.New("offline"))
			}
			_, err = NewRankingsRepository(db).Aggregate(context.Background(), bounds)
			require.Error(t, err)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
