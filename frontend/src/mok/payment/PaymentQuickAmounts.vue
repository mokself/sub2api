<template>
  <!-- The fork's quick amounts card in the payment tab of views/admin/SettingsView.vue; saves on its own. -->
  <div class="card" data-testid="mok-quick-amounts">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <h3 class="text-base font-medium text-gray-900 dark:text-white">{{ mt('paymentQuickAmounts.title') }}</h3>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{ mt('paymentQuickAmounts.description', { defaults: upstreamQuickAmounts.join('、') }) }}
      </p>
    </div>

    <div v-if="loading" class="flex justify-center py-6">
      <div class="h-5 w-5 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
    </div>

    <div
      v-else-if="!loaded"
      class="m-6 flex items-center justify-between gap-3 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-300"
      role="alert"
    >
      <span>{{ mt('paymentQuickAmounts.loadFailed') }}</span>
      <button type="button" class="btn btn-secondary btn-sm" @click="load">{{ mt('paymentQuickAmounts.retry') }}</button>
    </div>

    <div v-else class="space-y-4 px-6 py-6">
      <div>
        <label class="input-label">{{ mt('paymentQuickAmounts.label') }}</label>
        <input
          v-model="text"
          type="text"
          class="input"
          :placeholder="mt('paymentQuickAmounts.placeholder')"
          data-testid="mok-quick-amounts-input"
          @keyup.enter="save"
        />
        <p v-if="errorText" class="mt-1 text-xs text-red-600 dark:text-red-400" data-testid="mok-quick-amounts-error">{{ errorText }}</p>
        <p v-else class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ mt('paymentQuickAmounts.hint') }}</p>
      </div>

      <div>
        <div class="mb-2 text-xs font-medium text-gray-500 dark:text-gray-400">
          {{ mt('paymentQuickAmounts.preview') }}
          <span v-if="previewIsDefault" class="ml-1 font-normal text-gray-400 dark:text-gray-500">{{ mt('paymentQuickAmounts.usingDefault') }}</span>
        </div>
        <div class="grid max-w-md grid-cols-3 gap-2" data-testid="mok-quick-amounts-preview">
          <span
            v-for="amount in preview"
            :key="amount"
            class="rounded-lg border-2 border-gray-200 bg-white px-3 py-2 text-center text-sm font-medium text-gray-700 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-200"
          >
            {{ amount }}
          </span>
        </div>
      </div>

      <div class="flex flex-wrap items-center gap-3">
        <button type="button" class="btn btn-primary" :disabled="saving || !parsed.ok" data-testid="mok-quick-amounts-save" @click="save">
          {{ saving ? mt('paymentQuickAmounts.saving') : mt('paymentQuickAmounts.save') }}
        </button>
        <button
          type="button"
          class="btn btn-secondary"
          :disabled="saving || saved.length === 0"
          data-testid="mok-quick-amounts-reset"
          @click="reset"
        >
          {{ mt('paymentQuickAmounts.reset') }}
        </button>
        <span v-if="dirty" class="text-xs text-amber-600 dark:text-amber-400" data-testid="mok-quick-amounts-unsaved">
          {{ mt('paymentQuickAmounts.unsaved') }}
        </span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { getAdminPaymentQuickAmounts, savePaymentQuickAmounts } from '../api/paymentQuickAmounts'
import { useMokI18n } from '../i18n'
import { parseQuickAmounts, quickAmountsMax, setQuickAmounts, upstreamQuickAmounts } from './quickAmounts'

const { mt } = useMokI18n()
const appStore = useAppStore()

const loading = ref(true)
const loaded = ref(false)
const saving = ref(false)
const text = ref('')
/** The stored list; empty means upstream's buttons. */
const saved = ref<number[]>([])

const parsed = computed(() => parseQuickAmounts(text.value))
const errorText = computed(() => {
  const result = parsed.value
  if (result.ok) return ''
  return mt(`paymentQuickAmounts.errors.${result.error}`, { value: result.value, max: quickAmountsMax })
})
const preview = computed(() => (parsed.value.ok && parsed.value.amounts.length > 0 ? parsed.value.amounts : upstreamQuickAmounts))
const previewIsDefault = computed(() => !parsed.value.ok || parsed.value.amounts.length === 0)
const dirty = computed(() => {
  const result = parsed.value
  return !result.ok || result.amounts.join(',') !== saved.value.join(',')
})

function apply(amounts: number[]): void {
  saved.value = [...amounts]
  text.value = amounts.join(', ')
  // The admin's own recharge page shows the new buttons without a reload.
  setQuickAmounts(amounts)
}

async function load(): Promise<void> {
  loading.value = true
  try {
    apply((await getAdminPaymentQuickAmounts()).amounts ?? [])
    loaded.value = true
  } catch (error) {
    console.error('Failed to load the quick amounts:', error)
    loaded.value = false
  } finally {
    loading.value = false
  }
}

async function store(amounts: number[], success: string): Promise<void> {
  if (saving.value) return
  saving.value = true
  try {
    apply((await savePaymentQuickAmounts(amounts)).amounts ?? [])
    appStore.showSuccess(success)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, mt('paymentQuickAmounts.saveFailed')))
  } finally {
    saving.value = false
  }
}

function save(): void {
  const result = parsed.value
  if (!result.ok) return
  void store(result.amounts, result.amounts.length > 0 ? mt('paymentQuickAmounts.saved') : mt('paymentQuickAmounts.resetDone'))
}

function reset(): void {
  void store([], mt('paymentQuickAmounts.resetDone'))
}

onMounted(load)
</script>
