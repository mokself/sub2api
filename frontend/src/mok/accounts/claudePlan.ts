import { reactive } from 'vue'
import { getClaudePlanTiers, type ClaudePlanItem } from '../api/claudePlan'
import { mokKey } from '../i18n'

// ==================== Plan badge ====================

export interface ClaudePlanBadge {
  label: string
  className: string
}

const BADGE_CLASS = {
  ultra: 'bg-purple-100 text-purple-600 dark:bg-purple-900/40 dark:text-purple-300',
  max: 'bg-indigo-100 text-indigo-600 dark:bg-indigo-900/40 dark:text-indigo-300',
  pro: 'bg-blue-100 text-blue-600 dark:bg-blue-900/40 dark:text-blue-300',
  team: 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300',
  enterprise: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300',
  other: 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
}

// Plan names by the multiplier in rate_limit_tier (default_claude_max_<N>x).
// This site sells Max 20x as "Ultra" and Max 5x as "Max".
const MAX_MULTIPLIER_BADGES: Record<string, ClaudePlanBadge> = {
  '20': { label: 'Ultra', className: BADGE_CLASS.ultra },
  '5': { label: 'Max', className: BADGE_CLASS.max }
}

const ORGANIZATION_TYPE_BADGES: Record<string, ClaudePlanBadge> = {
  claude_max: { label: 'Max', className: BADGE_CLASS.max },
  claude_pro: { label: 'Pro', className: BADGE_CLASS.pro },
  claude_team: { label: 'Team', className: BADGE_CLASS.team },
  claude_enterprise: { label: 'Enterprise', className: BADGE_CLASS.enterprise },
  claude_free: { label: 'Free', className: BADGE_CLASS.other }
}

/** Maps the profile's organization_type and rate_limit_tier to a display badge. */
export function resolveClaudePlan(organizationType: string, rateLimitTier: string): ClaudePlanBadge {
  const multiplier = /max_(\d+)x/.exec(rateLimitTier.toLowerCase())?.[1]
  if (multiplier) {
    return MAX_MULTIPLIER_BADGES[multiplier] ?? { label: `Max ${multiplier}x`, className: BADGE_CLASS.max }
  }
  const byType = ORGANIZATION_TYPE_BADGES[organizationType.trim().toLowerCase()]
  if (byType) {
    return byType
  }
  return { label: rateLimitTier || organizationType, className: BADGE_CLASS.other }
}

// ==================== Column ====================

export const MOK_CLAUDE_PLAN_COLUMN = 'mok_claude_plan'

interface AccountColumn {
  key: string
  label: string
  sortable: boolean
}

/**
 * Adds the fork's columns to AccountsView's column list: the Claude plan goes right
 * after the platform/type column.
 */
export function mokAddAccountColumns(columns: AccountColumn[], t: (key: string) => string): void {
  if (columns.some((column) => column.key === MOK_CLAUDE_PLAN_COLUMN)) {
    return
  }
  const platformAt = columns.findIndex((column) => column.key === 'platform_type')
  const actionsAt = columns.findIndex((column) => column.key === 'actions')
  const at = platformAt >= 0 ? platformAt + 1 : actionsAt >= 0 ? actionsAt : columns.length
  columns.splice(at, 0, { key: MOK_CLAUDE_PLAN_COLUMN, label: t(mokKey('claudePlan.column')), sortable: false })
}

// ==================== Batched loading ====================

export interface ClaudePlanEntry {
  loading: boolean
  item: ClaudePlanItem | null
  /** The request itself failed, as opposed to the server reporting a fetch error. */
  failed: boolean
}

const BATCH_SIZE = 20

// Shared by every cell, so rows that re-mount (virtual scrolling, list reloads) reuse
// results instead of requesting them again.
const entries = reactive(new Map<number, ClaudePlanEntry>())
const pending = new Map<number, boolean>()
let flushScheduled = false

export function getClaudePlanEntry(accountId: number): ClaudePlanEntry | undefined {
  return entries.get(accountId)
}

/**
 * Queues an account for the next batched request. Without refresh each account is
 * requested once per page load; the server keeps its own cache.
 */
export function requestClaudePlan(accountId: number, refresh = false): void {
  const entry = entries.get(accountId)
  if (entry?.loading || (entry && !refresh)) {
    return
  }
  entries.set(accountId, { loading: true, item: entry?.item ?? null, failed: false })
  pending.set(accountId, refresh || pending.get(accountId) === true)
  if (!flushScheduled) {
    flushScheduled = true
    setTimeout(flushClaudePlanRequests, 0)
  }
}

function flushClaudePlanRequests(): void {
  flushScheduled = false
  const queued = [...pending]
  pending.clear()
  for (const refresh of [false, true]) {
    const ids = queued.filter(([, r]) => r === refresh).map(([id]) => id)
    for (let i = 0; i < ids.length; i += BATCH_SIZE) {
      void loadChunk(ids.slice(i, i + BATCH_SIZE), refresh)
    }
  }
}

async function loadChunk(ids: number[], refresh: boolean): Promise<void> {
  let byId = new Map<number, ClaudePlanItem>()
  let failed = false
  try {
    const items = await getClaudePlanTiers(ids, refresh)
    byId = new Map(items.map((item) => [item.account_id, item]))
  } catch {
    failed = true
  }
  for (const id of ids) {
    const item = byId.get(id)
    entries.set(id, { loading: false, item: item ?? entries.get(id)?.item ?? null, failed: failed || !item })
  }
}

/** Test helper: forgets every cached result and queued request. */
export function resetClaudePlanStore(): void {
  entries.clear()
  pending.clear()
  flushScheduled = false
}
