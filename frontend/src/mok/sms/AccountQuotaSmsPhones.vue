<template>
  <!--
    The SMS phones of the account quota alerts, below the notification emails of upstream's
    account quota notification card (views/admin/SettingsView.vue). Laid out like the email
    list; saved by the page's save button, see ./adminAlerts.ts.
  -->
  <div data-testid="mok-quota-sms">
    <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">{{ mt('smsQuota.phones') }}</label>

    <div v-if="state.status === 'loading' || state.status === 'idle'" class="text-sm text-gray-500">…</div>

    <div
      v-else-if="state.status === 'failed'"
      class="flex items-center justify-between gap-3 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-300"
      role="alert"
    >
      <span>{{ mt('smsQuota.loadFailed') }}</span>
      <button type="button" class="btn btn-secondary btn-sm" @click="loadAccountQuotaSms">{{ mt('smsQuota.retry') }}</button>
    </div>

    <template v-else>
      <p
        v-if="!state.ready && state.draft.phones.length > 0"
        class="mb-2 rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:bg-amber-900/20 dark:text-amber-300"
        data-testid="mok-quota-sms-not-ready"
      >
        {{ mt('smsQuota.notReady') }}
      </p>
      <div class="space-y-2">
        <div v-for="(entry, index) in state.draft.phones" :key="index" class="flex items-center gap-2" data-testid="mok-quota-sms-row">
          <label class="relative inline-flex shrink-0 cursor-pointer items-center">
            <input type="checkbox" :checked="!entry.disabled" class="peer sr-only" @change="entry.disabled = !entry.disabled" />
            <div
              class="peer h-5 w-9 rounded-full bg-gray-200 after:absolute after:left-[2px] after:top-[2px] after:h-4 after:w-4 after:rounded-full after:border after:border-gray-300 after:bg-white after:transition-all after:content-[''] peer-checked:bg-primary-600 peer-checked:after:translate-x-full peer-checked:after:border-white peer-focus:outline-none dark:bg-gray-600 dark:after:border-gray-500"
            ></div>
          </label>
          <input
            v-model="entry.phone"
            type="tel"
            class="input flex-1"
            :placeholder="mt('smsQuota.phonePlaceholder')"
            data-testid="mok-quota-sms-phone"
          />
          <button
            type="button"
            class="btn btn-secondary px-2"
            :aria-label="mt('smsQuota.remove')"
            data-testid="mok-quota-sms-remove"
            @click="state.draft.phones.splice(index, 1)"
          >
            <Icon name="x" size="xs" class="h-4 w-4" />
          </button>
        </div>
        <button
          v-if="state.draft.phones.length < maxAdminPhones"
          type="button"
          class="btn btn-secondary btn-sm"
          data-testid="mok-quota-sms-add"
          @click="state.draft.phones.push({ phone: '', disabled: false })"
        >
          + {{ mt('smsQuota.add') }}
        </button>
      </div>
      <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ mt('smsQuota.hint') }}</p>
    </template>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { useMokI18n } from '../i18n'
import { accountQuotaState, loadAccountQuotaSms, maxAdminPhones } from './adminAlerts'

const { mt } = useMokI18n()

const state = accountQuotaState

// Every visit to the settings page starts from the stored phones.
onMounted(loadAccountQuotaSms)
</script>
