import { beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'

const { getPaymentQuickAmounts } = vi.hoisted(() => ({ getPaymentQuickAmounts: vi.fn() }))
vi.mock('../../api/paymentQuickAmounts', () => ({ getPaymentQuickAmounts }))

import {
  mokQuickAmounts,
  parseQuickAmounts,
  resetQuickAmountsForTest,
  setQuickAmounts,
  upstreamQuickAmounts
} from '../quickAmounts'

const flush = () => new Promise((resolve) => setTimeout(resolve, 0))

describe('parseQuickAmounts', () => {
  it('reads amounts separated by commas or spaces, deduplicated and ascending', () => {
    expect(parseQuickAmounts(' 100, 30，50 、 9.9  30\n200 ')).toEqual({ ok: true, amounts: [9.9, 30, 50, 100, 200] })
    expect(parseQuickAmounts('')).toEqual({ ok: true, amounts: [] })
  })

  it('reports the first problem', () => {
    expect(parseQuickAmounts('10, abc')).toEqual({ ok: false, error: 'invalid', value: 'abc' })
    expect(parseQuickAmounts('-5')).toEqual({ ok: false, error: 'invalid', value: '-5' })
    expect(parseQuickAmounts('0')).toEqual({ ok: false, error: 'range', value: '0' })
    expect(parseQuickAmounts('1000001')).toEqual({ ok: false, error: 'range', value: '1000001' })
    expect(parseQuickAmounts('9.999')).toEqual({ ok: false, error: 'decimals', value: '9.999' })
    expect(parseQuickAmounts('1 2 3 4 5 6 7 8 9 10 11 12 13')).toEqual({ ok: false, error: 'tooMany', value: '' })
    expect(parseQuickAmounts('1 2 3 4 5 6 7 8 9 10 11 12 12').ok).toBe(true)
  })
})

describe('mokQuickAmounts', () => {
  beforeEach(() => {
    localStorage.clear()
    getPaymentQuickAmounts.mockReset()
    resetQuickAmountsForTest()
  })

  it('returns the defaults until a list arrives, then the list', async () => {
    getPaymentQuickAmounts.mockResolvedValue({ amounts: [30, 88] })

    expect(mokQuickAmounts(upstreamQuickAmounts)).toEqual(upstreamQuickAmounts)
    await flush()
    await nextTick()
    expect(mokQuickAmounts(upstreamQuickAmounts)).toEqual([30, 88])
    expect(getPaymentQuickAmounts).toHaveBeenCalledTimes(1)
    expect(JSON.parse(localStorage.getItem('mok_payment_quick_amounts') ?? 'null')).toEqual([30, 88])
  })

  it('keeps the defaults when none is set or the request fails', async () => {
    getPaymentQuickAmounts.mockResolvedValue({ amounts: [] })
    expect(mokQuickAmounts([1, 2])).toEqual([1, 2])
    await flush()
    expect(mokQuickAmounts([1, 2])).toEqual([1, 2])

    resetQuickAmountsForTest()
    getPaymentQuickAmounts.mockRejectedValue(new Error('down'))
    vi.spyOn(console, 'error').mockImplementation(() => {})
    mokQuickAmounts([1, 2])
    await flush()
    expect(mokQuickAmounts([1, 2])).toEqual([1, 2])
  })

  it('shows the cached list at once and follows what the admin saves', async () => {
    localStorage.setItem('mok_payment_quick_amounts', JSON.stringify([50, 100]))
    resetQuickAmountsForTest()
    getPaymentQuickAmounts.mockReturnValue(new Promise(() => {}))
    expect(mokQuickAmounts(upstreamQuickAmounts)).toEqual([50, 100])

    setQuickAmounts([])
    expect(mokQuickAmounts(upstreamQuickAmounts)).toEqual(upstreamQuickAmounts)
    expect(localStorage.getItem('mok_payment_quick_amounts')).toBeNull()
  })
})
