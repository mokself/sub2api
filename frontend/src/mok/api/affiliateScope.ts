import { apiClient } from '@/api/client'
import type { PaginatedResponse } from '@/types'

/** "all": the affiliate feature is on for everyone; "selected": only for selected users. */
export type AffiliateScope = 'all' | 'selected'

/** GET/PUT /api/v1/mok/admin/affiliate-scope */
export interface AffiliateScopeStatus {
  /** Upstream's switch for the whole feature. */
  affiliate_enabled: boolean
  scope: AffiliateScope
  selected_count: number
}

/** A user the feature is switched on for. */
export interface AffiliateSelectedUser {
  user_id: number
  email: string
  username: string
  selected_at: string
}

/** One user's switch (GET/PUT /api/v1/mok/admin/affiliate-users/:user_id). */
export interface AffiliateUserAccess {
  user_id: number
  /** The user's own switch; it only matters while the scope is "selected". */
  selected: boolean
  scope: AffiliateScope
  affiliate_enabled: boolean
  /** Whether the feature is on for the user right now. */
  allowed: boolean
}

export async function getAffiliateScope(): Promise<AffiliateScopeStatus> {
  const { data } = await apiClient.get<AffiliateScopeStatus>('/mok/admin/affiliate-scope')
  return data
}

export async function setAffiliateScope(scope: AffiliateScope): Promise<AffiliateScopeStatus> {
  const { data } = await apiClient.put<AffiliateScopeStatus>('/mok/admin/affiliate-scope', { scope })
  return data
}

/** The selected users, most recently selected first. */
export async function listAffiliateSelectedUsers(page: number, pageSize: number): Promise<PaginatedResponse<AffiliateSelectedUser>> {
  const { data } = await apiClient.get<PaginatedResponse<AffiliateSelectedUser>>('/mok/admin/affiliate-users', {
    params: { page, page_size: pageSize }
  })
  return data
}

export async function getAffiliateUserAccess(userId: number): Promise<AffiliateUserAccess> {
  const { data } = await apiClient.get<AffiliateUserAccess>(`/mok/admin/affiliate-users/${userId}`)
  return data
}

export async function setAffiliateUserSelected(userId: number, selected: boolean): Promise<AffiliateUserAccess> {
  const { data } = await apiClient.put<AffiliateUserAccess>(`/mok/admin/affiliate-users/${userId}`, { selected })
  return data
}

/** GET /api/v1/mok/affiliate/access */
export interface AffiliateOwnAccess {
  /** The affiliate feature is on for the signed-in user. */
  allowed: boolean
  /** The feature is off for the user, who still has rebate quota to transfer. */
  has_quota: boolean
}

export async function getMyAffiliateAccess(): Promise<AffiliateOwnAccess> {
  const { data } = await apiClient.get<Partial<AffiliateOwnAccess>>('/mok/affiliate/access')
  return { allowed: data.allowed === true, has_quota: data.has_quota === true }
}
