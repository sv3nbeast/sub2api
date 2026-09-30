//go:build integration

package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestGPT61SolMigrationScopesAndIdempotency(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("SUB2API_GPT61_TEST_POSTGRES_DSN")
	// Optional disposable database over an SSH tunnel when local Docker is unavailable.
	if dsn == "" {
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
		dsn, err = container.ConnectionString(ctx, "sslmode=disable")
		require.NoError(t, err)
	}
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
 INSERT INTO accounts(id,platform,type,parent_account_id,credentials,deleted_at) VALUES
 (5,'openai','oauth',NULL,'{"model_mapping":{"gpt-6-astra":"gpt-6-astra","gpt-6.1-sol":"custom"}}',NULL),
 (6,'openai','oauth',1,'{"model_mapping":{"gpt-6-astra":"gpt-6-astra"}}',NULL),
 (7,'openai','oauth',NULL,'{"model_mapping":{"gpt-6-astra":"gpt-6-astra"}}',NOW()),
 (8,'openai','oauth',NULL,'{"model_mapping":{"gpt-6-astra":"custom"}}',NULL),
 (9,'openai','oauth',NULL,'{}',NULL);
 INSERT INTO groups(id,platform,model_allowlist) VALUES
 (10,'openai','{"enabled":true,"models":["gpt-6-astra"]}'),
 (11,'openai','{"enabled":true,"models":["gpt-6-astra"]}');
 INSERT INTO account_groups VALUES (1,10),(2,11);
 INSERT INTO channel_groups VALUES (100,10),(101,11);
 INSERT INTO channel_model_pricing(id,channel_id,platform,models,input_price,output_price) VALUES
 (1000,100,'openai','["gpt-6-astra","gpt-5.5"]',0.00001,0.00005),
 (1001,101,'openai','["gpt-6-astra"]',0.00001,0.00005);
 INSERT INTO groups(id,platform,model_allowlist) VALUES
 (12,'openai','{"enabled":true,"models":["gpt-6-astra"]}'),
 (13,'openai','{"enabled":false,"models":["gpt-6-astra","gpt-6-astra"]}');
 INSERT INTO account_groups VALUES (5,12),(8,12),(9,13);
 `)
	require.NoError(t, err)
	// Both independent billing surfaces: dedicated overrides, duplicate rows,
	// mixed-row splitting and custom interval preservation.
	for _, tc := range []struct{ table, scope, intervals string }{
		{"channel_model_pricing", "channel_id", "channel_pricing_intervals"},
		{"channel_account_stats_model_pricing", "rule_id", "channel_account_stats_pricing_intervals"},
	} {
		_, err = db.Exec(fmt.Sprintf(`
 ALTER TABLE %s ADD COLUMN cache_write_1h_price NUMERIC;
 ALTER TABLE %s ADD COLUMN cache_write_1h_price NUMERIC;
 INSERT INTO %s(id,%s,platform,models,input_price,output_price,cache_write_1h_price) VALUES
 (2000,200,'openai','["gpt-6.1-sol","gpt-6.1-sol"]',0.000123,0.000456,0.000789),
 (2001,200,'openai','["gpt-6.1-sol"]',0.000999,0.000999,0.000999),
 (2002,200,'openai','["gpt-6-astra","gpt-6.1-sol"]',0.000555,0.000555,0.000555),
 (2010,201,'openai','["gpt-6-astra","gpt-6.1-sol"]',0.000234,0.000567,0.000890),
 (2020,202,'openai','["gpt-6-astra"]',0.000111,0.000222,0.000333);
 INSERT INTO %s(pricing_id,min_tokens,max_tokens,tier_label,input_price,cache_write_1h_price,sort_order) VALUES
 (2000,0,NULL,'Custom',0.000321,0.000654,0),
 (2010,0,NULL,'Mixed custom',0.000432,0.000765,0);
 `, tc.table, tc.intervals, tc.table, tc.scope, tc.intervals))
		require.NoError(t, err)
	}

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
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM accounts WHERE id IN (2,3,4,6,7,8,9) AND credentials->'model_mapping' ? 'gpt-6.1-sol'`).Scan(&count))
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
	require.NoError(t, db.QueryRow(`SELECT credentials->'model_mapping'->>'gpt-6.1-sol' FROM accounts WHERE id=5`).Scan(&value))
	require.Equal(t, "custom", value)
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM groups WHERE id=12 AND model_allowlist->'models' ? 'gpt-6.1-sol'`).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, db.QueryRow(`SELECT model_allowlist FROM groups WHERE id=13`).Scan(&value))
	require.JSONEq(t, `{"enabled":false,"models":["gpt-6-astra","gpt-6.1-sol"]}`, value)
	for _, tc := range []struct{ table, scope, intervals string }{
		{"channel_model_pricing", "channel_id", "channel_pricing_intervals"},
		{"channel_account_stats_model_pricing", "rule_id", "channel_account_stats_pricing_intervals"},
	} {
		for _, scope := range []int{200, 201} {
			require.NoError(t, db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s=$1 AND enabled AND models='["gpt-6.1-sol"]'`, tc.table, tc.scope), scope).Scan(&count))
			require.Equal(t, 1, count)
		}
		var enabled bool
		require.NoError(t, db.QueryRow(`SELECT models,enabled FROM `+tc.table+` WHERE id=2001`).Scan(&value, &enabled))
		require.JSONEq(t, `[]`, value)
		require.False(t, enabled)
		for _, id := range []int{2002, 2010, 2020} {
			require.NoError(t, db.QueryRow(`SELECT models FROM `+tc.table+` WHERE id=$1`, id).Scan(&value))
			require.JSONEq(t, `["gpt-6-astra"]`, value)
		}
		for _, scope := range []int{200, 201} {
			var rowPrice, tierPrice, write1h float64
			require.NoError(t, db.QueryRow(fmt.Sprintf(`SELECT p.input_price,i.input_price,i.cache_write_1h_price FROM %s p JOIN %s i ON i.pricing_id=p.id WHERE p.%s=$1 AND p.enabled AND p.models='["gpt-6.1-sol"]'`, tc.table, tc.intervals, tc.scope), scope).Scan(&rowPrice, &tierPrice, &write1h))
			if scope == 200 {
				require.InDelta(t, 0.000123, rowPrice, 1e-14)
				require.InDelta(t, 0.000321, tierPrice, 1e-14)
				require.InDelta(t, 0.000654, write1h, 1e-14)
			} else {
				require.InDelta(t, 0.000234, rowPrice, 1e-14)
				require.InDelta(t, 0.000432, tierPrice, 1e-14)
				require.InDelta(t, 0.000765, write1h, 1e-14)
			}
		}
	}

	var before string
	require.NoError(t, db.QueryRow(`SELECT jsonb_build_array((SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM accounts a),(SELECT jsonb_agg(to_jsonb(g) ORDER BY id) FROM groups g),(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM channel_model_pricing p),(SELECT jsonb_agg(to_jsonb(i) ORDER BY id) FROM channel_pricing_intervals i),(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM channel_account_stats_model_pricing p),(SELECT jsonb_agg(to_jsonb(i) ORDER BY id) FROM channel_account_stats_pricing_intervals i))`).Scan(&before))
	run()
	var after string
	require.NoError(t, db.QueryRow(`SELECT jsonb_build_array((SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM accounts a),(SELECT jsonb_agg(to_jsonb(g) ORDER BY id) FROM groups g),(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM channel_model_pricing p),(SELECT jsonb_agg(to_jsonb(i) ORDER BY id) FROM channel_pricing_intervals i),(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM channel_account_stats_model_pricing p),(SELECT jsonb_agg(to_jsonb(i) ORDER BY id) FROM channel_account_stats_pricing_intervals i))`).Scan(&after))
	require.JSONEq(t, before, after)
}
