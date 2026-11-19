import { defineAsyncComponent, type App } from 'vue'
import { installAffiliateAccess } from './affiliate/access'
import { installMokI18n } from './i18n'
import { installPaymentQuickAmounts } from './payment/quickAmounts'

/**
 * Entry point of the fork's frontend features (see FORK.md). main.ts calls it after
 * initI18n() and before app.use(router), with Pinia already installed.
 *
 * Components placed into upstream templates are registered globally here, so each
 * [mok] seam in an upstream template is a single tag without an import. Their types
 * are declared in ./components.d.ts.
 */
export async function installMok(app: App): Promise<void> {
  installMokI18n()
  installAffiliateAccess()
  installPaymentQuickAmounts(app)
  app.component(
    'MokAffiliateSettings',
    defineAsyncComponent(() => import('./affiliate/AffiliateSettings.vue'))
  )
  app.component(
    'MokAffiliateUserPanel',
    defineAsyncComponent(() => import('./affiliate/AffiliateUserPanel.vue'))
  )
  app.component(
    'MokUserAffiliateSwitch',
    defineAsyncComponent(() => import('./affiliate/UserAffiliateSwitch.vue'))
  )
  app.component('MokSmsSettings', defineAsyncComponent(() => import('./sms/SmsSettings.vue')))
  app.component('MokSmsNotifyPhone', defineAsyncComponent(() => import('./sms/SmsNotifyPhone.vue')))
  app.component(
    'MokPaymentQuickAmounts',
    defineAsyncComponent(() => import('./payment/PaymentQuickAmounts.vue'))
  )
}
