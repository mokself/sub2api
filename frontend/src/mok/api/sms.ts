import { apiClient } from '@/api/client'
import type { PaginatedResponse } from '@/types'

/** Notifications that can go out by SMS (backend service/sms_notify_mok.go). */
export type SmsEvent = 'balance_low' | 'subscription_expiry' | 'ops_alert' | 'account_quota'
/** Every message kind with a template: the notifications and the verification code. */
export type SmsTemplateKind = 'verify_code' | SmsEvent
/** Message kinds in the send log: the template kinds plus the admin's test messages. */
export type SmsLogEvent = SmsTemplateKind | 'test'
export type SmsLogStatus = 'pending' | 'sent' | 'failed' | 'dry_run' | 'limited'
export type SmsRegion = 'ap-guangzhou' | 'ap-beijing' | 'ap-nanjing'
/** Lowest ops alert level sent by SMS; empty sends every level. */
export type SmsMinLevel = '' | 'info' | 'warning' | 'critical'

export interface SmsConfig {
  enabled: boolean
  /** Log every message instead of sending it. */
  dry_run: boolean
  secret_id: string
  /** Never returned; an empty value on save keeps the stored key. */
  secret_key: string
  sdk_app_id: string
  sign_name: string
  region: SmsRegion
  /** Notifications a user (admin alerts: a phone) gets a day; 0 means no cap. */
  daily_limit: number
  templates: Record<SmsTemplateKind, string>
  events: Record<SmsEvent, boolean>
  /** Receive the ops alerts and account quota alerts; 11-digit mainland numbers. */
  admin_phones: string[]
  ops_alert_min_level: SmsMinLevel
}

export interface SmsConfigView extends SmsConfig {
  secret_key_set: boolean
}

export interface SmsTestResult {
  status: SmsLogStatus
  /** The provider's error when the status is failed. */
  error: string
}

export interface SmsLogEntry {
  id: number
  created_at: string
  event: SmsLogEvent
  user_id: number | null
  user_email: string
  phone: string
  template_id: string
  params: string[]
  status: SmsLogStatus
  error: string
  request_id: string
  finished_at: string | null
}

export interface SmsPhoneStatus {
  /** False until the admin has set SMS reminders up; the profile page then hides them. */
  available: boolean
  /** The bound number, empty without one. */
  phone: string
  enabled: boolean
  /** The reminders the bound phone receives. */
  events: SmsEvent[]
}

export interface SmsCodeSent {
  cooldown_seconds: number
  expires_in_seconds: number
}

export async function getSmsConfig(): Promise<SmsConfigView> {
  const { data } = await apiClient.get<SmsConfigView>('/mok/admin/sms/config')
  return data
}

export async function saveSmsConfig(config: SmsConfig): Promise<SmsConfigView> {
  const { data } = await apiClient.put<SmsConfigView>('/mok/admin/sms/config', config)
  return data
}

/** Sends one template with sample values, using the saved settings. */
export async function sendTestSms(phone: string, event: SmsTemplateKind): Promise<SmsTestResult> {
  const { data } = await apiClient.post<SmsTestResult>('/mok/admin/sms/test', { phone, event })
  return data
}

export async function listSmsLogs(query: { page: number; page_size: number; search?: string }): Promise<PaginatedResponse<SmsLogEntry>> {
  const { data } = await apiClient.get<PaginatedResponse<SmsLogEntry>>('/mok/admin/sms/logs', {
    params: { page: query.page, page_size: query.page_size, search: query.search ?? '' }
  })
  return data
}

export async function getMySmsPhone(): Promise<SmsPhoneStatus> {
  const { data } = await apiClient.get<SmsPhoneStatus>('/mok/sms/phone')
  return data
}

export async function sendSmsBindCode(phone: string): Promise<SmsCodeSent> {
  const { data } = await apiClient.post<SmsCodeSent>('/mok/sms/phone/code', { phone })
  return data
}

export async function bindSmsPhone(phone: string, code: string): Promise<SmsPhoneStatus> {
  const { data } = await apiClient.post<SmsPhoneStatus>('/mok/sms/phone', { phone, code })
  return data
}

export async function setSmsPhoneEnabled(enabled: boolean): Promise<SmsPhoneStatus> {
  const { data } = await apiClient.put<SmsPhoneStatus>('/mok/sms/phone', { enabled })
  return data
}

export async function removeSmsPhone(): Promise<SmsPhoneStatus> {
  const { data } = await apiClient.delete<SmsPhoneStatus>('/mok/sms/phone')
  return data
}
