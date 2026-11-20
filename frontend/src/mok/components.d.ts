// Types of the global components that installMok (./index.ts) registers for [mok] seams
// in upstream templates.
export {}

declare module 'vue' {
  export interface GlobalComponents {
    MokAffiliateSettings: (typeof import('./affiliate/AffiliateSettings.vue'))['default']
    MokAffiliateUserPanel: (typeof import('./affiliate/AffiliateUserPanel.vue'))['default']
    MokUserAffiliateSwitch: (typeof import('./affiliate/UserAffiliateSwitch.vue'))['default']
    MokSmsSettings: (typeof import('./sms/SmsSettings.vue'))['default']
    MokSmsNotifyPhone: (typeof import('./sms/SmsNotifyPhone.vue'))['default']
    MokOpsAlertSms: (typeof import('./sms/OpsAlertSms.vue'))['default']
    MokAccountQuotaSmsPhones: (typeof import('./sms/AccountQuotaSmsPhones.vue'))['default']
    MokPaymentQuickAmounts: (typeof import('./payment/PaymentQuickAmounts.vue'))['default']
  }

  export interface ComponentCustomProperties {
    /** The recharge page's quick amounts, see ./payment/quickAmounts.ts. */
    $mokQuickAmounts: (defaults: number[]) => number[]
  }
}
