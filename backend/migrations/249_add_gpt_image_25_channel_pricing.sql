-- Expose the two official GPT Image 2.5 models on direct OpenAI image channels.
-- There is no upstream model named "gpt-image-2.5": the exact IDs are
-- gpt-image-2.5-flare and gpt-image-2.5-sunburst.
--
-- Official Standard rates (USD/token, retrieved 2026-09-29): text input 5e-6,
-- cached text input 1.25e-6, image input 8e-6, cached image input 2e-6,
-- image output 30e-6. The existing dynamic/static pricing supplies the separate
-- cached-image rate; channel_model_pricing has no cached-image override column.
-- Official source: https://developers.openai.com/api/docs/pricing#image-generation-models
--
-- A production OAuth account explicitly maps both IDs and generated an image
-- with each on the native Codex Images endpoint on 2026-09-29. Two other active
-- accounts in the same group have empty mappings; both IDs also generated an
-- image on each. Do not infer support for mapped accounts or other providers.
--
-- Account-statistics prices are independently operator-owned. No existing
-- account-statistics rule contains either ID, so this migration leaves that
-- table and its interval table untouched. Forward-only; safe to run twice.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

-- Preserve the allowlist's enabled flag and the order of its existing models.
-- Only groups with an explicitly mapped direct OAuth account are expanded.
WITH eligible AS (
    SELECT g.id, g.model_allowlist
    FROM groups g
    WHERE g.platform = 'openai' AND g.deleted_at IS NULL
      AND g.allow_image_generation
      AND jsonb_typeof(g.model_allowlist->'models') = 'array'
      AND g.model_allowlist->'models' ? 'gpt-image-2'
      AND EXISTS (
          SELECT 1 FROM account_groups ag JOIN accounts a ON a.id = ag.account_id
          WHERE ag.group_id = g.id AND a.platform = 'openai' AND a.type = 'oauth'
            AND a.deleted_at IS NULL AND a.parent_account_id IS NULL
            AND a.credentials->'model_mapping'->>'gpt-image-2.5-flare' = 'gpt-image-2.5-flare'
            AND a.credentials->'model_mapping'->>'gpt-image-2.5-sunburst' = 'gpt-image-2.5-sunburst'
      )
), expanded AS (
    SELECT id, jsonb_set(model_allowlist, '{models}', (
        SELECT jsonb_agg(model ORDER BY first_pos) FROM (
            SELECT model, MIN(pos) AS first_pos
            FROM jsonb_array_elements_text(
                model_allowlist->'models' ||
                '["gpt-image-2.5-flare","gpt-image-2.5-sunburst"]'::jsonb
            ) WITH ORDINALITY AS v(model, pos)
            GROUP BY model
        ) dedup
    )) AS config FROM eligible
)
UPDATE groups g SET model_allowlist = e.config, updated_at = NOW()
FROM expanded e WHERE g.id = e.id AND g.model_allowlist IS DISTINCT FROM e.config;

-- Each exact upstream ID gets one enabled, admin-visible dedicated pricing row.
-- Existing dedicated custom rows retain every price and interval. Remove only
-- this ID from older mixed/duplicate rows; keep an emptied row disabled for audit.
DO $gpt_image_25$
DECLARE
    model_name TEXT;
    target_channel_id BIGINT;
    canonical_id BIGINT;
    other_row RECORD;
    remaining JSONB;
BEGIN
    FOREACH model_name IN ARRAY ARRAY['gpt-image-2.5-flare', 'gpt-image-2.5-sunburst']
    LOOP
        FOR target_channel_id IN
            SELECT DISTINCT c.id
            FROM channels c
            JOIN channel_groups cg ON cg.channel_id = c.id
            JOIN groups g ON g.id = cg.group_id
            WHERE c.status = 'active' AND NOT c.display_only
              AND g.platform = 'openai' AND g.deleted_at IS NULL
              AND g.allow_image_generation
              AND jsonb_typeof(g.model_allowlist->'models') = 'array'
              AND g.model_allowlist->'models' ? model_name
              AND EXISTS (
                  SELECT 1 FROM channel_model_pricing predecessor
                  WHERE predecessor.channel_id = c.id AND predecessor.platform = 'openai'
                    AND predecessor.enabled AND predecessor.models ? 'gpt-image-2'
              )
              AND EXISTS (
                  SELECT 1 FROM account_groups ag JOIN accounts a ON a.id = ag.account_id
                  WHERE ag.group_id = g.id AND a.platform = 'openai' AND a.type = 'oauth'
                    AND a.deleted_at IS NULL AND a.parent_account_id IS NULL
                    AND a.credentials->'model_mapping'->>model_name = model_name
              )
        LOOP
            SELECT p.id INTO canonical_id
            FROM channel_model_pricing p
            WHERE p.channel_id = target_channel_id AND p.platform = 'openai' AND p.enabled
              AND p.models = jsonb_build_array(model_name)
            ORDER BY p.id LIMIT 1;

            IF canonical_id IS NULL THEN
                INSERT INTO channel_model_pricing (
                    channel_id, platform, models, billing_mode, input_price,
                    output_price, cache_read_price, image_input_price,
                    image_output_price, enabled
                ) VALUES (
                    target_channel_id, 'openai', jsonb_build_array(model_name), 'token',
                    0.000005, 0, 0.00000125, 0.000008, 0.000030, true
                ) RETURNING id INTO canonical_id;
            END IF;

            FOR other_row IN
                SELECT p.id, p.models FROM channel_model_pricing p
                WHERE p.channel_id = target_channel_id AND p.platform = 'openai' AND p.enabled
                  AND p.id <> canonical_id AND p.models ? model_name
            LOOP
                SELECT COALESCE(jsonb_agg(model ORDER BY first_pos), '[]'::jsonb)
                INTO remaining FROM (
                    SELECT model, MIN(pos) AS first_pos
                    FROM jsonb_array_elements_text(other_row.models)
                         WITH ORDINALITY AS v(model, pos)
                    WHERE model <> model_name
                    GROUP BY model
                ) dedup;
                UPDATE channel_model_pricing
                SET models = remaining, enabled = (jsonb_array_length(remaining) > 0),
                    updated_at = NOW()
                WHERE id = other_row.id;
            END LOOP;
        END LOOP;
    END LOOP;
END $gpt_image_25$;
