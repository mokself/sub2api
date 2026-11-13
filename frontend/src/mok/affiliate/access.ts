import { ref, watch } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { getMyAffiliateAccess } from '../api/affiliateScope'

// Whether the signed-in user gets the affiliate entry in the sidebar. Upstream's public
// setting affiliate_enabled is the same for everyone; with the scope "selected" (see
// FORK.md) the feature is on for some users only, so the sidebar entry also asks this
// module. A user the feature is off for keeps the entry while they still have rebate quota
// to transfer.

const CACHE_KEY = 'mok_affiliate_access'

/** true / false once known for the signed-in user, null before that. */
export const affiliateEntryVisible = ref<boolean | null>(null)

let currentUserId: number | null = null

// The last answer is kept per user, so the sidebar entry does not pop in on every page load.
function readCache(userId: number): boolean | null {
  try {
    const cached = JSON.parse(window.localStorage.getItem(CACHE_KEY) ?? 'null') as { user_id?: number; visible?: boolean } | null
    return cached && cached.user_id === userId && typeof cached.visible === 'boolean' ? cached.visible : null
  } catch {
    return null
  }
}

function writeCache(userId: number, visible: boolean): void {
  try {
    window.localStorage.setItem(CACHE_KEY, JSON.stringify({ user_id: userId, visible }))
  } catch {
    // Ignore storage errors; the answer is simply asked for again next time.
  }
}

/** Asks again, e.g. after an admin changed the scope or the affiliate page was opened. */
export async function refreshAffiliateAccess(): Promise<void> {
  const userId = currentUserId
  if (userId === null) return
  try {
    const access = await getMyAffiliateAccess()
    if (currentUserId === userId) {
      affiliateEntryVisible.value = access.allowed || access.has_quota
      writeCache(userId, affiliateEntryVisible.value)
    }
  } catch (error) {
    console.error('Failed to load affiliate access:', error)
    // Without an earlier answer the entry is shown: the page itself says when the feature
    // is off for the user, and for most sites it is on for everyone.
    if (currentUserId === userId && affiliateEntryVisible.value === null) {
      affiliateEntryVisible.value = true
    }
  }
}

/** Follows the signed-in user. Called once from installMok, after Pinia is installed. */
export function installAffiliateAccess(): void {
  const authStore = useAuthStore()
  watch(
    () => authStore.user?.id ?? null,
    (userId) => {
      currentUserId = userId
      affiliateEntryVisible.value = userId === null ? null : readCache(userId)
      if (userId !== null) {
        void refreshAffiliateAccess()
      }
    },
    { immediate: true }
  )
}

const AFFILIATE_PATH = '/affiliate'

/**
 * Called by the sidebar on the signed-in user's own nav items ([mok] seam in finalizeNav of
 * components/layout/AppSidebar.vue), after upstream's feature flags: drops the affiliate
 * entry unless this module says it shows for the user.
 */
export function mokFilterNavItems<T extends { path: string }>(items: T[]): T[] {
  return affiliateEntryVisible.value === true ? items : items.filter((item) => item.path !== AFFILIATE_PATH)
}
