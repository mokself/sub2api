<template>
  <div class="border-t border-gray-100 pt-6 dark:border-dark-700" data-testid="mok-affiliate-mode">
    <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ mt('affiliateMode.title') }}</h3>
    <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ mt('affiliateMode.description') }}</p>

    <div v-if="loading" class="flex justify-center py-4">
      <div class="h-5 w-5 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
    </div>

    <div
      v-else-if="!status"
      class="mt-3 flex items-center justify-between gap-3 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-300"
      role="alert"
    >
      <span>{{ mt('affiliateMode.loadFailed') }}</span>
      <button type="button" class="btn btn-secondary btn-sm" @click="load">{{ mt('affiliateMode.retry') }}</button>
    </div>

    <template v-else>
      <div
        class="mt-3 inline-flex rounded-lg border border-gray-200 bg-gray-50 p-0.5 dark:border-dark-600 dark:bg-dark-800"
        role="group"
      >
        <button
          v-for="option in modes"
          :key="option"
          type="button"
          :class="[
            'rounded-md px-3 py-1 text-sm font-medium transition-colors',
            status.mode === option
              ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white'
              : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'
          ]"
          :aria-pressed="status.mode === option"
          :disabled="switching"
          :data-testid="`mok-affiliate-mode-${option}`"
          @click="requestSwitch(option)"
        >
          {{ mt(`affiliateMode.${option}`) }}
        </button>
      </div>

      <p v-if="windowText" class="mt-2 text-xs text-gray-500 dark:text-gray-400" data-testid="mok-affiliate-mode-window">
        {{ windowText }}
      </p>
      <p v-if="status.mode === 'usage'" class="mt-1 text-xs text-gray-400 dark:text-gray-500">
        {{ mt('affiliateMode.settleHint') }}
      </p>

      <div v-if="status.mode === 'usage' || status.recent_days.length > 0" class="mt-4">
        <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ mt('affiliateMode.recentDays') }}</p>
        <p
          v-if="status.recent_days.length === 0"
          class="mt-2 rounded-xl border border-dashed border-gray-300 px-4 py-3 text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400"
        >
          {{ mt('affiliateMode.noDays') }}
        </p>
        <div v-else class="mt-2 overflow-hidden rounded-xl border border-gray-200 dark:border-dark-600">
          <table class="w-full text-sm">
            <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="px-3 py-2 text-left font-medium">{{ mt('affiliateMode.columns.day') }}</th>
                <th class="px-3 py-2 text-right font-medium">{{ mt('affiliateMode.columns.invitees') }}</th>
                <th class="px-3 py-2 text-right font-medium">{{ mt('affiliateMode.columns.usage') }}</th>
                <th class="px-3 py-2 text-right font-medium">{{ mt('affiliateMode.columns.rebate') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="day in status.recent_days" :key="day.day" data-testid="mok-affiliate-mode-day">
                <td class="px-3 py-2 text-gray-700 dark:text-gray-300">{{ day.day }}</td>
                <template v-if="day.skipped_reason">
                  <td colspan="3" class="px-3 py-2 text-right text-xs text-amber-600 dark:text-amber-400">
                    {{ skippedText(day.skipped_reason) }}
                  </td>
                </template>
                <template v-else>
                  <td class="px-3 py-2 text-right tabular-nums text-gray-700 dark:text-gray-300">{{ day.invitee_count }}</td>
                  <td class="px-3 py-2 text-right tabular-nums text-gray-700 dark:text-gray-300">{{ formatCurrency(day.usage_total) }}</td>
                  <td class="px-3 py-2 text-right tabular-nums font-medium text-gray-900 dark:text-white">{{ formatCurrency(day.rebate_total) }}</td>
                </template>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </template>

    <ConfirmDialog
      :show="pendingMode !== null"
      :title="mt('affiliateMode.confirmTitle')"
      :message="pendingMode === 'usage' ? mt('affiliateMode.confirmToUsage') : mt('affiliateMode.confirmToRecharge')"
      :confirm-text="mt('affiliateMode.confirm')"
      :cancel-text="mt('affiliateMode.cancel')"
      @confirm="confirmSwitch"
      @cancel="pendingMode = null"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatCurrency, formatDateTime } from '@/utils/format'
import {
  getAffiliateRebateMode,
  setAffiliateRebateMode,
  type AffiliateRebateMode,
  type AffiliateRebateModeStatus
} from '../api/affiliateMode'
import { useMokI18n } from '../i18n'

const { mt } = useMokI18n()
const appStore = useAppStore()

const modes: AffiliateRebateMode[] = ['recharge', 'usage']
const loading = ref(true)
const switching = ref(false)
const status = ref<AffiliateRebateModeStatus | null>(null)
const pendingMode = ref<AffiliateRebateMode | null>(null)

const windowText = computed(() => {
  const s = status.value
  if (!s) return ''
  if (s.mode === 'usage' && s.usage_since) {
    return mt('affiliateMode.since', { time: formatDateTime(s.usage_since) })
  }
  if (s.mode === 'recharge' && s.usage_until) {
    return mt('affiliateMode.until', { time: formatDateTime(s.usage_until) })
  }
  return ''
})

function skippedText(reason: string): string {
  return reason === 'affiliate_disabled' ? mt('affiliateMode.skipped.affiliate_disabled') : reason
}

async function load(): Promise<void> {
  loading.value = true
  try {
    status.value = await getAffiliateRebateMode()
  } catch (error) {
    console.error('Failed to load affiliate rebate mode:', error)
    status.value = null
  } finally {
    loading.value = false
  }
}

function requestSwitch(mode: AffiliateRebateMode): void {
  if (status.value && status.value.mode !== mode) {
    pendingMode.value = mode
  }
}

async function confirmSwitch(): Promise<void> {
  const mode = pendingMode.value
  pendingMode.value = null
  if (!mode || switching.value) return
  switching.value = true
  try {
    status.value = await setAffiliateRebateMode(mode)
    appStore.showSuccess(mt('affiliateMode.switched'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, mt('affiliateMode.switchFailed')))
  } finally {
    switching.value = false
  }
}

onMounted(load)
</script>
