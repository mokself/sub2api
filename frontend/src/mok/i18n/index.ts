import { watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { i18n } from '@/i18n'
import en from './en'
import zh from './zh'

export const mokMessages: Record<string, Record<string, any>> = { en, zh }

/** Full key of a fork message: mokKey('claudePlan.column') is 'mok.claudePlan.column'. */
export function mokKey(key: string): string {
  return `mok.${key}`
}

function mergeMokMessages(locale: string): void {
  const messages = mokMessages[locale]
  if (messages) {
    i18n.global.mergeLocaleMessage(locale, messages)
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
