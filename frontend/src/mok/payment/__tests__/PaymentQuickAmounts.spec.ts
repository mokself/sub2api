import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import PaymentQuickAmounts from '../PaymentQuickAmounts.vue'

const { getAdminPaymentQuickAmounts, savePaymentQuickAmounts, setQuickAmounts, showSuccess, showError } = vi.hoisted(() => ({
  getAdminPaymentQuickAmounts: vi.fn(),
  savePaymentQuickAmounts: vi.fn(),
  setQuickAmounts: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn()
}))

vi.mock('../../api/paymentQuickAmounts', () => ({ getAdminPaymentQuickAmounts, savePaymentQuickAmounts }))
vi.mock('../quickAmounts', async () => {
  const actual = await vi.importActual<typeof import('../quickAmounts')>('../quickAmounts')
  return { ...actual, setQuickAmounts }
})
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess, showError }) }))
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

async function mountCard() {
  const wrapper = mount(PaymentQuickAmounts)
  await flushPromises()
  return wrapper
}

const preview = (wrapper: Awaited<ReturnType<typeof mountCard>>) =>
  wrapper.findAll('[data-testid="mok-quick-amounts-preview"] span').map((span) => span.text())

describe('PaymentQuickAmounts', () => {
  beforeEach(() => vi.clearAllMocks())

  it('previews the defaults while no list is set', async () => {
    getAdminPaymentQuickAmounts.mockResolvedValue({ amounts: [] })
    const wrapper = await mountCard()

    expect(preview(wrapper)).toEqual(['10', '20', '50', '100', '200', '500', '1000', '2000', '5000'])
    expect(wrapper.text()).toContain('mok.paymentQuickAmounts.usingDefault')
    expect(wrapper.get('[data-testid="mok-quick-amounts-reset"]').attributes('disabled')).toBeDefined()
    expect(setQuickAmounts).toHaveBeenCalledWith([])
  })

  it('saves a list and shows it sorted', async () => {
    getAdminPaymentQuickAmounts.mockResolvedValue({ amounts: [] })
    savePaymentQuickAmounts.mockResolvedValue({ amounts: [30, 88, 200] })
    const wrapper = await mountCard()

    await wrapper.get('[data-testid="mok-quick-amounts-input"]').setValue('200, 30 88')
    expect(preview(wrapper)).toEqual(['30', '88', '200'])
    expect(wrapper.find('[data-testid="mok-quick-amounts-unsaved"]').exists()).toBe(true)

    await wrapper.get('[data-testid="mok-quick-amounts-save"]').trigger('click')
    await flushPromises()

    expect(savePaymentQuickAmounts).toHaveBeenCalledWith([30, 88, 200])
    expect(setQuickAmounts).toHaveBeenLastCalledWith([30, 88, 200])
    expect(showSuccess).toHaveBeenCalledWith('mok.paymentQuickAmounts.saved')
    expect((wrapper.get('[data-testid="mok-quick-amounts-input"]').element as HTMLInputElement).value).toBe('30, 88, 200')
    expect(wrapper.find('[data-testid="mok-quick-amounts-unsaved"]').exists()).toBe(false)
  })

  it('blocks invalid input', async () => {
    getAdminPaymentQuickAmounts.mockResolvedValue({ amounts: [50] })
    const wrapper = await mountCard()

    await wrapper.get('[data-testid="mok-quick-amounts-input"]').setValue('50, 9.999')
    expect(wrapper.get('[data-testid="mok-quick-amounts-error"]').text()).toContain('mok.paymentQuickAmounts.errors.decimals')
    expect(wrapper.get('[data-testid="mok-quick-amounts-save"]').attributes('disabled')).toBeDefined()
  })

  it('restores the defaults', async () => {
    getAdminPaymentQuickAmounts.mockResolvedValue({ amounts: [50, 100] })
    savePaymentQuickAmounts.mockResolvedValue({ amounts: [] })
    const wrapper = await mountCard()

    await wrapper.get('[data-testid="mok-quick-amounts-reset"]').trigger('click')
    await flushPromises()

    expect(savePaymentQuickAmounts).toHaveBeenCalledWith([])
    expect(showSuccess).toHaveBeenCalledWith('mok.paymentQuickAmounts.resetDone')
    expect((wrapper.get('[data-testid="mok-quick-amounts-input"]').element as HTMLInputElement).value).toBe('')
  })

  it('shows a retry when loading fails', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    getAdminPaymentQuickAmounts.mockRejectedValueOnce(new Error('down'))
    getAdminPaymentQuickAmounts.mockResolvedValueOnce({ amounts: [] })
    const wrapper = await mountCard()
    expect(wrapper.text()).toContain('mok.paymentQuickAmounts.loadFailed')

    await wrapper.get('[role="alert"] button').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="mok-quick-amounts-input"]').exists()).toBe(true)
  })
})
