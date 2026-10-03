<template>
  <div>
    <label :class="styles.label">
      {{ t('payment.paymentMethod') }}
    </label>
    <div
      data-testid="payment-method-grid"
      :class="styles.grid"
    >
      <button
        v-for="method in sortedMethods"
        :key="method.type"
        type="button"
        :title="methodLabel(method)"
        :disabled="!method.available"
        :class="[styles.button, stateClass(method)]"
        @click="method.available && emit('select', method.type)"
      >
        <span :class="styles.inner">
          <img :src="methodIcon(method.type)" :alt="methodLabel(method)" :class="styles.icon" />
          <span :class="styles.text">
            <span data-testid="payment-method-label" :class="styles.name">
              {{ methodLabel(method) }}
            </span>
            <span
              v-if="method.fee_rate > 0"
              :class="styles.fee"
            >
              {{ t('payment.fee') }} {{ method.fee_rate }}%
            </span>
          </span>
        </span>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { METHOD_ORDER, isBuiltInAlipayMethod, isBuiltInWxpayMethod } from './providerConfig'
import alipayIcon from '@/assets/icons/alipay.svg'
import wxpayIcon from '@/assets/icons/wxpay.svg'
import stripeIcon from '@/assets/icons/stripe.svg'
import airwallexIcon from '@/assets/icons/airwallex.svg'
import bscIcon from '@/assets/icons/bsc.svg'
import paymentIcon from '@/assets/icons/payment.svg'

export interface PaymentMethodOption {
  type: string
  display_name?: string
  fee_rate: number
  available: boolean
}

const props = withDefaults(defineProps<{
  methods: PaymentMethodOption[]
  selected: string
  /**
   * 'grid' (default) is the tile grid used on the subscription confirmation.
   * 'row' is the compact, left-aligned row inside the recharge card. There the
   * selection is a neutral emphasis rather than each provider's brand color:
   * the card already ends in a dark primary action, and a second saturated
   * color next to it made the selected method read as the thing to click.
   */
  variant?: 'grid' | 'row'
}>(), {
  variant: 'grid',
})

const emit = defineEmits<{
  select: [type: string]
}>()

const { t } = useI18n()

const VARIANT_STYLES = {
  grid: {
    label: 'mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300',
    grid: 'grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4',
    button: 'relative flex h-[60px] min-w-0 flex-col items-center justify-center rounded-lg border px-3 transition-all',
    inner: 'flex w-full min-w-0 items-center justify-center gap-2',
    icon: 'h-7 w-7 shrink-0 object-contain',
    text: 'flex min-w-0 flex-col items-start leading-none',
    name: 'block w-full truncate text-base font-semibold',
    fee: 'text-[10px] tracking-wide text-gray-500 dark:text-dark-400',
    idle: 'border-gray-300 bg-white text-gray-700 hover:border-gray-400 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-200 dark:hover:border-dark-500',
  },
  row: {
    label: 'mb-2.5 block text-sm font-semibold text-gray-950 dark:text-white',
    grid: 'grid grid-cols-2 gap-2.5 sm:grid-cols-3',
    button: 'relative flex min-w-0 items-center rounded-xl border px-3.5 py-3 text-left transition-colors',
    inner: 'flex w-full min-w-0 items-center gap-2.5',
    icon: 'h-6 w-6 shrink-0 object-contain',
    text: 'flex min-w-0 flex-col items-start leading-tight',
    name: 'block w-full truncate text-sm font-semibold text-gray-950 dark:text-white',
    fee: 'text-[10px] text-gray-400 dark:text-dark-500',
    idle: 'border-gray-200 bg-white hover:border-gray-300 hover:bg-gray-50 dark:border-dark-600 dark:bg-dark-800 dark:hover:border-dark-500',
  },
} as const

const ROW_SELECTED_CLASS = 'border-gray-900 bg-white ring-1 ring-gray-900 dark:border-white dark:bg-dark-800 dark:ring-white'
const DISABLED_CLASS = 'cursor-not-allowed border-gray-200 bg-gray-50 opacity-50 dark:border-dark-700 dark:bg-dark-800/50'

const styles = computed(() => VARIANT_STYLES[props.variant])

const METHOD_ICONS: Record<string, string> = {
  alipay: alipayIcon,
  wxpay: wxpayIcon,
  stripe: stripeIcon,
  airwallex: airwallexIcon,
  credit_card: paymentIcon,
  // USDT on BNB Smart Chain. The chain mark (not the Tether mark) is the right
  // symbol here: this method's identity is "which network you pay on", and the
  // label already spells out the asset in parentheses.
  usdt_bep20: bscIcon,
}

const sortedMethods = computed(() => {
  const order: readonly string[] = METHOD_ORDER
  return [...props.methods].sort((a, b) => {
    const ai = order.indexOf(a.type)
    const bi = order.indexOf(b.type)
    return (ai === -1 ? 999 : ai) - (bi === -1 ? 999 : bi)
  })
})

function methodIcon(type: string): string {
  if (isBuiltInAlipayMethod(type)) return METHOD_ICONS.alipay
  if (isBuiltInWxpayMethod(type)) return METHOD_ICONS.wxpay
  if (type === 'airwallex') return METHOD_ICONS.airwallex
  return METHOD_ICONS[type] || paymentIcon
}

function methodLabel(method: PaymentMethodOption): string {
  return method.display_name || t(`payment.methods.${method.type}`, method.type)
}

function stateClass(method: PaymentMethodOption): string {
  if (!method.available) return DISABLED_CLASS
  if (props.selected !== method.type) return styles.value.idle
  return props.variant === 'row' ? ROW_SELECTED_CLASS : methodSelectedClass(method.type)
}

function methodSelectedClass(type: string): string {
  if (isBuiltInAlipayMethod(type)) return 'border-[#02A9F1] bg-blue-50 text-gray-900 shadow-sm dark:bg-blue-950 dark:text-gray-100'
  if (isBuiltInWxpayMethod(type)) return 'border-[#09BB07] bg-green-50 text-gray-900 shadow-sm dark:bg-green-950 dark:text-gray-100'
  if (type === 'stripe') return 'border-[#676BE5] bg-indigo-50 text-gray-900 shadow-sm dark:bg-indigo-950 dark:text-gray-100'
  if (type === 'airwallex') return 'border-[#FF6B3D] bg-orange-50 text-gray-900 shadow-sm dark:border-[#FF8E3C] dark:bg-orange-950 dark:text-gray-100'
  return 'border-primary-500 bg-primary-50 text-gray-900 shadow-sm dark:bg-primary-950 dark:text-gray-100'
}
</script>
