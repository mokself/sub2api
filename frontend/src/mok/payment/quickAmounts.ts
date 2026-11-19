import { ref, type App } from 'vue'
import { getPaymentQuickAmounts } from '../api/paymentQuickAmounts'

/**
 * The recharge page's quick amount buttons. Upstream hard-codes them in
 * views/user/PaymentView.vue; a [mok] seam there renders
 * `$mokQuickAmounts([10, 20, ...])` instead, which returns the admin's list
 * (backend internal/mok/payment_quick_amounts.go) or upstream's when none is set.
 */

/** Upstream's buttons, shown while the admin has not set a list. */
export const upstreamQuickAmounts = [10, 20, 50, 100, 200, 500, 1000, 2000, 5000]

export const quickAmountsMax = 12
const maxAmount = 1_000_000
const storageKey = 'mok_payment_quick_amounts'

function readCache(): number[] | null {
  try {
    const parsed = JSON.parse(localStorage.getItem(storageKey) ?? 'null')
    return Array.isArray(parsed) && parsed.every((n) => typeof n === 'number' && n > 0) ? parsed : null
  } catch {
    return null
  }
}

// The last list seen, kept in localStorage so the page does not show upstream's buttons
// for a moment before the request returns.
const amounts = ref<number[] | null>(readCache())
let requested = false

async function load(): Promise<void> {
  try {
    setQuickAmounts((await getPaymentQuickAmounts()).amounts ?? [])
  } catch (error) {
    // Keep what is shown; the next page load asks again.
    console.error('Failed to load the quick amounts:', error)
  }
}

/** Remembers a list the admin just saved, or the one the server returned. */
export function setQuickAmounts(list: number[]): void {
  amounts.value = list.length > 0 ? [...list] : null
  try {
    if (amounts.value) localStorage.setItem(storageKey, JSON.stringify(amounts.value))
    else localStorage.removeItem(storageKey)
  } catch {
    // Storage unavailable: the page still works, it just asks every time.
  }
}

/**
 * The amounts the recharge page shows. Called while rendering, so the first call fetches
 * the list once per page load; the page re-renders when it arrives.
 */
export function mokQuickAmounts(defaults: number[]): number[] {
  const list = amounts.value
  if (!requested) {
    requested = true
    void load()
  }
  return list && list.length > 0 ? list : defaults
}

export function installPaymentQuickAmounts(app: App): void {
  app.config.globalProperties.$mokQuickAmounts = mokQuickAmounts
}

export type QuickAmountsParse =
  | { ok: true; amounts: number[] }
  | { ok: false; error: 'invalid' | 'range' | 'decimals' | 'tooMany'; value: string }

/**
 * Parses the settings field: amounts separated by commas or spaces, returned
 * deduplicated and in ascending order, as the backend stores them.
 */
export function parseQuickAmounts(text: string): QuickAmountsParse {
  const out: number[] = []
  for (const part of text.split(/[\s,，、;；]+/)) {
    if (!part) continue
    if (!/^\d+(\.\d+)?$/.test(part)) return { ok: false, error: 'invalid', value: part }
    const value = Number(part)
    if (!(value > 0) || value > maxAmount) return { ok: false, error: 'range', value: part }
    if (!/^\d+(\.\d{1,2})?$/.test(part)) return { ok: false, error: 'decimals', value: part }
    if (!out.includes(value)) out.push(value)
  }
  if (out.length > quickAmountsMax) return { ok: false, error: 'tooMany', value: '' }
  return { ok: true, amounts: out.sort((a, b) => a - b) }
}

/** Test hook: forget the loaded list. */
export function resetQuickAmountsForTest(): void {
  amounts.value = readCache()
  requested = false
}
