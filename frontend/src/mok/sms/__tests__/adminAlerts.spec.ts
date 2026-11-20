import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'

import AccountQuotaSmsPhones from '../AccountQuotaSmsPhones.vue'
import OpsAlertSms from '../OpsAlertSms.vue'
import {
  accountQuotaState,
  mokSaveAccountQuotaSms,
  mokSaveOpsAlertSms,
  opsAlertState
} from '../adminAlerts'

const { getSmsOpsAlert, saveSmsOpsAlert, getSmsAccountQuota, saveSmsAccountQuota, showError } = vi.hoisted(() => ({
  getSmsOpsAlert: vi.fn(),
  saveSmsOpsAlert: vi.fn(),
  getSmsAccountQuota: vi.fn(),
  saveSmsAccountQuota: vi.fn(),
  showError: vi.fn()
}))

vi.mock('../../api/sms', () => ({ getSmsOpsAlert, saveSmsOpsAlert, getSmsAccountQuota, saveSmsAccountQuota }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

enableAutoUnmount(afterEach)

const ToggleStub = defineComponent({
  props: { modelValue: Boolean },
  emits: ['update:modelValue'],
  setup(props, { emit, attrs }) {
    return () => h('button', { ...attrs, 'data-on': String(props.modelValue), onClick: () => emit('update:modelValue', !props.modelValue) })
  }
})
const SelectStub = defineComponent({
  props: { modelValue: String, options: Array },
  emits: ['update:modelValue'],
  setup(props, { emit, attrs }) {
    return () =>
      h(
        'select',
        { ...attrs, value: props.modelValue, onChange: (e: Event) => emit('update:modelValue', (e.target as HTMLSelectElement).value) },
        (props.options as { value: string; label: string }[]).map((o) => h('option', { value: o.value }, o.label))
      )
  }
})

function resetState() {
  opsAlertState.status = 'idle'
  accountQuotaState.status = 'idle'
}

describe('ops alert SMS section', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    resetState()
  })

  async function mountSection() {
    const wrapper = mount(OpsAlertSms, { global: { stubs: { Toggle: ToggleStub, Select: SelectStub } } })
    await flushPromises()
    return wrapper
  }

  it('edits the switch, phones and level; the dialog save writes them', async () => {
    getSmsOpsAlert.mockResolvedValue({ enabled: false, phones: [], min_level: '', ready: true })
    saveSmsOpsAlert.mockImplementation(async (ops) => ({ ...ops, ready: true }))
    const wrapper = await mountSection()

    await mokSaveOpsAlertSms()
    expect(saveSmsOpsAlert).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="mok-ops-alert-sms-phone"]').exists()).toBe(false)

    await wrapper.get('[data-testid="mok-ops-alert-sms-enabled"]').trigger('click')
    await wrapper.get('[data-testid="mok-ops-alert-sms-phone"]').setValue('12345')
    await wrapper.get('[data-testid="mok-ops-alert-sms-add"]').trigger('click')
    expect(showError).toHaveBeenCalledWith('mok.smsOps.invalidPhone')

    await wrapper.get('[data-testid="mok-ops-alert-sms-phone"]').setValue('+86 138 0013 8000')
    await wrapper.get('[data-testid="mok-ops-alert-sms-add"]').trigger('click')
    await wrapper.get('[data-testid="mok-ops-alert-sms-phone"]').setValue('13800138000')
    await wrapper.get('[data-testid="mok-ops-alert-sms-add"]').trigger('click')
    expect(showError).toHaveBeenLastCalledWith('mok.smsOps.duplicate')
    expect(wrapper.findAll('[data-testid="mok-ops-alert-sms-chip"]').map((c) => c.text().replace('×', '').trim())).toEqual(['13800138000'])
    await wrapper.get('[data-testid="mok-ops-alert-sms-level"]').setValue('critical')

    await mokSaveOpsAlertSms()
    expect(saveSmsOpsAlert).toHaveBeenCalledWith({ enabled: true, phones: ['13800138000'], min_level: 'critical' })

    saveSmsOpsAlert.mockClear()
    await mokSaveOpsAlertSms()
    expect(saveSmsOpsAlert).not.toHaveBeenCalled()
  })

  it('switches off without a phone, warns when SMS is not set up, and reports a failed save', async () => {
    getSmsOpsAlert.mockResolvedValue({ enabled: false, phones: [], min_level: '', ready: false })
    saveSmsOpsAlert.mockRejectedValueOnce({ message: 'boom' })
    const wrapper = await mountSection()

    await wrapper.get('[data-testid="mok-ops-alert-sms-enabled"]').trigger('click')
    expect(wrapper.find('[data-testid="mok-ops-alert-sms-not-ready"]').exists()).toBe(true)
    await wrapper.get('[data-testid="mok-ops-alert-sms-phone"]').setValue('13800138000')
    await wrapper.get('[data-testid="mok-ops-alert-sms-add"]').trigger('click')

    // The dialog shows response.data.message and stays open.
    await expect(mokSaveOpsAlertSms()).rejects.toMatchObject({ message: 'boom', response: { data: { message: 'boom' } } })

    saveSmsOpsAlert.mockImplementation(async (ops) => ({ ...ops, ready: false }))
    await wrapper.get('[data-testid="mok-ops-alert-sms-chip"] button').trigger('click')
    await mokSaveOpsAlertSms()
    expect(saveSmsOpsAlert).toHaveBeenLastCalledWith({ enabled: false, phones: [], min_level: '' })
  })

  it('does nothing when the settings could not be loaded', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    getSmsOpsAlert.mockRejectedValue(new Error('down'))
    const wrapper = await mountSection()
    expect(wrapper.text()).toContain('mok.smsOps.loadFailed')

    await mokSaveOpsAlertSms()
    expect(saveSmsOpsAlert).not.toHaveBeenCalled()
  })
})

describe('account quota SMS phones', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    resetState()
  })

  async function mountPhones() {
    const wrapper = mount(AccountQuotaSmsPhones)
    await flushPromises()
    return wrapper
  }

  it('edits the phones like the email list; the page save writes them', async () => {
    getSmsAccountQuota.mockResolvedValue({ phones: [{ phone: '13800138000', disabled: false }], ready: true })
    saveSmsAccountQuota.mockImplementation(async (quota) => ({ ...quota, ready: true }))
    const wrapper = await mountPhones()
    expect(wrapper.findAll('[data-testid="mok-quota-sms-row"]')).toHaveLength(1)

    await mokSaveAccountQuotaSms()
    expect(saveSmsAccountQuota).not.toHaveBeenCalled()

    await wrapper.get('[data-testid="mok-quota-sms-add"]').trigger('click')
    await wrapper.findAll('[data-testid="mok-quota-sms-phone"]')[1].setValue('139 0013 9000')
    await wrapper.findAll('[data-testid="mok-quota-sms-row"]')[0].get('input[type="checkbox"]').setValue(false)
    await wrapper.get('[data-testid="mok-quota-sms-add"]').trigger('click') // an empty row is skipped

    await mokSaveAccountQuotaSms()
    expect(saveSmsAccountQuota).toHaveBeenCalledWith({
      phones: [
        { phone: '13800138000', disabled: true },
        { phone: '13900139000', disabled: false }
      ]
    })
    expect(wrapper.findAll('[data-testid="mok-quota-sms-row"]')).toHaveLength(2)
  })

  it('refuses an invalid number with a message the page shows', async () => {
    getSmsAccountQuota.mockResolvedValue({ phones: [], ready: true })
    const wrapper = await mountPhones()

    await wrapper.get('[data-testid="mok-quota-sms-add"]').trigger('click')
    await wrapper.get('[data-testid="mok-quota-sms-phone"]').setValue('12345')

    await expect(mokSaveAccountQuotaSms()).rejects.toMatchObject({ message: 'mok.smsQuota.invalidPhone' })
    expect(saveSmsAccountQuota).not.toHaveBeenCalled()
  })

  it('warns when SMS is not set up for these alerts', async () => {
    getSmsAccountQuota.mockResolvedValue({ phones: [{ phone: '13800138000', disabled: false }], ready: false })
    const wrapper = await mountPhones()
    expect(wrapper.find('[data-testid="mok-quota-sms-not-ready"]').exists()).toBe(true)
  })
})
