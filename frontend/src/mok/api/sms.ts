import { apiClient } from '@/api/client'
import type { PaginatedResponse } from '@/types'

/** Notifications that can go out by SMS (backend service/sms_notify_mok.go). */
export type SmsEvent = 'balance_low' | 'subscription_expiry' | 'ops_alert' | 'account_quota'
/** The notifications users get on their bound phone, switched on and off in the SMS card. */
export type SmsUserEvent = 'balance_low' | 'subscription_expiry'
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
  /** The admin alerts are switched where their phones are set, see SmsOpsAlert and SmsAccountQuota. */
  events: Record<SmsUserEvent, boolean>
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
  events: SmsUserEvent[]
}

/** The SMS part of the ops alert settings, edited in ops monitoring → settings. */
export interface SmsOpsAlert {
  enabled: boolean
  /** 11-digit mainland numbers. */
  phones: string[]
  min_level: SmsMinLevel
}

export interface SmsQuotaPhone {
  phone: string
  disabled: boolean
}

/** The SMS phones of the account quota alerts, edited in the account quota notification card. */
export interface SmsAccountQuota {
  phones: SmsQuotaPhone[]
}

/** Whether the SMS card is set up for these alerts: SMS switched on and the template set. */
export interface SmsReady {
  ready: boolean
}

export interface SmsCodeSent {
  cooldown_seconds: number
  expires_in_seconds: number
}

export async function getSmsConfig(): Promise<SmsConfigView> {
  const { data } = await apiClient.get<SmsConfigView>('/admin/mok/sms/config')
  return data
}

export async function saveSmsConfig(config: SmsConfig): Promise<SmsConfigView> {
  const { data } = await apiClient.put<SmsConfigView>('/admin/mok/sms/config', config)
  return data
}

/** Sends one template with sample values, using the saved settings. */
export async function sendTestSms(phone: string, event: SmsTemplateKind): Promise<SmsTestResult> {
  const { data } = await apiClient.post<SmsTestResult>('/admin/mok/sms/test', { phone, event })
  return data
}

export async function listSmsLogs(query: { page: number; page_size: number; search?: string }): Promise<PaginatedResponse<SmsLogEntry>> {
  const { data } = await apiClient.get<PaginatedResponse<SmsLogEntry>>('/admin/mok/sms/logs', {
    params: { page: query.page, page_size: query.page_size, search: query.search ?? '' }
  })
  return data
}

export async function getSmsOpsAlert(): Promise<SmsOpsAlert & SmsReady> {
  const { data } = await apiClient.get<SmsOpsAlert & SmsReady>('/admin/mok/sms/ops-alert')
  return data
}

export async function saveSmsOpsAlert(ops: SmsOpsAlert): Promise<SmsOpsAlert & SmsReady> {
  const { data } = await apiClient.put<SmsOpsAlert & SmsReady>('/admin/mok/sms/ops-alert', ops)
  return data
}

export async function getSmsAccountQuota(): Promise<SmsAccountQuota & SmsReady> {
  const { data } = await apiClient.get<SmsAccountQuota & SmsReady>('/admin/mok/sms/account-quota')
  return data
}

export async function saveSmsAccountQuota(quota: SmsAccountQuota): Promise<SmsAccountQuota & SmsReady> {
  const { data } = await apiClient.put<SmsAccountQuota & SmsReady>('/admin/mok/sms/account-quota', quota)
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
