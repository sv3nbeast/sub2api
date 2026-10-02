-- Long-lived anonymous model usage facts for the public rankings page.
-- The source usage_logs table is intentionally retained only for a short
-- operational window. These rows are written by the existing scheduled
-- dashboard job after a local day closes and never participate in billing.
CREATE TABLE IF NOT EXISTS public_rankings_daily_rollups (
    bucket_date DATE NOT NULL,
    requested_model VARCHAR(200) NOT NULL,
    creator_model VARCHAR(200) NOT NULL,
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    cache_creation_tokens BIGINT NOT NULL DEFAULT 0,
    cache_read_tokens BIGINT NOT NULL DEFAULT 0,
    requests BIGINT NOT NULL DEFAULT 0,
    computed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (bucket_date, requested_model, creator_model)
);

CREATE INDEX IF NOT EXISTS idx_public_rankings_daily_rollups_bucket
    ON public_rankings_daily_rollups (bucket_date DESC);

COMMENT ON TABLE public_rankings_daily_rollups IS
    'Long-lived anonymous daily model usage facts for the public rankings page.';

CREATE TABLE IF NOT EXISTS public_rankings_rollup_state (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    recording_started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_completed_date DATE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE public_rankings_rollup_state IS
    'Watermark for the public rankings history recording window.';

INSERT INTO public_rankings_rollup_state (id, recording_started_at, last_completed_date)
VALUES (1, CURRENT_TIMESTAMP, CURRENT_DATE - 1)
ON CONFLICT (id) DO NOTHING;
