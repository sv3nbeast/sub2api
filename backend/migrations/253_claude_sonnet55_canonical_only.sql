-- Keep the public Claude Sonnet 5.5 contract on Anthropic's canonical model ID.
-- Older aliases remain accepted by request normalization for compatibility, but
-- they must not be exposed in group allowlists (including subscription groups),
-- account defaults, or pricing.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

UPDATE accounts
SET credentials = jsonb_set(
  credentials,
  '{model_mapping}',
  (credentials->'model_mapping') - ARRAY[
    'claude-sonnet-5-5-thinking',
    'claude-sonnet-5.5',
    'claude-sonnet-5.5-thinking'
  ]::text[],
  false
), updated_at = NOW()
WHERE deleted_at IS NULL
  AND jsonb_typeof(credentials->'model_mapping') = 'object'
  AND (credentials->'model_mapping') ?| ARRAY[
    'claude-sonnet-5-5-thinking',
    'claude-sonnet-5.5',
    'claude-sonnet-5.5-thinking'
  ];

WITH normalized AS (
  SELECT id,
    jsonb_set(
      model_allowlist,
      '{models}',
      COALESCE((
        SELECT jsonb_agg(model ORDER BY ord)
        FROM jsonb_array_elements_text(model_allowlist->'models') WITH ORDINALITY AS v(model, ord)
        WHERE model NOT IN (
          'claude-sonnet-5-5-thinking',
          'claude-sonnet-5.5',
          'claude-sonnet-5.5-thinking'
        )
      ), '[]'::jsonb),
      false
    ) AS config
  FROM groups
  WHERE deleted_at IS NULL AND jsonb_typeof(model_allowlist->'models') = 'array'
)
UPDATE groups g
SET model_allowlist = n.config, updated_at = NOW()
FROM normalized n
WHERE g.id = n.id AND g.model_allowlist IS DISTINCT FROM n.config;

DO $sonnet55_canonical$
DECLARE
  pricing_table TEXT;
  normalized JSONB;
  row_record RECORD;
BEGIN
  FOREACH pricing_table IN ARRAY ARRAY[
    'channel_model_pricing',
    'channel_account_stats_model_pricing'
  ] LOOP
    FOR row_record IN EXECUTE format(
      'SELECT id, models FROM %I WHERE models ?| ARRAY[''claude-sonnet-5-5-thinking'', ''claude-sonnet-5.5'', ''claude-sonnet-5.5-thinking'']',
      pricing_table
    ) LOOP
      SELECT COALESCE(jsonb_agg(model ORDER BY ord), '[]'::jsonb)
      INTO normalized
      FROM jsonb_array_elements_text(row_record.models) WITH ORDINALITY AS v(model, ord)
      WHERE model NOT IN (
        'claude-sonnet-5-5-thinking',
        'claude-sonnet-5.5',
        'claude-sonnet-5.5-thinking'
      );

      EXECUTE format(
        'UPDATE %I SET models = $1, enabled = (jsonb_array_length($1) > 0), updated_at = NOW() WHERE id = $2',
        pricing_table
      ) USING normalized, row_record.id;
    END LOOP;
  END LOOP;
END $sonnet55_canonical$;
