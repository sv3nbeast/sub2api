import type { RankingHistoryPoint } from '@/api/rankings'

export const RANKING_COLORS = ['#0d9488', '#5b7cfa', '#d89058', '#9975d8', '#e3ad3b', '#e67385', '#64748b', '#53a3b9']

const vendorColors: Record<string, string> = {
  anthropic: '#d89058', openai: '#0d9488', google: '#5b7cfa', xai: '#64748b',
  moonshot: '#9975d8', kimi: '#9975d8', zhipu: '#e3ad3b', deepseek: '#53a3b9', minimax: '#e67385',
  others: '#94a3b8', mixed: '#94a3b8',
}

export function vendorColor(id: string, index = 0): string {
  return vendorColors[id.toLowerCase()] || RANKING_COLORS[index % RANKING_COLORS.length]
}

export interface HistorySeries {
  id: string
  label: string
  color: string
  values: number[]
}

export interface HistoryChartData {
  labels: string[]
  series: HistorySeries[]
}

export function isOtherHistoryPoint(point: RankingHistoryPoint, kind: 'models' | 'vendors'): boolean {
  return point.vendor_id === 'others' || (kind === 'models' && point.model === 'Others')
}

// The public API sends aggregated buckets, not raw requests. Group by timestamp
// (labels can repeat across months), fill missing series with zero and collapse
// the long tail so rendering cost remains bounded when model counts grow.
export function buildHistoryChart(
  points: RankingHistoryPoint[],
  kind: 'models' | 'vendors',
  otherLabel: string,
  maxSeries = 8,
): HistoryChartData {
  const buckets = new Map<string, string>()
  const totals = new Map<string, { label: string; vendorId: string; total: number }>()
  let hasOther = false
  for (const point of points) {
    if (!point.ts || !Number.isFinite(Date.parse(point.ts)) || !Number.isFinite(point.tokens) || point.tokens < 0) continue
    buckets.set(point.ts, point.label)
    if (isOtherHistoryPoint(point, kind)) { hasOther = hasOther || point.tokens > 0; continue }
    const id = kind === 'models' ? point.model : point.vendor_id
    if (!id) continue
    const existing = totals.get(id) || { label: kind === 'models' ? id : point.vendor, vendorId: point.vendor_id, total: 0 }
    existing.total += point.tokens
    totals.set(id, existing)
  }
  const timestamps = [...buckets.keys()].sort((a, b) => Date.parse(a) - Date.parse(b))
  const bucketIndex = new Map(timestamps.map((ts, index) => [ts, index]))
  const ranked = [...totals.entries()].filter(([, info]) => info.total > 0)
    .sort((a, b) => b[1].total - a[1].total || a[0].localeCompare(b[0]))
  const selected = ranked.slice(0, maxSeries)
  const series: HistorySeries[] = selected.map(([id, info], index) => ({
    id, label: info.label,
    color: kind === 'vendors' ? vendorColor(info.vendorId, index) : RANKING_COLORS[index % RANKING_COLORS.length],
    values: Array(timestamps.length).fill(0),
  }))
  const byId = new Map(series.map((item) => [item.id, item]))
  const other = hasOther || ranked.length > maxSeries
    ? { id: '__other__', label: otherLabel, color: '#94a3b8', values: Array(timestamps.length).fill(0) } : null
  const bucketTotals = Array(timestamps.length).fill(0)
  for (const point of points) {
    if (!Number.isFinite(point.tokens) || point.tokens < 0) continue
    const index = bucketIndex.get(point.ts)
    const id = kind === 'models' ? point.model : point.vendor_id
    if (index === undefined || !id) continue
    const target = isOtherHistoryPoint(point, kind) ? other : byId.get(id) || other
    if (target) target.values[index] += point.tokens
    bucketTotals[index] += point.tokens
  }
  if (other) series.push(other)
  if (kind === 'vendors') {
    for (const item of series) {
      item.values = item.values.map((value, index) => bucketTotals[index] > 0 ? value / bucketTotals[index] * 100 : 0)
    }
  }
  return { labels: timestamps.map((ts) => buckets.get(ts) || ts), series }
}
