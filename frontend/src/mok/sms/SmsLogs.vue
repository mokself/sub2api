<template>
  <!-- The SMS send log as a dialog, opened from ./SmsSettings.vue. -->
  <BaseDialog :show="show" :title="mt('sms.log.title')" width="wide" @close="emit('close')">
    <div data-testid="mok-sms-logs">
      <p class="text-sm text-gray-500 dark:text-gray-400">{{ mt('sms.log.hint') }}</p>

      <div class="mt-3 flex gap-2">
        <input
          v-model="searchInput"
          type="search"
          class="input"
          :placeholder="mt('sms.log.searchPlaceholder')"
          :aria-label="mt('sms.log.searchPlaceholder')"
          data-testid="mok-sms-logs-search"
          @keydown.enter.prevent="applySearch"
        />
        <button type="button" class="btn btn-secondary flex-shrink-0" data-testid="mok-sms-logs-search-button" @click="applySearch">
          {{ mt('sms.log.search') }}
        </button>
      </div>

      <div
        v-if="failed"
        class="mt-4 flex items-center justify-between gap-3 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-300"
        role="alert"
      >
        <span>{{ mt('sms.log.loadFailed') }}</span>
        <button type="button" class="btn btn-secondary btn-sm" @click="load">{{ mt('sms.log.retry') }}</button>
      </div>

      <div v-else-if="loading && entries.length === 0" class="flex justify-center py-10">
        <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
      </div>

      <p
        v-else-if="entries.length === 0"
        class="mt-4 rounded-xl border border-dashed border-gray-300 px-4 py-8 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400"
        data-testid="mok-sms-logs-empty"
      >
        {{ search ? mt('sms.log.noMatch') : mt('sms.log.empty') }}
      </p>

      <div
        v-else
        :class="['mt-4 overflow-hidden rounded-xl border border-gray-200 transition-opacity dark:border-dark-600', loading ? 'opacity-60' : '']"
      >
        <div class="overflow-x-auto">
          <table class="w-full min-w-[760px] text-sm">
            <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="px-3 py-2 text-left font-medium">{{ mt('sms.log.columns.time') }}</th>
                <th class="px-3 py-2 text-left font-medium">{{ mt('sms.log.columns.event') }}</th>
                <th class="px-3 py-2 text-left font-medium">{{ mt('sms.log.columns.phone') }}</th>
                <th class="px-3 py-2 text-left font-medium">{{ mt('sms.log.columns.user') }}</th>
                <th class="px-3 py-2 text-left font-medium">{{ mt('sms.log.columns.content') }}</th>
                <th class="px-3 py-2 text-left font-medium">{{ mt('sms.log.columns.status') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="entry in entries" :key="entry.id" data-testid="mok-sms-log-row">
                <td class="whitespace-nowrap px-3 py-2 tabular-nums text-gray-700 dark:text-gray-300">
                  {{ formatDateTime(entry.created_at) }}
                </td>
                <td class="whitespace-nowrap px-3 py-2 text-gray-700 dark:text-gray-300">{{ mt(`sms.log.events.${entry.event}`) }}</td>
                <td class="whitespace-nowrap px-3 py-2 tabular-nums text-gray-700 dark:text-gray-300">{{ entry.phone }}</td>
                <td class="max-w-[12rem] truncate px-3 py-2 text-gray-700 dark:text-gray-300" :title="entry.user_email">
                  {{ entry.user_id ? entry.user_email || `#${entry.user_id}` : entry.event === 'test' ? '-' : mt('sms.log.admin') }}
                </td>
                <td class="max-w-[16rem] px-3 py-2 text-xs text-gray-600 dark:text-gray-400">
                  <span class="font-mono">{{ entry.template_id || '-' }}</span>
                  <span v-if="entry.params.length > 0" class="ml-1 break-all">{{ entry.params.join(' / ') }}</span>
                </td>
                <td class="px-3 py-2">
                  <span :class="['inline-flex rounded-full px-2 py-0.5 text-xs font-medium', statusClass(entry.status)]">
                    {{ mt(`sms.log.statuses.${entry.status}`) }}
                  </span>
                  <div v-if="entry.error" class="mt-1 max-w-[14rem] break-all text-xs text-red-600 dark:text-red-400" data-testid="mok-sms-log-error">
                    {{ entry.error }}
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <Pagination
          v-if="total > pageSize"
          :total="total"
          :page="page"
          :page-size="pageSize"
          :show-page-size-selector="false"
          @update:page="goToPage"
        />
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import { formatDateTime } from '@/utils/format'
import { listSmsLogs, type SmsLogEntry, type SmsLogStatus } from '../api/sms'
import { useMokI18n } from '../i18n'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ (e: 'close'): void }>()

const { mt } = useMokI18n()

const pageSize = 20
const entries = ref<SmsLogEntry[]>([])
const total = ref(0)
const page = ref(1)
const searchInput = ref('')
const search = ref('')
const loading = ref(false)
const failed = ref(false)
// Only the latest request may fill the table, whatever order the responses arrive in.
let latestRequest = 0

function statusClass(status: SmsLogStatus): string {
  switch (status) {
    case 'sent':
      return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-400'
    case 'failed':
      return 'bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-400'
    case 'limited':
      return 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-400'
    default:
      return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
  }
}

async function load(): Promise<void> {
  const request = ++latestRequest
  loading.value = true
  failed.value = false
  try {
    const result = await listSmsLogs({ page: page.value, page_size: pageSize, search: search.value })
    if (request !== latestRequest) return
    entries.value = result.items ?? []
    total.value = result.total ?? 0
  } catch (error) {
    if (request !== latestRequest) return
    console.error('Failed to load the SMS send log:', error)
    failed.value = true
  } finally {
    if (request === latestRequest) loading.value = false
  }
}

function goToPage(next: number): void {
  page.value = next
  void load()
}

function applySearch(): void {
  search.value = searchInput.value.trim()
  page.value = 1
  void load()
}

// Every opening starts from the newest messages.
watch(
  () => props.show,
  (show) => {
    if (!show) return
    entries.value = []
    total.value = 0
    page.value = 1
    searchInput.value = ''
    search.value = ''
    void load()
  },
  { immediate: true }
)
</script>
