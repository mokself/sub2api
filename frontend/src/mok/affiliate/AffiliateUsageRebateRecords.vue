<template>
  <!--
    The usage rebate records as a dialog: every record with a search for admins, the
    signed-in inviter's own records for users.
  -->
  <BaseDialog :show="show" :title="mt('affiliateUsage.title')" width="wide" @close="emit('close')">
    <div data-testid="mok-affiliate-usage-records">
      <p class="text-sm text-gray-500 dark:text-gray-400">
        {{ scope === 'admin' ? mt('affiliateUsage.adminHint') : mt('affiliateUsage.userHint') }}
      </p>

      <div v-if="scope === 'admin'" class="mt-3 flex gap-2">
        <input
          v-model="searchInput"
          type="search"
          class="input"
          :placeholder="mt('affiliateUsage.searchPlaceholder')"
          :aria-label="mt('affiliateUsage.searchPlaceholder')"
          data-testid="mok-affiliate-usage-search"
          @keydown.enter.prevent="applySearch"
        />
        <button type="button" class="btn btn-secondary flex-shrink-0" data-testid="mok-affiliate-usage-search-button" @click="applySearch">
          {{ mt('affiliateUsage.search') }}
        </button>
      </div>

      <div
        v-if="failed"
        class="mt-4 flex items-center justify-between gap-3 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-300"
        role="alert"
      >
        <span>{{ mt('affiliateUsage.loadFailed') }}</span>
        <button type="button" class="btn btn-secondary btn-sm" @click="load">{{ mt('affiliateUsage.retry') }}</button>
      </div>

      <div v-else-if="loading && records.length === 0" class="flex justify-center py-10">
        <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
      </div>

      <p
        v-else-if="records.length === 0"
        class="mt-4 rounded-xl border border-dashed border-gray-300 px-4 py-8 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400"
        data-testid="mok-affiliate-usage-empty"
      >
        {{ search ? mt('affiliateUsage.noMatch') : mt('affiliateUsage.empty') }}
      </p>

      <div
        v-else
        :class="['mt-4 overflow-hidden rounded-xl border border-gray-200 transition-opacity dark:border-dark-600', loading ? 'opacity-60' : '']"
      >
        <div class="overflow-x-auto">
          <table class="w-full min-w-[640px] text-sm">
            <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="px-3 py-2 text-left font-medium">{{ mt('affiliateUsage.columns.period') }}</th>
                <th v-if="scope === 'admin'" class="px-3 py-2 text-left font-medium">{{ mt('affiliateUsage.columns.inviter') }}</th>
                <th class="px-3 py-2 text-left font-medium">{{ mt('affiliateUsage.columns.invitee') }}</th>
                <th class="px-3 py-2 text-right font-medium">{{ mt('affiliateUsage.columns.usage') }}</th>
                <th class="px-3 py-2 text-right font-medium">{{ mt('affiliateUsage.columns.rate') }}</th>
                <th class="px-3 py-2 text-right font-medium">{{ mt('affiliateUsage.columns.rebate') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="(record, index) in records" :key="index" data-testid="mok-affiliate-usage-row">
                <td class="whitespace-nowrap px-3 py-2 tabular-nums text-gray-700 dark:text-gray-300">
                  {{ formatRebatePeriod(record.period_start, record.period_end) }}
                </td>
                <td v-if="scope === 'admin'" class="max-w-[14rem] truncate px-3 py-2 text-gray-700 dark:text-gray-300" :title="record.inviter_email">
                  {{ record.inviter_email || '-' }}
                </td>
                <td class="max-w-[14rem] truncate px-3 py-2 text-gray-700 dark:text-gray-300" :title="record.invitee_email">
                  {{ record.invitee_email || '-' }}
                </td>
                <td class="whitespace-nowrap px-3 py-2 text-right tabular-nums text-gray-700 dark:text-gray-300">
                  {{ formatCurrency(record.usage_amount) }}
                </td>
                <td class="whitespace-nowrap px-3 py-2 text-right tabular-nums text-gray-700 dark:text-gray-300">
                  {{ formatRebateRate(record.rate_percent) }}%
                </td>
                <td class="whitespace-nowrap px-3 py-2 text-right font-medium tabular-nums text-emerald-600 dark:text-emerald-400">
                  {{ formatCurrency(record.rebate_amount) }}
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
import { formatCurrency } from '@/utils/format'
import {
  listAffiliateUsageRebates,
  listMyAffiliateUsageRebates,
  type AffiliateUsageRebateRecord
} from '../api/affiliateUsageRebates'
import { useMokI18n } from '../i18n'
import { formatRebatePeriod, formatRebateRate } from './usageRebate'

const props = defineProps<{
  show: boolean
  /** "admin" lists every record and can search; "user" lists the signed-in inviter's own. */
  scope: 'admin' | 'user'
}>()
const emit = defineEmits<{ (e: 'close'): void }>()

const { mt } = useMokI18n()

const pageSize = 20
const records = ref<AffiliateUsageRebateRecord[]>([])
const total = ref(0)
const page = ref(1)
const searchInput = ref('')
const search = ref('')
const loading = ref(false)
const failed = ref(false)
// Only the latest request may fill the table, whatever order the responses arrive in.
let latestRequest = 0

async function load(): Promise<void> {
  const request = ++latestRequest
  loading.value = true
  failed.value = false
  try {
    const query = { page: page.value, page_size: pageSize, search: search.value }
    const result = props.scope === 'admin' ? await listAffiliateUsageRebates(query) : await listMyAffiliateUsageRebates(query)
    if (request !== latestRequest) return
    records.value = result.items ?? []
    total.value = result.total ?? 0
  } catch (error) {
    if (request !== latestRequest) return
    console.error('Failed to load affiliate usage rebates:', error)
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

// Every opening starts from the first page of all records.
watch(
  () => props.show,
  (show) => {
    if (!show) return
    records.value = []
    total.value = 0
    page.value = 1
    searchInput.value = ''
    search.value = ''
    void load()
  },
  { immediate: true }
)
</script>
