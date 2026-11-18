import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import SmsNotifyPhone from '../SmsNotifyPhone.vue'
import type { SmsPhoneStatus } from '../../api/sms'

const { getMySmsPhone, sendSmsBindCode, bindSmsPhone, setSmsPhoneEnabled, removeSmsPhone, showSuccess, showError } = vi.hoisted(() => ({
  getMySmsPhone: vi.fn(),
  sendSmsBindCode: vi.fn(),
  bindSmsPhone: vi.fn(),
  setSmsPhoneEnabled: vi.fn(),
  removeSmsPhone: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn()
}))

vi.mock('../../api/sms', () => ({ getMySmsPhone, sendSmsBindCode, bindSmsPhone, setSmsPhoneEnabled, removeSmsPhone }))
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

function phoneStatus(patch: Partial<SmsPhoneStatus> = {}): SmsPhoneStatus {
  return { available: true, phone: '', enabled: false, events: ['balance_low', 'subscription_expiry'], ...patch }
}

async function mountPhone(props: { standalone?: boolean } = {}) {
  const wrapper = mount(SmsNotifyPhone, { props })
  await flushPromises()
  return wrapper
}

describe('SmsNotifyPhone', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.useRealTimers()
  })

  it('stays hidden until the admin has set SMS reminders up', async () => {
    getMySmsPhone.mockResolvedValue(phoneStatus({ available: false }))
    expect((await mountPhone()).find('[data-testid="mok-sms-phone"]').exists()).toBe(false)

    getMySmsPhone.mockRejectedValue(new Error('down'))
    expect((await mountPhone()).find('[data-testid="mok-sms-phone"]').exists()).toBe(false)
  })

  it('names the reminders that reach the phone', async () => {
    getMySmsPhone.mockResolvedValue(phoneStatus())
    const inCard = await mountPhone()
    expect(inCard.text()).toContain('mok.sms.user.events.balance_low')
    expect(inCard.text()).toContain('mok.sms.user.events.subscription_expiry')
    expect(inCard.find('.card').exists()).toBe(false)

    // On its own card the low balance reminder is off upstream, so only the expiry reminder counts.
    const standalone = await mountPhone({ standalone: true })
    expect(standalone.find('.card').exists()).toBe(true)
    expect(standalone.text()).not.toContain('mok.sms.user.events.balance_low')
    expect(standalone.text()).toContain('mok.sms.user.events.subscription_expiry')

    getMySmsPhone.mockResolvedValue(phoneStatus({ events: ['balance_low'] }))
    expect((await mountPhone({ standalone: true })).find('[data-testid="mok-sms-phone"]').exists()).toBe(false)
  })

  it('binds a phone with a code', async () => {
    vi.useFakeTimers()
    getMySmsPhone.mockResolvedValue(phoneStatus())
    sendSmsBindCode.mockResolvedValue({ cooldown_seconds: 60, expires_in_seconds: 300 })
    bindSmsPhone.mockResolvedValue(phoneStatus({ phone: '13800138000', enabled: true }))
    const wrapper = await mountPhone()

    await wrapper.get('[data-testid="mok-sms-phone-input"]').setValue('12345')
    await wrapper.get('[data-testid="mok-sms-phone-send"]').trigger('click')
    expect(sendSmsBindCode).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('mok.sms.errors.MOK_SMS_PHONE_INVALID')

    await wrapper.get('[data-testid="mok-sms-phone-input"]').setValue('+86 138 0013 8000')
    await wrapper.get('[data-testid="mok-sms-phone-send"]').trigger('click')
    await flushPromises()
    expect(sendSmsBindCode).toHaveBeenCalledWith('13800138000')
    const send = wrapper.get('[data-testid="mok-sms-phone-send"]')
    expect(send.attributes('disabled')).toBeDefined()
    expect(send.text()).toContain('"seconds":60')
    vi.advanceTimersByTime(60_000)
    await flushPromises()
    expect(wrapper.get('[data-testid="mok-sms-phone-send"]').text()).toBe('mok.sms.user.resend')

    const bind = wrapper.get('[data-testid="mok-sms-phone-bind"]')
    expect(bind.attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="mok-sms-phone-code"]').setValue('123456')
    await bind.trigger('click')
    await flushPromises()
    expect(bindSmsPhone).toHaveBeenCalledWith('13800138000', '123456')
    expect(wrapper.get('[data-testid="mok-sms-phone-bound"]').text()).toContain('+86 13800138000')
    expect(showSuccess).toHaveBeenLastCalledWith('mok.sms.user.bound')
  })

  it('shows the backend reason when the code is refused', async () => {
    getMySmsPhone.mockResolvedValue(phoneStatus())
    sendSmsBindCode.mockRejectedValue({ reason: 'MOK_SMS_CODE_COOLDOWN', metadata: { retry_after: '17' } })
    const wrapper = await mountPhone()

    await wrapper.get('[data-testid="mok-sms-phone-input"]').setValue('13800138000')
    await wrapper.get('[data-testid="mok-sms-phone-send"]').trigger('click')
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('mok.sms.errors.MOK_SMS_CODE_COOLDOWN {"seconds":17}')
    expect(wrapper.find('[data-testid="mok-sms-phone-code"]').exists()).toBe(false)
  })

  it('pauses, resumes and unbinds the phone', async () => {
    getMySmsPhone.mockResolvedValue(phoneStatus({ phone: '13800138000', enabled: true }))
    setSmsPhoneEnabled.mockResolvedValueOnce(phoneStatus({ phone: '13800138000', enabled: false }))
    setSmsPhoneEnabled.mockRejectedValueOnce(new Error('down'))
    removeSmsPhone.mockResolvedValue(phoneStatus())
    const wrapper = await mountPhone()
    const box = () => wrapper.get('[data-testid="mok-sms-phone-toggle"]').element as HTMLInputElement

    await wrapper.get('[data-testid="mok-sms-phone-toggle"]').setValue(false)
    await flushPromises()
    expect(setSmsPhoneEnabled).toHaveBeenLastCalledWith(false)
    expect(showSuccess).toHaveBeenLastCalledWith('mok.sms.user.paused')
    expect(box().checked).toBe(false)

    await wrapper.get('[data-testid="mok-sms-phone-toggle"]').setValue(true)
    await flushPromises()
    expect(setSmsPhoneEnabled).toHaveBeenLastCalledWith(true)
    expect(showError).toHaveBeenCalledWith('down')
    expect(box().checked, 'a failed switch puts the box back').toBe(false)

    await wrapper.get('[data-testid="mok-sms-phone-remove"]').trigger('click')
    await flushPromises()
    expect(removeSmsPhone).toHaveBeenCalled()
    expect(wrapper.find('[data-testid="mok-sms-phone-bound"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="mok-sms-phone-input"]').exists()).toBe(true)
  })
})
