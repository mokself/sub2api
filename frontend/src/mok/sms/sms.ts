import { extractApiErrorCode, extractApiErrorMetadata, extractApiErrorMessage } from '@/utils/apiError'
import type { SmsTemplateKind, SmsUserEvent } from '../api/sms'

/** Template kinds in the order the settings card lists them. */
export const smsTemplateKinds: SmsTemplateKind[] = [
  'verify_code',
  'balance_low',
  'subscription_expiry',
  'ops_alert',
  'account_quota'
]

/** The user notifications the SMS card switches on and off; the admin alerts are switched where their phones are set. */
export const smsUserEvents: SmsUserEvent[] = ['balance_low', 'subscription_expiry']

/** Template parameters per kind, in order; the backend fills them in this order. */
export const smsTemplateParamCount: Record<SmsTemplateKind, number> = {
  verify_code: 2,
  balance_low: 2,
  subscription_expiry: 3,
  ops_alert: 4,
  account_quota: 4
}

/**
 * Suggested template texts to apply for at Tencent Cloud. Mainland China templates are
 * Chinese whatever the admin's locale, so these are not translated. Variables are numbered
 * in the order of smsTemplateParamCount.
 */
export const smsSuggestedTemplates: Record<SmsTemplateKind, string> = {
  verify_code: '您的验证码为{1}，{2}分钟内有效，如非本人操作，请忽略本短信。',
  balance_low: '您的账户余额为{1}，已低于提醒阈值{2}，请及时充值，以免影响使用。',
  subscription_expiry: '您订阅的{1}将于{2}到期，剩余{3}天，请及时续费。',
  ops_alert: '运维告警：{1}（{2}），当前值{3}，阈值{4}，请及时处理。',
  account_quota: '账号限额告警：账号{1}的{2}剩余{3}，额度上限{4}，请及时处理。'
}

/**
 * Accepts a mainland China mobile number, with or without +86 and with spaces or dashes,
 * and returns its 11 digits; null otherwise. Mirrors normalizeSMSPhone in the backend.
 */
export function normalizeSmsPhone(raw: string): string | null {
  let phone = raw.trim().replace(/[\s-]/g, '')
  if (phone.startsWith('+86')) phone = phone.slice(3)
  else if (phone.startsWith('0086')) phone = phone.slice(4)
  else if (phone.length === 13 && phone.startsWith('86')) phone = phone.slice(2)
  return /^1[3-9]\d{9}$/.test(phone) ? phone : null
}

/** Template IDs are numbers such as 1234567. */
export function isSmsTemplateId(value: string): boolean {
  return value.trim() === '' || /^\d{1,20}$/.test(value.trim())
}

type Translate = (key: string, params?: Record<string, unknown>) => string

/** The user-facing message of an SMS error, by its reason code. */
export function smsErrorMessage(error: unknown, mt: Translate, fallback: string): string {
  const code = extractApiErrorCode(error)
  switch (code) {
    case 'MOK_SMS_UNAVAILABLE':
    case 'MOK_SMS_PHONE_INVALID':
    case 'MOK_SMS_CODE_DAILY_LIMIT':
    case 'MOK_SMS_CODE_INVALID':
    case 'MOK_SMS_CODE_EXPIRED':
    case 'MOK_SMS_SEND_FAILED':
    case 'MOK_SMS_PHONE_NOT_BOUND':
    case 'MOK_SMS_NOT_CONFIGURED':
      return mt(`sms.errors.${code}`)
    case 'MOK_SMS_CODE_COOLDOWN': {
      const seconds = Number(extractApiErrorMetadata(error)?.retry_after ?? 60)
      return mt('sms.errors.MOK_SMS_CODE_COOLDOWN', { seconds: Number.isFinite(seconds) ? seconds : 60 })
    }
  }
  return extractApiErrorMessage(error, fallback)
}
