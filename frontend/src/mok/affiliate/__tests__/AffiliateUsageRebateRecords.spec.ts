import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'

import AffiliateUsageRebateRecords from '../AffiliateUsageRebateRecords.vue'
import type { AffiliateUsageRebatePage, AffiliateUsageRebateRecord } from '../../api/affiliateUsageRebates'

const { listAffiliateUsageRebates, listMyAffiliateUsageRebates } = vi.hoisted(() => ({
  listAffiliateUsageRebates: vi.fn(),
  listMyAffiliateUsageRebates: vi.fn()
}))

vi.mock('../../api/affiliateUsageRebates', () => ({ listAffiliateUsageRebates, listMyAffiliateUsageRebates }))
vi.mock('@/utils/format', () => ({ formatCurrency: (amount: number) => `$${amount}` }))
vi.mock('../usageRebate', () => ({
  formatRebatePeriod: (start: string, end: string) => `${start}~${end}`,
  formatRebateRate: (rate: number) => String(rate)
}))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const BaseDialogStub = defineComponent({
  props: { show: Boolean, title: String },
  emits: ['close'],
  setup(props, { slots, emit }) {
    return () =>
      props.show
        ? h('section', [h('h3', props.title), h('button', { 'data-testid': 'dialog-close', onClick: () => emit('close') }), slots.default?.()])
        : null
  }
})

const PaginationStub = defineComponent({
  props: { total: Number, page: Number, pageSize: Number },
  emits: ['update:page'],
  setup(props, { emit }) {
    return () =>
      h('button', { 'data-testid': 'next-page', onClick: () => emit('update:page', (props.page ?? 1) + 1) }, `${props.page}/${props.total}/${props.pageSize}`)
  }
})

function record(patch: Partial<AffiliateUsageRebateRecord> = {}): AffiliateUsageRebateRecord {
  return {
    period_start: '11:00',
    period_end: '12:00',
    inviter_email: 'inviter@example.com',
    invitee_email: 'invitee@example.com',
    usage_amount: 20,
    rate_percent: 12,
    rebate_amount: 2.4,
    settled_at: '12:05',
    ...patch
  }
}

function pageOf(items: AffiliateUsageRebateRecord[], total = items.length): AffiliateUsageRebatePage {
  return { items, total, page: 1, page_size: 20, pages: 1 }
}

function mountRecords(scope: 'admin' | 'user', show = true) {
  return mount(AffiliateUsageRebateRecords, {
    props: { show, scope },
    global: { stubs: { BaseDialog: BaseDialogStub, Pagination: PaginationStub } }
  })
}

function rowTexts(wrapper: ReturnType<typeof mountRecords>): string[][] {
  return wrapper.findAll('[data-testid="mok-affiliate-usage-row"]').map((row) => row.findAll('td').map((cell) => cell.text()))
}

beforeEach(() => {
  vi.clearAllMocks()
  listAffiliateUsageRebates.mockResolvedValue(pageOf([record()]))
  listMyAffiliateUsageRebates.mockResolvedValue(pageOf([record({ inviter_email: undefined, invitee_email: 'i***@e***.com' })]))
})

describe('AffiliateUsageRebateRecords', () => {
  it('lists every record with inviter and invitee for admins', async () => {
    const wrapper = mountRecords('admin')
    await flushPromises()

    expect(listAffiliateUsageRebates).toHaveBeenCalledWith({ page: 1, page_size: 20, search: '' })
    expect(listMyAffiliateUsageRebates).not.toHaveBeenCalled()
    expect(wrapper.findAll('th')).toHaveLength(6)
    expect(rowTexts(wrapper)).toEqual([['11:00~12:00', 'inviter@example.com', 'invitee@example.com', '$20', '12%', '$2.4']])
    expect(wrapper.text()).toContain('mok.affiliateUsage.adminHint')
    expect(wrapper.find('[data-testid="next-page"]').exists()).toBe(false)
  })

  it('lists the own records without inviter column or search for users', async () => {
    const wrapper = mountRecords('user')
    await flushPromises()

    expect(listMyAffiliateUsageRebates).toHaveBeenCalledWith({ page: 1, page_size: 20, search: '' })
    expect(listAffiliateUsageRebates).not.toHaveBeenCalled()
    expect(wrapper.findAll('th')).toHaveLength(5)
    expect(rowTexts(wrapper)).toEqual([['11:00~12:00', 'i***@e***.com', '$20', '12%', '$2.4']])
    expect(wrapper.text()).toContain('mok.affiliateUsage.userHint')
    expect(wrapper.find('[data-testid="mok-affiliate-usage-search"]').exists()).toBe(false)
  })

  it('loads only once it is shown and starts over on every opening', async () => {
    const wrapper = mountRecords('admin', false)
    await flushPromises()
    expect(listAffiliateUsageRebates).not.toHaveBeenCalled()

    listAffiliateUsageRebates.mockResolvedValue(pageOf([record()], 45))
    await wrapper.setProps({ show: true })
    await flushPromises()
    await wrapper.get('[data-testid="mok-affiliate-usage-search"]').setValue('alice')
    await wrapper.get('[data-testid="mok-affiliate-usage-search-button"]').trigger('click')
    await wrapper.get('[data-testid="next-page"]').trigger('click')
    await flushPromises()
    expect(listAffiliateUsageRebates).toHaveBeenLastCalledWith({ page: 2, page_size: 20, search: 'alice' })

    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true })
    await flushPromises()
    expect(listAffiliateUsageRebates).toHaveBeenLastCalledWith({ page: 1, page_size: 20, search: '' })
    expect((wrapper.get('[data-testid="mok-affiliate-usage-search"]').element as HTMLInputElement).value).toBe('')
  })

  it('pages through the records', async () => {
    listAffiliateUsageRebates.mockResolvedValue(pageOf([record()], 45))
    const wrapper = mountRecords('admin')
    await flushPromises()
    expect(wrapper.get('[data-testid="next-page"]').text()).toBe('1/45/20')

    listAffiliateUsageRebates.mockResolvedValue(pageOf([record({ period_start: '10:00', period_end: '11:00' })], 45))
    await wrapper.get('[data-testid="next-page"]').trigger('click')
    await flushPromises()

    expect(listAffiliateUsageRebates).toHaveBeenLastCalledWith({ page: 2, page_size: 20, search: '' })
    expect(rowTexts(wrapper)[0][0]).toBe('10:00~11:00')
    expect(wrapper.get('[data-testid="next-page"]').text()).toBe('2/45/20')
  })

  it('searches from the first page with Enter, without submitting a surrounding form', async () => {
    listAffiliateUsageRebates.mockResolvedValue(pageOf([record()], 45))
    const wrapper = mountRecords('admin')
    await flushPromises()
    await wrapper.get('[data-testid="next-page"]').trigger('click')
    await flushPromises()

    listAffiliateUsageRebates.mockResolvedValue(pageOf([]))
    const input = wrapper.get('[data-testid="mok-affiliate-usage-search"]')
    await input.setValue('  nobody  ')
    const enter = new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true })
    input.element.dispatchEvent(enter)
    await flushPromises()

    expect(enter.defaultPrevented).toBe(true)
    expect(listAffiliateUsageRebates).toHaveBeenLastCalledWith({ page: 1, page_size: 20, search: 'nobody' })
    expect(wrapper.get('[data-testid="mok-affiliate-usage-empty"]').text()).toBe('mok.affiliateUsage.noMatch')
  })

  it('says so when nothing is settled yet', async () => {
    listAffiliateUsageRebates.mockResolvedValue(pageOf([]))
    const wrapper = mountRecords('admin')
    await flushPromises()
    expect(wrapper.get('[data-testid="mok-affiliate-usage-empty"]').text()).toBe('mok.affiliateUsage.empty')
  })

  it('offers a retry when loading fails', async () => {
    listAffiliateUsageRebates.mockRejectedValueOnce(new Error('network'))
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
    const wrapper = mountRecords('admin')
    await flushPromises()

    expect(wrapper.get('[role="alert"]').text()).toContain('mok.affiliateUsage.loadFailed')
    await wrapper.get('[role="alert"] button').trigger('click')
    await flushPromises()

    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(rowTexts(wrapper)).toHaveLength(1)
    errorSpy.mockRestore()
  })

  it('shows the latest request even when an earlier one answers later', async () => {
    let answerFirst: (page: AffiliateUsageRebatePage) => void = () => {}
    listAffiliateUsageRebates.mockReturnValueOnce(new Promise<AffiliateUsageRebatePage>((resolve) => (answerFirst = resolve)))
    const wrapper = mountRecords('admin')
    await flushPromises()

    listAffiliateUsageRebates.mockResolvedValue(pageOf([record({ invitee_email: 'second@example.com' })]))
    await wrapper.get('[data-testid="mok-affiliate-usage-search"]').setValue('second')
    await wrapper.get('[data-testid="mok-affiliate-usage-search-button"]').trigger('click')
    await flushPromises()
    answerFirst(pageOf([record({ invitee_email: 'first@example.com' })]))
    await flushPromises()

    expect(rowTexts(wrapper)[0][2]).toBe('second@example.com')
  })

  it('asks to close', async () => {
    const wrapper = mountRecords('user')
    await wrapper.get('[data-testid="dialog-close"]').trigger('click')
    expect(wrapper.emitted('close')).toHaveLength(1)
  })
})
