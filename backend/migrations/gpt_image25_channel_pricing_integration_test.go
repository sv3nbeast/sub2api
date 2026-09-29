//go:build integration

package migrations

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestGPTImage25ChannelPricingScopeAndIdempotency(t *testing.T) {
	ctx := context.Background()
	image := os.Getenv("SUB2API_TEST_POSTGRES_IMAGE")
	if image == "" {
		image = "postgres:18-alpine"
	}
	container, err := tcpostgres.Run(ctx, image,
		tcpostgres.WithDatabase("gpt_image25_test"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(ctx)) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	_, err = db.Exec(`
CREATE TABLE accounts (id BIGINT PRIMARY KEY, platform TEXT, type TEXT, parent_account_id BIGINT, credentials JSONB, deleted_at TIMESTAMPTZ);
CREATE TABLE groups (id BIGINT PRIMARY KEY, platform TEXT, model_allowlist JSONB, allow_image_generation BOOLEAN, deleted_at TIMESTAMPTZ, updated_at TIMESTAMPTZ DEFAULT NOW());
CREATE TABLE account_groups (account_id BIGINT, group_id BIGINT);
CREATE TABLE channels (id BIGINT PRIMARY KEY, status TEXT, display_only BOOLEAN);
CREATE TABLE channel_groups (channel_id BIGINT, group_id BIGINT);
CREATE TABLE channel_model_pricing (id BIGSERIAL PRIMARY KEY, channel_id BIGINT, platform TEXT, models JSONB, billing_mode TEXT, input_price NUMERIC, output_price NUMERIC, cache_write_price NUMERIC, cache_read_price NUMERIC, image_input_price NUMERIC, image_output_price NUMERIC, enabled BOOLEAN DEFAULT true, updated_at TIMESTAMPTZ DEFAULT NOW());
CREATE TABLE channel_pricing_intervals (id BIGSERIAL PRIMARY KEY, pricing_id BIGINT, min_tokens INT, input_price NUMERIC);
CREATE TABLE channel_account_stats_model_pricing (id BIGSERIAL PRIMARY KEY, rule_id BIGINT, platform TEXT, models JSONB, input_price NUMERIC, enabled BOOLEAN DEFAULT true);
CREATE TABLE channel_account_stats_pricing_intervals (id BIGSERIAL PRIMARY KEY, pricing_id BIGINT, min_tokens INT, input_price NUMERIC);

INSERT INTO accounts VALUES
 (1,'openai','oauth',NULL,'{"model_mapping":{"gpt-image-2":"gpt-image-2","gpt-image-2.5-flare":"gpt-image-2.5-flare","gpt-image-2.5-sunburst":"gpt-image-2.5-sunburst"}}',NULL),
 (2,'openai','oauth',NULL,'{"model_mapping":{"gpt-image-2":"gpt-image-2"}}',NULL),
 (3,'kiro','oauth',NULL,'{"model_mapping":{"gpt-image-2.5-flare":"gpt-image-2.5-flare","gpt-image-2.5-sunburst":"gpt-image-2.5-sunburst"}}',NULL),
 (4,'openai','oauth',1,'{"model_mapping":{"gpt-image-2.5-flare":"gpt-image-2.5-flare","gpt-image-2.5-sunburst":"gpt-image-2.5-sunburst"}}',NULL);
INSERT INTO groups VALUES
 (1,'openai','{"enabled":false,"models":["gpt-5.5","gpt-image-2","gpt-image-2"]}',true,NULL,NOW()),
 (2,'openai','{"enabled":true,"models":["gpt-image-2"]}',true,NULL,NOW()),
 (3,'openai','{"enabled":true,"models":["gpt-image-2"]}',false,NULL,NOW()),
 (4,'openai','{"enabled":true,"models":["gpt-image-2"]}',true,NOW(),NOW());
INSERT INTO account_groups VALUES (1,1),(1,3),(1,4),(2,2),(3,2),(4,2);
INSERT INTO channels VALUES (10,'active',false),(11,'active',false),(12,'active',false),(13,'active',false),(14,'active',false);
INSERT INTO channel_groups VALUES (10,1),(11,1),(12,2),(13,3),(14,4);
INSERT INTO channel_model_pricing (id,channel_id,platform,models,billing_mode,input_price,output_price,cache_read_price,image_output_price) VALUES
 (100,10,'openai','["gpt-image-2"]','token',0.0000025,0,0.000000625,0.000015),
 (101,10,'openai','["gpt-image-2","gpt-image-2.5-flare","gpt-image-2.5-sunburst","gpt-image-2.5-flare"]','token',0.000321,0,0.000009,0.000654),
 (110,11,'openai','["gpt-image-2"]','token',0.0000025,0,0.000000625,0.000015),
 (111,11,'openai','["gpt-image-2.5-flare"]','token',0.000123,0,0.000007,0.000456),
 (112,11,'openai','["gpt-image-2.5-flare"]','token',0.000222,0,0.000008,0.000333),
 (120,12,'openai','["gpt-image-2"]','token',0.0000025,0,0.000000625,0.000015),
 (130,13,'openai','["gpt-image-2"]','token',0.0000025,0,0.000000625,0.000015),
 (140,14,'openai','["gpt-image-2"]','token',0.0000025,0,0.000000625,0.000015);
INSERT INTO channel_pricing_intervals (pricing_id,min_tokens,input_price) VALUES (111,0,0.000789);
INSERT INTO channel_account_stats_model_pricing (id,rule_id,platform,models,input_price) VALUES (200,20,'openai','["gpt-image-2","gpt-image-2.5-flare"]',0.000077);
INSERT INTO channel_account_stats_pricing_intervals (pricing_id,min_tokens,input_price) VALUES (200,0,0.000066);
`)
	require.NoError(t, err)

	migration, err := FS.ReadFile("249_add_gpt_image_25_channel_pricing.sql")
	require.NoError(t, err)
	run := func() {
		tx, e := db.BeginTx(ctx, nil)
		require.NoError(t, e)
		_, e = tx.Exec(string(migration))
		if e != nil {
			_ = tx.Rollback()
		}
		require.NoError(t, e)
		require.NoError(t, tx.Commit())
	}
	run()

	var models string
	var enabled bool
	require.NoError(t, db.QueryRow(`SELECT model_allowlist->'models', (model_allowlist->>'enabled')::boolean FROM groups WHERE id=1`).Scan(&models, &enabled))
	require.JSONEq(t, `["gpt-5.5","gpt-image-2","gpt-image-2.5-flare","gpt-image-2.5-sunburst"]`, models)
	require.False(t, enabled)
	for _, groupID := range []int{2, 3, 4} {
		require.NoError(t, db.QueryRow(`SELECT model_allowlist->'models' FROM groups WHERE id=$1`, groupID).Scan(&models))
		require.JSONEq(t, `["gpt-image-2"]`, models)
	}

	for _, model := range []string{"gpt-image-2.5-flare", "gpt-image-2.5-sunburst"} {
		for _, channelID := range []int{10, 11} {
			var count int
			require.NoError(t, db.QueryRow(`SELECT count(*) FROM channel_model_pricing WHERE channel_id=$1 AND enabled AND models ? $2`, channelID, model).Scan(&count))
			require.Equal(t, 1, count, "channel %d model %s", channelID, model)
		}
		var input, output, cached, imageInput, imageOutput float64
		require.NoError(t, db.QueryRow(`SELECT models,input_price,output_price,cache_read_price,image_input_price,image_output_price FROM channel_model_pricing WHERE channel_id=10 AND enabled AND models ? $1`, model).Scan(&models, &input, &output, &cached, &imageInput, &imageOutput))
		require.JSONEq(t, `["`+model+`"]`, models)
		require.InDelta(t, 5e-6, input, 1e-14)
		require.Zero(t, output)
		require.InDelta(t, 1.25e-6, cached, 1e-14)
		require.InDelta(t, 8e-6, imageInput, 1e-14)
		require.InDelta(t, 30e-6, imageOutput, 1e-14)
	}
	// The custom dedicated row keeps both its price and its interval. The
	// duplicate is retained for audit but disabled; mixed predecessor stays.
	var customPrice float64
	require.NoError(t, db.QueryRow(`SELECT input_price FROM channel_model_pricing WHERE id=111`).Scan(&customPrice))
	require.InDelta(t, 0.000123, customPrice, 1e-14)
	require.NoError(t, db.QueryRow(`SELECT input_price FROM channel_pricing_intervals WHERE pricing_id=111`).Scan(&customPrice))
	require.InDelta(t, 0.000789, customPrice, 1e-14)
	require.NoError(t, db.QueryRow(`SELECT models,enabled FROM channel_model_pricing WHERE id=112`).Scan(&models, &enabled))
	require.JSONEq(t, `[]`, models)
	require.False(t, enabled)
	require.NoError(t, db.QueryRow(`SELECT models FROM channel_model_pricing WHERE id=101`).Scan(&models))
	require.JSONEq(t, `["gpt-image-2"]`, models)
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM channel_model_pricing WHERE channel_id IN (12,13,14) AND models ?| ARRAY['gpt-image-2.5-flare','gpt-image-2.5-sunburst']`).Scan(&count))
	require.Zero(t, count)

	snapshot := func() string {
		var value string
		require.NoError(t, db.QueryRow(`SELECT jsonb_build_object(
  'groups', (SELECT jsonb_agg(to_jsonb(g) ORDER BY id) FROM groups g),
  'prices', (SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM channel_model_pricing p),
  'intervals', (SELECT jsonb_agg(to_jsonb(i) ORDER BY id) FROM channel_pricing_intervals i),
  'stats', (SELECT jsonb_agg(to_jsonb(s) ORDER BY id) FROM channel_account_stats_model_pricing s),
  'stats_intervals', (SELECT jsonb_agg(to_jsonb(i) ORDER BY id) FROM channel_account_stats_pricing_intervals i)
)::text`).Scan(&value))
		return value
	}
	first := snapshot()
	run()
	require.JSONEq(t, first, snapshot(), "second run must not change pricing, allowlists, or independent account-statistics rules")
}
