import { describe, expect, it } from 'vitest'
import { progressToNextTier, sameTiers, toDrafts, validateDrafts } from '../tiers'

describe('validateDrafts', () => {
  it('rounds, sorts and accepts both strings and numbers', () => {
    const result = validateDrafts([
      { min_amount: '100000', rate_percent: 15 },
      { min_amount: 50000.004, rate_percent: '12.345' }
    ])
    expect(result.errors).toEqual([null, null])
    expect(result.tiers).toEqual([
      { min_amount: 50000, rate_percent: 12.35 },
      { min_amount: 100000, rate_percent: 15 }
    ])
  })

  it.each([
    [{ min_amount: '', rate_percent: '10' }, 'minAmountInvalid'],
    [{ min_amount: '0', rate_percent: '10' }, 'minAmountInvalid'],
    [{ min_amount: '0.004', rate_percent: '10' }, 'minAmountInvalid'],
    [{ min_amount: '-5', rate_percent: '10' }, 'minAmountInvalid'],
    [{ min_amount: '100', rate_percent: '' }, 'rateInvalid'],
    [{ min_amount: '100', rate_percent: '100.01' }, 'rateInvalid'],
    [{ min_amount: '100', rate_percent: '-1' }, 'rateInvalid']
  ])('rejects %j', (draft, error) => {
    const result = validateDrafts([draft])
    expect(result.tiers).toBeNull()
    expect(result.errors).toEqual([error])
  })

  it('flags a threshold used twice after rounding', () => {
    const result = validateDrafts([
      { min_amount: '500', rate_percent: '5' },
      { min_amount: '500.001', rate_percent: '6' }
    ])
    expect(result.tiers).toBeNull()
    expect(result.errors).toEqual([null, 'minAmountDuplicate'])
  })

  it('accepts an empty list, which disables the tiers', () => {
    expect(validateDrafts([])).toEqual({ tiers: [], errors: [] })
  })
})

describe('sameTiers', () => {
  const saved = [
    { min_amount: 50000, rate_percent: 12 },
    { min_amount: 100000, rate_percent: 15 }
  ]

  it('ignores row order and number formatting', () => {
    expect(sameTiers(saved, toDrafts([...saved].reverse()))).toBe(true)
    expect(sameTiers(saved, [{ min_amount: '50000.00', rate_percent: '12' }, { min_amount: 100000, rate_percent: 15 }])).toBe(true)
  })

  it('detects changes and invalid drafts', () => {
    expect(sameTiers(saved, [{ min_amount: 50000, rate_percent: 13 }, { min_amount: 100000, rate_percent: 15 }])).toBe(false)
    expect(sameTiers(saved, toDrafts(saved).slice(1))).toBe(false)
    expect(sameTiers(saved, [...toDrafts(saved), { min_amount: '', rate_percent: '' }])).toBe(false)
  })
})

describe('progressToNextTier', () => {
  const tiers = [
    { min_amount: 50000, rate_percent: 12 },
    { min_amount: 100000, rate_percent: 15 }
  ]

  it('measures progress from the current tier to the next', () => {
    expect(progressToNextTier(0, tiers, tiers[0])).toBe(0)
    expect(progressToNextTier(25000, tiers, tiers[0])).toBe(50)
    expect(progressToNextTier(75000, tiers, tiers[1])).toBe(50)
  })

  it('is full at the top tier', () => {
    expect(progressToNextTier(150000, tiers, undefined)).toBe(100)
  })
})
