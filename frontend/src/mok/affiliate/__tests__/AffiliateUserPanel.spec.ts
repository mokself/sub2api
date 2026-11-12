import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'

import AffiliateUserPanel from '../AffiliateUserPanel.vue'
import type { AffiliateTierStatus } from '../../api/affiliateTiers'
import type { AffiliateUsageRebateRecord } from '../../api/affiliateUsageRebates'

const { getAffiliateTierStatus, listMyAffiliateUsageRebates, setAffiliateUsageWording } = vi.hoisted(() => ({
  getAffiliateTierStatus: vi.fn(),
  listMyAffiliateUsageRebates: vi.fn(),
  setAffiliateUsageWording: vi.fn()
}))

vi.mock('../../api/affiliateTiers', () => ({ getAffiliateTierStatus }))
vi.mock('../../api/affiliateUsageRebates', () => ({ listMyAffiliateUsageRebates, listAffiliateUsageRebates: vi.fn() }))
vi.mock('../../i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../i18n')>()),
  setAffiliateUsageWording
}))
vi.mock('@/utils/format', () => ({ formatCurrency: (amount: number) => `$${amount}` }))
vi.mock('../usageRebate', () => ({
  formatRebatePeriod: (start: string, end: string) => `${start}~${end}`,
  formatRebateRate: (rate: number) => String(rate)
}))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, named: Record<string, unknown> = {}) =>
        Object.keys(named).length ? `${key} ${JSON.stringify(named)}` : key
    })
  }
})

const RecordsStub = defineComponent({
  props: { show: Boolean, scope: String },
  emits: ['close'],
  setup(props, { emit }) {
    return () =>
      props.show ? h('button', { 'data-testid': 'records', onClick: () => emit('close') }, `records:${props.scope}`) : null
  }
})

function status(patch: Partial<AffiliateTierStatus> = {}): AffiliateTierStatus {
  return {
    rebate_mode: 'recharge',
    enabled: true,
    exclusive: false,
    base_rate_percent: 10,
    current_rate_percent: 12,
    team_volume: 60000,
    tiers: [
      { min_amount: 50000, rate_percent: 12 },
      { min_amount: 100000, rate_percent: 15 }
    ],
    next_tier: { min_amount: 100000, rate_percent: 15 },
    next_tier_remaining: 40000,
    ...patch
  }
}

function record(patch: Partial<AffiliateUsageRebateRecord> = {}): AffiliateUsageRebateRecord {
  return {
    period_start: '11:00',
    period_end: '12:00',
    invitee_email: 'i***@e***.com',
    usage_amount: 20,
    rate_percent: 12,
    rebate_amount: 2.4,
    settled_at: '12:05',
    ...patch
  }
}

function records(items: AffiliateUsageRebateRecord[], total = items.length) {
  return { items, total, page: 1, page_size: 1, pages: 1 }
}

async function mountWith(value: AffiliateTierStatus) {
  getAffiliateTierStatus.mockResolvedValue(value)
  const wrapper = mount(AffiliateUserPanel, { global: { stubs: { AffiliateUsageRebateRecords: RecordsStub } } })
  await flushPromises()
  return wrapper
}

beforeEach(() => {
  vi.clearAllMocks()
  listMyAffiliateUsageRebates.mockResolvedValue(records([]))
})

describe('AffiliateUserPanel tier card', () => {
  it('shows the volume, the next tier and the ladder with the current tier marked', async () => {
    const wrapper = await mountWith(status())

    expect(wrapper.get('[data-testid="mok-affiliate-tier-volume"]').text()).toBe('$60000')
    expect(wrapper.get('[data-testid="mok-affiliate-tier-next"]').text()).toBe(
      'mok.affiliateTiers.user.nextTier {"amount":"$40000","rate":"15"}'
    )
    const rows = wrapper.findAll('[data-testid="mok-affiliate-tier-row"]')
    expect(rows.map((row) => row.text())).toEqual([
      'mok.affiliateTiers.user.baseRow {"amount":"$50000"}10%',
      'mok.affiliateTiers.user.tierRow {"amount":"$50000"}mok.affiliateTiers.user.current12%',
      'mok.affiliateTiers.user.tierRow {"amount":"$100000"}15%'
    ])
  })

  it('marks the base rate below the first tier and the top tier at the end', async () => {
    let wrapper = await mountWith(status({ team_volume: 1000, current_rate_percent: 10, next_tier: { min_amount: 50000, rate_percent: 12 } }))
    expect(wrapper.findAll('[data-testid="mok-affiliate-tier-row"]')[0].text()).toContain('mok.affiliateTiers.user.current')

    wrapper = await mountWith(status({ team_volume: 150000, current_rate_percent: 15, next_tier: undefined, next_tier_remaining: 0 }))
    expect(wrapper.get('[data-testid="mok-affiliate-tier-next"]').text()).toBe('mok.affiliateTiers.user.topTier')
    expect(wrapper.findAll('[data-testid="mok-affiliate-tier-row"]')[2].text()).toContain('mok.affiliateTiers.user.current')
  })

  it.each([
    ['tiers are off', status({ enabled: false, tiers: [] })],
    ['the user has an exclusive rate', status({ exclusive: true })]
  ])('is hidden when %s', async (_, value) => {
    const wrapper = await mountWith(value)
    expect(wrapper.find('[data-testid="mok-affiliate-tier-progress"]').exists()).toBe(false)
  })

  it('keeps the upstream wording in recharge mode', async () => {
    const wrapper = await mountWith(status())
    expect(setAffiliateUsageWording).toHaveBeenCalledWith(false)
    expect(wrapper.find('[data-testid="mok-affiliate-tier-usage-mode"]').exists()).toBe(false)
  })

  it('rewords the page and explains usage rebates in usage mode', async () => {
    const wrapper = await mountWith(status({ rebate_mode: 'usage' }))
    expect(setAffiliateUsageWording).toHaveBeenCalledWith(true)
    expect(wrapper.get('[data-testid="mok-affiliate-tier-usage-mode"]').text()).toBe('mok.affiliateTiers.user.usageMode')
  })

  it('rewords the page in usage mode even without tiers', async () => {
    const wrapper = await mountWith(status({ rebate_mode: 'usage', enabled: false, tiers: [] }))
    expect(setAffiliateUsageWording).toHaveBeenCalledWith(true)
    expect(wrapper.find('[data-testid="mok-affiliate-tier-progress"]').exists()).toBe(false)
  })

  it('renders nothing when the status cannot be loaded', async () => {
    getAffiliateTierStatus.mockRejectedValue(new Error('network'))
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
    const wrapper = mount(AffiliateUserPanel, { global: { stubs: { AffiliateUsageRebateRecords: RecordsStub } } })
    await flushPromises()
    expect(wrapper.find('[data-testid="mok-affiliate-tier-progress"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="mok-affiliate-usage-card"]').exists()).toBe(false)
    expect(setAffiliateUsageWording).not.toHaveBeenCalled()
    errorSpy.mockRestore()
  })
})

describe('AffiliateUserPanel usage rebate card', () => {
  it('is hidden in recharge mode without records', async () => {
    const wrapper = await mountWith(status())
    expect(listMyAffiliateUsageRebates).toHaveBeenCalledWith({ page: 1, page_size: 1 })
    expect(wrapper.find('[data-testid="mok-affiliate-usage-card"]').exists()).toBe(false)
  })

  it('explains hourly settlement in usage mode before anything is settled', async () => {
    const wrapper = await mountWith(status({ rebate_mode: 'usage', enabled: false, tiers: [] }))

    const card = wrapper.get('[data-testid="mok-affiliate-usage-card"]')
    expect(card.text()).toContain('mok.affiliateUsage.user.usageOn')
    expect(card.get('[data-testid="mok-affiliate-usage-latest"]').text()).toBe('mok.affiliateUsage.user.none')
    expect(card.get('[data-testid="mok-affiliate-usage-open"]').attributes('disabled')).toBeDefined()
  })

  it('shows the latest record and opens the own records', async () => {
    listMyAffiliateUsageRebates.mockResolvedValue(records([record()], 37))
    const wrapper = await mountWith(status({ rebate_mode: 'usage' }))

    expect(wrapper.get('[data-testid="mok-affiliate-usage-latest"]').text()).toBe(
      'mok.affiliateUsage.user.latest {"period":"11:00~12:00","amount":"$2.4"}'
    )
    const open = wrapper.get('[data-testid="mok-affiliate-usage-open"]')
    expect(open.text()).toBe('mok.affiliateUsage.user.view {"count":37}')
    expect(wrapper.find('[data-testid="records"]').exists()).toBe(false)

    await open.trigger('click')
    expect(wrapper.get('[data-testid="records"]').text()).toBe('records:user')
    await wrapper.get('[data-testid="records"]').trigger('click')
    expect(wrapper.find('[data-testid="records"]').exists()).toBe(false)
  })

  it('keeps earlier records reachable after the switch back to recharge mode', async () => {
    listMyAffiliateUsageRebates.mockResolvedValue(records([record()], 5))
    const wrapper = await mountWith(status())

    const card = wrapper.get('[data-testid="mok-affiliate-usage-card"]')
    expect(card.text()).toContain('mok.affiliateUsage.user.usageOff')
    expect(card.get('[data-testid="mok-affiliate-usage-open"]').attributes('disabled')).toBeUndefined()
  })

  it('still shows the tier card when the records cannot be loaded', async () => {
    listMyAffiliateUsageRebates.mockRejectedValue(new Error('network'))
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
    const wrapper = await mountWith(status({ rebate_mode: 'usage' }))

    expect(wrapper.find('[data-testid="mok-affiliate-tier-progress"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="mok-affiliate-usage-latest"]').text()).toBe('mok.affiliateUsage.user.none')
    errorSpy.mockRestore()
  })
})
