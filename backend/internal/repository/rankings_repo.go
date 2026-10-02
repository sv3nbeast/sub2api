package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type rankingsRepository struct{ sql sqlExecutor }

func NewRankingsRepository(db *sql.DB) service.RankingsRepository {
	return &rankingsRepository{sql: db}
}

func (r *rankingsRepository) RankingsArchiveCoverage(ctx context.Context) (time.Time, time.Time, error) {
	var startedAt, lastCompleted time.Time
	err := scanSingleRow(ctx, r.sql, `
		SELECT recording_started_at, last_completed_date
		FROM public_rankings_rollup_state
		WHERE id = 1
	`, nil, &startedAt, &lastCompleted)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("read rankings archive coverage: %w", err)
	}
	return startedAt, lastCompleted, nil
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

const RankingsArchiveAggregateSQL = `
WITH source AS (
    SELECT
        ar.bucket_date::timestamp AT TIME ZONE $5 AS event_at,
        ar.requested_model AS model_name,
        ar.creator_model,
        ar.bucket_date < $1::date AS previous,
        ar.input_tokens,
        ar.output_tokens,
        ar.cache_read_tokens,
        ar.cache_creation_tokens,
        ar.requests
    FROM public_rankings_daily_rollups ar
    WHERE ar.bucket_date >= LEAST($1::date, $3::date)
      AND ar.bucket_date < GREATEST($2::date, $4::date)
      AND ar.bucket_date < $7::date
    UNION ALL
    SELECT
        ul.created_at AS event_at,
        COALESCE(NULLIF(BTRIM(ul.requested_model), ''), ul.model) AS model_name,
        COALESCE(NULLIF(BTRIM(ul.upstream_model), ''), ul.model) AS creator_model,
        ul.created_at < $1 AS previous,
        GREATEST(ul.input_tokens, 0),
        GREATEST(ul.output_tokens, 0),
        GREATEST(ul.cache_read_tokens, 0),
        GREATEST(ul.cache_creation_tokens, 0),
        1::bigint
    FROM usage_logs ul
    JOIN groups g ON g.id = ul.group_id
        AND g.is_exclusive = FALSE
        AND g.deleted_at IS NULL
        AND g.status = 'active'
    WHERE ul.created_at >= $7
      AND ((ul.created_at >= $1 AND ul.created_at < $2)
        OR (ul.created_at >= $3 AND ul.created_at < $4))
)
SELECT
    CASE WHEN $6 = 'hour'
        THEN DATE_BIN('1 hour'::interval, source.event_at, $1::timestamptz)
        ELSE DATE_TRUNC($6, source.event_at AT TIME ZONE $5) AT TIME ZONE $5
    END AS bucket,
    source.model_name,
    source.creator_model,
    source.previous,
    SUM(source.input_tokens)::bigint,
    SUM(source.output_tokens)::bigint,
    SUM(source.cache_read_tokens)::bigint,
    SUM(source.cache_creation_tokens)::bigint,
    SUM(source.requests)::bigint
FROM source
GROUP BY 1, 2, 3, 4
ORDER BY 1, 2, 3, 4`

func (r *rankingsRepository) Aggregate(ctx context.Context, bounds service.RankingsRange) ([]service.RankingsBucket, error) {
	if r == nil || r.sql == nil {
		return nil, fmt.Errorf("rankings repository is unavailable")
	}
	query := RankingsAggregateSQL
	args := []any{bounds.Start, bounds.End, bounds.ComparisonStart, bounds.ComparisonEnd, bounds.Timezone, bounds.Granularity}
	if bounds.UseArchive {
		query = RankingsArchiveAggregateSQL
		cutoff := bounds.ArchiveCutoff
		if cutoff.IsZero() {
			cutoff = bounds.Start
		}
		args = append(args, cutoff)
	}
	rows, err := r.sql.QueryContext(ctx, query, args...)
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
