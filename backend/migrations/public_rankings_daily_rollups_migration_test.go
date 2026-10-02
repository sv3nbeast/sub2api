package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPublicRankingsDailyRollupsMigrationCreatesDurableRecordingWindow(t *testing.T) {
	content, err := FS.ReadFile("251_public_rankings_daily_rollups.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")
	for _, fragment := range []string{
		"CREATE TABLE IF NOT EXISTS public_rankings_daily_rollups",
		"bucket_date DATE NOT NULL",
		"requested_model VARCHAR(200) NOT NULL",
		"creator_model VARCHAR(200) NOT NULL",
		"cache_creation_tokens BIGINT NOT NULL DEFAULT 0",
		"cache_read_tokens BIGINT NOT NULL DEFAULT 0",
		"PRIMARY KEY (bucket_date, requested_model, creator_model)",
		"CREATE TABLE IF NOT EXISTS public_rankings_rollup_state",
		"recording_started_at TIMESTAMPTZ NOT NULL DEFAULT NOW()",
		"last_completed_date DATE",
		"ON CONFLICT (id) DO NOTHING",
	} {
		require.Contains(t, sql, fragment)
	}
}
