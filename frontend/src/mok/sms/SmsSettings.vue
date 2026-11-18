<template>
  <!-- The fork's SMS card at the end of the email tab in views/admin/SettingsView.vue; saves on its own. -->
  <div class="card" data-testid="mok-sms-settings">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <h3 class="text-base font-medium text-gray-900 dark:text-white">{{ mt('sms.title') }}</h3>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ mt('sms.description') }}</p>
    </div>

    <div v-if="loading" class="flex justify-center py-8">
      <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
    </div>

    <div
      v-else-if="!loaded"
      class="m-6 flex items-center justify-between gap-3 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-300"
      role="alert"
    >
      <span>{{ mt('sms.loadFailed') }}</span>
      <button type="button" class="btn btn-secondary btn-sm" @click="load">{{ mt('sms.retry') }}</button>
    </div>

    <div v-else class="space-y-6 px-6 py-6">
      <div class="flex items-center justify-between gap-4">
        <label class="mb-0 block text-sm font-medium text-gray-700 dark:text-gray-300">{{ mt('sms.enabled') }}</label>
        <Toggle v-model="form.enabled" data-testid="mok-sms-enabled" />
      </div>
      <div class="flex items-start justify-between gap-4">
        <div>
          <label class="mb-0 block text-sm font-medium text-gray-700 dark:text-gray-300">{{ mt('sms.dryRun') }}</label>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ mt('sms.dryRunHint') }}</p>
        </div>
        <Toggle v-model="form.dry_run" data-testid="mok-sms-dry-run" />
      </div>

      <section class="border-t border-gray-100 pt-6 dark:border-dark-700">
        <h4 class="text-sm font-semibold text-gray-900 dark:text-white">{{ mt('sms.provider') }}</h4>
        <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ mt('sms.providerHint') }}</p>
        <div class="mt-3 grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">{{ mt('sms.secretId') }}</label>
            <input v-model="form.secret_id" type="text" class="input" autocomplete="off" data-testid="mok-sms-secret-id" />
          </div>
          <div>
            <label class="input-label">{{ mt('sms.secretKey') }}</label>
            <input
              v-model="secretKeyInput"
              type="password"
              class="input"
              autocomplete="new-password"
              :placeholder="secretKeySet ? mt('sms.secretKeySet') : ''"
              data-testid="mok-sms-secret-key"
            />
          </div>
          <div>
            <label class="input-label">{{ mt('sms.sdkAppId') }}</label>
            <input v-model="form.sdk_app_id" type="text" inputmode="numeric" class="input" placeholder="1400000000" data-testid="mok-sms-app-id" />
          </div>
          <div>
            <label class="input-label">{{ mt('sms.signName') }}</label>
            <input v-model="form.sign_name" type="text" class="input" :placeholder="mt('sms.signNamePlaceholder')" data-testid="mok-sms-sign-name" />
          </div>
          <div>
            <label class="input-label">{{ mt('sms.region') }}</label>
            <select v-model="form.region" class="input" data-testid="mok-sms-region">
              <option v-for="region in regions" :key="region.value" :value="region.value">{{ region.label }}</option>
            </select>
          </div>
        </div>
      </section>

      <section class="border-t border-gray-100 pt-6 dark:border-dark-700">
        <h4 class="text-sm font-semibold text-gray-900 dark:text-white">{{ mt('sms.templates') }}</h4>
        <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ mt('sms.templatesHint') }}</p>
        <div class="mt-3 divide-y divide-gray-100 rounded-xl border border-gray-200 dark:divide-dark-700 dark:border-dark-600">
          <div v-for="kind in templateKinds" :key="kind" class="px-4 py-3" :data-testid="`mok-sms-template-${kind}`">
            <div class="flex flex-wrap items-center gap-3">
              <div class="min-w-[9rem] flex-1">
                <div class="text-sm font-medium text-gray-900 dark:text-white">{{ mt(`sms.kinds.${kind}.name`) }}</div>
              </div>
              <div v-if="kind !== 'verify_code'" class="flex items-center gap-2">
                <span class="text-xs text-gray-500 dark:text-gray-400">{{ mt('sms.columns.send') }}</span>
                <Toggle v-model="form.events[kind]" :data-testid="`mok-sms-event-${kind}`" />
              </div>
              <span v-else class="text-xs text-gray-400 dark:text-gray-500">{{ mt('sms.always') }}</span>
              <input
                v-model="form.templates[kind]"
                type="text"
                inputmode="numeric"
                class="input w-36"
                :placeholder="mt('sms.templateIdPlaceholder')"
                :aria-label="`${mt(`sms.kinds.${kind}.name`)} ${mt('sms.columns.templateId')}`"
                :data-testid="`mok-sms-template-id-${kind}`"
              />
            </div>
            <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">{{ mt(`sms.kinds.${kind}.hint`) }}</p>
            <div class="mt-1.5 flex items-start gap-2 rounded-lg bg-gray-50 px-3 py-2 text-xs dark:bg-dark-800">
              <div class="flex-1 text-gray-600 dark:text-gray-300">
                <span class="text-gray-400 dark:text-gray-500">{{ mt('sms.suggested') }}：</span>{{ suggestedTemplates[kind] }}
                <div class="mt-0.5 text-gray-400 dark:text-gray-500">{{ mt('sms.params', { params: mt(`sms.kinds.${kind}.params`) }) }}</div>
              </div>
              <button
                type="button"
                class="shrink-0 text-primary-600 hover:text-primary-700 dark:text-primary-400"
                :data-testid="`mok-sms-copy-${kind}`"
                @click="copyTemplate(kind)"
              >
                {{ mt('sms.copy') }}
              </button>
            </div>
          </div>
        </div>
      </section>

      <section class="grid grid-cols-1 gap-4 border-t border-gray-100 pt-6 dark:border-dark-700 md:grid-cols-2">
        <div class="md:row-span-2">
          <label class="input-label">{{ mt('sms.adminPhones') }}</label>
          <textarea v-model="adminPhonesText" rows="4" class="input font-mono" placeholder="13800000000" data-testid="mok-sms-admin-phones"></textarea>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ mt('sms.adminPhonesHint') }}</p>
        </div>
        <div>
          <label class="input-label">{{ mt('sms.minLevel') }}</label>
          <select v-model="form.ops_alert_min_level" class="input" data-testid="mok-sms-min-level">
            <option v-for="level in minLevels" :key="level.value" :value="level.value">{{ level.label }}</option>
          </select>
        </div>
        <div>
          <label class="input-label">{{ mt('sms.dailyLimit') }}</label>
          <div class="flex items-center gap-2">
            <input v-model.number="form.daily_limit" type="number" min="0" max="1000" step="1" class="input w-32" data-testid="mok-sms-daily-limit" />
            <span class="text-sm text-gray-500 dark:text-gray-400">{{ mt('sms.dailyLimitUnit') }}</span>
          </div>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ mt('sms.dailyLimitHint') }}</p>
        </div>
      </section>

      <div class="flex flex-wrap items-center gap-3 border-t border-gray-100 pt-6 dark:border-dark-700">
        <button type="button" class="btn btn-primary" :disabled="saving" data-testid="mok-sms-save" @click="save">
          {{ saving ? mt('sms.saving') : mt('sms.save') }}
        </button>
        <button type="button" class="btn btn-secondary" data-testid="mok-sms-show-logs" @click="showLogs = true">
          {{ mt('sms.showLogs') }}
        </button>
        <span v-if="dirty" class="text-xs text-amber-600 dark:text-amber-400" data-testid="mok-sms-unsaved">{{ mt('sms.unsaved') }}</span>
      </div>

      <section class="rounded-xl border border-gray-200 px-4 py-4 dark:border-dark-600">
        <h4 class="text-sm font-semibold text-gray-900 dark:text-white">{{ mt('sms.test.title') }}</h4>
        <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ mt('sms.test.hint') }}</p>
        <div class="mt-3 flex flex-wrap gap-2">
          <input v-model="testPhone" type="tel" class="input w-56" :placeholder="mt('sms.test.phonePlaceholder')" data-testid="mok-sms-test-phone" />
          <select v-model="testKind" class="input w-48" data-testid="mok-sms-test-kind">
            <option v-for="kind in templateKinds" :key="kind" :value="kind">{{ mt(`sms.kinds.${kind}.name`) }}</option>
          </select>
          <button
            type="button"
            class="btn btn-secondary"
            :disabled="testing || dirty || !testPhone.trim()"
            :title="dirty ? mt('sms.test.saveFirst') : ''"
            data-testid="mok-sms-test-send"
            @click="sendTest"
          >
            {{ testing ? mt('sms.test.sending') : mt('sms.test.send') }}
          </button>
        </div>
        <p v-if="dirty" class="mt-2 text-xs text-gray-400 dark:text-gray-500">{{ mt('sms.test.saveFirst') }}</p>
      </section>
    </div>

    <SmsLogs :show="showLogs" @close="showLogs = false" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import Toggle from '@/components/common/Toggle.vue'
import { useClipboard } from '@/composables/useClipboard'
import { useAppStore } from '@/stores/app'
import {
  getSmsConfig,
  saveSmsConfig,
  sendTestSms,
  type SmsConfig,
  type SmsConfigView,
  type SmsMinLevel,
  type SmsRegion,
  type SmsTemplateKind
} from '../api/sms'
import { useMokI18n } from '../i18n'
import SmsLogs from './SmsLogs.vue'
import { isSmsTemplateId, parseSmsPhoneLines, smsErrorMessage, smsSuggestedTemplates, smsTemplateKinds } from './sms'

const { mt } = useMokI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const templateKinds = smsTemplateKinds
const suggestedTemplates = smsSuggestedTemplates
const regions = computed<{ value: SmsRegion; label: string }[]>(() => [
  { value: 'ap-guangzhou', label: mt('sms.regions.guangzhou') },
  { value: 'ap-beijing', label: mt('sms.regions.beijing') },
  { value: 'ap-nanjing', label: mt('sms.regions.nanjing') }
])
const minLevels = computed<{ value: SmsMinLevel; label: string }[]>(() => [
  { value: '', label: mt('sms.minLevels.all') },
  { value: 'info', label: mt('sms.minLevels.info') },
  { value: 'warning', label: mt('sms.minLevels.warning') },
  { value: 'critical', label: mt('sms.minLevels.critical') }
])

const loading = ref(true)
const loaded = ref(false)
const saving = ref(false)
const testing = ref(false)
const showLogs = ref(false)
const secretKeySet = ref(false)
const secretKeyInput = ref('')
const adminPhonesText = ref('')
const testPhone = ref('')
const testKind = ref<SmsTemplateKind>('verify_code')
// What the form held right after loading or saving, to tell unsaved changes apart.
const savedSnapshot = ref('')

const form = reactive<SmsConfig>(emptyConfig())

function emptyConfig(): SmsConfig {
  return {
    enabled: false,
    dry_run: false,
    secret_id: '',
    secret_key: '',
    sdk_app_id: '',
    sign_name: '',
    region: 'ap-guangzhou',
    daily_limit: 10,
    templates: { verify_code: '', balance_low: '', subscription_expiry: '', ops_alert: '', account_quota: '' },
    events: { balance_low: true, subscription_expiry: true, ops_alert: true, account_quota: true },
    admin_phones: [],
    ops_alert_min_level: ''
  }
}

function snapshot(): string {
  return JSON.stringify({ form, adminPhones: adminPhonesText.value.trim(), secretKey: secretKeyInput.value })
}

const dirty = computed(() => loaded.value && snapshot() !== savedSnapshot.value)

function apply(view: SmsConfigView): void {
  const base = emptyConfig()
  Object.assign(form, {
    ...base,
    ...view,
    secret_key: '',
    templates: { ...base.templates, ...view.templates },
    events: { ...base.events, ...view.events },
    admin_phones: [...(view.admin_phones ?? [])],
    ops_alert_min_level: view.ops_alert_min_level ?? ''
  })
  secretKeySet.value = view.secret_key_set
  secretKeyInput.value = ''
  adminPhonesText.value = (view.admin_phones ?? []).join('\n')
  savedSnapshot.value = snapshot()
}

async function load(): Promise<void> {
  loading.value = true
  try {
    apply(await getSmsConfig())
    loaded.value = true
  } catch (error) {
    console.error('Failed to load the SMS settings:', error)
    loaded.value = false
  } finally {
    loading.value = false
  }
}

/** Checks the form; returns the payload or shows the first problem. */
function validate(): SmsConfig | null {
  const { phones, invalid } = parseSmsPhoneLines(adminPhonesText.value)
  const fail = (message: string) => {
    appStore.showError(message)
    return null
  }
  if (invalid.length > 0) return fail(mt('sms.validation.phones', { phones: invalid.join('、') }))
  if (phones.length > 10) return fail(mt('sms.validation.tooManyPhones'))
  if (Object.values(form.templates).some((id) => !isSmsTemplateId(id))) return fail(mt('sms.validation.templateId'))
  if (form.sdk_app_id.trim() !== '' && !/^\d+$/.test(form.sdk_app_id.trim())) return fail(mt('sms.validation.appId'))
  const limit = Number(form.daily_limit)
  if (!Number.isInteger(limit) || limit < 0 || limit > 1000) return fail(mt('sms.validation.dailyLimit'))
  const hasKey = secretKeySet.value || secretKeyInput.value.trim() !== ''
  const credentials = form.secret_id.trim() && hasKey && form.sdk_app_id.trim() && form.sign_name.trim()
  if (form.enabled && !form.dry_run && !credentials) return fail(mt('sms.validation.credentials'))
  return {
    ...form,
    secret_key: secretKeyInput.value.trim(),
    daily_limit: limit,
    templates: { ...form.templates },
    events: { ...form.events },
    admin_phones: phones
  }
}

async function save(): Promise<void> {
  const payload = validate()
  if (!payload || saving.value) return
  saving.value = true
  try {
    apply(await saveSmsConfig(payload))
    appStore.showSuccess(mt('sms.saved'))
  } catch (error) {
    appStore.showError(smsErrorMessage(error, mt, mt('sms.saveFailed')))
  } finally {
    saving.value = false
  }
}

async function sendTest(): Promise<void> {
  if (testing.value || dirty.value) return
  testing.value = true
  try {
    const result = await sendTestSms(testPhone.value.trim(), testKind.value)
    if (result.status === 'sent') appStore.showSuccess(mt('sms.test.sent'))
    else if (result.status === 'dry_run') appStore.showSuccess(mt('sms.test.dryRun'))
    else appStore.showError(mt('sms.test.failed', { error: result.error || result.status }))
  } catch (error) {
    appStore.showError(smsErrorMessage(error, mt, mt('sms.test.requestFailed')))
  } finally {
    testing.value = false
  }
}

function copyTemplate(kind: SmsTemplateKind): void {
  void copyToClipboard(smsSuggestedTemplates[kind], mt('sms.copied'))
}

onMounted(load)
</script>
