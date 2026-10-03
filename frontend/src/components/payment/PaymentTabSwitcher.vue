<template>
  <!-- A segmented control rather than a pair of buttons. The options are modes
       of this page, not actions: drawn as a filled dark button, the active tab
       matched the banner's "立即充值" action right above it, so the two read as
       the same kind of control. A raised thumb on a recessed track is the
       conventional "pick one of these" signal and stays visually subordinate
       to the real primary action. -->
  <div
    role="tablist"
    class="relative inline-grid w-full rounded-xl bg-gray-900/[0.06] p-1 ring-1 ring-inset ring-gray-900/[0.04] dark:bg-white/[0.06] dark:ring-white/[0.06] sm:w-auto"
    :style="{ gridTemplateColumns: `repeat(${tabs.length}, minmax(0, 1fr))` }"
  >
    <!-- One thumb that slides, instead of per-button backgrounds that swap: the
         movement shows where the selection went. Columns are equal width, so a
         translate of the thumb's own width lands exactly on the next option. -->
    <span
      aria-hidden="true"
      class="pointer-events-none absolute inset-y-1 left-1 rounded-lg bg-white shadow-sm ring-1 ring-gray-900/5 transition-transform duration-200 ease-out motion-reduce:transition-none dark:bg-dark-600 dark:ring-white/10"
      :style="thumbStyle"
    />
    <button
      v-for="tab in tabs"
      :key="tab.key"
      type="button"
      role="tab"
      :aria-selected="tab.key === modelValue"
      :class="[
        'relative inline-flex items-center justify-center gap-2 rounded-lg px-4 py-2 text-sm font-semibold transition-colors sm:min-w-[7.5rem]',
        tab.key === modelValue
          ? 'text-gray-950 dark:text-white'
          : 'text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200',
      ]"
      @click="emit('update:modelValue', tab.key)"
    >
      <Icon :name="TAB_ICONS[tab.key]" size="sm" />
      {{ tab.label }}
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import Icon from '@/components/icons/Icon.vue'

export type PaymentTabKey = 'recharge' | 'subscription'

export interface PaymentTab {
  key: PaymentTabKey
  label: string
}

const props = defineProps<{
  tabs: PaymentTab[]
  modelValue: PaymentTabKey
}>()

const emit = defineEmits<{
  'update:modelValue': [key: PaymentTabKey]
}>()

// Recharge tops up a balance, subscription buys a period. The credit-card icon
// is deliberately not used for recharge: the sidebar already uses it for
// "my subscriptions", and reusing it here would point at the wrong concept.
const TAB_ICONS = {
  recharge: 'dollar',
  subscription: 'calendar',
} as const

const activeIndex = computed(() => Math.max(0, props.tabs.findIndex((tab) => tab.key === props.modelValue)))

// 0.5rem = the track's padding on both sides (p-1), shared by every column.
const thumbStyle = computed(() => ({
  width: `calc((100% - 0.5rem) / ${props.tabs.length})`,
  transform: `translateX(${activeIndex.value * 100}%)`,
}))
</script>
