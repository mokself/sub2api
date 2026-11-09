import { defineAsyncComponent, type App } from 'vue'
import { installMokI18n } from './i18n'

/**
 * Entry point of the fork's frontend features (see FORK.md). main.ts calls it after
 * initI18n() and before app.use(router).
 *
 * Components placed into upstream templates are registered globally here, so each
 * [mok] seam in an upstream template is a single tag without an import. Their types
 * are declared in ./components.d.ts.
 */
export async function installMok(app: App): Promise<void> {
  installMokI18n()
  app.component(
    'MokAffiliateSettings',
    defineAsyncComponent(() => import('./affiliate/AffiliateSettings.vue'))
  )
  app.component(
    'MokAffiliateUserPanel',
    defineAsyncComponent(() => import('./affiliate/AffiliateUserPanel.vue'))
  )
}
