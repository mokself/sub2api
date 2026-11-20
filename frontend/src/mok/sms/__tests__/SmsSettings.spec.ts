import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'

import SmsSettings from '../SmsSettings.vue'
import type { SmsConfigView } from '../../api/sms'

const { getSmsConfig, saveSmsConfig, sendTestSms, showSuccess, showError, copyToClipboard } = vi.hoisted(() => ({
  getSmsConfig: vi.fn(),
  saveSmsConfig: vi.fn(),
  sendTestSms: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
  copyToClipboard: vi.fn()
}))

vi.mock('../../api/sms', () => ({ getSmsConfig, saveSmsConfig, sendTestSms, listSmsLogs: vi.fn() }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess, showError }) }))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard }) }))
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

const ToggleStub = defineComponent({
  props: { modelValue: Boolean },
  emits: ['update:modelValue'],
  setup(props, { emit, attrs }) {
    return () =>
      h('button', { ...attrs, 'data-on': String(props.modelValue), onClick: () => emit('update:modelValue', !props.modelValue) })
  }
})

function configView(patch: Partial<SmsConfigView> = {}): SmsConfigView {
  return {
    enabled: false,
    dry_run: false,
    secret_id: '',
    secret_key: '',
    secret_key_set: false,
    sdk_app_id: '',
    sign_name: '',
    region: 'ap-guangzhou',
    daily_limit: 10,
    templates: { verify_code: '', balance_low: '', subscription_expiry: '', ops_alert: '', account_quota: '' },
    events: { balance_low: true, subscription_expiry: true },
    ...patch
  }
}

async function mountSettings() {
  const wrapper = mount(SmsSettings, { global: { stubs: { Toggle: ToggleStub, SmsLogs: true } } })
  await flushPromises()
  return wrapper
}

describe('SmsSettings', () => {
  beforeEach(() => vi.clearAllMocks())

  it('shows a retry when loading fails', async () => {
    getSmsConfig.mockRejectedValueOnce(new Error('down'))
    getSmsConfig.mockResolvedValueOnce(configView())
    const wrapper = await mountSettings()
    expect(wrapper.text()).toContain('mok.sms.loadFailed')

    await wrapper.get('[role="alert"] button').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="mok-sms-save"]').exists()).toBe(true)
  })

  it('saves the form, keeping a stored secret key when the field is left empty', async () => {
    getSmsConfig.mockResolvedValue(
      configView({ secret_id: 'id', secret_key_set: true, sdk_app_id: '1400000000', sign_name: '站点' })
    )
    saveSmsConfig.mockImplementation(async (config) => configView({ ...config, secret_key: '', secret_key_set: true }))
    const wrapper = await mountSettings()
    expect((wrapper.get('[data-testid="mok-sms-secret-key"]').element as HTMLInputElement).placeholder).toBe('mok.sms.secretKeySet')
    expect(wrapper.find('[data-testid="mok-sms-unsaved"]').exists()).toBe(false)

    await wrapper.get('[data-testid="mok-sms-enabled"]').trigger('click')
    await wrapper.get('[data-testid="mok-sms-template-id-balance_low"]').setValue(' 2002 ')
    await wrapper.get('[data-testid="mok-sms-event-subscription_expiry"]').trigger('click')
    expect(wrapper.find('[data-testid="mok-sms-unsaved"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="mok-sms-test-send"]').attributes('disabled')).toBeDefined()

    await wrapper.get('[data-testid="mok-sms-save"]').trigger('click')
    await flushPromises()

    const payload = saveSmsConfig.mock.calls[0][0]
    expect(payload.enabled).toBe(true)
    expect(payload.secret_key).toBe('')
    expect(payload.templates.balance_low).toBe(' 2002 ')
    expect(payload.events).toEqual({ balance_low: true, subscription_expiry: false })
    expect(payload).not.toHaveProperty('admin_phones')
    expect(showSuccess).toHaveBeenCalledWith('mok.sms.saved')
    expect(wrapper.find('[data-testid="mok-sms-unsaved"]').exists()).toBe(false)
  })

  it('switches only the user notifications; the admin alerts point to where they are set', async () => {
    getSmsConfig.mockResolvedValue(configView())
    const wrapper = await mountSettings()

    expect(wrapper.find('[data-testid="mok-sms-event-balance_low"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="mok-sms-event-subscription_expiry"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="mok-sms-event-ops_alert"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="mok-sms-event-account_quota"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="mok-sms-template-ops_alert"]').text()).toContain('mok.sms.kinds.ops_alert.switchedAt')
    expect(wrapper.get('[data-testid="mok-sms-template-account_quota"]').text()).toContain('mok.sms.kinds.account_quota.switchedAt')
    expect(wrapper.get('[data-testid="mok-sms-template-verify_code"]').text()).toContain('mok.sms.kinds.verify_code.switchedAt')
    expect(wrapper.find('[data-testid="mok-sms-template-id-ops_alert"]').exists()).toBe(true)
  })

  it('refuses invalid input before saving', async () => {
    getSmsConfig.mockResolvedValue(configView())
    const wrapper = await mountSettings()

    await wrapper.get('[data-testid="mok-sms-template-id-verify_code"]').setValue('SMS_1')
    await wrapper.get('[data-testid="mok-sms-save"]').trigger('click')
    expect(showError).toHaveBeenLastCalledWith('mok.sms.validation.templateId')

    await wrapper.get('[data-testid="mok-sms-template-id-verify_code"]').setValue('1')
    await wrapper.get('[data-testid="mok-sms-enabled"]').trigger('click')
    await wrapper.get('[data-testid="mok-sms-save"]').trigger('click')
    expect(showError).toHaveBeenLastCalledWith('mok.sms.validation.credentials')

    await wrapper.get('[data-testid="mok-sms-dry-run"]').trigger('click')
    saveSmsConfig.mockResolvedValue(configView({ enabled: true, dry_run: true }))
    await wrapper.get('[data-testid="mok-sms-save"]').trigger('click')
    await flushPromises()
    expect(saveSmsConfig).toHaveBeenCalledTimes(1)
  })

  it('sends a test message with the saved settings', async () => {
    getSmsConfig.mockResolvedValue(configView({ dry_run: true, templates: { verify_code: '1', balance_low: '2', subscription_expiry: '', ops_alert: '', account_quota: '' } }))
    sendTestSms.mockResolvedValueOnce({ status: 'dry_run', error: '' })
    sendTestSms.mockResolvedValueOnce({ status: 'failed', error: 'FailedOperation.TemplateParamSetNotMatchApprovedTemplate' })
    const wrapper = await mountSettings()

    await wrapper.get('[data-testid="mok-sms-test-phone"]').setValue('13800138000')
    await wrapper.get('[data-testid="mok-sms-test-kind"]').setValue('balance_low')
    await wrapper.get('[data-testid="mok-sms-test-send"]').trigger('click')
    await flushPromises()
    expect(sendTestSms).toHaveBeenCalledWith('13800138000', 'balance_low')
    expect(showSuccess).toHaveBeenCalledWith('mok.sms.test.dryRun')

    await wrapper.get('[data-testid="mok-sms-test-send"]').trigger('click')
    await flushPromises()
    expect(showError).toHaveBeenCalledWith('mok.sms.test.failed {"error":"FailedOperation.TemplateParamSetNotMatchApprovedTemplate"}')
  })

  it('copies the suggested template', async () => {
    getSmsConfig.mockResolvedValue(configView())
    const wrapper = await mountSettings()

    await wrapper.get('[data-testid="mok-sms-copy-subscription_expiry"]').trigger('click')
    expect(copyToClipboard).toHaveBeenCalledWith('您的订阅即将到期。订阅名称：{1}，到期时间：{2}，剩余天数：{3}。', 'mok.sms.copied')
  })
})
