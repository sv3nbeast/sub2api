-- Sonnet 5.5: verified Anthropic-compatible API-key and Kiro OAuth generation.
-- Public id claude-sonnet-5-5; Kiro native id claude-sonnet-5.5.
-- Official USD/MTok: input 2, output 10, read .20, 5m write 2.50, 1h write 4.
-- Latest Claude direct/AWS groups only; subscription and history lists stay unchanged.
-- Forward-only; preserve operator overrides and independent statistics prices.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

WITH eligible AS (
 SELECT id, credentials, jsonb_build_object(
   'claude-sonnet-5-5', native_id, 'claude-sonnet-5-5-thinking', native_id,
   'claude-sonnet-5.5', native_id, 'claude-sonnet-5.5-thinking', native_id
 ) || (credentials->'model_mapping') AS mapping
 FROM accounts a CROSS JOIN LATERAL (
   SELECT CASE WHEN a.platform='kiro' THEN 'claude-sonnet-5.5' ELSE 'claude-sonnet-5-5' END AS native_id
 ) native
 WHERE a.deleted_at IS NULL AND a.parent_account_id IS NULL
   AND ((a.platform='kiro' AND a.type='oauth') OR (a.platform='anthropic' AND a.type='apikey'))
   AND jsonb_typeof(a.credentials->'model_mapping')='object'
   AND (a.credentials->'model_mapping'->>'claude-sonnet-5'='claude-sonnet-5'
        OR a.credentials->'model_mapping'->>'claude-opus-5-5' =
           CASE WHEN a.platform='kiro' THEN 'claude-opus-5.5' ELSE 'claude-opus-5-5' END)
   AND NOT EXISTS (SELECT 1 FROM jsonb_object_keys(a.credentials->'model_mapping') k WHERE k LIKE '%*%')
)
UPDATE accounts a SET credentials=jsonb_set(a.credentials,'{model_mapping}',e.mapping),updated_at=NOW()
FROM eligible e WHERE a.id=e.id AND a.credentials->'model_mapping' IS DISTINCT FROM e.mapping;

WITH eligible AS (
 SELECT id, model_allowlist FROM groups
 WHERE deleted_at IS NULL AND subscription_type='standard'
   AND ((platform='anthropic' AND name='Claude最新模型-官转渠道')
        OR (platform='kiro' AND name='Claude最新模型-AWS渠道'))
   AND jsonb_typeof(model_allowlist->'models')='array'
   AND model_allowlist->'models' ?| ARRAY['claude-sonnet-5','claude-opus-5-5']
), expanded AS (
 SELECT id,jsonb_set(model_allowlist,'{models}',(
   SELECT jsonb_agg(model ORDER BY first_pos) FROM (
    SELECT model,min(pos) AS first_pos FROM jsonb_array_elements_text(
     model_allowlist->'models' || '["claude-sonnet-5-5","claude-sonnet-5-5-thinking","claude-sonnet-5.5","claude-sonnet-5.5-thinking"]'::jsonb
    ) WITH ORDINALITY v(model,pos) GROUP BY model
   ) dedup
 )) AS config FROM eligible
)
UPDATE groups g SET model_allowlist=e.config,updated_at=NOW() FROM expanded e
WHERE g.id=e.id AND g.model_allowlist IS DISTINCT FROM e.config;

DO $sonnet55$
DECLARE
 price_table TEXT; scope_column TEXT; interval_table TEXT; provider TEXT;
 scope_id BIGINT; canonical_id BIGINT; source_id BIGINT; other_row RECORD;
 columns_to_copy TEXT; intervals_to_copy TEXT; scopes_query TEXT;
 remaining JSONB; target_aliases JSONB;
 aliases CONSTANT JSONB := '["claude-sonnet-5-5","claude-sonnet-5-5-thinking","claude-sonnet-5.5","claude-sonnet-5.5-thinking"]'::jsonb;
BEGIN
 FOR price_table,scope_column,interval_table IN VALUES
  ('channel_model_pricing','channel_id','channel_pricing_intervals'),
  ('channel_account_stats_model_pricing','rule_id','channel_account_stats_pricing_intervals')
 LOOP
  SELECT string_agg(quote_ident(attname),',' ORDER BY attnum) INTO columns_to_copy
   FROM pg_attribute WHERE attrelid=price_table::regclass AND attnum>0 AND NOT attisdropped
    AND attname NOT IN ('id','models','created_at','updated_at');
  SELECT string_agg(quote_ident(attname),',' ORDER BY attnum) INTO intervals_to_copy
   FROM pg_attribute WHERE attrelid=interval_table::regclass AND attnum>0 AND NOT attisdropped
    AND attname NOT IN ('id','pricing_id','created_at','updated_at');
  FOREACH provider IN ARRAY ARRAY['anthropic','kiro'] LOOP
   scopes_query := format('SELECT DISTINCT %I FROM %I p WHERE platform=$1 AND enabled
    AND models ?| ARRAY(SELECT jsonb_array_elements_text($2))',scope_column,price_table);
   IF price_table='channel_model_pricing' THEN
    -- group19-safe does not price Sonnet5: latest-channel membership plus an
    -- existing modern Claude price is the eligibility boundary, not Sonnet5 alone.
    scopes_query := scopes_query || ' UNION SELECT DISTINCT p.channel_id
     FROM channel_model_pricing p JOIN channels c ON c.id=p.channel_id
     WHERE p.platform=$1 AND p.enabled AND p.models ?| ARRAY[''claude-sonnet-5'',''claude-opus-5-5'']
      AND (c.name LIKE ''claude-最新模型%'' OR c.name=''Kiro Claude-AWS''
       OR EXISTS (SELECT 1 FROM channel_groups cg JOIN groups g ON g.id=cg.group_id
        WHERE cg.channel_id=p.channel_id AND g.deleted_at IS NULL AND g.subscription_type=''standard''
         AND g.name IN (''Claude最新模型-官转渠道'',''Claude最新模型-AWS渠道'')))';
   END IF;
   FOR scope_id IN EXECUTE scopes_query USING provider,aliases LOOP
    target_aliases := aliases;
    IF price_table='channel_account_stats_model_pricing' THEN
     -- Never invent operator upstream costs, nor expand their model scope.
     EXECUTE format('SELECT jsonb_agg(alias ORDER BY pos) FROM jsonb_array_elements_text($1)
      WITH ORDINALITY v(alias,pos) WHERE EXISTS (SELECT 1 FROM %I WHERE %I=$2 AND platform=$3
       AND enabled AND models ? alias)',price_table,scope_column)
      INTO target_aliases USING aliases,scope_id,provider;
    END IF;
    canonical_id := NULL;
    EXECUTE format('SELECT id FROM %I WHERE %I=$1 AND platform=$2 AND enabled
     AND jsonb_array_length(models)>0 AND models <@ $3 ORDER BY id LIMIT 1',price_table,scope_column)
     INTO canonical_id USING scope_id,provider,target_aliases;
    IF canonical_id IS NULL THEN
     source_id := NULL;
     EXECUTE format('SELECT id FROM %I WHERE %I=$1 AND platform=$2 AND enabled
      AND models ?| ARRAY(SELECT jsonb_array_elements_text($3)) ORDER BY id LIMIT 1',price_table,scope_column)
      INTO source_id USING scope_id,provider,target_aliases;
     IF source_id IS NOT NULL THEN
      -- Split explicit mixed operator rows without losing prices or intervals.
      EXECUTE format('INSERT INTO %I(models,%s) SELECT $1,%s FROM %I WHERE id=$2 RETURNING id',
       price_table,columns_to_copy,columns_to_copy,price_table) INTO canonical_id USING target_aliases,source_id;
      EXECUTE format('INSERT INTO %I(pricing_id,%s) SELECT $1,%s FROM %I WHERE pricing_id=$2',
       interval_table,intervals_to_copy,intervals_to_copy,interval_table) USING canonical_id,source_id;
     ELSIF price_table='channel_model_pricing' THEN
      EXECUTE format('INSERT INTO %I(%I,platform,models,billing_mode,input_price,output_price,
       cache_write_price,cache_read_price,cache_write_5m_price,cache_write_1h_price,enabled)
       VALUES($1,$2,$3,''token'',.000002,.000010,.0000025,.0000002,.0000025,.000004,true) RETURNING id',
       price_table,scope_column) INTO canonical_id USING scope_id,provider,target_aliases;
     ELSE
      CONTINUE;
     END IF;
    ELSE
     EXECUTE format('UPDATE %I SET models=$1,updated_at=NOW() WHERE id=$2 AND models IS DISTINCT FROM $1',price_table)
      USING target_aliases,canonical_id;
    END IF;
    -- Only matching official flat user prices receive missing cache breakdown.
    -- Custom prices/tiers and statistics costs are never rewritten.
    IF price_table='channel_model_pricing' THEN
     UPDATE channel_model_pricing p SET cache_write_5m_price=COALESCE(cache_write_5m_price,.0000025),
      cache_write_1h_price=COALESCE(cache_write_1h_price,.000004),updated_at=NOW()
     WHERE p.id=canonical_id AND billing_mode='token' AND input_price=.000002 AND output_price=.000010
      AND cache_write_price=.0000025 AND cache_read_price=.0000002
      AND (cache_write_5m_price IS NULL OR cache_write_1h_price IS NULL)
      AND NOT EXISTS (SELECT 1 FROM channel_pricing_intervals i WHERE i.pricing_id=p.id);
    END IF;
    FOR other_row IN EXECUTE format('SELECT id,models FROM %I WHERE %I=$1 AND platform=$2
     AND enabled AND id<>$3 AND models ?| ARRAY(SELECT jsonb_array_elements_text($4))',price_table,scope_column)
     USING scope_id,provider,canonical_id,target_aliases LOOP
     SELECT COALESCE(jsonb_agg(model ORDER BY pos),'[]'::jsonb) INTO remaining
      FROM jsonb_array_elements_text(other_row.models) WITH ORDINALITY v(model,pos) WHERE NOT target_aliases ? model;
     EXECUTE format('UPDATE %I SET models=$1,enabled=(jsonb_array_length($1)>0),updated_at=NOW() WHERE id=$2',price_table)
      USING remaining,other_row.id;
    END LOOP;
   END LOOP;
  END LOOP;
 END LOOP;
END $sonnet55$;
