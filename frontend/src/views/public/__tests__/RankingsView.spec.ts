import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import RankingsView from '../RankingsView.vue'
import { rankingsSnapshot, rankedModel } from '@/__tests__/fixtures/rankings'
import type { RankingPeriod, RankingsSnapshot } from '@/api/rankings'

const { getRankings } = vi.hoisted(() => ({ getRankings: vi.fn() }))
vi.mock('@/api/rankings', async (original) => ({ ...await original<typeof import('@/api/rankings')>(), getRankings }))
vi.mock('@/composables/usePublicUiVersion', async () => {
  const { ref } = await import('vue')
  return { usePublicUiVersion: () => ({ isPublicUiV2: ref(false) }) }
})
vi.mock('@/utils/featureFlags', () => ({ FeatureFlags: { publicModelMarket: 'model-market' }, isFeatureFlagEnabled: () => true }))
// The test build uses vue-i18n's runtime-only alias; locale compilation is a
// production Vite transform. Resolve this feature's messages directly here.
vi.mock('vue-i18n', async (original) => {
  const actual = await original<typeof import('vue-i18n')>()
  const { ref } = await import('vue')
  const { default: messages } = await import('@/i18n/locales/zh/rankings')
  return { ...actual, useI18n: () => ({
    locale: ref('zh'),
    t: (key: string, values: Record<string, unknown> = {}) => {
      let message: unknown = messages
      for (const part of key.split('.')) message = (message as Record<string, unknown>)[part]
      return String(message || key).replace(/\{(\w+)\}/g, (_, part) => String(values[part] ?? ''))
    },
  }) }
})

const wrappers: VueWrapper[] = []
async function mountView(url = '/rankings') {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/rankings', component: RankingsView }, { path: '/model-plaza', component: { template: '<div />' } }] })
  await router.push(url)
  await router.isReady()
  const wrapper = mount(RankingsView, {
    global: {
      plugins: [router],
      stubs: {
        PublicLayout: { template: '<div><slot /></div>' }, PublicHeader: true, PublicFooter: true,
        RankingsChart: { props: ['kind', 'snapshot'], template: '<div class="chart-stub" :data-kind="kind" :data-period="snapshot.period" />' },
        Icon: true, ModelIcon: true,
      },
    },
  })
  wrappers.push(wrapper)
  await flushPromises()
  return { wrapper, router }
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (error: Error) => void
  const promise = new Promise<T>((ok, fail) => { resolve = ok; reject = fail })
  return { promise, resolve, reject }
}

describe('public RankingsView', () => {
  beforeEach(() => {
    localStorage.clear()
    getRankings.mockReset().mockImplementation((period: RankingPeriod) => Promise.resolve(rankingsSnapshot(period)))
  })
  afterEach(() => wrappers.splice(0).forEach((wrapper) => wrapper.unmount()))

  it('renders real totals including cache, null comparisons, and three charts', async () => {
    const { wrapper } = await mountView()
    expect(getRankings).toHaveBeenCalledWith('week', { signal: expect.any(AbortSignal) })
    expect(wrapper.get('[data-period="week"]').attributes('aria-pressed')).toBe('true')
    expect(wrapper.text()).toContain('1.25M')
    expect(wrapper.text()).toContain('claude-opus-5')
    expect(wrapper.text()).toContain('无对比数据')
    expect(wrapper.text()).toContain('本站各渠道')
    expect(wrapper.get('.rankings-definition').text()).toContain('缓存读取')
    expect(wrapper.findAll('.chart-stub').map((chart) => chart.attributes('data-kind'))).toEqual(['models', 'share', 'vendors'])
    expect(wrapper.get('.rankings-rank-change.is-new').attributes('aria-label')).toBe('新上榜')
    expect(wrapper.get('.rankings-tokens').attributes('title')).toContain('缓存读取 500,000')
  })

  it('keeps URL period selection and browser back in sync while preserving other query values', async () => {
    const { wrapper, router } = await mountView('/rankings?period=month&source=docs')
    expect(getRankings).toHaveBeenLastCalledWith('month', expect.anything())
    await wrapper.get('[data-period="quarter"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query).toEqual({ period: 'quarter', source: 'docs' })
    expect(getRankings).toHaveBeenLastCalledWith('quarter', expect.anything())
    router.back()
    await flushPromises()
    expect(router.currentRoute.value.query.period).toBe('month')
    expect(wrapper.get('[data-period="month"]').attributes('aria-pressed')).toBe('true')
  })

  it('filters without renumbering ranks or recalculating sitewide shares', async () => {
    const { wrapper } = await mountView()
    await wrapper.get('[data-testid="rankings-search"]').setValue('  GPT  ')
    expect(wrapper.findAll('tbody tr[data-model]')).toHaveLength(1)
    expect(wrapper.get('tbody tr[data-model] .rankings-rank').text()).toBe('2')
    expect(wrapper.get('tbody tr[data-model] .rankings-share').text()).toContain('20%')
    await wrapper.get('[data-testid="rankings-vendor"]').setValue('anthropic')
    expect(wrapper.get('[data-testid="rankings-filter-empty"]').text()).toContain('没有匹配的模型')
    await wrapper.get('[data-testid="rankings-filter-empty"] button').trigger('click')
    expect(wrapper.findAll('tbody tr[data-model]')).toHaveLength(2)
  })

  it('cancels old requests and ignores their success or failure after a newer period completes', async () => {
    const old = deferred<RankingsSnapshot>()
    getRankings.mockImplementationOnce(() => old.promise)
    const { wrapper, router } = await mountView()
    const oldSignal = getRankings.mock.calls[0][1].signal as AbortSignal
    expect(wrapper.find('[data-testid="rankings-loading"]').exists()).toBe(true)
    await router.push('/rankings?period=quarter')
    await flushPromises()
    expect(oldSignal.aborted).toBe(true)
    expect(wrapper.get('.chart-stub').attributes('data-period')).toBe('quarter')
    old.resolve(rankingsSnapshot('week', { total_tokens: 999 }))
    await flushPromises()
    expect(wrapper.get('.chart-stub').attributes('data-period')).toBe('quarter')
    expect(wrapper.find('[data-testid="rankings-error"]').exists()).toBe(false)

    const staleError = deferred<RankingsSnapshot>()
    getRankings.mockImplementationOnce(() => staleError.promise)
    await router.push('/rankings?period=month')
    await router.push('/rankings?period=today')
    await flushPromises()
    staleError.reject(new Error('old request failed'))
    await flushPromises()
    expect(wrapper.get('.chart-stub').attributes('data-period')).toBe('today')
    expect(wrapper.find('[data-testid="rankings-error"]').exists()).toBe(false)
  })

  it('provides an explicit retry and never substitutes mock rankings on failure', async () => {
    getRankings.mockRejectedValueOnce(new Error('unavailable'))
    const { wrapper } = await mountView()
    expect(wrapper.get('[data-testid="rankings-error"]').text()).toContain('暂时无法获取排行榜')
    expect(wrapper.find('table').exists()).toBe(false)
    await wrapper.get('[data-testid="rankings-error"] button').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="rankings-error"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('claude-opus-5')
  })

  it('shows empty usage without rendering empty charts or fabricated models', async () => {
    getRankings.mockResolvedValueOnce(rankingsSnapshot('week', { models: [], vendors: [], models_count: 0, total_tokens: 0, total_requests: 0 }))
    const { wrapper } = await mountView()
    expect(wrapper.get('[data-testid="rankings-empty"]').text()).toContain('还没有用量记录')
    expect(wrapper.findAll('.chart-stub')).toHaveLength(0)
    expect(wrapper.find('table').exists()).toBe(false)
  })

  it('bounds initial model rendering and expands all models only on request', async () => {
    getRankings.mockResolvedValueOnce(rankingsSnapshot('week', { models: Array.from({ length: 15 }, (_, index) => rankedModel({ rank: index + 1, model_name: `model-${index}` })) }))
    const { wrapper } = await mountView()
    expect(wrapper.findAll('tbody tr[data-model]')).toHaveLength(10)
    await wrapper.get('.rankings-expand').trigger('click')
    expect(wrapper.findAll('tbody tr[data-model]')).toHaveLength(15)
    await wrapper.get('.rankings-expand').trigger('click')
    expect(wrapper.findAll('tbody tr[data-model]')).toHaveLength(10)
  })

  it('uses the complete model count and includes years in a cross-year range', async () => {
    getRankings.mockResolvedValueOnce(rankingsSnapshot('quarter', { models_count: 132, start_at: '2025-10-02T00:00:00Z' }))
    const { wrapper } = await mountView('/rankings?period=quarter')
    expect(wrapper.get('.rankings-stat-grid').text()).toContain('132')
    expect(wrapper.get('.rankings-limit-hint').text()).toContain('132 个活跃模型')
    expect(wrapper.get('.rankings-meta').text()).toContain('2025')
    expect(wrapper.get('.rankings-meta').text()).toContain('2026')
  })
})
