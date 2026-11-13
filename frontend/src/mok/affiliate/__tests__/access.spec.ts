import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { nextTick, reactive } from 'vue'

import type { AffiliateOwnAccess } from '../../api/affiliateScope'

const { getMyAffiliateAccess, authStore } = vi.hoisted(() => ({
  getMyAffiliateAccess: vi.fn(),
  authStore: { current: null as { user: { id: number } | null } | null }
}))

vi.mock('../../api/affiliateScope', () => ({ getMyAffiliateAccess }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => authStore.current }))

const CACHE_KEY = 'mok_affiliate_access'
const on: AffiliateOwnAccess = { allowed: true, has_quota: false }
const off: AffiliateOwnAccess = { allowed: false, has_quota: false }

// The module keeps its state at module level, so every test loads a fresh copy.
async function install(user: { id: number } | null) {
  vi.resetModules()
  authStore.current = reactive({ user })
  const access = await import('../access')
  access.installAffiliateAccess()
  const items = [{ path: '/keys' }, { path: '/affiliate' }, { path: '/profile' }]
  return { access, shown: () => access.mokFilterNavItems(items).some((item) => item.path === '/affiliate') }
}

async function signIn(user: { id: number } | null) {
  authStore.current!.user = user
  await nextTick()
  await flushPromises()
}

// Makes the next question stay open until the returned function answers it.
function pendingAnswer(): (access: AffiliateOwnAccess) => void {
  let answer: (access: AffiliateOwnAccess) => void = () => {}
  getMyAffiliateAccess.mockReturnValueOnce(new Promise<AffiliateOwnAccess>((resolve) => (answer = resolve)))
  return (access) => answer(access)
}

const cached = () => JSON.parse(window.localStorage.getItem(CACHE_KEY)!)

beforeEach(() => {
  vi.clearAllMocks()
  window.localStorage.clear()
  getMyAffiliateAccess.mockResolvedValue(on)
})

describe('affiliate entry in the sidebar', () => {
  it('shows once the feature is known to be on for the user', async () => {
    const answer = pendingAnswer()
    const { shown } = await install({ id: 7 })
    expect(shown()).toBe(false)

    answer(on)
    await flushPromises()
    expect(getMyAffiliateAccess).toHaveBeenCalledTimes(1)
    expect(shown()).toBe(true)
    expect(cached()).toEqual({ user_id: 7, visible: true })
  })

  it('is hidden for a user the feature is off for', async () => {
    getMyAffiliateAccess.mockResolvedValue(off)
    const { shown } = await install({ id: 7 })
    await flushPromises()
    expect(shown()).toBe(false)
    expect(cached()).toEqual({ user_id: 7, visible: false })
  })

  it('stays for a user the feature is off for who still has rebate quota', async () => {
    getMyAffiliateAccess.mockResolvedValue({ allowed: false, has_quota: true })
    const { shown } = await install({ id: 7 })
    await flushPromises()
    expect(shown()).toBe(true)
  })

  it('leaves every other entry alone, in order', async () => {
    getMyAffiliateAccess.mockResolvedValueOnce(off)
    const { access } = await install({ id: 7 })
    await flushPromises()
    const items = [{ path: '/keys' }, { path: '/affiliate' }, { path: '/admin/affiliates' }, { path: '/profile' }]
    expect(access.mokFilterNavItems(items).map((item) => item.path)).toEqual(['/keys', '/admin/affiliates', '/profile'])

    await access.refreshAffiliateAccess()
    expect(access.mokFilterNavItems(items)).toEqual(items)
  })

  it('asks nothing while nobody is signed in and follows sign-in and sign-out', async () => {
    const { access, shown } = await install(null)
    await flushPromises()
    expect(getMyAffiliateAccess).not.toHaveBeenCalled()
    expect(shown()).toBe(false)

    await signIn({ id: 7 })
    expect(shown()).toBe(true)

    await signIn(null)
    expect(access.affiliateEntryVisible.value).toBeNull()
    expect(shown()).toBe(false)
  })

  it("uses the last answer for the same user right away, not another user's", async () => {
    window.localStorage.setItem(CACHE_KEY, JSON.stringify({ user_id: 7, visible: true }))
    getMyAffiliateAccess.mockReturnValue(new Promise(() => {}))

    expect((await install({ id: 7 })).shown()).toBe(true)
    expect((await install({ id: 8 })).shown()).toBe(false)
  })

  it('replaces the last answer with the new one', async () => {
    window.localStorage.setItem(CACHE_KEY, JSON.stringify({ user_id: 7, visible: true }))
    const answer = pendingAnswer()
    const { shown } = await install({ id: 7 })
    expect(shown()).toBe(true)
    answer(off)
    await flushPromises()
    expect(shown()).toBe(false)
  })

  it('shows when the question fails and there is no earlier answer', async () => {
    getMyAffiliateAccess.mockRejectedValue(new Error('network'))
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})

    const first = await install({ id: 7 })
    await flushPromises()
    expect(first.shown()).toBe(true)
    expect(window.localStorage.getItem(CACHE_KEY)).toBeNull()

    window.localStorage.setItem(CACHE_KEY, JSON.stringify({ user_id: 7, visible: false }))
    const second = await install({ id: 7 })
    await flushPromises()
    expect(second.shown()).toBe(false)
    errorSpy.mockRestore()
  })

  it('ignores an answer that arrives after another user signed in', async () => {
    const answer = pendingAnswer()
    const { shown } = await install({ id: 7 })

    getMyAffiliateAccess.mockResolvedValue(off)
    await signIn({ id: 8 })
    answer(on)
    await flushPromises()

    expect(shown()).toBe(false)
    expect(cached()).toEqual({ user_id: 8, visible: false })
  })

  it('asks again on request', async () => {
    getMyAffiliateAccess.mockResolvedValueOnce(off)
    const { access, shown } = await install({ id: 7 })
    await flushPromises()
    expect(shown()).toBe(false)

    await access.refreshAffiliateAccess()
    expect(getMyAffiliateAccess).toHaveBeenCalledTimes(2)
    expect(shown()).toBe(true)
    expect(cached()).toEqual({ user_id: 7, visible: true })
  })

  it('survives a broken or outdated cache entry', async () => {
    for (const entry of ['{not json', JSON.stringify({ user_id: 7, allowed: true })]) {
      window.localStorage.setItem(CACHE_KEY, entry)
      const answer = pendingAnswer()
      const { shown } = await install({ id: 7 })
      expect(shown()).toBe(false)
      answer(on)
      await flushPromises()
      expect(shown()).toBe(true)
    }
  })
})
