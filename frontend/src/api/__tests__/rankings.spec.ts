import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiClient } from '@/api/client'
import { getRankings, normalizeRankingPeriod } from '@/api/rankings'
import { rankingsSnapshot } from '@/__tests__/fixtures/rankings'

vi.mock('@/api/client', () => ({ apiClient: { get: vi.fn() } }))

describe('rankings public API', () => {
  beforeEach(() => vi.mocked(apiClient.get).mockReset())

  it('calls the public endpoint with a period and cancellation signal', async () => {
    const response = rankingsSnapshot('month')
    const controller = new AbortController()
    vi.mocked(apiClient.get).mockResolvedValue({ data: response })
    await expect(getRankings('month', { signal: controller.signal })).resolves.toBe(response)
    expect(apiClient.get).toHaveBeenCalledWith('/rankings', { params: { period: 'month' }, signal: controller.signal })
  })

  it('rejects malformed payloads instead of rendering false zero usage', async () => {
    vi.mocked(apiClient.get).mockResolvedValue({ data: '<html>Not found</html>' })
    await expect(getRankings('week')).rejects.toThrow('Invalid rankings response')
    vi.mocked(apiClient.get).mockResolvedValue({ data: rankingsSnapshot('week', { models_history: undefined } as never) })
    await expect(getRankings('week')).rejects.toThrow('Invalid rankings response')
  })

  it('normalizes unsupported URL values to the default without guessing', () => {
    expect(normalizeRankingPeriod('year')).toBe('year')
    expect(normalizeRankingPeriod('all')).toBe('week')
    expect(normalizeRankingPeriod(['month', 'year'])).toBe('week')
    expect(normalizeRankingPeriod(null)).toBe('week')
  })
})
