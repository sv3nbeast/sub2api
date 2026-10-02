<template>
  <div class="rankings-chart" :class="{ 'rankings-chart--donut': kind === 'share' }">
    <Doughnut
      v-if="kind === 'share' && shareData.datasets[0].data.length"
      :data="shareData" :options="shareOptions" role="img" :aria-label="label"
    />
    <template v-else-if="kind !== 'share' && history.series.length">
      <Bar v-if="kind === 'models'" :data="historyData" :options="barOptions" role="img" :aria-label="label" />
      <Line v-else :data="historyData" :options="lineOptions" role="img" :aria-label="label" />
    </template>
    <div v-else class="rankings-chart__empty">{{ t('rankings.noChart') }}</div>
  </div>
  <div v-if="kind !== 'share' && history.series.length" class="rankings-chart__legend" :aria-label="label">
    <span v-for="item in history.series" :key="item.id" :title="item.label">
      <i :style="{ backgroundColor: item.color }" aria-hidden="true"></i>{{ item.label }}
    </span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Bar, Doughnut, Line } from 'vue-chartjs'
import {
  Chart as ChartJS, ArcElement, BarElement, CategoryScale, LinearScale,
  PointElement, LineElement, Tooltip, Filler,
  type ChartOptions,
} from 'chart.js'
import type { RankingsSnapshot } from '@/api/rankings'
import { buildHistoryChart, vendorColor } from '@/features/rankings/chartData'

ChartJS.register(ArcElement, BarElement, CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Filler)

const props = defineProps<{ snapshot: RankingsSnapshot; kind: 'models' | 'share' | 'vendors'; dark: boolean; label: string }>()
const { t, locale } = useI18n()
const compact = (value: number) => new Intl.NumberFormat(locale.value, { notation: 'compact', maximumFractionDigits: 1 }).format(value)
const exact = (value: number) => new Intl.NumberFormat(locale.value).format(value)
const foreground = computed(() => props.dark ? '#b8c2cc' : '#6b7280')
const grid = computed(() => props.dark ? 'rgba(255,255,255,.07)' : 'rgba(15,23,42,.06)')
const history = computed(() => buildHistoryChart(
  props.kind === 'models' ? props.snapshot.models_history.points : props.snapshot.vendor_share_history.points,
  props.kind === 'models' ? 'models' : 'vendors', t('rankings.other'),
))
const historyData = computed(() => ({
  labels: history.value.labels,
  datasets: history.value.series.map((item) => ({
    label: item.label,
    data: item.values,
    backgroundColor: props.kind === 'models' ? item.color : `${item.color}c9`,
    borderColor: item.color,
    borderWidth: props.kind === 'models' ? 0 : 1.5,
    borderRadius: props.kind === 'models' ? 2 : 0,
    pointRadius: 0,
    pointHitRadius: 10,
    fill: true,
    tension: 0.2,
    stack: 'usage',
  })),
}))
const shareData = computed(() => ({
  labels: props.snapshot.vendors.map((vendor) => vendor.vendor_id === 'others' ? t('rankings.other') : vendor.vendor),
  datasets: [{
    data: props.snapshot.vendors.map((vendor) => vendor.total_tokens),
    backgroundColor: props.snapshot.vendors.map((vendor, index) => vendorColor(vendor.vendor_id, index)),
    borderWidth: 2,
    borderColor: props.dark ? '#23272e' : '#fff',
    hoverOffset: 3,
  }],
}))
const commonOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  // Statistics should appear immediately, including for reduced-motion users.
  animation: false as const,
  interaction: { mode: 'index' as const, intersect: false },
  plugins: {
    legend: { display: false },
    tooltip: {
      padding: 11,
      backgroundColor: props.dark ? '#e4e8ed' : '#17232c',
      titleColor: props.dark ? '#17232c' : '#fff',
      bodyColor: props.dark ? '#334155' : '#e2e8f0',
      cornerRadius: 9,
    },
  },
}))
const barOptions = computed<ChartOptions<'bar'>>(() => ({
  ...commonOptions.value,
  plugins: {
    ...commonOptions.value.plugins,
    tooltip: {
      ...commonOptions.value.plugins.tooltip,
      callbacks: { label: (context) => `${context.dataset.label}: ${exact(context.parsed.y || 0)}` },
    },
  },
  scales: {
    x: { stacked: true, grid: { display: false }, border: { display: false }, ticks: { color: foreground.value, maxRotation: 0, maxTicksLimit: 6, font: { size: 10 } } },
    y: { stacked: true, beginAtZero: true, border: { display: false }, grid: { color: grid.value }, ticks: { color: foreground.value, maxTicksLimit: 5, callback: (value) => compact(Number(value)), font: { size: 10 } } },
  },
}))
const lineOptions = computed<ChartOptions<'line'>>(() => ({
  ...commonOptions.value,
  plugins: {
    ...commonOptions.value.plugins,
    tooltip: {
      ...commonOptions.value.plugins.tooltip,
      callbacks: { label: (context) => `${context.dataset.label}: ${(context.parsed.y || 0).toFixed(1)}%` },
    },
  },
  scales: {
    x: { grid: { display: false }, border: { display: false }, ticks: { color: foreground.value, maxRotation: 0, maxTicksLimit: 8, font: { size: 10 } } },
    y: { stacked: true, beginAtZero: true, max: 100, border: { display: false }, grid: { color: grid.value }, ticks: { color: foreground.value, stepSize: 25, callback: (value) => `${value}%`, font: { size: 10 } } },
  },
}))
const shareOptions = computed<ChartOptions<'doughnut'>>(() => ({
  ...commonOptions.value,
  cutout: '76%',
  interaction: { mode: 'nearest', intersect: true },
  plugins: {
    ...commonOptions.value.plugins,
    tooltip: {
      ...commonOptions.value.plugins.tooltip,
      callbacks: { label: (context) => `${context.label}: ${exact(context.parsed)} (${props.snapshot.total_tokens > 0 ? (context.parsed / props.snapshot.total_tokens * 100).toFixed(1) : '0'}%)` },
    },
  },
}))
</script>

<style scoped>
.rankings-chart { position: relative; height: 14rem; min-width: 0; }
.rankings-chart--donut { height: 10rem; width: 10rem; margin: 0 auto; }
.rankings-chart__empty { display: grid; height: 100%; place-items: center; color: var(--ranking-muted, #6b7280); font-size: .875rem; }
.rankings-chart__legend { display: flex; flex-wrap: wrap; gap: .625rem 1rem; margin: 1rem .25rem 0; color: var(--ranking-muted, #6b7280); font-size: .6875rem; }
.rankings-chart__legend span { display: inline-flex; min-width: 0; max-width: 100%; align-items: center; gap: .375rem; overflow-wrap: anywhere; }
.rankings-chart__legend i { flex-shrink: 0; width: .5rem; height: .5rem; border-radius: 2px; }
@media (max-width: 40rem) { .rankings-chart { height: 12.5rem; } .rankings-chart--donut { height: 9rem; width: 9rem; } }
</style>
