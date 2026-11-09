import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AffiliateRebateTiersSettings from '../AffiliateRebateTiersSettings.vue'

const { getAffiliateRebateTiers, saveAffiliateRebateTiers, showSuccess, showError } = vi.hoisted(() => ({
  getAffiliateRebateTiers: vi.fn(),
  saveAffiliateRebateTiers: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn()
}))

vi.mock('../../api/affiliateTiers', () => ({ getAffiliateRebateTiers, saveAffiliateRebateTiers }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess, showError }) }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

async function mountLoaded() {
  const wrapper = mount(AffiliateRebateTiersSettings)
  await flushPromises()
  return wrapper
}

beforeEach(() => {
  vi.clearAllMocks()
  getAffiliateRebateTiers.mockResolvedValue([
    { min_amount: 50000, rate_percent: 12 },
    { min_amount: 100000, rate_percent: 15 }
  ])
  saveAffiliateRebateTiers.mockImplementation(async (tiers) => tiers)
})

describe('AffiliateRebateTiersSettings', () => {
  it('shows the saved tiers with nothing to save', async () => {
    const wrapper = await mountLoaded()
    const rows = wrapper.findAll('[data-testid="mok-affiliate-tiers-row"]')
    expect(rows).toHaveLength(2)
    expect((rows[0].get('[data-testid="mok-affiliate-tiers-min"]').element as HTMLInputElement).value).toBe('50000')
    expect(wrapper.get('[data-testid="mok-affiliate-tiers-save"]').attributes('disabled')).toBeDefined()
    expect(wrapper.find('[data-testid="mok-affiliate-tiers-dirty"]').exists()).toBe(false)
  })

  it('blocks saving an incomplete tier and saves it once valid', async () => {
    const wrapper = await mountLoaded()
    await wrapper.get('[data-testid="mok-affiliate-tiers-add"]').trigger('click')

    const rows = wrapper.findAll('[data-testid="mok-affiliate-tiers-row"]')
    expect(rows).toHaveLength(3)
    expect(wrapper.get('[data-testid="mok-affiliate-tiers-error"]').text()).toBe('mok.affiliateTiers.errors.minAmountInvalid')
    expect(wrapper.get('[data-testid="mok-affiliate-tiers-save"]').attributes('disabled')).toBeDefined()

    await rows[2].get('[data-testid="mok-affiliate-tiers-min"]').setValue('20000')
    await rows[2].get('[data-testid="mok-affiliate-tiers-rate"]').setValue('11')
    expect(wrapper.find('[data-testid="mok-affiliate-tiers-error"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="mok-affiliate-tiers-dirty"]').exists()).toBe(true)

    await wrapper.get('[data-testid="mok-affiliate-tiers-save"]').trigger('click')
    await flushPromises()

    expect(saveAffiliateRebateTiers).toHaveBeenCalledWith([
      { min_amount: 20000, rate_percent: 11 },
      { min_amount: 50000, rate_percent: 12 },
      { min_amount: 100000, rate_percent: 15 }
    ])
    expect(showSuccess).toHaveBeenCalledWith('mok.affiliateTiers.saved')
    expect(wrapper.find('[data-testid="mok-affiliate-tiers-dirty"]').exists()).toBe(false)
  })

  it('flags a duplicate threshold', async () => {
    const wrapper = await mountLoaded()
    const rows = wrapper.findAll('[data-testid="mok-affiliate-tiers-row"]')
    await rows[1].get('[data-testid="mok-affiliate-tiers-min"]').setValue('50000')
    expect(wrapper.get('[data-testid="mok-affiliate-tiers-error"]').text()).toBe('mok.affiliateTiers.errors.minAmountDuplicate')
    expect(wrapper.get('[data-testid="mok-affiliate-tiers-save"]').attributes('disabled')).toBeDefined()
  })

  it('saves an empty list after removing every tier', async () => {
    const wrapper = await mountLoaded()
    await wrapper.findAll('[data-testid="mok-affiliate-tiers-remove"]')[0].trigger('click')
    await wrapper.findAll('[data-testid="mok-affiliate-tiers-remove"]')[0].trigger('click')
    expect(wrapper.find('[data-testid="mok-affiliate-tiers-empty"]').exists()).toBe(true)

    await wrapper.get('[data-testid="mok-affiliate-tiers-save"]').trigger('click')
    await flushPromises()
    expect(saveAffiliateRebateTiers).toHaveBeenCalledWith([])
  })

  it('reports a failed save and keeps the edits', async () => {
    saveAffiliateRebateTiers.mockRejectedValueOnce({ message: 'tier 1: rate_percent must be between 0 and 100' })
    const wrapper = await mountLoaded()
    await wrapper.findAll('[data-testid="mok-affiliate-tiers-rate"]')[0].setValue('13')
    await wrapper.get('[data-testid="mok-affiliate-tiers-save"]').trigger('click')
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('tier 1: rate_percent must be between 0 and 100')
    expect(wrapper.find('[data-testid="mok-affiliate-tiers-dirty"]').exists()).toBe(true)
  })

  it('offers a retry when loading fails', async () => {
    getAffiliateRebateTiers.mockRejectedValueOnce(new Error('network'))
    const wrapper = await mountLoaded()
    expect(wrapper.find('[role="alert"]').text()).toContain('mok.affiliateTiers.loadFailed')

    await wrapper.get('[role="alert"] button').trigger('click')
    await flushPromises()
    expect(wrapper.findAll('[data-testid="mok-affiliate-tiers-row"]')).toHaveLength(2)
  })
})
