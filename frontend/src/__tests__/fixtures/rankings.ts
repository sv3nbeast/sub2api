import type { RankedModel, RankedVendor, RankingPeriod, RankingsSnapshot } from '@/api/rankings'

export function rankedModel(overrides: Partial<RankedModel> = {}): RankedModel {
  return {
    rank: 1, previous_rank: 3, model_name: 'claude-opus-5', vendor: 'Anthropic', vendor_id: 'anthropic',
    total_tokens: 1_000_000, input_tokens: 300_000, output_tokens: 100_000, cache_read_tokens: 500_000, cache_creation_tokens: 100_000,
    requests: 20, share: 0.8, growth_pct: 25, rank_delta: 2,
    ...overrides,
  }
}

export function rankingsSnapshot(period: RankingPeriod = 'week', overrides: Partial<RankingsSnapshot> = {}): RankingsSnapshot {
  const models = [
    rankedModel(),
    rankedModel({ rank: 2, previous_rank: null, rank_delta: null, model_name: 'gpt-5.6', vendor: 'OpenAI', vendor_id: 'openai', total_tokens: 250_000, input_tokens: 100_000, output_tokens: 50_000, cache_read_tokens: 100_000, cache_creation_tokens: 0, share: 0.2, growth_pct: null }),
  ]
  const vendors: RankedVendor[] = models.map((model) => ({
    rank: model.rank, previous_rank: model.previous_rank, vendor: model.vendor, vendor_id: model.vendor_id,
    total_tokens: model.total_tokens, share: model.share, growth_pct: model.growth_pct,
    models_count: 1, top_model: model.model_name, requests: model.requests,
  }))
  const points = models.map((model) => ({ ts: '2026-10-02T00:00:00Z', label: '10/02', model: model.model_name, vendor: model.vendor, vendor_id: model.vendor_id, tokens: model.total_tokens }))
  return {
    period, generated_at: '2026-10-02T08:00:00Z', start_at: '2026-09-26T00:00:00Z', end_at: '2026-10-02T08:00:00Z', timezone: 'Asia/Shanghai',
    comparison_start_at: '2026-09-19T00:00:00Z', comparison_end_at: '2026-09-25T08:00:00Z',
    total_tokens: 1_250_000, total_requests: 40, models_count: models.length, models, vendors, top_movers: [models[0]], top_droppers: [],
    models_history: { points }, vendor_share_history: { points },
    ...overrides,
  }
}
