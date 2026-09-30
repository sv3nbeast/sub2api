//go:build integration

package migrations

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/stretchr/testify/require"
)

func TestGPT61SolMigrationScopesAndIdempotency(t *testing.T) {
	ctx := context.Background()
	image := os.Getenv("SUB2API_TEST_POSTGRES_IMAGE")
	if image == "" {
		image = "postgres:18-alpine"
	}
	container, err := tcpostgres.Run(ctx, image,
		tcpostgres.WithDatabase("gpt61_sol_test"),
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
 CREATE TABLE accounts (id BIGINT PRIMARY KEY,platform TEXT,type TEXT,parent_account_id BIGINT,credentials JSONB,deleted_at TIMESTAMPTZ,updated_at TIMESTAMPTZ DEFAULT NOW());
 CREATE TABLE groups (id BIGINT PRIMARY KEY,platform TEXT,model_allowlist JSONB,deleted_at TIMESTAMPTZ,updated_at TIMESTAMPTZ DEFAULT NOW());
 CREATE TABLE account_groups (account_id BIGINT,group_id BIGINT);
 CREATE TABLE channel_groups (channel_id BIGINT,group_id BIGINT);
 CREATE TABLE channel_model_pricing (id BIGSERIAL PRIMARY KEY,channel_id BIGINT,platform TEXT,models JSONB,billing_mode TEXT DEFAULT 'token',input_price NUMERIC,output_price NUMERIC,cache_write_price NUMERIC,cache_read_price NUMERIC,fast_multiplier NUMERIC,flex_multiplier NUMERIC,enabled BOOLEAN DEFAULT true,updated_at TIMESTAMPTZ DEFAULT NOW());
 CREATE TABLE channel_pricing_intervals (id BIGSERIAL PRIMARY KEY,pricing_id BIGINT,min_tokens INT,max_tokens INT,tier_label TEXT,input_price NUMERIC,output_price NUMERIC,cache_write_price NUMERIC,cache_read_price NUMERIC,sort_order INT);
 CREATE TABLE channel_account_stats_model_pricing (id BIGSERIAL PRIMARY KEY,rule_id BIGINT,platform TEXT,models JSONB,billing_mode TEXT DEFAULT 'token',input_price NUMERIC,output_price NUMERIC,cache_write_price NUMERIC,cache_read_price NUMERIC,enabled BOOLEAN DEFAULT true,updated_at TIMESTAMPTZ DEFAULT NOW());
 CREATE TABLE channel_account_stats_pricing_intervals (id BIGSERIAL PRIMARY KEY,pricing_id BIGINT,min_tokens INT,max_tokens INT,tier_label TEXT,input_price NUMERIC,output_price NUMERIC,cache_write_price NUMERIC,cache_read_price NUMERIC,sort_order INT);
 INSERT INTO accounts(id,platform,type,parent_account_id,credentials,deleted_at) VALUES
 (1,'openai','oauth',NULL,'{"model_mapping":{"gpt-6-astra":"gpt-6-astra"}}',NULL),
 (2,'kiro','oauth',NULL,'{"model_mapping":{"gpt-6-astra":"gpt-6-astra"}}',NULL),
 (3,'openai','apikey',NULL,'{"model_mapping":{"gpt-6-astra":"gpt-6-astra"}}',NULL),
 (4,'openai','oauth',NULL,'{"model_mapping":{"gpt-6-astra":"gpt-6-astra","*":"*"}}',NULL);
 INSERT INTO groups(id,platform,model_allowlist) VALUES
 (10,'openai','{"enabled":true,"models":["gpt-6-astra"]}'),
 (11,'openai','{"enabled":true,"models":["gpt-6-astra"]}');
 INSERT INTO account_groups VALUES (1,10),(2,11);
 INSERT INTO channel_groups VALUES (100,10),(101,11);
 INSERT INTO channel_model_pricing(id,channel_id,platform,models,input_price,output_price) VALUES
 (1000,100,'openai','["gpt-6-astra","gpt-5.5"]',0.00001,0.00005),
 (1001,101,'openai','["gpt-6-astra"]',0.00001,0.00005);
 `)
	require.NoError(t, err)

	migration, err := FS.ReadFile("250_add_gpt61_sol_support.sql")
	require.NoError(t, err)
	run := func() {
		tx, e := db.BeginTx(ctx, nil)
		require.NoError(t, e)
		if _, e = tx.Exec(string(migration)); e != nil {
			_ = tx.Rollback()
		}
		require.NoError(t, e)
		require.NoError(t, tx.Commit())
	}
	run()

	var value string
	require.NoError(t, db.QueryRow(`SELECT credentials->'model_mapping'->>'gpt-6.1-sol' FROM accounts WHERE id=1`).Scan(&value))
	require.Equal(t, "gpt-6.1-sol", value)
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM accounts WHERE id IN (2,3,4) AND credentials->'model_mapping' ? 'gpt-6.1-sol'`).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, db.QueryRow(`SELECT model_allowlist->'models' FROM groups WHERE id=10`).Scan(&value))
	require.JSONEq(t, `["gpt-6-astra","gpt-6.1-sol"]`, value)
	require.NoError(t, db.QueryRow(`SELECT model_allowlist->'models' FROM groups WHERE id=11`).Scan(&value))
	require.JSONEq(t, `["gpt-6-astra"]`, value)
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM channel_model_pricing WHERE channel_id=100 AND enabled AND models ? 'gpt-6.1-sol'`).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM channel_model_pricing WHERE channel_id=101 AND models ? 'gpt-6.1-sol'`).Scan(&count))
	require.Zero(t, count)
	var input, cacheRead float64
	require.NoError(t, db.QueryRow(`SELECT input_price,cache_read_price FROM channel_model_pricing WHERE channel_id=100 AND enabled AND models ? 'gpt-6.1-sol'`).Scan(&input, &cacheRead))
	require.InDelta(t, 2e-6, input, 1e-14)
	require.InDelta(t, 0.1e-6, cacheRead, 1e-14)

	var before string
	require.NoError(t, db.QueryRow(`SELECT jsonb_build_array((SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM accounts a),(SELECT jsonb_agg(to_jsonb(g) ORDER BY id) FROM groups g),(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM channel_model_pricing p),(SELECT jsonb_agg(to_jsonb(i) ORDER BY id) FROM channel_pricing_intervals i))`).Scan(&before))
	run()
	var after string
	require.NoError(t, db.QueryRow(`SELECT jsonb_build_array((SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM accounts a),(SELECT jsonb_agg(to_jsonb(g) ORDER BY id) FROM groups g),(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM channel_model_pricing p),(SELECT jsonb_agg(to_jsonb(i) ORDER BY id) FROM channel_pricing_intervals i))`).Scan(&after))
	require.JSONEq(t, before, after)
}
