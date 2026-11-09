import { apiClient } from '@/api/client'

/** From min_amount of team volume on, the inviter earns rate_percent. */
export interface AffiliateRebateTier {
  min_amount: number
  rate_percent: number
}

/** The signed-in user's progress through the tiers (GET /api/v1/mok/affiliate/rebate-tiers). */
export interface AffiliateTierStatus {
  /** "recharge" or "usage" while the affiliate feature is on, empty while it is off. */
  rebate_mode: '' | 'recharge' | 'usage'
  /** The affiliate feature is on and tiers are configured. */
  enabled: boolean
  /** The user has an exclusive rate, which tiers do not change. */
  exclusive: boolean
  base_rate_percent: number
  current_rate_percent: number
  team_volume: number
  tiers: AffiliateRebateTier[]
  next_tier?: AffiliateRebateTier
  next_tier_remaining: number
}

export async function getAffiliateRebateTiers(): Promise<AffiliateRebateTier[]> {
  const { data } = await apiClient.get<{ tiers: AffiliateRebateTier[] }>('/mok/admin/affiliate-rebate-tiers')
  return data.tiers ?? []
}

/** Replaces the tiers and returns them as stored (sorted, rounded). An empty list disables them. */
export async function saveAffiliateRebateTiers(tiers: AffiliateRebateTier[]): Promise<AffiliateRebateTier[]> {
  const { data } = await apiClient.put<{ tiers: AffiliateRebateTier[] }>('/mok/admin/affiliate-rebate-tiers', { tiers })
  return data.tiers ?? []
}

export async function getAffiliateTierStatus(): Promise<AffiliateTierStatus> {
  const { data } = await apiClient.get<AffiliateTierStatus>('/mok/affiliate/rebate-tiers')
  return data
}
