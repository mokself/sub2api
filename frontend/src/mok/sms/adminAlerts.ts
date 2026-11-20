import { reactive } from 'vue'
import { i18n } from '@/i18n'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  getSmsAccountQuota,
  getSmsOpsAlert,
  saveSmsAccountQuota,
  saveSmsOpsAlert,
  type SmsAccountQuota,
  type SmsOpsAlert
} from '../api/sms'
import { mokKey } from '../i18n'
import { normalizeSmsPhone } from './sms'

/**
 * The admin alert phones sit next to their emails on upstream's pages and are saved with
 * them, by upstream's own save button:
 *   - ops alerts: the SMS section (./OpsAlertSms.vue) in ops monitoring → settings; the
 *     dialog's saveAllSettings calls mokSaveOpsAlertSms through a [mok] seam;
 *   - account quota alerts: the phones (./AccountQuotaSmsPhones.vue) in the account quota
 *     notification card; the settings page's saveSettings calls mokSaveAccountQuotaSms.
 * The sections keep their state here so that those save functions can reach it. A section
 * reloads it whenever it is mounted; a save only writes after a load and with changes.
 */

export const maxAdminPhones = 10

type Status = 'idle' | 'loading' | 'loaded' | 'failed'

export const opsAlertState = reactive({
  status: 'idle' as Status,
  /** Whether the SMS card is set up for ops alerts (SMS on, template set). */
  ready: false,
  saved: { enabled: false, phones: [], min_level: '' } as SmsOpsAlert,
  draft: { enabled: false, phones: [], min_level: '' } as SmsOpsAlert
})

export const accountQuotaState = reactive({
  status: 'idle' as Status,
  ready: false,
  saved: { phones: [] } as SmsAccountQuota,
  draft: { phones: [] } as SmsAccountQuota
})

function copyOps(ops: SmsOpsAlert): SmsOpsAlert {
  return { enabled: !!ops.enabled, phones: [...(ops.phones ?? [])], min_level: ops.min_level ?? '' }
}

function copyQuota(quota: SmsAccountQuota): SmsAccountQuota {
  return { phones: (quota.phones ?? []).map((p) => ({ phone: p.phone, disabled: !!p.disabled })) }
}

function mt(key: string, named: Record<string, unknown> = {}): string {
  return i18n.global.t(mokKey(key), named)
}

/**
 * The error upstream's save functions show. The ops dialog reads response.data.message,
 * the settings page reads message.
 */
function saveError(message: string): Error {
  return Object.assign(new Error(message), { response: { data: { message } } })
}

export async function loadOpsAlertSms(): Promise<void> {
  opsAlertState.status = 'loading'
  try {
    const ops = await getSmsOpsAlert()
    opsAlertState.ready = !!ops.ready
    opsAlertState.saved = copyOps(ops)
    opsAlertState.draft = copyOps(ops)
    opsAlertState.status = 'loaded'
  } catch (error) {
    console.error('Failed to load the SMS alert settings:', error)
    opsAlertState.status = 'failed'
  }
}

export function opsAlertDirty(): boolean {
  return JSON.stringify(opsAlertState.draft) !== JSON.stringify(opsAlertState.saved)
}

/** Adds a phone to the ops alert draft; returns the problem as a message, or null. */
export function addOpsAlertPhone(raw: string): string | null {
  const phone = normalizeSmsPhone(raw)
  if (!phone) return mt('smsOps.invalidPhone')
  if (opsAlertState.draft.phones.includes(phone)) return mt('smsOps.duplicate')
  if (opsAlertState.draft.phones.length >= maxAdminPhones) return mt('smsOps.tooMany')
  opsAlertState.draft.phones.push(phone)
  return null
}

/**
 * Called by the ops settings dialog's save through a [mok] seam, before upstream's own
 * settings are written. Throws when saving fails, so the dialog shows the error and
 * stays open.
 */
export async function mokSaveOpsAlertSms(): Promise<void> {
  if (opsAlertState.status !== 'loaded' || !opsAlertDirty()) return
  // As upstream does for the alert email: no recipient, no alert.
  const draft = copyOps(opsAlertState.draft)
  if (draft.phones.length === 0) draft.enabled = false
  try {
    const saved = await saveSmsOpsAlert(draft)
    opsAlertState.ready = !!saved.ready
    opsAlertState.saved = copyOps(saved)
    opsAlertState.draft = copyOps(saved)
  } catch (error) {
    throw saveError(extractApiErrorMessage(error, mt('smsOps.saveFailed')))
  }
}

export async function loadAccountQuotaSms(): Promise<void> {
  accountQuotaState.status = 'loading'
  try {
    const quota = await getSmsAccountQuota()
    accountQuotaState.ready = !!quota.ready
    accountQuotaState.saved = copyQuota(quota)
    accountQuotaState.draft = copyQuota(quota)
    accountQuotaState.status = 'loaded'
  } catch (error) {
    console.error('Failed to load the account quota SMS phones:', error)
    accountQuotaState.status = 'failed'
  }
}

export function accountQuotaDirty(): boolean {
  return JSON.stringify(accountQuotaState.draft) !== JSON.stringify(accountQuotaState.saved)
}

/** The draft with every number normalized, or the first problem as a message. */
function normalizedQuotaDraft(): { quota: SmsAccountQuota } | { error: string } {
  const phones: SmsAccountQuota['phones'] = []
  for (const entry of accountQuotaState.draft.phones) {
    if (!entry.phone.trim()) continue
    const phone = normalizeSmsPhone(entry.phone)
    if (!phone) return { error: mt('smsQuota.invalidPhone', { phone: entry.phone.trim() }) }
    if (!phones.some((p) => p.phone === phone)) phones.push({ phone, disabled: entry.disabled })
  }
  if (phones.length > maxAdminPhones) return { error: mt('smsQuota.tooMany') }
  return { quota: { phones } }
}

/**
 * Called by the settings page's save through a [mok] seam, after upstream's settings are
 * written. Throws when the phones are invalid or saving fails; the page shows the error
 * instead of its success message.
 */
export async function mokSaveAccountQuotaSms(): Promise<void> {
  if (accountQuotaState.status !== 'loaded' || !accountQuotaDirty()) return
  const result = normalizedQuotaDraft()
  if ('error' in result) throw saveError(result.error)
  try {
    const saved = await saveSmsAccountQuota(result.quota)
    accountQuotaState.ready = !!saved.ready
    accountQuotaState.saved = copyQuota(saved)
    accountQuotaState.draft = copyQuota(saved)
  } catch (error) {
    throw saveError(extractApiErrorMessage(error, mt('smsQuota.saveFailed')))
  }
}
