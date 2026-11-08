import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { ClaudePlanItem } from '../../api/claudePlan'
import ClaudePlanCell from '../ClaudePlanCell.vue'
import {
  getClaudePlanEntry,
  MOK_CLAUDE_PLAN_COLUMN,
  mokAddAccountColumns,
  requestClaudePlan,
  resetClaudePlanStore,
  resolveClaudePlan
} from '../claudePlan'

const { getClaudePlanTiers } = vi.hoisted(() => ({ getClaudePlanTiers: vi.fn() }))

vi.mock('../../api/claudePlan', () => ({ getClaudePlanTiers }))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

function planItem(id: number, patch: Partial<ClaudePlanItem> = {}): ClaudePlanItem {
  return {
    account_id: id,
    status: 'ok',
    organization_type: 'claude_max',
    rate_limit_tier: 'default_claude_max_20x',
    seat_tier: '',
    subscription_status: '',
    fetched_at: '2026-11-08T04:00:00Z',
    attempted_at: '2026-11-08T04:00:00Z',
    ...patch
  }
}

async function settle(): Promise<void> {
  await vi.runAllTimersAsync()
  await flushPromises()
}

beforeEach(() => {
  vi.useFakeTimers()
  resetClaudePlanStore()
  getClaudePlanTiers.mockReset()
  getClaudePlanTiers.mockImplementation(async (ids: number[]) => ids.map((id) => planItem(id)))
})

afterEach(() => {
  vi.useRealTimers()
})

describe('resolveClaudePlan', () => {
  it.each([
    ['claude_max', 'default_claude_max_20x', 'Ultra'],
    ['claude_max', 'default_claude_max_5x', 'Max'],
    ['claude_max', 'default_claude_max_10x', 'Max 10x'],
    ['claude_max', '', 'Max'],
    ['claude_pro', 'default_claude_ai', 'Pro'],
    ['claude_team', 'default_claude_team', 'Team'],
    ['claude_enterprise', '', 'Enterprise'],
    ['', 'some_new_tier', 'some_new_tier']
  ])('%s / %s shows %s', (organizationType, rateLimitTier, label) => {
    expect(resolveClaudePlan(organizationType, rateLimitTier).label).toBe(label)
  })
})

describe('mokAddAccountColumns', () => {
  it('inserts the plan column after the platform column once', () => {
    const columns = [
      { key: 'name', label: 'Name', sortable: true },
      { key: 'platform_type', label: 'Platform', sortable: false },
      { key: 'actions', label: 'Actions', sortable: false }
    ]
    mokAddAccountColumns(columns, (key) => key)
    mokAddAccountColumns(columns, (key) => key)
    expect(columns.map((column) => column.key)).toEqual(['name', 'platform_type', MOK_CLAUDE_PLAN_COLUMN, 'actions'])
    expect(columns[2].label).toBe('mok.claudePlan.column')
  })
})

describe('requestClaudePlan', () => {
  it('batches requests and asks for each account once', async () => {
    requestClaudePlan(1)
    requestClaudePlan(2)
    requestClaudePlan(1)
    await settle()
    requestClaudePlan(2)
    await settle()

    expect(getClaudePlanTiers).toHaveBeenCalledTimes(1)
    expect(getClaudePlanTiers).toHaveBeenCalledWith([1, 2], false)
    expect(getClaudePlanEntry(2)?.item?.rate_limit_tier).toBe('default_claude_max_20x')
  })

  it('refetches with refresh and keeps the old plan while loading', async () => {
    requestClaudePlan(1)
    await settle()
    getClaudePlanTiers.mockImplementation(async (ids: number[]) => ids.map((id) => planItem(id, { rate_limit_tier: 'default_claude_max_5x' })))

    requestClaudePlan(1, true)
    expect(getClaudePlanEntry(1)).toMatchObject({ loading: true, item: { rate_limit_tier: 'default_claude_max_20x' } })
    await settle()

    expect(getClaudePlanTiers).toHaveBeenLastCalledWith([1], true)
    expect(getClaudePlanEntry(1)).toMatchObject({ loading: false, failed: false, item: { rate_limit_tier: 'default_claude_max_5x' } })
  })

  it('splits large pages into chunks of 20', async () => {
    for (let id = 1; id <= 45; id++) requestClaudePlan(id)
    await settle()
    expect(getClaudePlanTiers.mock.calls.map(([ids]) => ids.length)).toEqual([20, 20, 5])
  })

  it('marks entries failed when the request fails', async () => {
    getClaudePlanTiers.mockRejectedValueOnce(new Error('network'))
    requestClaudePlan(1)
    await settle()
    expect(getClaudePlanEntry(1)).toMatchObject({ loading: false, failed: true, item: null })
  })
})

describe('ClaudePlanCell', () => {
  it('shows a dash for non-Claude-OAuth accounts without requesting', async () => {
    const wrapper = mount(ClaudePlanCell, { props: { account: { id: 1, platform: 'openai', type: 'oauth' } } })
    await settle()
    expect(wrapper.text()).toBe('-')
    expect(getClaudePlanTiers).not.toHaveBeenCalled()
  })

  it('shows the plan badge with the raw fields in its tooltip', async () => {
    const wrapper = mount(ClaudePlanCell, { props: { account: { id: 7, platform: 'anthropic', type: 'oauth' } } })
    await settle()
    const badge = wrapper.get('[data-testid="mok-claude-plan-badge"]')
    expect(badge.text()).toBe('Ultra')
    expect(badge.attributes('title')).toContain('rate_limit_tier: default_claude_max_20x')
    expect(wrapper.find('[data-testid="mok-claude-plan-error"]').exists()).toBe(false)
  })

  it('shows the failure and a refresh button when the fetch failed', async () => {
    getClaudePlanTiers.mockImplementation(async (ids: number[]) =>
      ids.map((id) => planItem(id, { status: 'error', reason: 'MOK_CLAUDE_PROFILE_FAILED', message: 'OAuth profile upstream HTTP 401', organization_type: '', rate_limit_tier: '', fetched_at: null }))
    )
    const wrapper = mount(ClaudePlanCell, { props: { account: { id: 7, platform: 'anthropic', type: 'oauth' } } })
    await settle()

    const error = wrapper.get('[data-testid="mok-claude-plan-error"]')
    expect(error.text()).toBe('mok.claudePlan.failed')
    expect(error.attributes('title')).toContain('mok.claudePlan.lastError')

    getClaudePlanTiers.mockImplementation(async (ids: number[]) => ids.map((id) => planItem(id)))
    await wrapper.get('[data-testid="mok-claude-plan-refresh"]').trigger('click')
    await settle()
    expect(getClaudePlanTiers).toHaveBeenLastCalledWith([7], true)
    expect(wrapper.get('[data-testid="mok-claude-plan-badge"]').text()).toBe('Ultra')
  })

  it('explains a missing user:profile scope', async () => {
    getClaudePlanTiers.mockImplementation(async (ids: number[]) =>
      ids.map((id) => planItem(id, { status: 'unsupported', reason: 'missing_profile_scope', organization_type: '', rate_limit_tier: '' }))
    )
    const wrapper = mount(ClaudePlanCell, { props: { account: { id: 7, platform: 'anthropic', type: 'oauth' } } })
    await settle()
    expect(wrapper.find('[title="mok.claudePlan.unsupportedScope"]').exists()).toBe(true)
  })
})
