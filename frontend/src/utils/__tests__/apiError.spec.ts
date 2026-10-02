import { describe, expect, it } from 'vitest'
import {
  extractApiErrorCode,
  extractApiErrorMessage,
  extractI18nErrorMessage,
} from '@/utils/apiError'

// The API envelope for a domain failure carries BOTH a numeric HTTP status in
// `code` and a semantic identifier in `reason`:
//   { code: 400, message: "amount out of range", reason: "INVALID_AMOUNT", metadata: {...} }
// Callers key i18n lookups on the semantic identifier, so resolving `code`
// first would surface "400" and silently defeat every message mapping.

describe('extractApiErrorCode', () => {
  it('prefers the semantic reason over the numeric HTTP status', () => {
    expect(extractApiErrorCode({
      status: 400,
      code: 400,
      reason: 'INVALID_AMOUNT',
      message: 'amount out of range',
    })).toBe('INVALID_AMOUNT')
  })

  it('returns a string code when no reason is present', () => {
    expect(extractApiErrorCode({ status: 423, code: 'ADMIN_COMPLIANCE_ACK_REQUIRED' }))
      .toBe('ADMIN_COMPLIANCE_ACK_REQUIRED')
  })

  it('skips a numeric code and falls through to the envelope code', () => {
    expect(extractApiErrorCode({ status: 400, code: 400, response: { data: { code: 'FALLBACK_CODE' } } }))
      .toBe('FALLBACK_CODE')
  })

  it('still returns the numeric status when no semantic identifier exists', () => {
    expect(extractApiErrorCode({ status: 502, code: 502 })).toBe('502')
  })

  it('ignores blank and whitespace-only identifiers', () => {
    expect(extractApiErrorCode({ status: 400, code: 400, reason: '   ' })).toBe('400')
  })

  it('returns undefined for non-object errors', () => {
    expect(extractApiErrorCode(null)).toBeUndefined()
    expect(extractApiErrorCode('boom')).toBeUndefined()
    expect(extractApiErrorCode(undefined)).toBeUndefined()
  })
})

describe('extractApiErrorMessage', () => {
  it('maps the reason-derived code through the i18n map', () => {
    const message = extractApiErrorMessage(
      { status: 400, code: 400, reason: 'INVALID_AMOUNT', message: 'amount out of range' },
      'fallback',
      { INVALID_AMOUNT: '金额无效' },
    )
    expect(message).toBe('金额无效')
  })

  it('falls back to the raw message when the code is unmapped', () => {
    const message = extractApiErrorMessage(
      { status: 400, code: 400, reason: 'SOMETHING_ELSE', message: 'amount out of range' },
      'fallback',
      { INVALID_AMOUNT: '金额无效' },
    )
    expect(message).toBe('amount out of range')
  })

  it('prefers the mapped message over the raw backend text', () => {
    const message = extractApiErrorMessage(
      { status: 403, code: 'BALANCE_PAYMENT_DISABLED', message: 'balance recharge has been disabled' },
      'fallback',
      { BALANCE_PAYMENT_DISABLED: '余额充值功能已关闭' },
    )
    expect(message).toBe('余额充值功能已关闭')
  })
})

describe('extractI18nErrorMessage', () => {
  it('resolves a namespaced key from the reason and interpolates metadata', () => {
    const translator = (key: string, params?: Record<string, unknown>) =>
      key === 'payment.errors.INVALID_AMOUNT' ? `range ${params?.min}-${params?.max}` : key

    const message = extractI18nErrorMessage(
      {
        status: 400,
        code: 400,
        reason: 'INVALID_AMOUNT',
        message: 'amount out of range',
        metadata: { min: '10.00', max: '10000.00' },
      },
      translator,
      'payment.errors',
      'fallback',
    )
    expect(message).toBe('range 10.00-10000.00')
  })

  it('falls back to the raw message when the namespaced key is missing', () => {
    const translator = (key: string) => key // unresolved key echoes back

    const message = extractI18nErrorMessage(
      { status: 400, code: 400, reason: 'UNKNOWN_REASON', message: 'raw backend text' },
      translator,
      'payment.errors',
      'fallback',
    )
    expect(message).toBe('raw backend text')
  })
})
