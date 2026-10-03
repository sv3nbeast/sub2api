import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import AmountInput from '../AmountInput.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
enableAutoUnmount(afterEach)

function mountInput(value: number | null = null) {
  return mount(AmountInput, { props: { modelValue: value } })
}

describe('recharge amount input', () => {
  it.each(['10abc', '10.555', '-10', '1e2'])('restores the accepted amount after rejecting %s', async (value) => {
    const wrapper = mountInput(10)
    const input = wrapper.get('input')
    await input.setValue(value)
    expect((input.element as HTMLInputElement).value).toBe('10')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('restores the last typed amount rather than a stale prop', async () => {
    const wrapper = mountInput()
    const input = wrapper.get('input')
    await input.setValue('12.50')
    await input.setValue('12.500')
    expect((input.element as HTMLInputElement).value).toBe('12.50')
    expect(wrapper.emitted('update:modelValue')).toEqual([[12.5]])
  })

  it('preserves decimal editing and allows clearing the amount', async () => {
    const wrapper = mountInput()
    const input = wrapper.get('input')
    for (const value of ['0', '0.', '0.5', '0.50', '']) await input.setValue(value)
    expect(wrapper.emitted('update:modelValue')).toEqual([[null], [null], [0.5], [0.5], [null]])
    expect((input.element as HTMLInputElement).value).toBe('')
  })
})

describe('minimum amount floor', () => {
  // The floor is applied on blur, not on input: clamping during typing would
  // make a value like "100" impossible to enter, since it passes through "1".
  it('leaves a below-minimum value alone while the user is still typing', async () => {
    const wrapper = mount(AmountInput, { props: { modelValue: null, min: 10 } })
    const input = wrapper.get('input')
    await input.setValue('1')
    expect((input.element as HTMLInputElement).value).toBe('1')
  })

  it('raises a below-minimum value to the floor on blur', async () => {
    const wrapper = mount(AmountInput, { props: { modelValue: null, min: 10 } })
    const input = wrapper.get('input')
    await input.setValue('1')
    await input.trigger('blur')
    expect((input.element as HTMLInputElement).value).toBe('10')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([10])
  })

  it('leaves an amount at or above the floor untouched on blur', async () => {
    const wrapper = mount(AmountInput, { props: { modelValue: null, min: 10 } })
    const input = wrapper.get('input')
    await input.setValue('25')
    await input.trigger('blur')
    expect((input.element as HTMLInputElement).value).toBe('25')
    expect(wrapper.emitted('update:modelValue')).toEqual([[25]])
  })

  it('does not clamp when no floor is configured', async () => {
    const wrapper = mountInput()
    const input = wrapper.get('input')
    await input.setValue('1')
    await input.trigger('blur')
    expect((input.element as HTMLInputElement).value).toBe('1')
  })

  it('does not invent a floor for an empty field', async () => {
    const wrapper = mount(AmountInput, { props: { modelValue: null, min: 10 } })
    const input = wrapper.get('input')
    await input.setValue('')
    await input.trigger('blur')
    expect((input.element as HTMLInputElement).value).toBe('')
    expect(wrapper.emitted('update:modelValue')).toEqual([[null]])
  })
})
