import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'

import AffiliateRebateModeSettings from '../AffiliateRebateModeSettings.vue'
import type { AffiliateRebateModeStatus } from '../../api/affiliateMode'

const { getAffiliateRebateMode, setAffiliateRebateMode, showSuccess, showError } = vi.hoisted(() => ({
  getAffiliateRebateMode: vi.fn(),
  setAffiliateRebateMode: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn()
}))

vi.mock('../../api/affiliateMode', () => ({ getAffiliateRebateMode, setAffiliateRebateMode }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess, showError }) }))
vi.mock('@/utils/format', () => ({
  formatCurrency: (amount: number) => `$${amount}`,
  formatDateTime: (value: string) => `[${value}]`
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

// Renders the dialog inline so the test can read its message and press its buttons.
const ConfirmDialogStub = defineComponent({
  props: { show: Boolean, message: String },
  emits: ['confirm', 'cancel'],
  setup(props, { emit }) {
    return () =>
      props.show
        ? h('div', { 'data-testid': 'confirm' }, [
            h('p', props.message),
            h('button', { 'data-testid': 'confirm-ok', onClick: () => emit('confirm') }),
            h('button', { 'data-testid': 'confirm-cancel', onClick: () => emit('cancel') })
          ])
        : null
  }
})

function modeStatus(patch: Partial<AffiliateRebateModeStatus> = {}): AffiliateRebateModeStatus {
  return { mode: 'recharge', recent_days: [], ...patch }
}

async function mountLoaded() {
  const wrapper = mount(AffiliateRebateModeSettings, {
    global: { stubs: { ConfirmDialog: ConfirmDialogStub } }
  })
  await flushPromises()
  return wrapper
}

type Wrapper = Awaited<ReturnType<typeof mountLoaded>>

function pressed(wrapper: Wrapper, mode: string): string | undefined {
  return wrapper.get(`[data-testid="mok-affiliate-mode-${mode}"]`).attributes('aria-pressed')
}

beforeEach(() => {
  vi.clearAllMocks()
  getAffiliateRebateMode.mockResolvedValue(modeStatus())
})

describe('AffiliateRebateModeSettings', () => {
  it('shows recharge mode without a settlement table', async () => {
    const wrapper = await mountLoaded()
    expect(pressed(wrapper, 'recharge')).toBe('true')
    expect(pressed(wrapper, 'usage')).toBe('false')
    expect(wrapper.find('[data-testid="mok-affiliate-mode-window"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('mok.affiliateMode.recentDays')
  })

  it('switches to usage mode after confirmation', async () => {
    setAffiliateRebateMode.mockResolvedValue(modeStatus({ mode: 'usage', usage_since: '2026-11-09T08:00:00Z' }))
    const wrapper = await mountLoaded()

    await wrapper.get('[data-testid="mok-affiliate-mode-usage"]').trigger('click')
    expect(wrapper.get('[data-testid="confirm"]').text()).toContain('mok.affiliateMode.confirmToUsage')
    expect(setAffiliateRebateMode).not.toHaveBeenCalled()

    await wrapper.get('[data-testid="confirm-ok"]').trigger('click')
    await flushPromises()

    expect(setAffiliateRebateMode).toHaveBeenCalledWith('usage')
    expect(showSuccess).toHaveBeenCalledWith('mok.affiliateMode.switched')
    expect(pressed(wrapper, 'usage')).toBe('true')
    expect(wrapper.get('[data-testid="mok-affiliate-mode-window"]').text()).toBe(
      'mok.affiliateMode.since {"time":"[2026-11-09T08:00:00Z]"}'
    )
    expect(wrapper.text()).toContain('mok.affiliateMode.noDays')
    expect(wrapper.find('[data-testid="confirm"]').exists()).toBe(false)
  })

  it('keeps the mode when the switch is cancelled or the current mode is clicked', async () => {
    const wrapper = await mountLoaded()
    await wrapper.get('[data-testid="mok-affiliate-mode-recharge"]').trigger('click')
    expect(wrapper.find('[data-testid="confirm"]').exists()).toBe(false)

    await wrapper.get('[data-testid="mok-affiliate-mode-usage"]').trigger('click')
    await wrapper.get('[data-testid="confirm-cancel"]').trigger('click')
    await flushPromises()

    expect(setAffiliateRebateMode).not.toHaveBeenCalled()
    expect(pressed(wrapper, 'recharge')).toBe('true')
  })

  it('reports a failed switch and keeps the old mode', async () => {
    setAffiliateRebateMode.mockRejectedValue(new Error('boom'))
    const wrapper = await mountLoaded()

    await wrapper.get('[data-testid="mok-affiliate-mode-usage"]').trigger('click')
    await wrapper.get('[data-testid="confirm-ok"]').trigger('click')
    await flushPromises()

    expect(showError).toHaveBeenCalledTimes(1)
    expect(showSuccess).not.toHaveBeenCalled()
    expect(pressed(wrapper, 'recharge')).toBe('true')
  })

  it('lists settled days, including skipped ones, and the end of the usage window', async () => {
    getAffiliateRebateMode.mockResolvedValue(
      modeStatus({
        usage_since: '2026-11-01T00:00:00Z',
        usage_until: '2026-11-08T00:00:00Z',
        recent_days: [
          {
            day: '2026-11-07',
            window_start: '',
            window_end: '',
            invitee_count: 3,
            usage_total: 120.5,
            rebate_total: 12.05,
            settled_at: ''
          },
          {
            day: '2026-11-06',
            window_start: '',
            window_end: '',
            invitee_count: 0,
            usage_total: 0,
            rebate_total: 0,
            skipped_reason: 'affiliate_disabled',
            settled_at: ''
          }
        ]
      })
    )
    const wrapper = await mountLoaded()

    expect(wrapper.get('[data-testid="mok-affiliate-mode-window"]').text()).toBe(
      'mok.affiliateMode.until {"time":"[2026-11-08T00:00:00Z]"}'
    )
    const rows = wrapper.findAll('[data-testid="mok-affiliate-mode-day"]')
    expect(rows.map((row) => row.findAll('td').map((cell) => cell.text()))).toEqual([
      ['2026-11-07', '3', '$120.5', '$12.05'],
      ['2026-11-06', 'mok.affiliateMode.skipped.affiliate_disabled']
    ])
  })

  it('offers a retry when the mode cannot be loaded', async () => {
    getAffiliateRebateMode.mockRejectedValueOnce(new Error('network'))
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
    const wrapper = await mountLoaded()

    expect(wrapper.get('[role="alert"]').text()).toContain('mok.affiliateMode.loadFailed')
    await wrapper.get('[role="alert"] button').trigger('click')
    await flushPromises()

    expect(getAffiliateRebateMode).toHaveBeenCalledTimes(2)
    expect(pressed(wrapper, 'recharge')).toBe('true')
    errorSpy.mockRestore()
  })
})
