<template>
  <div class="space-y-3.5">
    <!-- Quick Amount Buttons -->
    <div>
      <div class="grid grid-cols-2 gap-2 sm:grid-cols-4">
        <button
          v-for="amt in filteredAmounts"
          :key="amt"
          type="button"
          :class="[
            'rounded-xl border px-3 py-2.5 text-left transition-colors',
            modelValue === amt
              ? 'border-gray-900 bg-white ring-1 ring-gray-900 dark:border-white dark:bg-dark-800 dark:ring-white'
              : 'border-gray-200 bg-white hover:border-gray-300 hover:bg-gray-50 dark:border-dark-600 dark:bg-dark-800 dark:hover:border-dark-500',
          ]"
          @click="selectAmount(amt)"
        >
          <span class="block text-base font-bold tabular-nums text-gray-950 dark:text-white">
            {{ formatQuickAmount(amt) }}
          </span>
          <!-- The credited figure is the number the user actually cares about; the
               charged amount alone leaves them converting in their head. -->
          <span v-if="creditRate > 0" class="mt-0.5 block text-xs tabular-nums text-gray-400 dark:text-dark-500">
            {{ t('payment.creditedQuota', { amount: (amt * creditRate).toFixed(2) }) }}
          </span>
        </button>
      </div>
    </div>

    <!-- Custom Amount Input -->
    <div>
      <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
        {{ t('payment.orCustomAmount') }}
      </label>
      <div class="relative">
        <!-- Bare glyph rather than a bordered chip: a boxed symbol inside a boxed
             field reads as two competing containers and never sits on the text
             baseline. -->
        <span class="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-base font-semibold text-gray-400 dark:text-dark-400">
          $
        </span>
        <input
          type="text"
          inputmode="decimal"
          :value="customText"
          :placeholder="placeholderText"
          class="input w-full py-2.5 pl-9 pr-4 text-base font-semibold"
          @input="handleInput"
          @blur="commitAmount"
        />
      </div>
      <p v-if="rangeHint" class="mt-1.5 text-right text-xs text-gray-400 dark:text-dark-500">
        {{ rangeHint }}
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'

const props = withDefaults(defineProps<{
  amounts?: number[]
  modelValue: number | null
  min?: number
  max?: number
  /** USD credited per unit of the displayed currency; 0 hides the credited line. */
  creditRate?: number
  /** Pre-formatted range caption shown under the custom input. */
  rangeHint?: string
}>(), {
  amounts: () => [10, 30, 50, 100],
  min: 0,
  max: 0,
  creditRate: 0,
  rangeHint: '',
})

const emit = defineEmits<{
  'update:modelValue': [value: number | null]
}>()

const { t } = useI18n()

const customText = ref('')

// 0 = no limit
const filteredAmounts = computed(() =>
  props.amounts.filter((a) => (props.min <= 0 || a >= props.min) && (props.max <= 0 || a <= props.max))
)

const placeholderText = computed(() => {
  if (props.min > 0 && props.max > 0) return `${props.min} - ${props.max}`
  if (props.min > 0) return `≥ ${props.min}`
  if (props.max > 0) return `≤ ${props.max}`
  return t('payment.enterAmount')
})

// 0 means "no floor configured"; a real floor is always positive.
const hasMin = computed(() => Number.isFinite(props.min) && props.min > 0)

const AMOUNT_PATTERN = /^\d*(\.\d{0,2})?$/

function selectAmount(amt: number) {
  customText.value = String(amt)
  emit('update:modelValue', amt)
}

// Quick-amount tiles carry the ¥ prefix because the amount is charged in the
// settlement currency, while the credited figure below them is always USD.
function formatQuickAmount(amt: number): string {
  const rounded = Number.isInteger(amt) ? String(amt) : amt.toFixed(2)
  return `¥${rounded}`
}

function handleInput(e: Event) {
  const input = e.target as HTMLInputElement
  const val = input.value
  if (!AMOUNT_PATTERN.test(val)) {
    input.value = customText.value
    return
  }
  customText.value = val
  if (val === '') {
    emit('update:modelValue', null)
    return
  }
  const num = parseFloat(val)
  if (!isNaN(num) && num > 0) {
    emit('update:modelValue', num)
  } else {
    emit('update:modelValue', null)
  }
}

// The floor is enforced on blur rather than on every keystroke: typing "100"
// passes through "1", and clamping mid-entry would make the value impossible to
// finish entering. An out-of-range value is also announced to the parent so the
// page can show the reason instead of silently submitting.
function commitAmount() {
  if (!hasMin.value) return
  const num = parseFloat(customText.value)
  if (isNaN(num) || num <= 0) return
  if (num < props.min) {
    customText.value = String(props.min)
    emit('update:modelValue', props.min)
  }
}

watch(() => props.modelValue, (v) => {
  if (v !== null && String(v) !== customText.value) {
    customText.value = String(v)
  }
}, { immediate: true })
</script>
