<template>
  <!--
    The SMS part of the ops alerts, below the alert emails in upstream's ops monitoring
    settings dialog (views/admin/ops/components/OpsSettingsDialog.vue). Laid out like the
    email section; saved by the dialog's save button, see ./adminAlerts.ts.
  -->
  <div class="rounded-2xl bg-gray-50 p-4 dark:bg-dark-700/50" data-testid="mok-ops-alert-sms">
    <h4 class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">{{ mt('smsOps.title') }}</h4>

    <div v-if="state.status === 'loading' || state.status === 'idle'" class="py-2 text-sm text-gray-500">…</div>

    <div
      v-else-if="state.status === 'failed'"
      class="flex items-center justify-between gap-3 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-300"
      role="alert"
    >
      <span>{{ mt('smsOps.loadFailed') }}</span>
      <button type="button" class="btn btn-secondary btn-sm" @click="loadOpsAlertSms">{{ mt('smsOps.retry') }}</button>
    </div>

    <div v-else class="space-y-4">
      <div class="flex items-center justify-between">
        <label class="font-medium text-gray-900 dark:text-white">{{ mt('smsOps.enabled') }}</label>
        <Toggle v-model="state.draft.enabled" data-testid="mok-ops-alert-sms-enabled" />
      </div>

      <p
        v-if="state.draft.enabled && !state.ready"
        class="rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:bg-amber-900/20 dark:text-amber-300"
        data-testid="mok-ops-alert-sms-not-ready"
      >
        {{ mt('smsOps.notReady') }}
      </p>

      <div v-if="state.draft.enabled">
        <label class="input-label">{{ mt('smsOps.phones') }}</label>
        <div class="flex gap-2">
          <input
            v-model="phoneInput"
            type="tel"
            class="input"
            :placeholder="mt('smsOps.phonePlaceholder')"
            data-testid="mok-ops-alert-sms-phone"
            @keydown.enter.prevent="addPhone"
          />
          <button class="btn btn-secondary whitespace-nowrap" type="button" data-testid="mok-ops-alert-sms-add" @click="addPhone">
            {{ mt('smsOps.add') }}
          </button>
        </div>
        <div class="mt-2 flex flex-wrap gap-2">
          <span
            v-for="phone in state.draft.phones"
            :key="phone"
            class="inline-flex items-center gap-2 rounded-full bg-blue-100 px-3 py-1 text-xs font-medium tabular-nums text-blue-700 dark:bg-blue-900/30 dark:text-blue-400"
            data-testid="mok-ops-alert-sms-chip"
          >
            {{ phone }}
            <button
              type="button"
              class="text-blue-700/80 hover:text-blue-900"
              :aria-label="mt('smsOps.remove')"
              @click="removePhone(phone)"
            >
              ×
            </button>
          </span>
        </div>
        <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">{{ mt('smsOps.hint') }}</p>
      </div>

      <div v-if="state.draft.enabled">
        <label class="input-label">{{ mt('smsOps.minLevel') }}</label>
        <Select v-model="state.draft.min_level" :options="levelOptions" data-testid="mok-ops-alert-sms-level" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'
import Toggle from '@/components/common/Toggle.vue'
import { useAppStore } from '@/stores/app'
import { useMokI18n } from '../i18n'
import { addOpsAlertPhone, loadOpsAlertSms, opsAlertState } from './adminAlerts'

const { mt } = useMokI18n()
const { t } = useI18n()
const appStore = useAppStore()

const state = opsAlertState
const phoneInput = ref('')

// The same choices as the alert email's lowest level.
const levelOptions = computed(() => [
  { value: '', label: t('admin.ops.email.minSeverityAll') },
  { value: 'critical', label: t('common.critical') },
  { value: 'warning', label: t('common.warning') },
  { value: 'info', label: t('common.info') }
])

function addPhone(): void {
  if (!phoneInput.value.trim()) return
  const problem = addOpsAlertPhone(phoneInput.value)
  if (problem) {
    appStore.showError(problem)
    return
  }
  phoneInput.value = ''
}

function removePhone(phone: string): void {
  state.draft.phones = state.draft.phones.filter((p) => p !== phone)
}

// The dialog renders its content each time it opens; every opening starts from the stored settings.
onMounted(loadOpsAlertSms)
</script>
