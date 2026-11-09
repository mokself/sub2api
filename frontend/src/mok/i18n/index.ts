import { watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { i18n } from '@/i18n'
import affiliateUsageWording from './affiliateUsageWording'
import en from './en'
import zh from './zh'

export const mokMessages: Record<string, Record<string, any>> = { en, zh }

let affiliateUsageWordingOn = false
// Per locale, the upstream sentences that the usage wording replaced, to put them back.
const replacedUpstreamWording: Record<string, Record<string, any>> = {}

/** Full key of a fork message: mokKey('claudePlan.column') is 'mok.claudePlan.column'. */
export function mokKey(key: string): string {
  return `mok.${key}`
}

/** The values of `messages` at the leaves of `shape`. */
function pickLeaves(messages: unknown, shape: Record<string, any>): Record<string, any> {
  const source = (messages ?? {}) as Record<string, any>
  return Object.fromEntries(
    Object.entries(shape).map(([key, child]) => [
      key,
      child !== null && typeof child === 'object' ? pickLeaves(source[key], child) : source[key]
    ])
  )
}

function applyUsageWording(locale: string): void {
  const wording = affiliateUsageWording[locale]
  if (!wording) return
  const current = pickLeaves(i18n.global.getLocaleMessage(locale), wording)
  // A locale that is still reworded from an earlier merge keeps the upstream sentences saved then.
  if (JSON.stringify(current) !== JSON.stringify(wording)) {
    replacedUpstreamWording[locale] = current
  }
  i18n.global.mergeLocaleMessage(locale, wording)
}

function mergeMokMessages(locale: string): void {
  const messages = mokMessages[locale]
  if (messages) {
    i18n.global.mergeLocaleMessage(locale, messages)
  }
  if (affiliateUsageWordingOn) {
    applyUsageWording(locale)
  }
}

/**
 * Rewords the upstream affiliate page sentences about recharge rebates for usage rebates
 * (./affiliateUsageWording.ts), or puts the upstream sentences back.
 */
export function setAffiliateUsageWording(on: boolean): void {
  if (on === affiliateUsageWordingOn) return
  affiliateUsageWordingOn = on
  if (on) {
    applyUsageWording(i18n.global.locale.value)
    return
  }
  for (const [locale, upstream] of Object.entries(replacedUpstreamWording)) {
    i18n.global.mergeLocaleMessage(locale, upstream)
    delete replacedUpstreamWording[locale]
  }
}

/**
 * Merges the fork's messages into vue-i18n. Upstream loads each locale lazily with
 * setLocaleMessage, which replaces the whole locale, so the merge runs once the current
 * locale is loaded (installMok runs after initI18n) and again after every locale switch.
 */
export function installMokI18n(): void {
  mergeMokMessages(i18n.global.locale.value)
  watch(() => i18n.global.locale.value, mergeMokMessages)
}

/**
 * Translates fork messages: mt('claudePlan.column') reads 'mok.claudePlan.column'.
 * Fork code calls mt() instead of passing "mok." keys to the upstream translate function,
 * because upstream's locale completeness test scans every source file (comments included)
 * for such calls and only knows upstream locale files. src/mok/__tests__/i18n.spec.ts
 * checks the fork's keys instead.
 */
export function useMokI18n() {
  const { t } = useI18n()
  return {
    mt: (key: string, named: Record<string, unknown> = {}) => t(mokKey(key), named)
  }
}
