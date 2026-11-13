<template>
  <!--
    Sits in the admin user editor (components/admin/user/UserEditModal.vue). The switch
    saves on its own, right away, not with the editor's form.
  -->
  <div v-if="userId" data-testid="mok-user-affiliate">
    <div class="flex items-center justify-between gap-4">
      <label class="input-label !mb-0">{{ mt('affiliateScope.user.label') }}</label>
      <div v-if="loading" class="h-5 w-5 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
      <button v-else-if="!access" type="button" class="btn btn-secondary btn-sm" data-testid="mok-user-affiliate-retry" @click="load">
        {{ mt('affiliateScope.retry') }}
      </button>
      <Toggle
        v-else
        :model-value="access.selected"
        :class="saving ? 'opacity-60' : ''"
        :aria-label="mt('affiliateScope.user.label')"
        data-testid="mok-user-affiliate-toggle"
        @update:model-value="toggle"
      />
    </div>
    <p class="input-hint" data-testid="mok-user-affiliate-hint">{{ hint }}</p>
    <p v-if="access" class="input-hint">{{ mt('affiliateScope.user.instant') }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Toggle from '@/components/common/Toggle.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { getAffiliateUserAccess, setAffiliateUserSelected, type AffiliateUserAccess } from '../api/affiliateScope'
import { useMokI18n } from '../i18n'
import { refreshAffiliateAccess } from './access'

const props = defineProps<{ userId?: number | null }>()

const { mt } = useMokI18n()
const appStore = useAppStore()

const loading = ref(false)
const saving = ref(false)
const access = ref<AffiliateUserAccess | null>(null)
// Only the latest user's answer may fill the switch when the editor moves on to another user.
let latestRequest = 0

const hint = computed(() => {
  const a = access.value
  if (loading.value) return ''
  if (!a) return mt('affiliateScope.user.loadFailed')
  if (!a.affiliate_enabled) return mt('affiliateScope.user.hintFeatureOff')
  if (a.scope === 'all') return mt('affiliateScope.user.hintScopeAll')
  return a.selected ? mt('affiliateScope.user.hintOn') : mt('affiliateScope.user.hintOff')
})

async function load(): Promise<void> {
  const userId = props.userId
  const request = ++latestRequest
  access.value = null
  if (!userId) {
    loading.value = false
    return
  }
  loading.value = true
  try {
    const result = await getAffiliateUserAccess(userId)
    if (request === latestRequest) access.value = result
  } catch (error) {
    if (request === latestRequest) console.error('Failed to load the affiliate switch of the user:', error)
  } finally {
    if (request === latestRequest) loading.value = false
  }
}

async function toggle(selected: boolean): Promise<void> {
  const userId = props.userId
  if (!userId || saving.value) return
  const request = latestRequest
  saving.value = true
  try {
    const result = await setAffiliateUserSelected(userId, selected)
    if (request === latestRequest) access.value = result
    appStore.showSuccess(selected ? mt('affiliateScope.user.switchedOn') : mt('affiliateScope.user.switchedOff'))
    // The admin may have switched their own account.
    void refreshAffiliateAccess()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, mt('affiliateScope.user.saveFailed')))
  } finally {
    saving.value = false
  }
}

watch(() => props.userId, load, { immediate: true })
</script>
