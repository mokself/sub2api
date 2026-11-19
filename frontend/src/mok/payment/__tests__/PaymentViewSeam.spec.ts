import { describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'

import PaymentView from '@/views/user/PaymentView.vue'
import AmountInput from '@/components/payment/AmountInput.vue'

// The [mok] seam in upstream's recharge page: the quick amount buttons come from
// $mokQuickAmounts, and upstream's list stays when the fork is not installed (as in
// upstream's own tests of the page). Mocks follow views/user/__tests__/PaymentView.spec.ts.

const getCheckoutInfo = vi.hoisted(() => vi.fn())

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return {
    ...actual,
    useRoute: () => ({ path: '/purchase', query: {} }),
    useRouter: () => ({ replace: vi.fn(), push: vi.fn(), resolve: vi.fn(() => ({ href: '/' })) })
  }
})
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { username: 'demo', balance: 0 }, refreshUser: vi.fn() }) }))
vi.mock('@/stores/payment', () => ({ usePaymentStore: () => ({ createOrder: vi.fn() }) }))
vi.mock('@/stores/subscriptions', () => ({
  useSubscriptionStore: () => ({ activeSubscriptions: [], fetchActiveSubscriptions: vi.fn().mockResolvedValue(undefined) })
}))
vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError: vi.fn(), showInfo: vi.fn(), showWarning: vi.fn(), cachedPublicSettings: undefined })
}))
vi.mock('@/api/payment', () => ({ paymentAPI: { getCheckoutInfo } }))
vi.mock('@/utils/device', () => ({ isMobileDevice: () => false }))

function checkoutInfo() {
  return {
    data: {
      methods: {
        wxpay: { daily_limit: 0, daily_used: 0, daily_remaining: 0, single_min: 0, single_max: 0, fee_rate: 0, available: true }
      },
      global_min: 0,
      global_max: 0,
      plans: [],
      balance_disabled: false,
      balance_recharge_multiplier: 1,
      subscription_usd_to_cny_rate: 0,
      recharge_fee_rate: 0,
      help_text: '',
      help_image_url: '',
      stripe_publishable_key: ''
    }
  }
}

async function mountPage(globalProperties: Record<string, unknown> = {}) {
  getCheckoutInfo.mockResolvedValue(checkoutInfo())
  const wrapper = shallowMount(PaymentView, {
    global: {
      config: { globalProperties },
      stubs: { AppLayout: { template: '<div><slot /></div>' }, Teleport: true, Transition: false }
    }
  })
  await flushPromises()
  await flushPromises()
  return wrapper
}

describe('recharge page quick amounts seam', () => {
  it('shows the amounts the fork returns', async () => {
    const quickAmounts = vi.fn(() => [30, 88])
    const wrapper = await mountPage({ $mokQuickAmounts: quickAmounts })

    expect(wrapper.getComponent(AmountInput).props('amounts')).toEqual([30, 88])
    expect(quickAmounts).toHaveBeenCalledWith([10, 20, 50, 100, 200, 500, 1000, 2000, 5000])
  })

  it("keeps upstream's amounts without the fork", async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const wrapper = await mountPage()

    expect(wrapper.getComponent(AmountInput).props('amounts')).toEqual([10, 20, 50, 100, 200, 500, 1000, 2000, 5000])
    warn.mockRestore()
  })
})
