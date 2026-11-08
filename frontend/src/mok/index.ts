import type { App } from 'vue'
import { installMokI18n } from './i18n'

/**
 * Entry point of the fork's frontend features (see FORK.md). main.ts calls it after
 * initI18n() and before app.use(router).
 */
export async function installMok(_app: App): Promise<void> {
  installMokI18n()
}
