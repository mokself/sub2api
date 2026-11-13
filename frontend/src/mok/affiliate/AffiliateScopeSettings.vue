<template>
  <!-- Part of ./AffiliateSettings.vue; saves on its own. -->
  <div class="border-t border-gray-100 pt-6 dark:border-dark-700" data-testid="mok-affiliate-scope">
    <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ mt('affiliateScope.title') }}</h3>
    <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ mt('affiliateScope.description') }}</p>

    <div v-if="loading" class="flex justify-center py-4">
      <div class="h-5 w-5 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
    </div>

    <div
      v-else-if="!status"
      class="mt-3 flex items-center justify-between gap-3 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-300"
      role="alert"
    >
      <span>{{ mt('affiliateScope.loadFailed') }}</span>
      <button type="button" class="btn btn-secondary btn-sm" @click="load">{{ mt('affiliateScope.retry') }}</button>
    </div>

    <template v-else>
      <div
        class="mt-3 inline-flex rounded-lg border border-gray-200 bg-gray-50 p-0.5 dark:border-dark-600 dark:bg-dark-800"
        role="group"
      >
        <button
          v-for="option in scopes"
          :key="option"
          type="button"
          :class="[
            'rounded-md px-3 py-1 text-sm font-medium transition-colors',
            status.scope === option
              ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white'
              : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'
          ]"
          :aria-pressed="status.scope === option"
          :disabled="switching"
          :data-testid="`mok-affiliate-scope-${option}`"
          @click="requestSwitch(option)"
        >
          {{ option === 'all' ? mt('affiliateScope.all') : mt('affiliateScope.selected') }}
        </button>
      </div>

      <p class="mt-2 text-xs text-gray-500 dark:text-gray-400" data-testid="mok-affiliate-scope-summary">
        {{
          status.scope === 'selected'
            ? mt('affiliateScope.summarySelected', { count: status.selected_count })
            : mt('affiliateScope.summaryAll', { count: status.selected_count })
        }}
      </p>
      <p class="mt-1 text-xs text-gray-400 dark:text-gray-500">{{ mt('affiliateScope.howTo') }}</p>

      <div v-if="status.selected_count > 0 || users.length > 0" class="mt-4">
        <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ mt('affiliateScope.listTitle') }}</p>
        <p
          v-if="usersFailed"
          class="mt-2 flex items-center justify-between gap-3 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-300"
        >
          <span>{{ mt('affiliateScope.listFailed') }}</span>
          <button type="button" class="btn btn-secondary btn-sm" @click="loadUsers">{{ mt('affiliateScope.retry') }}</button>
        </p>
        <div v-else class="mt-2 overflow-hidden rounded-xl border border-gray-200 dark:border-dark-600">
          <table class="w-full text-sm">
            <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="px-3 py-2 text-left font-medium">{{ mt('affiliateScope.columns.user') }}</th>
                <th class="px-3 py-2 text-left font-medium">{{ mt('affiliateScope.columns.selectedAt') }}</th>
                <th class="px-3 py-2"></th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="user in users" :key="user.user_id" data-testid="mok-affiliate-scope-user">
                <td class="max-w-[16rem] px-3 py-2">
                  <div class="truncate text-gray-900 dark:text-white" :title="user.email">{{ user.email || `#${user.user_id}` }}</div>
                  <div v-if="user.username" class="truncate text-xs text-gray-500 dark:text-gray-400">{{ user.username }}</div>
                </td>
                <td class="whitespace-nowrap px-3 py-2 tabular-nums text-gray-700 dark:text-gray-300">
                  {{ formatDateTimeToMinute(user.selected_at) }}
                </td>
                <td class="px-3 py-2 text-right">
                  <button
                    type="button"
                    class="btn btn-secondary btn-sm"
                    :disabled="removing === user.user_id"
                    data-testid="mok-affiliate-scope-remove"
                    @click="remove(user)"
                  >
                    {{ mt('affiliateScope.remove') }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
          <Pagination
            v-if="status.selected_count > pageSize"
            :total="status.selected_count"
            :page="page"
            :page-size="pageSize"
            :show-page-size-selector="false"
            @update:page="goToPage"
          />
        </div>
      </div>
    </template>

    <ConfirmDialog
      :show="pendingScope !== null"
      :title="mt('affiliateScope.confirmTitle')"
      :message="
        pendingScope === 'selected'
          ? mt('affiliateScope.confirmToSelected', { count: status?.selected_count ?? 0 })
          : mt('affiliateScope.confirmToAll')
      "
      :confirm-text="mt('affiliateScope.confirm')"
      :cancel-text="mt('affiliateScope.cancel')"
      @confirm="confirmSwitch"
      @cancel="pendingScope = null"
    />
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTimeToMinute } from '@/utils/format'
import {
  getAffiliateScope,
  listAffiliateSelectedUsers,
  setAffiliateScope,
  setAffiliateUserSelected,
  type AffiliateScope,
  type AffiliateScopeStatus,
  type AffiliateSelectedUser
} from '../api/affiliateScope'
import { useMokI18n } from '../i18n'
import { refreshAffiliateAccess } from './access'

const { mt } = useMokI18n()
const appStore = useAppStore()

const scopes: AffiliateScope[] = ['all', 'selected']
const pageSize = 10
const loading = ref(true)
const switching = ref(false)
const status = ref<AffiliateScopeStatus | null>(null)
const pendingScope = ref<AffiliateScope | null>(null)
const users = ref<AffiliateSelectedUser[]>([])
const usersFailed = ref(false)
const page = ref(1)
const removing = ref<number | null>(null)

async function loadUsers(): Promise<void> {
  usersFailed.value = false
  try {
    const result = await listAffiliateSelectedUsers(page.value, pageSize)
    // Switching users off can empty the last page.
    if ((result.items ?? []).length === 0 && page.value > 1 && result.total > 0) {
      page.value = Math.max(1, Math.ceil(result.total / pageSize))
      return loadUsers()
    }
    users.value = result.items ?? []
    if (status.value) status.value.selected_count = result.total ?? 0
  } catch (error) {
    console.error('Failed to load the users with affiliate rebates:', error)
    usersFailed.value = true
  }
}

async function load(): Promise<void> {
  loading.value = true
  try {
    status.value = await getAffiliateScope()
  } catch (error) {
    console.error('Failed to load the affiliate scope:', error)
    status.value = null
  } finally {
    loading.value = false
  }
  if (status.value && status.value.selected_count > 0) {
    await loadUsers()
  }
}

function goToPage(next: number): void {
  page.value = next
  void loadUsers()
}

function requestSwitch(scope: AffiliateScope): void {
  if (status.value && status.value.scope !== scope) {
    pendingScope.value = scope
  }
}

async function confirmSwitch(): Promise<void> {
  const scope = pendingScope.value
  pendingScope.value = null
  if (!scope || switching.value) return
  switching.value = true
  try {
    status.value = await setAffiliateScope(scope)
    appStore.showSuccess(mt('affiliateScope.switched'))
    // The admin's own sidebar entry may change with the scope.
    void refreshAffiliateAccess()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, mt('affiliateScope.switchFailed')))
  } finally {
    switching.value = false
  }
}

async function remove(user: AffiliateSelectedUser): Promise<void> {
  if (removing.value !== null) return
  removing.value = user.user_id
  try {
    await setAffiliateUserSelected(user.user_id, false)
    appStore.showSuccess(mt('affiliateScope.removed', { user: user.email || `#${user.user_id}` }))
    void refreshAffiliateAccess()
    await loadUsers()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, mt('affiliateScope.removeFailed')))
  } finally {
    removing.value = null
  }
}

onMounted(load)
</script>
