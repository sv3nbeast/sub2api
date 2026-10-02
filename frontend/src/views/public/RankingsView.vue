<template>
  <PublicLayout content-class="public-ui-v2__content--flush">
    <div class="rankings-page" data-testid="rankings-page">
      <PublicHeader v-if="!isPublicUiV2" />
      <div class="rankings-content" :role="isPublicUiV2 ? undefined : 'main'">
        <header class="rankings-hero">
          <div>
            <p class="rankings-eyebrow"><span aria-hidden="true"></span>{{ t('rankings.eyebrow') }}</p>
            <h1>{{ t('rankings.title') }}</h1>
            <p class="rankings-description">{{ t('rankings.description') }}</p>
          </div>
          <RouterLink v-if="modelMarketEnabled" to="/model-plaza" class="rankings-text-link">
            {{ t('rankings.browseModels') }}<Icon name="arrowRight" size="sm" />
          </RouterLink>
        </header>

        <div class="rankings-controls">
          <div class="rankings-periods" role="group" :aria-label="t('rankings.timeRange')">
            <button
              v-for="value in RANKING_PERIODS" :key="value" type="button"
              :data-period="value" :aria-pressed="period === value"
              @click="selectPeriod(value)"
            >{{ t(`rankings.periods.${value}`) }}</button>
          </div>
          <div class="rankings-update">
            <time v-if="snapshot" :datetime="snapshot.generated_at">{{ t('rankings.updatedAt', { time: formatDate(snapshot.generated_at, true) }) }}</time>
            <button type="button" :disabled="loading" :aria-label="t('rankings.refresh')" :title="t('rankings.refresh')" @click="reload">
              <Icon name="refresh" size="sm" />
            </button>
          </div>
        </div>

        <div v-if="loading" class="rankings-loading" role="status" aria-live="polite" data-testid="rankings-loading">
          <span class="sr-only">{{ t('rankings.loading') }}</span>
          <div class="rankings-stat-grid" aria-hidden="true"><div v-for="i in 3" :key="i" class="rankings-skeleton rankings-skeleton--stat"></div></div>
          <div class="rankings-main-grid" aria-hidden="true"><div class="rankings-skeleton rankings-skeleton--large"></div><div class="rankings-skeleton rankings-skeleton--large"></div></div>
        </div>

        <section v-else-if="error" class="rankings-state" role="alert" data-testid="rankings-error">
          <div class="rankings-state__icon"><Icon name="chart" size="xl" /></div>
          <h2>{{ t('rankings.loadError') }}</h2><p>{{ t('rankings.loadErrorHint') }}</p>
          <button class="rankings-button" type="button" @click="reload">{{ t('rankings.retry') }}<Icon name="refresh" size="sm" /></button>
        </section>

        <template v-else-if="snapshot">
          <div class="rankings-meta">
            <p><time :datetime="snapshot.start_at">{{ formatDate(snapshot.start_at) }}</time><span aria-hidden="true"> — </span><time :datetime="snapshot.end_at">{{ formatDate(snapshot.end_at) }}</time> · {{ t('rankings.periodHint', { timezone: snapshot.timezone }) }}</p>
            <details class="rankings-definition"><summary>{{ t('rankings.tokenDefinition') }}<Icon name="infoCircle" size="sm" /></summary><p>{{ t('rankings.tokenHint') }}</p></details>
          </div>

          <div class="rankings-stat-grid">
            <div class="rankings-stat">
              <span>{{ t('rankings.totalTokens') }}</span><strong :title="exact(snapshot.total_tokens)">{{ compact(snapshot.total_tokens) }}</strong>
              <small>Token</small><Icon name="chart" size="md" />
            </div>
            <div class="rankings-stat">
              <span>{{ t('rankings.totalRequests') }}</span><strong :title="exact(snapshot.total_requests)">{{ compact(snapshot.total_requests) }}</strong>
              <small>{{ t('rankings.requests') }}</small><Icon name="bolt" size="md" />
            </div>
            <div class="rankings-stat">
              <span>{{ t('rankings.activeModels') }}</span><strong>{{ exact(snapshot.models_count) }}</strong>
              <small>{{ t('rankings.vendorsCount', { count: snapshot.vendors.filter((vendor) => vendor.vendor_id !== 'others').length }) }}</small><Icon name="cube" size="md" />
            </div>
          </div>

          <section v-if="snapshot.models.length === 0" class="rankings-state" data-testid="rankings-empty">
            <div class="rankings-state__icon"><Icon name="trophy" size="xl" /></div>
            <h2>{{ t('rankings.emptyTitle') }}</h2><p>{{ t('rankings.emptyHint') }}</p>
          </section>

          <template v-else>
            <div class="rankings-main-grid">
              <section class="rankings-card rankings-leaderboard" aria-labelledby="model-ranking-title">
                <div class="rankings-card__header">
                  <div><h2 id="model-ranking-title">{{ t('rankings.modelLeaderboard') }}</h2><p>{{ t('rankings.modelLeaderboardHint') }}</p></div>
                  <span class="rankings-count">{{ t('rankings.resultsCount', { count: filteredModels.length }) }}</span>
                </div>
                <div class="rankings-filters">
                  <label class="rankings-search"><span class="sr-only">{{ t('rankings.searchLabel') }}</span><Icon name="search" size="sm" /><input v-model="search" type="search" :placeholder="t('rankings.searchPlaceholder')" data-testid="rankings-search" /></label>
                  <label class="rankings-select"><span class="sr-only">{{ t('rankings.filterLabel') }}</span><select v-model="vendorFilter" data-testid="rankings-vendor"><option value="">{{ t('rankings.allVendors') }}</option><option v-for="vendor in filterVendors" :key="vendor.vendor_id" :value="vendor.vendor_id">{{ vendorLabel(vendor.vendor_id, vendor.vendor) }}</option></select></label>
                </div>
                <p v-if="snapshot.models_count > snapshot.models.length" class="rankings-limit-hint">{{ t('rankings.modelLimit', { count: snapshot.models.length, total: snapshot.models_count }) }}</p>
                <div v-if="visibleModels.length" class="rankings-table-wrap" tabindex="0" :aria-label="t('rankings.modelLeaderboard')">
                  <table class="rankings-table">
                    <thead><tr><th class="rankings-rank-column" scope="col">{{ t('rankings.rank') }}</th><th scope="col">{{ t('rankings.model') }}</th><th class="rankings-number-column" scope="col">{{ t('rankings.tokens') }}</th><th class="rankings-growth-column" scope="col">{{ t('rankings.growth') }}</th></tr></thead>
                    <tbody>
                      <tr v-for="model in visibleModels" :key="model.model_name" :data-model="model.model_name">
                        <td><span class="rankings-rank" :class="{ 'rankings-rank--top': model.rank <= 3 }">{{ model.rank }}</span><span class="rankings-rank-change" :class="rankClass(model)" :title="rankLabel(model)" :aria-label="rankLabel(model)">{{ rankSymbol(model) }}</span></td>
                        <td><div class="rankings-model"><span class="rankings-model-icon" aria-hidden="true"><ModelIcon :model="model.model_name" size="22px" /></span><div><span class="rankings-model-name">{{ model.model_name }}</span><small>{{ vendorLabel(model.vendor_id, model.vendor) }}<span aria-hidden="true"> · </span>{{ t('rankings.requests') }} {{ compact(model.requests) }}</small></div></div></td>
                        <td class="rankings-number-column"><span class="rankings-tokens" :title="tokenDetails(model)">{{ compact(model.total_tokens) }}</span><span class="rankings-share">{{ percent(model.share) }}<i aria-hidden="true"><b :style="{ width: `${Math.min(100, Math.max(0, model.share * 100))}%`, backgroundColor: vendorColor(model.vendor_id, model.rank - 1) }"></b></i></span></td>
                        <td class="rankings-growth-column"><span class="rankings-growth" :class="growthClass(model.growth_pct)">{{ growthLabel(model.growth_pct) }}</span></td>
                      </tr>
                    </tbody>
                  </table>
                </div>
                <div v-else class="rankings-filter-empty" data-testid="rankings-filter-empty"><h3>{{ t('rankings.filterEmpty') }}</h3><p>{{ t('rankings.filterEmptyHint') }}</p><button type="button" class="rankings-text-link" @click="clearFilters">{{ t('rankings.clearFilters') }}</button></div>
                <button v-if="filteredModels.length > 10" type="button" class="rankings-expand" :aria-expanded="showAll" @click="showAll = !showAll">{{ showAll ? t('rankings.showLess') : t('rankings.viewAll', { count: filteredModels.length }) }}<Icon :name="showAll ? 'chevronUp' : 'chevronDown'" size="sm" /></button>
              </section>

              <div class="rankings-side-stack">
                <section class="rankings-card" aria-labelledby="model-trend-title">
                  <div class="rankings-card__header"><div><h2 id="model-trend-title">{{ t('rankings.modelTrend') }}</h2><p>{{ t('rankings.modelTrendHint') }}</p></div><span class="rankings-count">{{ t(`rankings.${period === 'today' ? 'hourly' : period === 'year' ? 'monthly' : 'daily'}`) }}</span></div>
                  <div class="rankings-chart-body"><RankingsChart :snapshot="snapshot" kind="models" :dark="isDark" :label="t('rankings.modelTrend')" /></div>
                </section>
                <section class="rankings-card" aria-labelledby="vendor-share-title">
                  <div class="rankings-card__header"><div><h2 id="vendor-share-title">{{ t('rankings.vendorShare') }}</h2><p>{{ t('rankings.vendorShareHint') }}</p></div></div>
                  <div class="rankings-share-body">
                    <div class="rankings-donut"><RankingsChart :snapshot="snapshot" kind="share" :dark="isDark" :label="t('rankings.vendorShare')" /><div class="rankings-donut-center" aria-hidden="true"><strong>{{ snapshot.vendors.filter((vendor) => vendor.vendor_id !== 'others').length }}</strong><small>{{ t('rankings.vendor') }}</small></div></div>
                    <ul class="rankings-vendor-legend"><li v-for="(vendor, index) in snapshot.vendors" :key="vendor.vendor_id"><span><i :style="{ backgroundColor: vendorColor(vendor.vendor_id, index) }" aria-hidden="true"></i>{{ vendorLabel(vendor.vendor_id, vendor.vendor) }}</span><strong>{{ percent(vendor.share) }}</strong></li></ul>
                  </div>
                </section>
              </div>
            </div>

            <section class="rankings-card rankings-vendor-trend" aria-labelledby="vendor-trend-title">
              <div class="rankings-card__header"><div><h2 id="vendor-trend-title">{{ t('rankings.vendorTrend') }}</h2><p>{{ t('rankings.vendorTrendHint') }}</p></div></div>
              <div class="rankings-chart-body"><RankingsChart :snapshot="snapshot" kind="vendors" :dark="isDark" :label="t('rankings.vendorTrend')" /></div>
            </section>

            <section class="rankings-card rankings-vendor-ranking" aria-labelledby="vendor-ranking-title">
              <div class="rankings-card__header"><div><h2 id="vendor-ranking-title">{{ t('rankings.vendorLeaderboard') }}</h2><p>{{ t('rankings.vendorLeaderboardHint') }}</p></div></div>
              <div class="rankings-table-wrap" tabindex="0" :aria-label="t('rankings.vendorLeaderboard')"><table class="rankings-table rankings-table--vendors"><thead><tr><th scope="col">{{ t('rankings.rank') }}</th><th scope="col">{{ t('rankings.vendor') }}</th><th class="rankings-number-column" scope="col">{{ t('rankings.tokens') }}</th><th class="rankings-number-column" scope="col">{{ t('rankings.share') }}</th><th class="rankings-number-column" scope="col">{{ t('rankings.modelCount') }}</th><th scope="col">{{ t('rankings.topModel') }}</th><th class="rankings-growth-column" scope="col">{{ t('rankings.growth') }}</th></tr></thead><tbody><tr v-for="(vendor, index) in snapshot.vendors" :key="vendor.vendor_id"><td><span class="rankings-rank" :class="{ 'rankings-rank--top': vendor.rank <= 3 }">{{ vendor.rank }}</span></td><td><span class="rankings-vendor-label"><i :style="{ backgroundColor: vendorColor(vendor.vendor_id, index) }" aria-hidden="true"></i>{{ vendorLabel(vendor.vendor_id, vendor.vendor) }}</span></td><td class="rankings-number-column" :title="exact(vendor.total_tokens)">{{ compact(vendor.total_tokens) }}</td><td class="rankings-number-column">{{ percent(vendor.share) }}</td><td class="rankings-number-column">{{ exact(vendor.models_count) }}</td><td class="rankings-top-model">{{ vendor.top_model }}</td><td class="rankings-growth-column"><span class="rankings-growth" :class="growthClass(vendor.growth_pct)">{{ growthLabel(vendor.growth_pct) }}</span></td></tr></tbody></table></div>
            </section>

            <div class="rankings-movers-grid">
              <section v-for="list in moverLists" :key="list.direction" class="rankings-card rankings-movers" :aria-labelledby="`mover-title-${list.direction}`">
                <div class="rankings-card__header"><div><h2 :id="`mover-title-${list.direction}`"><span class="rankings-mover-heading" :class="list.direction === 'up' ? 'is-up' : 'is-down'" aria-hidden="true">{{ list.direction === 'up' ? '↗' : '↘' }}</span>{{ list.title }}</h2><p>{{ t('rankings.moversHint') }}</p></div></div>
                <ol v-if="list.models.length" class="rankings-mover-list"><li v-for="model in list.models.slice(0, 5)" :key="model.model_name"><span class="rankings-model-icon" aria-hidden="true"><ModelIcon :model="model.model_name" size="22px" /></span><div><strong>{{ model.model_name }}</strong><small>{{ model.previous_rank }} → {{ model.rank }} · {{ compact(model.total_tokens) }} Token</small></div><span class="rankings-mover-change" :class="list.direction === 'up' ? 'is-up' : 'is-down'" :aria-label="rankLabel(model)">{{ list.direction === 'up' ? '+' : '−' }}{{ Math.abs(model.rank_delta || 0) }}</span></li></ol>
                <p v-else class="rankings-no-movers">{{ list.direction === 'up' ? t('rankings.noMovers') : t('rankings.noDroppers') }}</p>
              </section>
            </div>
          </template>
          <p class="rankings-privacy"><Icon name="shield" size="sm" />{{ t('rankings.privacy') }}</p>
        </template>
      </div>
      <PublicFooter v-if="!isPublicUiV2" />
    </div>
  </PublicLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { normalizeRankingPeriod, RANKING_PERIODS, type RankedModel, type RankingPeriod } from '@/api/rankings'
import RankingsChart from '@/components/charts/RankingsChart.vue'
import ModelIcon from '@/components/common/ModelIcon.vue'
import Icon from '@/components/icons/Icon.vue'
import PublicFooter from '@/components/public/PublicFooter.vue'
import PublicHeader from '@/components/public/PublicHeader.vue'
import PublicLayout from '@/components/public/PublicLayout.vue'
import { usePublicUiVersion } from '@/composables/usePublicUiVersion'
import { useRankings } from '@/composables/useRankings'
import { vendorColor } from '@/features/rankings/chartData'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'

const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const { isPublicUiV2 } = usePublicUiVersion()
const period = computed(() => normalizeRankingPeriod(route.query.period))
const { snapshot, loading, error, reload } = useRankings(period)
const search = ref('')
const vendorFilter = ref('')
const showAll = ref(false)
const isDark = ref(document.documentElement.classList.contains('dark'))
let themeObserver: MutationObserver | undefined
const modelMarketEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.publicModelMarket))
const filteredModels = computed(() => {
  const query = search.value.trim().toLowerCase()
  return (snapshot.value?.models || []).filter((model) => (!vendorFilter.value || model.vendor_id === vendorFilter.value)
    && (!query || model.model_name.toLowerCase().includes(query)))
})
const filterVendors = computed(() => {
  // A requested alias can span creators. Include its "mixed" bucket in filters
  // and never offer the synthetic Others bucket as a real model creator.
  const available = new Map((snapshot.value?.vendors || []).filter((vendor) => vendor.vendor_id !== 'others').map((vendor) => [vendor.vendor_id, { vendor_id: vendor.vendor_id, vendor: vendor.vendor }]))
  for (const model of snapshot.value?.models || []) available.set(model.vendor_id, { vendor_id: model.vendor_id, vendor: model.vendor })
  return [...available.values()]
})
const visibleModels = computed(() => showAll.value ? filteredModels.value : filteredModels.value.slice(0, 10))
const moverLists = computed(() => [
  { direction: 'up', title: t('rankings.rising'), models: snapshot.value?.top_movers || [] },
  { direction: 'down', title: t('rankings.falling'), models: snapshot.value?.top_droppers || [] },
])
const exact = (value: number) => new Intl.NumberFormat(locale.value).format(value)
const compact = (value: number) => {
  // Fixed engineering units make Token volumes easy to compare in both locales.
  const divisor = value >= 1e9 ? 1e9 : value >= 1e6 ? 1e6 : value >= 1e3 ? 1e3 : 1
  const suffix = divisor === 1e9 ? 'B' : divisor === 1e6 ? 'M' : divisor === 1e3 ? 'K' : ''
  return `${new Intl.NumberFormat(locale.value, { maximumFractionDigits: divisor === 1 ? 0 : 2 }).format(value / divisor)}${suffix}`
}
const percent = (value: number) => new Intl.NumberFormat(locale.value, { style: 'percent', maximumFractionDigits: 1 }).format(value)
const growthLabel = (value: number | null) => value === null ? t('rankings.noBaseline') : `${value > 0 ? '+' : ''}${new Intl.NumberFormat(locale.value, { maximumFractionDigits: 1 }).format(value)}%`
const growthClass = (value: number | null) => value === null || value === 0 ? '' : value > 0 ? 'is-up' : 'is-down'
const rankClass = (model: RankedModel) => model.rank_delta === null ? 'is-new' : model.rank_delta > 0 ? 'is-up' : model.rank_delta < 0 ? 'is-down' : ''
const rankSymbol = (model: RankedModel) => model.rank_delta === null ? '•' : model.rank_delta > 0 ? `↑${model.rank_delta}` : model.rank_delta < 0 ? `↓${Math.abs(model.rank_delta)}` : '—'
const rankLabel = (model: RankedModel) => model.rank_delta === null ? t('rankings.newEntry') : model.rank_delta > 0 ? t('rankings.rankUp', { count: model.rank_delta }) : model.rank_delta < 0 ? t('rankings.rankDown', { count: Math.abs(model.rank_delta) }) : t('rankings.rankUnchanged')
const tokenDetails = (model: RankedModel) => t('rankings.tokenDetails', { input: exact(model.input_tokens), output: exact(model.output_tokens), read: exact(model.cache_read_tokens), write: exact(model.cache_creation_tokens) })
const vendorLabel = (id: string, name: string) => id === 'others' ? t('rankings.other') : id === 'mixed' ? t('rankings.mixed') : name

function formatDate(value: string, includeTime = false): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const spansYears = snapshot.value && new Date(snapshot.value.start_at).getUTCFullYear() !== new Date(snapshot.value.end_at).getUTCFullYear()
  const options: Intl.DateTimeFormatOptions = {
    month: '2-digit', day: '2-digit',
    ...(!includeTime && (period.value === 'year' || spansYears) ? { year: 'numeric' } : {}),
    ...(includeTime ? { hour: '2-digit', minute: '2-digit' } : {}),
  }
  try {
    return new Intl.DateTimeFormat(locale.value, { ...options, timeZone: snapshot.value?.timezone || 'UTC' }).format(date)
  } catch {
    return new Intl.DateTimeFormat(locale.value, { ...options, timeZone: 'UTC' }).format(date)
  }
}
function selectPeriod(value: RankingPeriod) {
  if (period.value === value) return
  void router.push({ query: { ...route.query, period: value } })
}
function clearFilters() { search.value = ''; vendorFilter.value = '' }
watch([search, vendorFilter, period], () => { showAll.value = false })
watch(snapshot, (value) => {
  if (value && vendorFilter.value && !filterVendors.value.some((vendor) => vendor.vendor_id === vendorFilter.value)) vendorFilter.value = ''
})
onMounted(() => {
  themeObserver = new MutationObserver(() => { isDark.value = document.documentElement.classList.contains('dark') })
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
})
onBeforeUnmount(() => themeObserver?.disconnect())
</script>

<style scoped>
.rankings-page {
  --ranking-page: #f6f8f9;
  --ranking-surface: #fff;
  --ranking-subtle: #f6f8fa;
  --ranking-text: #17232c;
  --ranking-muted: #687783;
  --ranking-line: #e9edf0;
  --ranking-accent: #0b8f83;
  --ranking-accent-soft: #e9f7f3;
  --ranking-up: #128664;
  --ranking-down: #c7615b;
  --ranking-shadow: 0 2px 5px rgba(15, 23, 42, .02);
  min-height: 100dvh; background: var(--ranking-page); color: var(--ranking-text);
  font-family: ui-sans-serif, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  font-optical-sizing: auto;
}
:global(.dark .rankings-page) {
  --ranking-page: #181c22; --ranking-surface: #23272e; --ranking-subtle: #292e36;
  --ranking-text: #f0f3f6; --ranking-muted: #a0aebb; --ranking-line: #333a44;
  --ranking-accent: #67cfbc; --ranking-accent-soft: #243d39; --ranking-up: #6bcbaa; --ranking-down: #f1938d;
  --ranking-shadow: none;
}
.rankings-content { max-width: 82rem; margin: 0 auto; padding: 2.75rem 1.5rem 2rem; }
.rankings-hero { display: flex; justify-content: space-between; align-items: center; gap: 1.5rem; }
.rankings-eyebrow { display: flex; align-items: center; gap: .5rem; color: var(--ranking-accent); font-size: .6875rem; font-weight: 650; letter-spacing: .04em; }
.rankings-eyebrow > span { height: .4rem; width: .4rem; background: currentColor; border-radius: 100%; }
.rankings-hero h1 { margin: .625rem 0 .5rem; font-size: clamp(1.75rem, 3.4vw, 2.5rem); line-height: 1.2; font-weight: 750; letter-spacing: -.035em; }
.rankings-description { color: var(--ranking-muted); font-size: .875rem; line-height: 1.6; }
.rankings-text-link { display: inline-flex; align-items: center; justify-content: center; gap: .375rem; min-height: 2.75rem; color: var(--ranking-accent); font-size: .8125rem; font-weight: 600; text-decoration: none; }
.rankings-controls { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: .75rem; margin-top: 1.875rem; }
.rankings-periods { display: flex; gap: .25rem; padding: .25rem; border: 1px solid var(--ranking-line); background: var(--ranking-subtle); border-radius: .75rem; }
.rankings-periods button { min-height: 2.5rem; padding: .5rem 1.25rem; border-radius: .5rem; color: var(--ranking-muted); font-size: .8125rem; font-weight: 600; white-space: nowrap; transition: color 100ms ease, background-color 100ms ease; }
.rankings-periods button[aria-pressed="true"] { background: var(--ranking-surface); color: var(--ranking-text); box-shadow: 0 1px 5px rgba(0,0,0,.07); }
.rankings-update { display: flex; align-items: center; gap: .5rem; color: var(--ranking-muted); font-size: .6875rem; }
.rankings-update button { display: grid; min-height: 2.5rem; min-width: 2.5rem; place-items: center; border-radius: .625rem; background: var(--ranking-surface); border: 1px solid var(--ranking-line); }
.rankings-update button:disabled { opacity: .4; cursor: wait; }
.rankings-meta { display: flex; justify-content: space-between; align-items: flex-start; flex-wrap: wrap; gap: .5rem 1rem; margin: .875rem 0 1.5rem; color: var(--ranking-muted); font-size: .6875rem; line-height: 1.65; }
.rankings-definition { max-width: 33rem; text-align: right; }
.rankings-definition summary { display: inline-flex; align-items: center; gap: .375rem; cursor: pointer; color: var(--ranking-accent); min-height: 1.125rem; }
.rankings-definition p { margin-top: .5rem; font-size: .75rem; line-height: 1.7; text-align: left; }
.rankings-stat-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 1rem; margin-bottom: 1.5rem; }
.rankings-stat { position: relative; display: grid; grid-template-columns: auto 1fr; padding: 1.375rem 1.5rem; background: var(--ranking-surface); border: 1px solid var(--ranking-line); border-radius: 1rem; box-shadow: var(--ranking-shadow); }
.rankings-stat > span { grid-column: 1 / -1; color: var(--ranking-muted); font-size: .75rem; font-weight: 550; }
.rankings-stat > strong { margin-top: .5rem; font-size: 2rem; font-weight: 700; line-height: 1.2; letter-spacing: -.04em; font-variant-numeric: tabular-nums; }
.rankings-stat > small { align-self: end; margin: 0 0 .1875rem .625rem; color: var(--ranking-muted); font-size: .6875rem; }
.rankings-stat > :deep(svg) { position: absolute; right: 1.5rem; top: 1.375rem; color: var(--ranking-muted); opacity: .7; }
.rankings-main-grid { display: grid; grid-template-columns: minmax(0, 1.4fr) minmax(0, 1fr); gap: 1.5rem; align-items: start; }
.rankings-card { min-width: 0; border: 1px solid var(--ranking-line); border-radius: 1rem; background: var(--ranking-surface); box-shadow: var(--ranking-shadow); }
.rankings-card__header { display: flex; justify-content: space-between; align-items: center; gap: 1rem; padding: 1.375rem 1.5rem 1rem; }
.rankings-card__header h2 { display: flex; align-items: center; gap: .625rem; margin: 0; font-size: 1rem; font-weight: 650; letter-spacing: -.015em; }
.rankings-card__header p { margin-top: .3125rem; color: var(--ranking-muted); font-size: .6875rem; line-height: 1.6; }
.rankings-count { padding: .25rem .5rem; border-radius: .375rem; background: var(--ranking-subtle); color: var(--ranking-muted); font-size: .625rem; white-space: nowrap; }
.rankings-filters { display: flex; gap: .625rem; padding: 0 1.5rem 1rem; }
.rankings-limit-hint { margin: -.375rem 1.5rem .75rem; color: var(--ranking-muted); font-size: .6875rem; line-height: 1.6; }
.rankings-search { display: flex; min-width: 0; flex: 1; align-items: center; gap: .5rem; padding: 0 .75rem; color: var(--ranking-muted); background: var(--ranking-subtle); border: 1px solid var(--ranking-line); border-radius: .5rem; }
.rankings-search input { width: 100%; min-width: 0; min-height: 2.5rem; font-size: .75rem; color: var(--ranking-text); background: transparent; border: 0; padding: .5rem 0; outline: none; box-shadow: none; }
.rankings-search:focus-within { outline: 2px solid var(--ranking-accent); outline-offset: 1px; }
.rankings-select { max-width: 40%; }
.rankings-select select { width: 100%; min-height: 2.5rem; background-color: var(--ranking-subtle); color: var(--ranking-text); border: 1px solid var(--ranking-line); border-radius: .5rem; font-size: .75rem; padding-top: .5rem; padding-bottom: .5rem; }
.rankings-table-wrap { max-width: 100%; overflow-x: auto; overscroll-behavior-x: contain; border-radius: 0 0 1rem 1rem; }
.rankings-table { width: 100%; border-collapse: collapse; text-align: left; font-size: .75rem; }
.rankings-table thead { background: var(--ranking-subtle); color: var(--ranking-muted); }
.rankings-table th { padding: .75rem .75rem; font-weight: 500; font-size: .625rem; white-space: nowrap; }
.rankings-table td { border-top: 1px solid var(--ranking-line); padding: .9375rem .75rem; vertical-align: middle; }
.rankings-table th:first-child, .rankings-table td:first-child { padding-left: 1.5rem; }
.rankings-table th:last-child, .rankings-table td:last-child { padding-right: 1.5rem; }
.rankings-rank-column { width: 3.5rem; }
.rankings-rank { display: block; min-width: 1.25rem; font-variant-numeric: tabular-nums; font-size: .8125rem; font-weight: 600; }
.rankings-rank--top { color: var(--ranking-accent); font-weight: 750; }
.rankings-rank-change { display: block; height: .9375rem; color: var(--ranking-muted); font-size: .5625rem; }
.rankings-rank-change.is-new { color: var(--ranking-accent); }
.rankings-model { display: flex; align-items: center; min-width: 0; gap: .75rem; }
.rankings-model-icon { display: grid; flex-shrink: 0; width: 2.25rem; height: 2.25rem; place-items: center; background: var(--ranking-subtle); border: 1px solid var(--ranking-line); border-radius: .625rem; }
:global(.dark) .rankings-model-icon :deep(path[fill="#000000"]) { fill: var(--ranking-text); }
.rankings-model > div { min-width: 0; }
.rankings-model-name { display: block; max-width: 19rem; line-height: 1.5; font-weight: 600; overflow-wrap: anywhere; }
.rankings-model small { display: block; color: var(--ranking-muted); font-size: .5625rem; line-height: 1.8; margin-top: .125rem; }
.rankings-number-column { text-align: right; white-space: nowrap; font-variant-numeric: tabular-nums; }
.rankings-tokens { display: block; font-weight: 650; font-size: .8125rem; }
.rankings-share { display: flex; align-items: center; justify-content: flex-end; gap: .375rem; color: var(--ranking-muted); font-size: .5625rem; margin-top: .125rem; }
.rankings-share i { display: block; height: .1875rem; width: 2rem; background: var(--ranking-line); overflow: hidden; border-radius: .125rem; }
.rankings-share b { display: block; height: 100%; border-radius: inherit; }
.rankings-growth-column { text-align: right; white-space: nowrap; }
.rankings-growth { font-variant-numeric: tabular-nums; color: var(--ranking-muted); font-size: .6875rem; }
.is-up { color: var(--ranking-up); }
.is-down { color: var(--ranking-down); }
.rankings-growth.is-up { color: var(--ranking-up); }
.rankings-growth.is-down { color: var(--ranking-down); }
.rankings-expand { display: flex; align-items: center; justify-content: center; gap: .5rem; width: 100%; min-height: 3rem; padding: .875rem; border-top: 1px solid var(--ranking-line); color: var(--ranking-muted); font-size: .75rem; font-weight: 550; }
.rankings-side-stack { display: flex; min-width: 0; flex-direction: column; gap: 1.5rem; }
.rankings-chart-body { padding: .5rem 1.25rem 1.25rem; }
.rankings-share-body { display: flex; align-items: center; gap: 1.5rem; padding: .25rem 1.5rem 1.5rem; }
.rankings-donut { position: relative; flex-shrink: 0; }
.rankings-donut-center { position: absolute; inset: 0; display: flex; align-items: center; justify-content: center; flex-direction: column; pointer-events: none; }
.rankings-donut-center strong { font-size: 1.75rem; letter-spacing: -.03em; font-weight: 650; }
.rankings-donut-center small { margin-top: .125rem; color: var(--ranking-muted); font-size: .625rem; }
.rankings-vendor-legend { display: flex; flex: 1; min-width: 0; flex-direction: column; gap: .6875rem; margin: 0; padding: 0; list-style: none; }
.rankings-vendor-legend li { display: flex; justify-content: space-between; align-items: center; gap: .5rem; font-size: .6875rem; }
.rankings-vendor-legend li > span { display: flex; min-width: 0; align-items: center; gap: .5rem; overflow-wrap: anywhere; color: var(--ranking-muted); }
.rankings-vendor-legend i, .rankings-vendor-label i { height: .4375rem; width: .4375rem; flex-shrink: 0; border-radius: 100%; }
.rankings-vendor-legend strong { font-weight: 600; white-space: nowrap; font-variant-numeric: tabular-nums; }
.rankings-vendor-trend, .rankings-vendor-ranking { margin-top: 1.5rem; }
.rankings-table--vendors { min-width: 45rem; }
.rankings-vendor-label { display: flex; align-items: center; gap: .625rem; font-weight: 600; white-space: nowrap; }
.rankings-top-model { max-width: 16rem; overflow-wrap: anywhere; color: var(--ranking-muted); font-size: .6875rem; }
.rankings-movers-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 1.5rem; margin-top: 1.5rem; }
.rankings-mover-heading { display: grid; height: 1.5rem; width: 1.5rem; place-items: center; background: var(--ranking-subtle); border-radius: .375rem; font-size: 1.125rem; }
.rankings-mover-list { list-style: none; margin: 0; padding: .25rem 1.5rem .75rem; }
.rankings-mover-list li { display: flex; align-items: center; gap: .75rem; padding: .75rem 0; }
.rankings-mover-list li + li { border-top: 1px solid var(--ranking-line); }
.rankings-mover-list li > div { min-width: 0; flex: 1; }
.rankings-mover-list strong { display: block; overflow-wrap: anywhere; font-size: .75rem; font-weight: 600; }
.rankings-mover-list small { display: block; margin-top: .25rem; color: var(--ranking-muted); font-size: .625rem; }
.rankings-mover-change { font-size: 1.125rem; font-weight: 650; font-variant-numeric: tabular-nums; }
.rankings-no-movers { padding: 1rem 1.5rem 2rem; color: var(--ranking-muted); font-size: .8125rem; }
.rankings-privacy { display: flex; justify-content: center; align-items: center; gap: .5rem; margin-top: 1.875rem; color: var(--ranking-muted); font-size: .6875rem; line-height: 1.6; }
.rankings-state { display: flex; align-items: center; justify-content: center; flex-direction: column; min-height: 21rem; padding: 3rem 1.5rem; margin-top: 1.5rem; border: 1px solid var(--ranking-line); border-radius: 1rem; background: var(--ranking-surface); text-align: center; }
.rankings-state__icon { display: grid; height: 3.5rem; width: 3.5rem; place-items: center; border-radius: 1rem; background: var(--ranking-subtle); color: var(--ranking-muted); margin-bottom: 1.25rem; }
.rankings-state h2 { font-size: 1.125rem; font-weight: 650; }
.rankings-state p { margin-top: .5rem; color: var(--ranking-muted); font-size: .8125rem; line-height: 1.6; }
.rankings-button { display: inline-flex; align-items: center; justify-content: center; gap: .5rem; min-height: 2.75rem; margin-top: 1.25rem; padding: .625rem 1.125rem; border-radius: .625rem; background: var(--ranking-accent-soft); color: var(--ranking-accent); font-size: .8125rem; font-weight: 600; }
.rankings-filter-empty { padding: 3rem 1.5rem; text-align: center; }
.rankings-filter-empty h3 { font-size: .875rem; font-weight: 600; }
.rankings-filter-empty p { margin-top: .5rem; color: var(--ranking-muted); font-size: .75rem; }
.rankings-filter-empty button { margin-top: .5rem; }
.rankings-loading { margin-top: 1.5rem; }
.rankings-skeleton { border: 1px solid var(--ranking-line); border-radius: 1rem; background: var(--ranking-subtle); }
.rankings-skeleton--stat { height: 7.375rem; }
.rankings-skeleton--large { height: 28rem; }
.rankings-page button:not(:disabled) { cursor: pointer; }
.rankings-page button:active:not(:disabled), .rankings-text-link:active { transform: scale(.98); }
.rankings-page button:hover:not(:disabled) { filter: brightness(.98); }
.rankings-page button:focus-visible, .rankings-page a:focus-visible, .rankings-page select:focus-visible, .rankings-page summary:focus-visible, .rankings-table-wrap:focus-visible { outline: 2px solid var(--ranking-accent); outline-offset: 3px; border-radius: .5rem; }
@media (max-width: 70rem) {
  .rankings-main-grid { grid-template-columns: minmax(0, 1.2fr) minmax(0, 1fr); gap: 1rem; }
  .rankings-share-body { flex-direction: column; gap: 1rem; }
  .rankings-vendor-legend { width: 100%; display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: .625rem 1rem; }
  .rankings-model-icon { width: 1.875rem; height: 1.875rem; }
  .rankings-model { gap: .5rem; }
  .rankings-growth { font-size: .625rem; }
}
@media (max-width: 56rem) {
  .rankings-main-grid { grid-template-columns: minmax(0, 1fr); }
  .rankings-side-stack { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 1rem; }
  .rankings-movers-grid { gap: 1rem; }
  .rankings-card__header { padding-inline: 1.25rem; }
}
@media (max-width: 40rem) {
  .rankings-content { padding: 1.75rem 1rem 1.5rem; }
  .rankings-hero { align-items: flex-start; flex-direction: column; gap: .375rem; }
  .rankings-hero h1 { font-size: 1.875rem; }
  .rankings-description { font-size: .8125rem; }
  .rankings-hero .rankings-text-link { min-height: 2.25rem; font-size: .75rem; }
  .rankings-controls { margin-top: 1.25rem; gap: .5rem; }
  .rankings-periods { max-width: 100%; flex: 1; gap: .125rem; }
  .rankings-periods button { flex: 1; padding-inline: .625rem; font-size: .75rem; }
  .rankings-update { justify-content: flex-end; margin-left: auto; }
  .rankings-meta { margin-bottom: 1rem; font-size: .625rem; }
  .rankings-definition { text-align: left; }
  .rankings-stat-grid { gap: .5rem; margin-bottom: 1rem; }
  .rankings-stat { display: flex; flex-direction: column; padding: 1rem .75rem; }
  .rankings-stat > span { font-size: .625rem; }
  .rankings-stat > strong { font-size: 1.4rem; }
  .rankings-stat > small { align-self: flex-start; margin: .375rem 0 0; font-size: .5625rem; }
  .rankings-stat > :deep(svg) { display: none; }
  .rankings-side-stack, .rankings-movers-grid { display: grid; grid-template-columns: minmax(0, 1fr); }
  .rankings-card__header { padding: 1.125rem 1rem .875rem; gap: .625rem; }
  .rankings-card__header h2 { font-size: .9375rem; }
  .rankings-card__header p { font-size: .625rem; }
  .rankings-filters { padding: 0 1rem 1rem; gap: .5rem; }
  .rankings-select { max-width: 42%; }
  .rankings-table th { padding-inline: .5rem; }
  .rankings-table td { padding-inline: .5rem; padding-block: .75rem; }
  .rankings-table th:first-child, .rankings-table td:first-child { padding-left: 1rem; }
  .rankings-table th:last-child, .rankings-table td:last-child { padding-right: 1rem; }
  .rankings-table:not(.rankings-table--vendors) .rankings-model-icon { display: none; }
  .rankings-model-name { font-size: .6875rem; }
  .rankings-tokens { font-size: .75rem; }
  .rankings-share i { display: none; }
  .rankings-rank-column { width: 2.375rem; }
  .rankings-growth { font-size: .5625rem; }
  .rankings-share-body { flex-direction: row; padding-inline: 1rem; }
  .rankings-vendor-legend { display: flex; }
  .rankings-chart-body { padding-inline: .75rem; }
  .rankings-vendor-trend, .rankings-vendor-ranking, .rankings-movers-grid { margin-top: 1rem; }
  .rankings-mover-list { padding-inline: 1rem; }
  .rankings-privacy { align-items: flex-start; font-size: .625rem; }
  .rankings-privacy :deep(svg) { flex-shrink: 0; }
}
@media (prefers-reduced-motion: reduce) {
  .rankings-page button:active:not(:disabled), .rankings-text-link:active { transform: none; }
  .rankings-periods button { transition: none; }
}
@media (prefers-contrast: more) { .rankings-page { --ranking-line: #929da6; } }
</style>
