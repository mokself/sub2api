import { describe, expect, it, vi } from 'vitest'

import { formatRebatePeriod, formatRebateRate } from '../usageRebate'

vi.mock('@/utils/format', () => ({
  formatDateTime: (value: string, options?: Intl.DateTimeFormatOptions) =>
    `time(${value.slice(11, 16)},${Object.keys(options ?? {}).join('+')})`,
  formatDateTimeToMinute: (value: string) => `minute(${value.slice(0, 16)})`
}))

describe('formatRebatePeriod', () => {
  it('shows the end of an hour by its time only', () => {
    expect(formatRebatePeriod('2026-11-01T11:00:00+08:00', '2026-11-01T12:00:00+08:00')).toBe(
      'minute(2026-11-01T11:00) – time(12:00,hour+minute+hour12)'
    )
  })

  it('does the same for the shorter period around a mode switch', () => {
    expect(formatRebatePeriod('2026-11-01T10:20:00+08:00', '2026-11-01T11:00:00+08:00')).toBe(
      'minute(2026-11-01T10:20) – time(11:00,hour+minute+hour12)'
    )
  })

  it('shows both dates for a day carried over from the daily settlement', () => {
    expect(formatRebatePeriod('2026-11-01T10:00:00+08:00', '2026-11-02T00:00:00+08:00')).toBe(
      'minute(2026-11-01T10:00) – minute(2026-11-02T00:00)'
    )
  })
})

describe('formatRebateRate', () => {
  it.each([
    [20, '20'],
    [12.5, '12.5'],
    [12.345, '12.35'],
    [0, '0']
  ])('formats %s as %s', (rate, text) => {
    expect(formatRebateRate(rate)).toBe(text)
  })
})
