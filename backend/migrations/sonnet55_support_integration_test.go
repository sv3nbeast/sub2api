//go:build integration

package migrations

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestSonnet55Migration(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("SUB2API_SONNET55_TEST_DSN")
	if dsn == "" {
		c, err := tcpostgres.Run(ctx, "postgres:18-alpine", tcpostgres.WithDatabase("sonnet55"), tcpostgres.WithUsername("postgres"), tcpostgres.WithPassword("test"), tcpostgres.BasicWaitStrategies())
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, c.Terminate(ctx)) })
		dsn, err = c.ConnectionString(ctx, "sslmode=disable")
		require.NoError(t, err)
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	_, err = db.Exec(`
 CREATE TABLE accounts(id BIGINT PRIMARY KEY,platform TEXT,type TEXT,parent_account_id BIGINT,credentials JSONB,deleted_at TIMESTAMPTZ,updated_at TIMESTAMPTZ DEFAULT NOW());
 CREATE TABLE groups(id BIGINT PRIMARY KEY,name TEXT,platform TEXT,subscription_type TEXT DEFAULT 'standard',model_allowlist JSONB,deleted_at TIMESTAMPTZ,updated_at TIMESTAMPTZ DEFAULT NOW());
 CREATE TABLE channels(id BIGINT PRIMARY KEY,name TEXT);
 CREATE TABLE channel_groups(channel_id BIGINT,group_id BIGINT);
 CREATE TABLE channel_model_pricing(id BIGSERIAL PRIMARY KEY,channel_id BIGINT,platform TEXT,models JSONB,billing_mode TEXT DEFAULT 'token',input_price NUMERIC,output_price NUMERIC,cache_write_price NUMERIC,cache_read_price NUMERIC,cache_write_5m_price NUMERIC,cache_write_1h_price NUMERIC,enabled BOOLEAN DEFAULT true,updated_at TIMESTAMPTZ DEFAULT NOW());
 CREATE TABLE channel_pricing_intervals(id BIGSERIAL PRIMARY KEY,pricing_id BIGINT,min_tokens INT,max_tokens INT,tier_label TEXT,input_price NUMERIC,output_price NUMERIC,cache_write_price NUMERIC,cache_read_price NUMERIC,sort_order INT);
 CREATE TABLE channel_account_stats_model_pricing(LIKE channel_model_pricing INCLUDING DEFAULTS INCLUDING CONSTRAINTS);
 ALTER TABLE channel_account_stats_model_pricing RENAME COLUMN channel_id TO rule_id;
 CREATE TABLE channel_account_stats_pricing_intervals(LIKE channel_pricing_intervals INCLUDING DEFAULTS INCLUDING CONSTRAINTS);
 INSERT INTO accounts(id,platform,type,credentials) VALUES
 (1,'kiro','oauth','{"model_mapping":{"claude-sonnet-5":"claude-sonnet-5","claude-sonnet-5-5-thinking":"custom"}}'),
 (2,'anthropic','apikey','{"model_mapping":{"claude-opus-5-5":"claude-opus-5-5"}}'),
 (3,'kiro','oauth','{"model_mapping":{"claude-sonnet-5":"claude-sonnet-5","*":"*"}}'),
 (4,'kiro','oauth','{"model_mapping":{"claude-sonnet-5":"claude-sonnet-4.6"}}'),
 (5,'anthropic','oauth','{"model_mapping":{"claude-sonnet-5":"claude-sonnet-5"}}'),
 (6,'kiro','apikey','{"model_mapping":{"claude-sonnet-5":"claude-sonnet-5"}}'),
 (7,'kiro','oauth','{"model_mapping":{"claude-sonnet-5":"claude-sonnet-5"}}'),
 (8,'kiro','oauth','{"model_mapping":{"claude-sonnet-5":"claude-sonnet-5"}}');
 UPDATE accounts SET deleted_at=NOW() WHERE id=7;UPDATE accounts SET parent_account_id=1 WHERE id=8;
 INSERT INTO groups(id,name,platform,subscription_type,model_allowlist) VALUES
 (19,'Claude最新模型-官转渠道','anthropic','standard','{"enabled":false,"models":["claude-opus-5-5","claude-opus-5-5"]}'),
 (29,'Claude最新模型-AWS渠道','kiro','standard','{"enabled":true,"models":["claude-sonnet-5"]}'),
 (9,'subapis-Pro','anthropic','subscription','{"enabled":false,"models":["claude-sonnet-5"]}'),
 (2,'Claude历史模型-官转渠道','anthropic','standard','{"enabled":false,"models":["claude-sonnet-5"]}');
 INSERT INTO channels VALUES (8,'claude-最新模型'),(10,'claude-最新模型-group19-safe'),(11,'Kiro Claude-AWS'),(12,'custom'),(13,'custom-mixed'),(14,'claude-订阅模型'),(6,'Claude 历史模型');
 INSERT INTO channel_groups VALUES(10,19),(11,29),(14,9),(6,2);
 INSERT INTO channel_model_pricing(id,channel_id,platform,models,input_price,output_price,cache_write_price,cache_read_price) VALUES
 (100,8,'anthropic','["claude-sonnet-5"]',.000002,.00001,.0000025,.0000002),
 (110,10,'anthropic','["claude-opus-5-5"]',.000004,.00002,.000005,.0000002),
 (120,11,'kiro','["claude-sonnet-4-6","claude-sonnet-5"]',.000003,.000015,.00000375,.0000003),
 (130,12,'anthropic','["claude-sonnet-5-5"]',.000123,.000456,NULL,NULL),
 (131,12,'anthropic','["claude-sonnet-5.5-thinking"]',.000222,.000333,NULL,NULL),
 (140,13,'kiro','["claude-sonnet-5","claude-sonnet-5-5-thinking"]',.000321,.000654,NULL,NULL),
 (150,14,'anthropic','["claude-sonnet-5"]',.000002,.00001,NULL,NULL),
 (160,6,'anthropic','["claude-sonnet-5"]',.000002,.00001,NULL,NULL);
 INSERT INTO channel_pricing_intervals(pricing_id,min_tokens,max_tokens,tier_label,input_price) VALUES (130,0,199999,'custom',.000789),(140,0,200000,'mixed',.000444);
 INSERT INTO channel_account_stats_model_pricing(id,rule_id,platform,models,input_price,output_price) VALUES
 (900,20,'kiro','["claude-sonnet-5"]',.000077,.000066),
 (910,21,'kiro','["claude-sonnet-5","claude-sonnet-5-5"]',.000077,.000066),
 (920,22,'anthropic','["claude-sonnet-5-5"]',.000333,.000444),
 (921,22,'anthropic','["claude-sonnet-5.5"]',.000555,.000666);
 INSERT INTO channel_account_stats_pricing_intervals(pricing_id,min_tokens,max_tokens,input_price) VALUES(910,0,5000,.000888),(920,0,6000,.000999);
 `)
	require.NoError(t, err)
	migration, err := FS.ReadFile("251_add_claude_sonnet55_support.sql")
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
	snapshot := func() string {
		var v string
		require.NoError(t, db.QueryRow(`SELECT jsonb_build_object('accounts',(SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM accounts a),'groups',(SELECT jsonb_agg(to_jsonb(g) ORDER BY id) FROM groups g),'user',(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM channel_model_pricing p),'stats',(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM channel_account_stats_model_pricing p),'intervals',(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM channel_pricing_intervals p),'stats_intervals',(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM channel_account_stats_pricing_intervals p))::text`).Scan(&v))
		return v
	}
	var countBefore int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM channel_model_pricing WHERE enabled`).Scan(&countBefore))
	run()
	var mappingRaw string
	require.NoError(t, db.QueryRow(`SELECT credentials->>'model_mapping' FROM accounts WHERE id=1`).Scan(&mappingRaw))
	var mapping map[string]string
	require.NoError(t, json.Unmarshal([]byte(mappingRaw), &mapping))
	require.Equal(t, "custom", mapping["claude-sonnet-5-5-thinking"])
	require.Equal(t, "claude-sonnet-5.5", mapping["claude-sonnet-5-5"])
	var eligible int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM accounts WHERE credentials->'model_mapping' ? 'claude-sonnet-5-5'`).Scan(&eligible))
	require.Equal(t, 2, eligible)
	var g string
	require.NoError(t, db.QueryRow(`SELECT model_allowlist->'models' FROM groups WHERE id=19`).Scan(&g))
	require.JSONEq(t, `["claude-opus-5-5","claude-sonnet-5-5","claude-sonnet-5-5-thinking","claude-sonnet-5.5","claude-sonnet-5.5-thinking"]`, g)
	var flag bool
	require.NoError(t, db.QueryRow(`SELECT (model_allowlist->>'enabled')::boolean FROM groups WHERE id=19`).Scan(&flag))
	require.False(t, flag)
	for _, id := range []int{9, 2} {
		require.NoError(t, db.QueryRow(`SELECT model_allowlist->'models' FROM groups WHERE id=$1`, id).Scan(&g))
		require.JSONEq(t, `["claude-sonnet-5"]`, g)
	}
	// Equivalent channel repository/admin response projection: one visible dedicated
	// row with all four billable aliases, and an increased rule count.
	var countAfter int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM channel_model_pricing WHERE enabled`).Scan(&countAfter))
	require.Equal(t, countBefore+3, countAfter)
	aliases := `["claude-sonnet-5-5","claude-sonnet-5-5-thinking","claude-sonnet-5.5","claude-sonnet-5.5-thinking"]`
	for _, id := range []int{8, 10, 11, 12, 13} {
		var n int
		require.NoError(t, db.QueryRow(`SELECT count(*) FROM channel_model_pricing WHERE channel_id=$1 AND enabled AND models ?| ARRAY['claude-sonnet-5-5','claude-sonnet-5-5-thinking','claude-sonnet-5.5','claude-sonnet-5.5-thinking']`, id).Scan(&n))
		require.Equal(t, 1, n)
		require.NoError(t, db.QueryRow(`SELECT models::text FROM channel_model_pricing WHERE channel_id=$1 AND enabled AND models ? 'claude-sonnet-5-5'`, id).Scan(&g))
		require.JSONEq(t, aliases, g)
	}
	var input, oneHour float64
	require.NoError(t, db.QueryRow(`SELECT input_price,cache_write_1h_price FROM channel_model_pricing WHERE channel_id=10 AND models ? 'claude-sonnet-5-5'`).Scan(&input, &oneHour))
	require.Equal(t, .000002, input)
	require.Equal(t, .000004, oneHour)
	require.NoError(t, db.QueryRow(`SELECT input_price FROM channel_model_pricing WHERE id=130`).Scan(&input))
	require.Equal(t, .000123, input)
	require.NoError(t, db.QueryRow(`SELECT enabled FROM channel_model_pricing WHERE id=131`).Scan(&flag))
	require.False(t, flag)
	require.NoError(t, db.QueryRow(`SELECT input_price FROM channel_pricing_intervals WHERE pricing_id=130`).Scan(&input))
	require.Equal(t, .000789, input)
	require.NoError(t, db.QueryRow(`SELECT i.input_price FROM channel_pricing_intervals i JOIN channel_model_pricing p ON p.id=i.pricing_id WHERE p.channel_id=13 AND p.models ? 'claude-sonnet-5-5'`).Scan(&input))
	require.Equal(t, .000444, input)
	require.NoError(t, db.QueryRow(`SELECT models::text FROM channel_account_stats_model_pricing WHERE id=900`).Scan(&g))
	require.JSONEq(t, `["claude-sonnet-5"]`, g)
	require.NoError(t, db.QueryRow(`SELECT models::text FROM channel_account_stats_model_pricing WHERE rule_id=21 AND models ? 'claude-sonnet-5-5'`).Scan(&g))
	require.JSONEq(t, `["claude-sonnet-5-5"]`, g)
	require.NoError(t, db.QueryRow(`SELECT i.input_price FROM channel_account_stats_pricing_intervals i JOIN channel_account_stats_model_pricing p ON p.id=i.pricing_id WHERE p.rule_id=21 AND p.models ? 'claude-sonnet-5-5'`).Scan(&input))
	require.Equal(t, .000888, input)
	before := snapshot()
	run()
	require.JSONEq(t, before, snapshot(), "second run preserves timestamps, prices, order and rows")
}
