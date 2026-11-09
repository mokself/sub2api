import type { AffiliateRebateTier } from '../api/affiliateTiers'

// Mirrors NormalizeMokAffiliateRebateTiers in backend/internal/service/affiliate_service_mok.go.
export const MAX_AFFILIATE_REBATE_TIERS = 20
const RATE_MIN = 0
const RATE_MAX = 100

/**
 * An editable row. v-model on a number input yields a number once the text parses and
 * leaves it a string otherwise (empty or half-typed), so both are accepted.
 */
export interface AffiliateRebateTierDraft {
  min_amount: string | number
  rate_percent: string | number
}

export type AffiliateTierRowError = 'minAmountInvalid' | 'rateInvalid' | 'minAmountDuplicate'

export interface AffiliateTierValidation {
  /** The tiers to save, or null while any row is invalid. */
  tiers: AffiliateRebateTier[] | null
  errors: (AffiliateTierRowError | null)[]
}

function round2(value: number): number {
  return Math.round(value * 100) / 100
}

function parseNumber(raw: string | number): number | null {
  if (typeof raw === 'number') return Number.isFinite(raw) ? raw : null
  const text = raw.trim()
  if (text === '') return null
  const value = Number(text)
  return Number.isFinite(value) ? value : null
}

export function toDrafts(tiers: AffiliateRebateTier[]): AffiliateRebateTierDraft[] {
  return tiers.map((tier) => ({ min_amount: String(tier.min_amount), rate_percent: String(tier.rate_percent) }))
}

export function validateDrafts(drafts: AffiliateRebateTierDraft[]): AffiliateTierValidation {
  const errors: (AffiliateTierRowError | null)[] = []
  const tiers: AffiliateRebateTier[] = []
  const seenAmounts = new Set<number>()
  drafts.forEach((draft) => {
    const minAmount = parseNumber(draft.min_amount)
    const rate = parseNumber(draft.rate_percent)
    if (minAmount === null || round2(minAmount) <= 0) {
      errors.push('minAmountInvalid')
      return
    }
    if (rate === null || rate < RATE_MIN || rate > RATE_MAX) {
      errors.push('rateInvalid')
      return
    }
    const rounded = round2(minAmount)
    if (seenAmounts.has(rounded)) {
      errors.push('minAmountDuplicate')
      return
    }
    seenAmounts.add(rounded)
    errors.push(null)
    tiers.push({ min_amount: rounded, rate_percent: round2(rate) })
  })
  const valid = errors.every((error) => error === null)
  return { tiers: valid ? tiers.sort((a, b) => a.min_amount - b.min_amount) : null, errors }
}

/** Compares saved tiers with the drafts as they would be saved, ignoring row order. */
export function sameTiers(saved: AffiliateRebateTier[], drafts: AffiliateRebateTierDraft[]): boolean {
  const { tiers } = validateDrafts(drafts)
  if (!tiers || tiers.length !== saved.length) return false
  const sortedSaved = [...saved].sort((a, b) => a.min_amount - b.min_amount)
  return tiers.every((tier, i) => tier.min_amount === sortedSaved[i].min_amount && tier.rate_percent === sortedSaved[i].rate_percent)
}

/** Progress from the start of the current tier towards the next one, in percent (0-100). */
export function progressToNextTier(volume: number, tiers: AffiliateRebateTier[], nextTier?: AffiliateRebateTier): number {
  if (!nextTier) return 100
  const reached = tiers.filter((tier) => tier.min_amount <= volume)
  const start = reached.length > 0 ? reached[reached.length - 1].min_amount : 0
  const span = nextTier.min_amount - start
  if (span <= 0) return 100
  return Math.max(0, Math.min(100, ((volume - start) / span) * 100))
}
