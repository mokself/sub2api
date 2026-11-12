import { apiClient } from '@/api/client'

export type AffiliateRebateMode = 'recharge' | 'usage'

/** The usage rebate periods settled for one day of the server time zone. */
export interface AffiliateUsageRebateDay {
  day: string
  period_count: number
  /** How many of the periods were closed without rebates because the affiliate feature was off. */
  skipped_count: number
  invitee_count: number
  usage_total: number
  rebate_total: number
}

/** GET/PUT /api/v1/mok/admin/affiliate-rebate-mode */
export interface AffiliateRebateModeStatus {
  mode: AffiliateRebateMode
  /** Start of the usage window: the last switch to usage mode. */
  usage_since?: string
  /** End of the usage window, only after switching back to recharge mode. */
  usage_until?: string
  /** How far the usage window has been settled. */
  settled_until?: string
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
