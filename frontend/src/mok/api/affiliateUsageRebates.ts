import { apiClient } from '@/api/client'
import type { PaginatedResponse } from '@/types'

/** One invitee's settled period (an hour, shorter around a mode switch). */
export interface AffiliateUsageRebateRecord {
  period_start: string
  period_end: string
  /** Admin list only. */
  inviter_id?: number
  /** Admin list only. */
  inviter_email?: string
  /** Admin list only. */
  invitee_id?: number
  /** Masked in a user's own list. */
  invitee_email: string
  usage_amount: number
  rate_percent: number
  rebate_amount: number
  settled_at: string
}

export interface AffiliateUsageRebateQuery {
  page: number
  page_size: number
  /** Admin list only: part of the inviter's or invitee's email or username. */
  search?: string
}

export type AffiliateUsageRebatePage = PaginatedResponse<AffiliateUsageRebateRecord>

/** Every settled record, newest period first (GET /api/v1/mok/admin/affiliate-usage-rebates). */
export async function listAffiliateUsageRebates(query: AffiliateUsageRebateQuery): Promise<AffiliateUsageRebatePage> {
  const { data } = await apiClient.get<AffiliateUsageRebatePage>('/mok/admin/affiliate-usage-rebates', {
    params: { page: query.page, page_size: query.page_size, search: query.search ?? '' }
  })
  return data
}

/** The signed-in inviter's own records (GET /api/v1/mok/affiliate/usage-rebates). */
export async function listMyAffiliateUsageRebates(query: AffiliateUsageRebateQuery): Promise<AffiliateUsageRebatePage> {
  const { data } = await apiClient.get<AffiliateUsageRebatePage>('/mok/affiliate/usage-rebates', {
    params: { page: query.page, page_size: query.page_size }
  })
  return data
}
