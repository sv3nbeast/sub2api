-- Expose the exact OpenAI GPT-6.1 Sol model on direct OpenAI OAuth routes.
-- Kiro-backed OpenAI groups are intentionally excluded: this model has not been
-- verified on Kiro and must never be silently remapped to GPT-6 Sol.
-- Official standard prices (USD/token): input 0.000002, cached input 0.0000001,
-- cache write 0.0000025, output 0.00001. Above 272K input, input/cache are x2
-- and output is x1.5; Fast is x2. Forward-only and idempotent.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

-- Add the exact mapping only to direct OpenAI OAuth accounts that already expose
-- GPT-6 Astra. Existing explicit mappings, wildcard mappings, shadows and
-- deleted accounts are preserved.
UPDATE accounts a
SET credentials = jsonb_set(credentials, '{model_mapping}',
        credentials->'model_mapping' || '{"gpt-6.1-sol":"gpt-6.1-sol"}'::jsonb),
    updated_at = NOW()
WHERE a.platform = 'openai' AND a.type = 'oauth' AND a.deleted_at IS NULL
  AND a.parent_account_id IS NULL
  AND jsonb_typeof(a.credentials->'model_mapping') = 'object'
  AND a.credentials->'model_mapping'->>'gpt-6-astra' = 'gpt-6-astra'
  AND NOT (a.credentials->'model_mapping' ? 'gpt-6.1-sol')
  AND NOT EXISTS (SELECT 1 FROM jsonb_object_keys(a.credentials->'model_mapping') k WHERE k LIKE '%*%');

-- Add the model to direct OpenAI groups that already advertise GPT-6 Astra.
-- The existing allowlist enabled flag, order and duplicate policy are retained.
WITH eligible AS (
    SELECT g.id, g.model_allowlist
    FROM groups g
    WHERE g.platform = 'openai' AND g.deleted_at IS NULL
      AND jsonb_typeof(g.model_allowlist->'models') = 'array'
      AND g.model_allowlist->'models' ? 'gpt-6-astra'
      AND EXISTS (
          SELECT 1 FROM account_groups ag JOIN accounts a ON a.id = ag.account_id
          WHERE ag.group_id = g.id AND a.platform = 'openai' AND a.type = 'oauth'
            AND a.deleted_at IS NULL AND a.parent_account_id IS NULL
      )
), expanded AS (
    SELECT id, jsonb_set(model_allowlist, '{models}', (
        SELECT jsonb_agg(model ORDER BY first_pos) FROM (
            SELECT model, MIN(pos) AS first_pos
            FROM jsonb_array_elements_text(model_allowlist->'models' || '["gpt-6.1-sol"]'::jsonb)
                 WITH ORDINALITY AS v(model, pos)
            GROUP BY model
        ) dedup
    )) AS config FROM eligible
)
UPDATE groups g SET model_allowlist = e.config, updated_at = NOW()
FROM expanded e WHERE g.id = e.id AND g.model_allowlist IS DISTINCT FROM e.config;

-- Create one dedicated pricing row per eligible channel. Existing dedicated
-- custom rows keep prices and intervals; mixed/duplicate rows only lose the new
-- alias and become disabled if emptied. Account-statistics pricing is normalized
-- only when an operator already bound this exact model.
DO $gpt61$
DECLARE
    price_table TEXT;
    scope_column TEXT;
    interval_table TEXT;
    scope_id BIGINT;
    canonical_id BIGINT;
    other_row RECORD;
    remaining JSONB;
BEGIN
    FOR price_table, scope_column, interval_table IN VALUES
        ('channel_model_pricing', 'channel_id', 'channel_pricing_intervals'),
        ('channel_account_stats_model_pricing', 'rule_id', 'channel_account_stats_pricing_intervals')
    LOOP
        IF price_table = 'channel_model_pricing' THEN
            FOR scope_id IN
                SELECT DISTINCT p.channel_id
                FROM channel_model_pricing p
                JOIN channel_groups cg ON cg.channel_id = p.channel_id
                JOIN groups g ON g.id = cg.group_id
                WHERE p.platform = 'openai' AND p.enabled
                  AND p.models ? 'gpt-6-astra'
                  AND g.platform = 'openai' AND g.deleted_at IS NULL
                  AND g.model_allowlist->'models' ? 'gpt-6.1-sol'
                  AND EXISTS (
                      SELECT 1 FROM account_groups ag JOIN accounts a ON a.id = ag.account_id
                      WHERE ag.group_id = g.id AND a.platform = 'openai' AND a.type = 'oauth'
                        AND a.deleted_at IS NULL AND a.parent_account_id IS NULL
                  )
            LOOP
                canonical_id := NULL;
                SELECT id INTO canonical_id
                FROM channel_model_pricing
                WHERE channel_id = scope_id AND platform = 'openai' AND enabled
                  AND jsonb_array_length(models) > 0 AND models <@ '["gpt-6.1-sol"]'::jsonb
                ORDER BY id LIMIT 1;
                IF canonical_id IS NULL THEN
                    INSERT INTO channel_model_pricing
                        (channel_id, platform, models, billing_mode, input_price, output_price,
                         cache_write_price, cache_read_price, enabled, fast_multiplier, flex_multiplier)
                    VALUES (scope_id, 'openai', '["gpt-6.1-sol"]'::jsonb, 'token',
                            0.000002, 0.00001, 0.0000025, 0.0000001, true, 2, 0.5)
                    RETURNING id INTO canonical_id;
                    INSERT INTO channel_pricing_intervals
                        (pricing_id, min_tokens, max_tokens, tier_label, input_price, output_price,
                         cache_write_price, cache_read_price, sort_order)
                    VALUES
                        (canonical_id, 0, 272000, 'Standard', 0.000002, 0.00001, 0.0000025, 0.0000001, 0),
                        (canonical_id, 272000, NULL, '>272K', 0.000004, 0.000015, 0.000005, 0.0000002, 1);
                ELSE
                    UPDATE channel_model_pricing
                    SET models = '["gpt-6.1-sol"]'::jsonb, updated_at = NOW()
                    WHERE id = canonical_id AND models IS DISTINCT FROM '["gpt-6.1-sol"]'::jsonb;
                END IF;
                FOR other_row IN
                    SELECT id, models FROM channel_model_pricing
                    WHERE channel_id = scope_id AND platform = 'openai' AND enabled AND id <> canonical_id
                      AND models ? 'gpt-6.1-sol'
                LOOP
                    SELECT COALESCE(jsonb_agg(model ORDER BY pos), '[]'::jsonb) INTO remaining
                    FROM jsonb_array_elements_text(other_row.models) WITH ORDINALITY v(model, pos)
                    WHERE model <> 'gpt-6.1-sol';
                    UPDATE channel_model_pricing
                    SET models = remaining, enabled = (jsonb_array_length(remaining) > 0), updated_at = NOW()
                    WHERE id = other_row.id;
                END LOOP;
            END LOOP;
        ELSE
            -- Existing account-statistics rows are operator-owned; only split a
            -- row when it already contains this exact model.
            FOR scope_id IN
                SELECT DISTINCT rule_id FROM channel_account_stats_model_pricing
                WHERE platform = 'openai' AND enabled AND models ? 'gpt-6.1-sol'
            LOOP
                canonical_id := NULL;
                SELECT id INTO canonical_id
                FROM channel_account_stats_model_pricing
                WHERE rule_id = scope_id AND platform = 'openai' AND enabled
                  AND jsonb_array_length(models) > 0 AND models <@ '["gpt-6.1-sol"]'::jsonb
                ORDER BY id LIMIT 1;
                IF canonical_id IS NULL THEN
                    SELECT id INTO canonical_id
                    FROM channel_account_stats_model_pricing
                    WHERE rule_id = scope_id AND platform = 'openai' AND enabled AND models ? 'gpt-6.1-sol'
                    ORDER BY id LIMIT 1;
                END IF;
                FOR other_row IN
                    SELECT id, models FROM channel_account_stats_model_pricing
                    WHERE rule_id = scope_id AND platform = 'openai' AND enabled AND id <> canonical_id
                      AND models ? 'gpt-6.1-sol'
                LOOP
                    SELECT COALESCE(jsonb_agg(model ORDER BY pos), '[]'::jsonb) INTO remaining
                    FROM jsonb_array_elements_text(other_row.models) WITH ORDINALITY v(model, pos)
                    WHERE model <> 'gpt-6.1-sol';
                    UPDATE channel_account_stats_model_pricing
                    SET models = remaining, enabled = (jsonb_array_length(remaining) > 0), updated_at = NOW()
                    WHERE id = other_row.id;
                END LOOP;
            END LOOP;
        END IF;
    END LOOP;
END $gpt61$;
