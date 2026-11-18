import { describe, expect, it } from 'vitest'

import {
  isSmsTemplateId,
  normalizeSmsPhone,
  parseSmsPhoneLines,
  smsErrorMessage,
  smsSuggestedTemplates,
  smsTemplateKinds,
  smsTemplateParamCount
} from '../sms'

describe('normalizeSmsPhone', () => {
  it('accepts mainland numbers in the usual spellings', () => {
    for (const raw of ['13800138000', ' 138 0013 8000 ', '138-0013-8000', '+8613800138000', '+86 138 0013 8000', '008613800138000', '8613800138000']) {
      expect(normalizeSmsPhone(raw), raw).toBe('13800138000')
    }
  })

  it('rejects everything else', () => {
    for (const raw of ['', '1380013800', '138001380001', '12800138000', '+12025550123', 'abc', '+85291234567']) {
      expect(normalizeSmsPhone(raw), raw).toBeNull()
    }
  })
})

describe('parseSmsPhoneLines', () => {
  it('splits, normalizes and deduplicates, reporting what is not a number', () => {
    expect(parseSmsPhoneLines('13800138000\n\n+86 138 0013 8000, 13900139000；12345\n')).toEqual({
      phones: ['13800138000', '13900139000'],
      invalid: ['12345']
    })
    expect(parseSmsPhoneLines('  ')).toEqual({ phones: [], invalid: [] })
  })
})

describe('templates', () => {
  it('accepts numeric template IDs or none', () => {
    expect(isSmsTemplateId('')).toBe(true)
    expect(isSmsTemplateId(' 1234567 ')).toBe(true)
    expect(isSmsTemplateId('SMS_1234')).toBe(false)
    expect(isSmsTemplateId('1'.repeat(21))).toBe(false)
  })

  it('suggests a template with the variables the backend fills, in order', () => {
    expect(smsTemplateKinds).toEqual(['verify_code', 'balance_low', 'subscription_expiry', 'ops_alert', 'account_quota'])
    for (const kind of smsTemplateKinds) {
      const numbers = [...smsSuggestedTemplates[kind].matchAll(/\{(\d+)\}/g)].map((match) => Number(match[1]))
      expect(numbers, kind).toEqual(Array.from({ length: smsTemplateParamCount[kind] }, (_, i) => i + 1))
    }
  })
})

describe('smsErrorMessage', () => {
  const mt = (key: string, params?: Record<string, unknown>) => (params ? `${key} ${JSON.stringify(params)}` : key)

  it('maps the SMS reason codes', () => {
    expect(smsErrorMessage({ reason: 'MOK_SMS_CODE_INVALID', message: 'x' }, mt, 'fallback')).toBe('sms.errors.MOK_SMS_CODE_INVALID')
    expect(smsErrorMessage({ reason: 'MOK_SMS_CODE_COOLDOWN', metadata: { retry_after: '42' } }, mt, 'fallback')).toBe(
      'sms.errors.MOK_SMS_CODE_COOLDOWN {"seconds":42}'
    )
    expect(smsErrorMessage({ reason: 'MOK_SMS_CODE_COOLDOWN' }, mt, 'fallback')).toBe('sms.errors.MOK_SMS_CODE_COOLDOWN {"seconds":60}')
  })

  it('falls back to the backend message', () => {
    expect(smsErrorMessage({ reason: 'MOK_SMS_CONFIG_INVALID', message: 'unsupported region' }, mt, 'fallback')).toBe('unsupported region')
    expect(smsErrorMessage(null, mt, 'fallback')).toBe('fallback')
  })
})
