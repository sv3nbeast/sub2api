import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import PaymentTabSwitcher, { type PaymentTab } from '@/components/payment/PaymentTabSwitcher.vue'

const TABS: PaymentTab[] = [
  { key: 'recharge', label: 'Top Up' },
  { key: 'subscription', label: 'Subscribe' },
]

const mountSwitcher = (modelValue: PaymentTab['key']) =>
  mount(PaymentTabSwitcher, { props: { tabs: TABS, modelValue } })

describe('PaymentTabSwitcher', () => {
  it('marks only the active tab as selected', () => {
    const tabs = mountSwitcher('subscription').findAll('[role="tab"]')

    expect(tabs.map((tab) => tab.text())).toEqual(['Top Up', 'Subscribe'])
    expect(tabs.map((tab) => tab.attributes('aria-selected'))).toEqual(['false', 'true'])
  })

  it('asks the parent to switch instead of switching itself', async () => {
    const wrapper = mountSwitcher('recharge')

    await wrapper.findAll('[role="tab"]')[1].trigger('click')

    expect(wrapper.emitted('update:modelValue')).toEqual([['subscription']])
    // Still controlled: nothing moves until the parent passes the new value.
    expect(wrapper.findAll('[role="tab"]')[0].attributes('aria-selected')).toBe('true')
  })

  // The thumb is sized to one column and moved by whole multiples of its own
  // width; if either half drifts, it stops lining up with the selected option.
  it('sizes the thumb to one column and slides it to the active one', async () => {
    const wrapper = mountSwitcher('recharge')
    const thumb = () => wrapper.get('[aria-hidden="true"]').attributes('style')

    // jsdom normalizes `calc((100% - 0.5rem) / 2)` to `calc(0.5 * (100% - 0.5rem))`;
    // both mean "half of the track minus its padding".
    expect(thumb()).toMatch(/width: calc\((\(100% - 0\.5rem\) \/ 2|0\.5 \* \(100% - 0\.5rem\))\)/)
    expect(thumb()).toContain('translateX(0%)')

    await wrapper.setProps({ modelValue: 'subscription' })

    expect(thumb()).toContain('translateX(100%)')
  })
})
