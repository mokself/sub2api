import { apiClient } from '@/api/client'

/** The recharge page's quick amounts; empty means upstream's buttons. */
export interface PaymentQuickAmounts {
  amounts: number[]
}

/** For the recharge page (GET /api/v1/mok/payment/quick-amounts). */
export async function getPaymentQuickAmounts(): Promise<PaymentQuickAmounts> {
  const { data } = await apiClient.get<PaymentQuickAmounts>('/mok/payment/quick-amounts')
  return data
}

/** For the settings card (GET /api/v1/admin/mok/payment/quick-amounts). */
export async function getAdminPaymentQuickAmounts(): Promise<PaymentQuickAmounts> {
  const { data } = await apiClient.get<PaymentQuickAmounts>('/admin/mok/payment/quick-amounts')
  return data
}

/** Saves the list; [] restores upstream's buttons. */
export async function savePaymentQuickAmounts(amounts: number[]): Promise<PaymentQuickAmounts> {
  const { data } = await apiClient.put<PaymentQuickAmounts>('/admin/mok/payment/quick-amounts', { amounts })
  return data
}
