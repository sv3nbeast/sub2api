import { describe, expect, it } from 'vitest'
import { buildHistoryChart } from '../chartData'
import type { RankingHistoryPoint } from '@/api/rankings'

function point(ts: string, model: string, tokens: number, vendor = 'openai'): RankingHistoryPoint {
  return { ts, label: ts.slice(0, 7), model, tokens, vendor, vendor_id: vendor }
}

describe('rankings chart aggregation', () => {
  it('orders timestamps, sums duplicate points and fills missing series with zero', () => {
    const result = buildHistoryChart([
      point('2026-10-02', 'gpt', 30), point('2026-10-01', 'claude', 10, 'anthropic'),
      point('2026-10-01', 'gpt', 20), point('2026-10-01', 'gpt', 5),
    ], 'models', 'Others')
    // Two timestamps share the same display label but remain distinct buckets.
    expect(result.labels).toHaveLength(2)
    expect(result.series.map((series) => [series.label, series.values])).toEqual([
      ['gpt', [25, 30]], ['claude', [10, 0]],
    ])
  })

  it('preserves all token totals while bounding the rendered long tail', () => {
    const result = buildHistoryChart(Array.from({ length: 30 }, (_, index) => point('2026-10-01', `model-${index}`, index + 1)), 'models', '其余', 6)
    expect(result.series).toHaveLength(7)
    expect(result.series.at(-1)?.label).toBe('其余')
    expect(result.series.reduce((sum, series) => sum + series.values[0], 0)).toBe(465)
  })

  it('normalizes vendor shares per bucket and never produces NaN for zero buckets', () => {
    const result = buildHistoryChart([
      point('2026-10-01', 'gpt', 30), point('2026-10-01', 'claude', 10, 'anthropic'),
      point('2026-10-02', 'gpt', 0), point('2026-10-02', 'claude', 0, 'anthropic'),
    ], 'vendors', 'Others')
    expect(result.series.map((series) => series.values)).toEqual([[75, 0], [25, 0]])
    expect(result.series.flatMap((series) => series.values).every(Number.isFinite)).toBe(true)
  })

  it('ignores malformed and negative points instead of corrupting the chart', () => {
    expect(buildHistoryChart([point('', 'gpt', 5), point('2026-10-01', 'gpt', NaN), point('2026-10-01', 'gpt', -2)], 'models', 'Others')).toEqual({ labels: [], series: [] })
  })

  it('combines the backend Others bucket and local long tail into one localized series', () => {
    const result = buildHistoryChart([
      ...Array.from({ length: 10 }, (_, index) => point('2026-10-01', `model-${index}`, index + 1)),
      point('2026-10-01', 'Others', 100, 'others'),
    ], 'models', '其余', 8)
    expect(result.series).toHaveLength(9)
    expect(result.series.filter((series) => series.label === '其余')).toHaveLength(1)
    expect(result.series.at(-1)?.values).toEqual([103])
    expect(result.series.reduce((sum, series) => sum + series.values[0], 0)).toBe(155)
    expect(result.series.some((series) => series.label === 'Others')).toBe(false)
  })

  it('sorts real time across a daylight-saving fallback rather than local ISO text', () => {
    const result = buildHistoryChart([
      point('2026-11-01T01:00:00-05:00', 'gpt', 3),
      point('2026-11-01T01:30:00-04:00', 'gpt', 2),
    ], 'models', 'Others')
    expect(result.series[0].values).toEqual([2, 3])
  })
})
