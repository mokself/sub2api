import { formatDateTime, formatDateTimeToMinute } from '@/utils/format'

const HOUR_MS = 60 * 60 * 1000

/**
 * A settlement period as "2026/11/01 11:00 – 12:00". Periods are an hour or shorter; a
 * longer one is a day carried over from the earlier daily settlement and shows both dates.
 */
export function formatRebatePeriod(start: string, end: string): string {
  const span = new Date(end).getTime() - new Date(start).getTime()
  const to =
    span > HOUR_MS
      ? formatDateTimeToMinute(end)
      : formatDateTime(end, { hour: '2-digit', minute: '2-digit', hour12: false })
  return `${formatDateTimeToMinute(start)} – ${to}`
}

/** Same formatting as the rebate rate card of the affiliate page: 20.00 -> "20", 12.50 -> "12.5". */
export function formatRebateRate(rate: number): string {
  return String(Math.round(rate * 100) / 100)
}
