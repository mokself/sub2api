import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import UserAffiliateSwitch from '../UserAffiliateSwitch.vue'
import type { AffiliateUserAccess } from '../../api/affiliateScope'

const { getAffiliateUserAccess, setAffiliateUserSelected, refreshAffiliateAccess, showSuccess, showError } = vi.hoisted(() => ({
  getAffiliateUserAccess: vi.fn(),
  setAffiliateUserSelected: vi.fn(),
  refreshAffiliateAccess: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn()
}))

vi.mock('../../api/affiliateScope', () => ({ getAffiliateUserAccess, setAffiliateUserSelected }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess, showError }) }))
vi.mock('../access', () => ({ refreshAffiliateAccess }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

function access(patch: Partial<AffiliateUserAccess> = {}): AffiliateUserAccess {
  return { user_id: 5, selected: false, scope: 'selected', affiliate_enabled: true, allowed: false, ...patch }
}

async function mountFor(userId: number | null | undefined) {
  const wrapper = mount(UserAffiliateSwitch, { props: { userId } })
  await flushPromises()
  return wrapper
}

type Wrapper = Awaited<ReturnType<typeof mountFor>>

const toggle = (wrapper: Wrapper) => wrapper.get('[data-testid="mok-user-affiliate-toggle"]')
const hint = (wrapper: Wrapper) => wrapper.get('[data-testid="mok-user-affiliate-hint"]').text()

beforeEach(() => {
  vi.clearAllMocks()
  getAffiliateUserAccess.mockResolvedValue(access())
  setAffiliateUserSelected.mockImplementation(async (userId: number, selected: boolean) =>
    access({ user_id: userId, selected, allowed: selected })
  )
})

describe('UserAffiliateSwitch', () => {
  it('shows the switch of the user being edited', async () => {
    const wrapper = await mountFor(5)
    expect(getAffiliateUserAccess).toHaveBeenCalledWith(5)
    expect(toggle(wrapper).attributes('aria-checked')).toBe('false')
    expect(hint(wrapper)).toBe('mok.affiliateScope.user.hintOff')
  })

  it('switches the feature on and off right away', async () => {
    const wrapper = await mountFor(5)

    await toggle(wrapper).trigger('click')
    await flushPromises()
    expect(setAffiliateUserSelected).toHaveBeenCalledWith(5, true)
    expect(toggle(wrapper).attributes('aria-checked')).toBe('true')
    expect(hint(wrapper)).toBe('mok.affiliateScope.user.hintOn')
    expect(showSuccess).toHaveBeenCalledWith('mok.affiliateScope.user.switchedOn')
    expect(refreshAffiliateAccess, 'the admin may have switched their own account').toHaveBeenCalledTimes(1)

    await toggle(wrapper).trigger('click')
    await flushPromises()
    expect(setAffiliateUserSelected).toHaveBeenLastCalledWith(5, false)
    expect(toggle(wrapper).attributes('aria-checked')).toBe('false')
    expect(showSuccess).toHaveBeenLastCalledWith('mok.affiliateScope.user.switchedOff')
  })

  it('does not submit the user form it sits in', async () => {
    const wrapper = await mountFor(5)
    expect(toggle(wrapper).attributes('type')).toBe('button')
  })

  it.each([
    ['the feature is on for everyone', access({ scope: 'all', allowed: true }), 'mok.affiliateScope.user.hintScopeAll'],
    ['the feature is off altogether', access({ affiliate_enabled: false }), 'mok.affiliateScope.user.hintFeatureOff'],
    ['it is off altogether with scope all', access({ affiliate_enabled: false, scope: 'all' }), 'mok.affiliateScope.user.hintFeatureOff']
  ])('explains that the switch has no effect while %s', async (_, value, expected) => {
    getAffiliateUserAccess.mockResolvedValue(value)
    const wrapper = await mountFor(5)
    expect(hint(wrapper)).toBe(expected)
    expect(toggle(wrapper).exists()).toBe(true)
  })

  it('keeps the old state and reports it when saving fails', async () => {
    setAffiliateUserSelected.mockRejectedValue(new Error('boom'))
    const wrapper = await mountFor(5)

    await toggle(wrapper).trigger('click')
    await flushPromises()

    expect(toggle(wrapper).attributes('aria-checked')).toBe('false')
    expect(showError).toHaveBeenCalledTimes(1)
    expect(showSuccess).not.toHaveBeenCalled()
  })

  it('ignores a second click while saving', async () => {
    let finish: (value: AffiliateUserAccess) => void = () => {}
    setAffiliateUserSelected.mockReturnValue(new Promise<AffiliateUserAccess>((resolve) => (finish = resolve)))
    const wrapper = await mountFor(5)

    await toggle(wrapper).trigger('click')
    await toggle(wrapper).trigger('click')
    expect(setAffiliateUserSelected).toHaveBeenCalledTimes(1)

    finish(access({ selected: true, allowed: true }))
    await flushPromises()
    expect(toggle(wrapper).attributes('aria-checked')).toBe('true')
  })

  it('offers a retry when the switch cannot be loaded', async () => {
    getAffiliateUserAccess.mockRejectedValueOnce(new Error('network'))
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
    const wrapper = await mountFor(5)

    expect(hint(wrapper)).toBe('mok.affiliateScope.user.loadFailed')
    expect(wrapper.find('[data-testid="mok-user-affiliate-toggle"]').exists()).toBe(false)

    await wrapper.get('[data-testid="mok-user-affiliate-retry"]').trigger('click')
    await flushPromises()
    expect(toggle(wrapper).attributes('aria-checked')).toBe('false')
    errorSpy.mockRestore()
  })

  it('follows the editor to another user', async () => {
    const wrapper = await mountFor(5)
    getAffiliateUserAccess.mockResolvedValue(access({ user_id: 6, selected: true, allowed: true }))

    await wrapper.setProps({ userId: 6 })
    await flushPromises()

    expect(getAffiliateUserAccess).toHaveBeenLastCalledWith(6)
    expect(toggle(wrapper).attributes('aria-checked')).toBe('true')
  })

  it('renders nothing without a user', async () => {
    const wrapper = await mountFor(undefined)
    expect(getAffiliateUserAccess).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="mok-user-affiliate"]').exists()).toBe(false)
  })
})
