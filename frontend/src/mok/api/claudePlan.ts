import { apiClient } from '@/api/client'

/** One account's plan, as returned by GET /api/v1/mok/admin/claude-plan-tiers. */
export interface ClaudePlanItem {
  account_id: number
  status: 'ok' | 'error' | 'unsupported'
  /** Unsupported reason or error code. */
  reason?: string
  message?: string
  organization_type: string
  rate_limit_tier: string
  seat_tier: string
  subscription_status: string
  /** Last successful fetch. With status "error" the plan fields are from this fetch. */
  fetched_at: string | null
  attempted_at: string | null
}

/**
 * Returns the plans of the given accounts in one request. The server fetches the missing
 * or stale ones from Anthropic before answering, so the timeout is longer than usual.
 */
export async function getClaudePlanTiers(ids: number[], refresh = false): Promise<ClaudePlanItem[]> {
  const { data } = await apiClient.get<{ items: ClaudePlanItem[] }>('/mok/admin/claude-plan-tiers', {
    params: refresh ? { ids: ids.join(','), refresh: true } : { ids: ids.join(',') },
    timeout: 120_000
  })
  return data.items ?? []
}
