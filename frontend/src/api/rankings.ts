import { apiClient } from './client'

export const RANKING_PERIODS = ['today', 'week', 'month', 'year'] as const
export type RankingPeriod = typeof RANKING_PERIODS[number]

export interface RankedModel {
  rank: number
  previous_rank: number | null
  model_name: string
  vendor: string
  vendor_id: string
  total_tokens: number
  input_tokens: number
  output_tokens: number
  cache_read_tokens: number
  cache_creation_tokens: number
  requests: number
  share: number
  growth_pct: number | null
  rank_delta: number | null
}

export interface RankedVendor {
  rank: number
  previous_rank: number | null
  vendor: string
  vendor_id: string
  total_tokens: number
  share: number
  growth_pct: number | null
  models_count: number
  top_model: string
  requests: number
}

export interface RankingHistoryPoint {
  ts: string
  label: string
  model?: string
  vendor: string
  vendor_id: string
  tokens: number
}

export interface RankingsSnapshot {
  period: RankingPeriod
  generated_at: string
  start_at: string
  end_at: string
  timezone: string
  comparison_start_at: string
  comparison_end_at: string
  total_tokens: number
  total_requests: number
  models_count: number
  models: RankedModel[]
  vendors: RankedVendor[]
  top_movers: RankedModel[]
  top_droppers: RankedModel[]
  models_history: { points: RankingHistoryPoint[] }
  vendor_share_history: { points: RankingHistoryPoint[] }
}

export function normalizeRankingPeriod(value: unknown): RankingPeriod {
  return RANKING_PERIODS.includes(value as RankingPeriod) ? value as RankingPeriod : 'week'
}

function assertRankingsSnapshot(value: unknown): asserts value is RankingsSnapshot {
  const candidate = value as Partial<RankingsSnapshot> | null
  if (!candidate || typeof candidate !== 'object'
    || !RANKING_PERIODS.includes(candidate.period as RankingPeriod)
    || !Number.isFinite(candidate.total_tokens)
    || !Number.isFinite(candidate.total_requests)
    || !Number.isFinite(candidate.models_count)
    || typeof candidate.generated_at !== 'string'
    || typeof candidate.start_at !== 'string'
    || typeof candidate.end_at !== 'string'
    || !Array.isArray(candidate.models) || !Array.isArray(candidate.vendors)
    || !Array.isArray(candidate.top_movers) || !Array.isArray(candidate.top_droppers)
    || !Array.isArray(candidate.models_history?.points)
    || !Array.isArray(candidate.vendor_share_history?.points)) {
    throw new Error('Invalid rankings response')
  }
}

export async function getRankings(period: RankingPeriod, options?: { signal?: AbortSignal }): Promise<RankingsSnapshot> {
  const { data } = await apiClient.get<unknown>('/rankings', {
    params: { period },
    signal: options?.signal,
  })
  assertRankingsSnapshot(data)
  return data
}
