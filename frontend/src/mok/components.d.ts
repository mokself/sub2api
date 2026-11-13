// Types of the global components that installMok (./index.ts) registers for [mok] seams
// in upstream templates.
export {}

declare module 'vue' {
  export interface GlobalComponents {
    MokAffiliateSettings: (typeof import('./affiliate/AffiliateSettings.vue'))['default']
    MokAffiliateUserPanel: (typeof import('./affiliate/AffiliateUserPanel.vue'))['default']
    MokUserAffiliateSwitch: (typeof import('./affiliate/UserAffiliateSwitch.vue'))['default']
  }
}
