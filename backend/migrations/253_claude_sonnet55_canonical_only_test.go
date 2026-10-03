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

func TestSonnet55CanonicalOnlyMigration(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("SUB2API_SONNET55_CANONICAL_TEST_DSN")
	if dsn == "" {
		c, err := tcpostgres.Run(ctx, "postgres:18-alpine", tcpostgres.WithDatabase("sonnet55canonical"), tcpostgres.WithUsername("postgres"), tcpostgres.WithPassword("test"), tcpostgres.BasicWaitStrategies())
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, c.Terminate(ctx)) })
		dsn, err = c.ConnectionString(ctx, "sslmode=disable")
		require.NoError(t, err)
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	_, err = db.Exec(`
CREATE TABLE accounts(id BIGINT PRIMARY KEY, credentials JSONB, deleted_at TIMESTAMPTZ, updated_at TIMESTAMPTZ DEFAULT NOW());
CREATE TABLE groups(id BIGINT PRIMARY KEY, model_allowlist JSONB, deleted_at TIMESTAMPTZ, updated_at TIMESTAMPTZ DEFAULT NOW());
CREATE TABLE channel_model_pricing(id BIGSERIAL PRIMARY KEY, models JSONB, enabled BOOLEAN DEFAULT true, updated_at TIMESTAMPTZ DEFAULT NOW());
CREATE TABLE channel_account_stats_model_pricing(id BIGSERIAL PRIMARY KEY, models JSONB, enabled BOOLEAN DEFAULT true, updated_at TIMESTAMPTZ DEFAULT NOW());
INSERT INTO accounts(id, credentials) VALUES (1, '{"model_mapping":{"claude-sonnet-5-5":"claude-sonnet-5.5","claude-sonnet-5-5-thinking":"claude-sonnet-5.5","claude-sonnet-5.5":"claude-sonnet-5.5","claude-sonnet-5.5-thinking":"claude-sonnet-5.5"}}');
INSERT INTO groups(id, model_allowlist) VALUES (9, '{"enabled":true,"models":["claude-sonnet-5","claude-sonnet-5-5","claude-sonnet-5-5-thinking","claude-sonnet-5.5","claude-sonnet-5.5-thinking"]}');
INSERT INTO groups(id, model_allowlist) VALUES (10, '{"enabled":true,"models":["claude-fable-5","claude-sonnet-5-5","claude-sonnet-5-5-thinking","claude-sonnet-5.5","claude-sonnet-5.5-thinking"]}');
INSERT INTO channel_model_pricing(models) VALUES ('["claude-sonnet-5-5","claude-sonnet-5-5-thinking"]'), ('["claude-sonnet-5.5-thinking"]');
INSERT INTO channel_account_stats_model_pricing(models) VALUES ('["claude-sonnet-5-5","claude-sonnet-5.5"]');
`)
	require.NoError(t, err)
	migration, err := FS.ReadFile("253_claude_sonnet55_canonical_only.sql")
	require.NoError(t, err)
	run := func() {
		_, err = db.Exec(string(migration))
		require.NoError(t, err)
	}
	run()
	var value string
	require.NoError(t, db.QueryRow(`SELECT credentials->'model_mapping' FROM accounts WHERE id=1`).Scan(&value))
	require.JSONEq(t, `{"claude-sonnet-5-5":"claude-sonnet-5.5"}`, value)
	require.NoError(t, db.QueryRow(`SELECT model_allowlist->'models' FROM groups WHERE id=9`).Scan(&value))
	require.JSONEq(t, `["claude-sonnet-5","claude-sonnet-5-5"]`, value)
	require.NoError(t, db.QueryRow(`SELECT model_allowlist->'models' FROM groups WHERE id=10`).Scan(&value))
	require.JSONEq(t, `["claude-fable-5","claude-sonnet-5-5"]`, value)
	rows, err := db.Query(`SELECT models, enabled FROM channel_model_pricing ORDER BY id`)
	require.NoError(t, err)
	defer rows.Close()
	var models []string
	var enabled []bool
	for rows.Next() {
		var model string
		var active bool
		require.NoError(t, rows.Scan(&model, &active))
		models = append(models, model)
		enabled = append(enabled, active)
	}
	require.Equal(t, []string{`["claude-sonnet-5-5"]`, `[]`}, models)
	require.Equal(t, []bool{true, false}, enabled)
	run()
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM channel_model_pricing WHERE models ? 'claude-sonnet-5-5-thinking' OR models ? 'claude-sonnet-5.5' OR models ? 'claude-sonnet-5.5-thinking'`).Scan(&value))
	require.Equal(t, "0", value)
}
