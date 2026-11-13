import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'

import AffiliateScopeSettings from '../AffiliateScopeSettings.vue'
import type { AffiliateScopeStatus, AffiliateSelectedUser } from '../../api/affiliateScope'

const {
  getAffiliateScope,
  setAffiliateScope,
  listAffiliateSelectedUsers,
  setAffiliateUserSelected,
  refreshAffiliateAccess,
  showSuccess,
  showError
} = vi.hoisted(() => ({
  getAffiliateScope: vi.fn(),
  setAffiliateScope: vi.fn(),
  listAffiliateSelectedUsers: vi.fn(),
  setAffiliateUserSelected: vi.fn(),
  refreshAffiliateAccess: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn()
}))

vi.mock('../../api/affiliateScope', () => ({
  getAffiliateScope,
  setAffiliateScope,
  listAffiliateSelectedUsers,
  setAffiliateUserSelected
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess, showError }) }))
vi.mock('../access', () => ({ refreshAffiliateAccess }))
vi.mock('@/utils/format', () => ({ formatDateTimeToMinute: (value: string) => `[${value}]` }))
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

const PaginationStub = defineComponent({
  props: { total: Number, page: Number, pageSize: Number },
  emits: ['update:page'],
  setup(props, { emit }) {
    return () =>
      h('button', { 'data-testid': 'next-page', onClick: () => emit('update:page', (props.page ?? 1) + 1) }, `${props.page}/${props.total}/${props.pageSize}`)
  }
})

function scopeStatus(patch: Partial<AffiliateScopeStatus> = {}): AffiliateScopeStatus {
  return { affiliate_enabled: true, scope: 'all', selected_count: 0, ...patch }
}

function selectedUser(id: number, patch: Partial<AffiliateSelectedUser> = {}): AffiliateSelectedUser {
  return { user_id: id, email: `user${id}@example.com`, username: '', selected_at: '2026-11-13T10:00:00Z', ...patch }
}

function usersPage(items: AffiliateSelectedUser[], total = items.length) {
  return { items, total, page: 1, page_size: 10, pages: 1 }
}

async function mountLoaded() {
  const wrapper = mount(AffiliateScopeSettings, {
    global: { stubs: { ConfirmDialog: ConfirmDialogStub, Pagination: PaginationStub } }
  })
  await flushPromises()
  return wrapper
}

type Wrapper = Awaited<ReturnType<typeof mountLoaded>>

const pressed = (wrapper: Wrapper, scope: string) =>
  wrapper.get(`[data-testid="mok-affiliate-scope-${scope}"]`).attributes('aria-pressed')
const summary = (wrapper: Wrapper) => wrapper.get('[data-testid="mok-affiliate-scope-summary"]').text()
const userRows = (wrapper: Wrapper) =>
  wrapper.findAll('[data-testid="mok-affiliate-scope-user"]').map((row) => row.findAll('td').map((cell) => cell.text()))

beforeEach(() => {
  vi.clearAllMocks()
  getAffiliateScope.mockResolvedValue(scopeStatus())
  listAffiliateSelectedUsers.mockResolvedValue(usersPage([]))
})

describe('AffiliateScopeSettings', () => {
  it('shows the feature on for everyone without a user list', async () => {
    const wrapper = await mountLoaded()

    expect(pressed(wrapper, 'all')).toBe('true')
    expect(pressed(wrapper, 'selected')).toBe('false')
    expect(summary(wrapper)).toBe('mok.affiliateScope.summaryAll {"count":0}')
    expect(listAffiliateSelectedUsers).not.toHaveBeenCalled()
    expect(wrapper.text()).not.toContain('mok.affiliateScope.listTitle')
  })

  it('narrows the feature to the selected users after confirmation', async () => {
    getAffiliateScope.mockResolvedValue(scopeStatus({ selected_count: 2 }))
    listAffiliateSelectedUsers.mockResolvedValue(usersPage([selectedUser(1), selectedUser(2, { username: 'bob' })]))
    setAffiliateScope.mockResolvedValue(scopeStatus({ scope: 'selected', selected_count: 2 }))
    const wrapper = await mountLoaded()

    await wrapper.get('[data-testid="mok-affiliate-scope-selected"]').trigger('click')
    expect(wrapper.get('[data-testid="confirm"]').text()).toContain('mok.affiliateScope.confirmToSelected {"count":2}')
    expect(setAffiliateScope).not.toHaveBeenCalled()

    await wrapper.get('[data-testid="confirm-ok"]').trigger('click')
    await flushPromises()

    expect(setAffiliateScope).toHaveBeenCalledWith('selected')
    expect(showSuccess).toHaveBeenCalledWith('mok.affiliateScope.switched')
    expect(refreshAffiliateAccess, "the admin's own sidebar entry follows").toHaveBeenCalledTimes(1)
    expect(pressed(wrapper, 'selected')).toBe('true')
    expect(summary(wrapper)).toBe('mok.affiliateScope.summarySelected {"count":2}')
    expect(userRows(wrapper)).toEqual([
      ['user1@example.com', '[2026-11-13T10:00:00Z]', 'mok.affiliateScope.remove'],
      ['user2@example.combob', '[2026-11-13T10:00:00Z]', 'mok.affiliateScope.remove']
    ])
  })

  it('keeps the scope when the change is cancelled, fails or is the current one', async () => {
    setAffiliateScope.mockRejectedValue(new Error('boom'))
    const wrapper = await mountLoaded()

    await wrapper.get('[data-testid="mok-affiliate-scope-all"]').trigger('click')
    expect(wrapper.find('[data-testid="confirm"]').exists()).toBe(false)

    await wrapper.get('[data-testid="mok-affiliate-scope-selected"]').trigger('click')
    await wrapper.get('[data-testid="confirm-cancel"]').trigger('click')
    expect(setAffiliateScope).not.toHaveBeenCalled()

    await wrapper.get('[data-testid="mok-affiliate-scope-selected"]').trigger('click')
    await wrapper.get('[data-testid="confirm-ok"]').trigger('click')
    await flushPromises()
    expect(showError).toHaveBeenCalledTimes(1)
    expect(pressed(wrapper, 'all')).toBe('true')
    expect(refreshAffiliateAccess).not.toHaveBeenCalled()
  })

  it('switches a listed user off', async () => {
    getAffiliateScope.mockResolvedValue(scopeStatus({ scope: 'selected', selected_count: 2 }))
    listAffiliateSelectedUsers.mockResolvedValueOnce(usersPage([selectedUser(1), selectedUser(2)]))
    listAffiliateSelectedUsers.mockResolvedValue(usersPage([selectedUser(2)]))
    setAffiliateUserSelected.mockResolvedValue({})
    const wrapper = await mountLoaded()

    await wrapper.findAll('[data-testid="mok-affiliate-scope-remove"]')[0].trigger('click')
    await flushPromises()

    expect(setAffiliateUserSelected).toHaveBeenCalledWith(1, false)
    expect(showSuccess).toHaveBeenCalledWith('mok.affiliateScope.removed {"user":"user1@example.com"}')
    expect(userRows(wrapper).map((row) => row[0])).toEqual(['user2@example.com'])
    expect(summary(wrapper)).toBe('mok.affiliateScope.summarySelected {"count":1}')
  })

  it('pages through the selected users and steps back from an emptied last page', async () => {
    getAffiliateScope.mockResolvedValue(scopeStatus({ scope: 'selected', selected_count: 11 }))
    listAffiliateSelectedUsers.mockResolvedValue(usersPage([selectedUser(1)], 11))
    const wrapper = await mountLoaded()
    expect(wrapper.get('[data-testid="next-page"]').text()).toBe('1/11/10')

    listAffiliateSelectedUsers.mockResolvedValue(usersPage([selectedUser(11)], 11))
    await wrapper.get('[data-testid="next-page"]').trigger('click')
    await flushPromises()
    expect(listAffiliateSelectedUsers).toHaveBeenLastCalledWith(2, 10)

    // The only user of page 2 is switched off: the list goes back to page 1.
    setAffiliateUserSelected.mockResolvedValue({})
    listAffiliateSelectedUsers.mockResolvedValueOnce(usersPage([], 10))
    listAffiliateSelectedUsers.mockResolvedValue(usersPage([selectedUser(1)], 10))
    await wrapper.get('[data-testid="mok-affiliate-scope-remove"]').trigger('click')
    await flushPromises()

    expect(listAffiliateSelectedUsers).toHaveBeenLastCalledWith(1, 10)
    expect(userRows(wrapper).map((row) => row[0])).toEqual(['user1@example.com'])
    expect(wrapper.find('[data-testid="next-page"]').exists()).toBe(false)
  })

  it('reports a user that could not be switched off', async () => {
    getAffiliateScope.mockResolvedValue(scopeStatus({ scope: 'selected', selected_count: 1 }))
    listAffiliateSelectedUsers.mockResolvedValue(usersPage([selectedUser(1)]))
    setAffiliateUserSelected.mockRejectedValue(new Error('boom'))
    const wrapper = await mountLoaded()

    await wrapper.get('[data-testid="mok-affiliate-scope-remove"]').trigger('click')
    await flushPromises()

    expect(showError).toHaveBeenCalledTimes(1)
    expect(userRows(wrapper)).toHaveLength(1)
  })

  it('offers a retry when the scope cannot be loaded', async () => {
    getAffiliateScope.mockRejectedValueOnce(new Error('network'))
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
    const wrapper = await mountLoaded()

    expect(wrapper.get('[role="alert"]').text()).toContain('mok.affiliateScope.loadFailed')
    await wrapper.get('[role="alert"] button').trigger('click')
    await flushPromises()

    expect(pressed(wrapper, 'all')).toBe('true')
    errorSpy.mockRestore()
  })
})
