package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type rankingsRepository struct{ sql sqlExecutor }

func NewRankingsRepository(db *sql.DB) service.RankingsRepository {
	return &rankingsRepository{sql: db}
}

// RankingsAggregateSQL reads only two bounded, indexed time ranges. It does
// not join users, accounts or keys: deleting them must not remove anonymous
// historical usage. Group visibility is checked at query time rather than
// inferred from an account's serving platform.
const RankingsAggregateSQL = `
SELECT
    CASE WHEN $6 = 'hour'
        THEN DATE_BIN('1 hour'::interval, ul.created_at, $1::timestamptz)
        ELSE DATE_TRUNC($6, ul.created_at AT TIME ZONE $5) AT TIME ZONE $5
    END AS bucket,
    COALESCE(NULLIF(BTRIM(ul.requested_model), ''), ul.model) AS model_name,
    COALESCE(NULLIF(BTRIM(ul.upstream_model), ''), ul.model) AS creator_model,
    ul.created_at < $1 AS previous,
    SUM(GREATEST(ul.input_tokens, 0))::bigint AS input_tokens,
    SUM(GREATEST(ul.output_tokens, 0))::bigint AS output_tokens,
    SUM(GREATEST(ul.cache_read_tokens, 0))::bigint AS cache_read_tokens,
    SUM(GREATEST(ul.cache_creation_tokens, 0))::bigint AS cache_creation_tokens,
    COUNT(*)::bigint AS requests
FROM usage_logs ul
JOIN groups g ON g.id = ul.group_id
    AND g.is_exclusive = FALSE
    AND g.deleted_at IS NULL
    AND g.status = 'active'
WHERE ul.created_at >= $3 AND ul.created_at < $2
    AND ((ul.created_at >= $1 AND ul.created_at < $2)
        OR (ul.created_at >= $3 AND ul.created_at < $4))
GROUP BY 1, 2, 3, 4
ORDER BY 1, 2, 3, 4`

func (r *rankingsRepository) Aggregate(ctx context.Context, bounds service.RankingsRange) ([]service.RankingsBucket, error) {
	if r == nil || r.sql == nil {
		return nil, fmt.Errorf("rankings repository is unavailable")
	}
	rows, err := r.sql.QueryContext(ctx, RankingsAggregateSQL, bounds.Start, bounds.End,
		bounds.ComparisonStart, bounds.ComparisonEnd, bounds.Timezone, bounds.Granularity)
	if err != nil {
		return nil, fmt.Errorf("aggregate public rankings: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := []service.RankingsBucket{}
	for rows.Next() {
		var row service.RankingsBucket
		if err := rows.Scan(&row.Bucket, &row.Model, &row.CreatorModel, &row.Previous,
			&row.InputTokens, &row.OutputTokens, &row.CacheReadTokens, &row.CacheCreationTokens, &row.Requests); err != nil {
			return nil, fmt.Errorf("scan public rankings: %w", err)
		}
		row.Bucket = row.Bucket.In(bounds.Start.Location())
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read public rankings: %w", err)
	}
	return result, nil
}
