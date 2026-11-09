import { apiClient } from '@/api/client'

export type AffiliateRebateMode = 'recharge' | 'usage'

/** One settled day of usage rebates. */
export interface AffiliateUsageRebateDay {
  day: string
  window_start: string
  window_end: string
  invitee_count: number
  usage_total: number
  rebate_total: number
  /** Non-empty when the day was closed without rebates, e.g. "affiliate_disabled". */
  skipped_reason?: string
  settled_at: string
}

/** GET/PUT /api/v1/mok/admin/affiliate-rebate-mode */
export interface AffiliateRebateModeStatus {
  mode: AffiliateRebateMode
  /** Start of the usage window: the last switch to usage mode. */
  usage_since?: string
  /** End of the usage window, only after switching back to recharge mode. */
  usage_until?: string
  recent_days: AffiliateUsageRebateDay[]
}

export async function getAffiliateRebateMode(): Promise<AffiliateRebateModeStatus> {
  const { data } = await apiClient.get<AffiliateRebateModeStatus>('/mok/admin/affiliate-rebate-mode')
  return data
}

export async function setAffiliateRebateMode(mode: AffiliateRebateMode): Promise<AffiliateRebateModeStatus> {
  const { data } = await apiClient.put<AffiliateRebateModeStatus>('/mok/admin/affiliate-rebate-mode', { mode })
  return data
}
