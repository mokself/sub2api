<template>
  <!--
    The user's SMS reminder phone. Placed under the notification emails of upstream's
    low balance card (components/user/profile/ProfileBalanceNotifyCard.vue); as a card of
    its own on the profile page when the admin has switched the low balance reminder off
    (standalone), where it is only about the subscription expiry reminder.
  -->
  <div v-if="visible" :class="standalone ? 'card' : ''" data-testid="mok-sms-phone">
    <div v-if="standalone" class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <h2 class="text-lg font-medium text-gray-900 dark:text-white">{{ mt('sms.user.title') }}</h2>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ description }}</p>
    </div>
    <div :class="standalone ? 'px-6 py-6' : ''">
      <template v-if="!standalone">
        <label class="input-label">{{ mt('sms.user.title') }}</label>
        <p class="mb-2 text-xs text-gray-500 dark:text-gray-400">{{ description }}</p>
      </template>

      <div
        v-if="status?.phone"
        class="flex items-center justify-between rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-700"
        data-testid="mok-sms-phone-bound"
      >
        <div class="flex min-w-0 flex-1 items-center gap-2">
          <label class="relative inline-flex shrink-0 cursor-pointer items-center">
            <input
              type="checkbox"
              class="peer sr-only"
              :checked="status.enabled"
              :disabled="busy"
              data-testid="mok-sms-phone-toggle"
              @change="toggle($event)"
            />
            <div class="peer h-5 w-9 rounded-full bg-gray-200 after:absolute after:left-[2px] after:top-[2px] after:h-4 after:w-4 after:rounded-full after:border after:border-gray-300 after:bg-white after:transition-all after:content-[''] peer-checked:bg-primary-600 peer-checked:after:translate-x-full peer-checked:after:border-white peer-focus:outline-none dark:bg-gray-600 dark:after:border-gray-500"></div>
          </label>
          <span class="truncate text-sm tabular-nums text-gray-700 dark:text-gray-300">+86 {{ status.phone }}</span>
        </div>
        <div class="flex shrink-0 items-center gap-2">
          <span class="text-xs text-green-500">{{ mt('sms.user.verified') }}</span>
          <button type="button" class="text-xs text-red-500 hover:text-red-700" :disabled="busy" data-testid="mok-sms-phone-remove" @click="remove">
            {{ mt('sms.user.remove') }}
          </button>
        </div>
      </div>

      <div v-else class="space-y-2">
        <div class="flex gap-2">
          <div class="relative flex-1">
            <span class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-sm text-gray-400">+86</span>
            <input
              v-model="phoneInput"
              type="tel"
              maxlength="20"
              class="input pl-12"
              :placeholder="mt('sms.user.phonePlaceholder')"
              :disabled="codeSent"
              data-testid="mok-sms-phone-input"
              @keyup.enter="sendCode"
            />
          </div>
          <button
            type="button"
            class="btn btn-secondary whitespace-nowrap"
            :disabled="sending || countdown > 0 || !phoneInput.trim()"
            data-testid="mok-sms-phone-send"
            @click="sendCode"
          >
            {{ countdown > 0 ? mt('sms.user.countdown', { seconds: countdown }) : codeSent ? mt('sms.user.resend') : mt('sms.user.sendCode') }}
          </button>
        </div>
        <div v-if="codeSent" class="flex gap-2">
          <input
            v-model="codeInput"
            type="text"
            inputmode="numeric"
            maxlength="6"
            class="input flex-1"
            :placeholder="mt('sms.user.codePlaceholder')"
            data-testid="mok-sms-phone-code"
            @keyup.enter="bind"
          />
          <button
            type="button"
            class="btn btn-primary whitespace-nowrap"
            :disabled="binding || codeInput.trim().length !== 6"
            data-testid="mok-sms-phone-bind"
            @click="bind"
          >
            {{ mt('sms.user.bind') }}
          </button>
          <button type="button" class="btn btn-secondary whitespace-nowrap" data-testid="mok-sms-phone-cancel" @click="reset">
            {{ mt('sms.user.cancel') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useAppStore } from '@/stores/app'
import {
  bindSmsPhone,
  getMySmsPhone,
  removeSmsPhone,
  sendSmsBindCode,
  setSmsPhoneEnabled,
  type SmsPhoneStatus
} from '../api/sms'
import { useMokI18n } from '../i18n'
import { normalizeSmsPhone, smsErrorMessage } from './sms'

const props = defineProps<{
  /** Rendered as a card of its own, without the low balance reminder. */
  standalone?: boolean
}>()

const { mt } = useMokI18n()
const appStore = useAppStore()

const status = ref<SmsPhoneStatus | null>(null)
const phoneInput = ref('')
const codeInput = ref('')
const codeSent = ref(false)
const sending = ref(false)
const binding = ref(false)
const busy = ref(false)
const countdown = ref(0)
let timer: ReturnType<typeof setInterval> | null = null

// The standalone card shows up while the low balance reminder is switched off upstream, so
// only the subscription expiry reminder can reach the phone then.
const events = computed(() =>
  (status.value?.events ?? []).filter((event) => !props.standalone || event !== 'balance_low')
)
const visible = computed(() => !!status.value?.available && events.value.length > 0)
const description = computed(() =>
  mt('sms.user.description', {
    events: events.value.map((event) => mt(`sms.user.events.${event}`)).join(mt('sms.user.and'))
  })
)

function stopCountdown(): void {
  if (timer) clearInterval(timer)
  timer = null
}

function startCountdown(seconds: number): void {
  stopCountdown()
  countdown.value = seconds
  timer = setInterval(() => {
    countdown.value -= 1
    if (countdown.value <= 0) stopCountdown()
  }, 1000)
}

function reset(): void {
  stopCountdown()
  countdown.value = 0
  codeSent.value = false
  codeInput.value = ''
}

async function load(): Promise<void> {
  try {
    status.value = await getMySmsPhone()
  } catch (error) {
    console.error('Failed to load the SMS reminder phone:', error)
    status.value = null
  }
}

async function sendCode(): Promise<void> {
  if (sending.value || countdown.value > 0) return
  const phone = normalizeSmsPhone(phoneInput.value)
  if (!phone) {
    appStore.showError(mt('sms.errors.MOK_SMS_PHONE_INVALID'))
    return
  }
  sending.value = true
  try {
    const sent = await sendSmsBindCode(phone)
    phoneInput.value = phone
    codeSent.value = true
    startCountdown(sent.cooldown_seconds || 60)
    appStore.showSuccess(mt('sms.user.codeSent', { phone }))
  } catch (error) {
    appStore.showError(smsErrorMessage(error, mt, mt('sms.user.sendFailed')))
  } finally {
    sending.value = false
  }
}

async function bind(): Promise<void> {
  if (binding.value || codeInput.value.trim().length !== 6) return
  binding.value = true
  try {
    status.value = await bindSmsPhone(phoneInput.value, codeInput.value.trim())
    reset()
    phoneInput.value = ''
    appStore.showSuccess(mt('sms.user.bound'))
  } catch (error) {
    appStore.showError(smsErrorMessage(error, mt, mt('sms.user.bindFailed')))
  } finally {
    binding.value = false
  }
}

async function toggle(event: Event): Promise<void> {
  const box = event.target as HTMLInputElement
  if (!status.value || busy.value) {
    box.checked = !!status.value?.enabled
    return
  }
  const enabled = !status.value.enabled
  busy.value = true
  try {
    status.value = await setSmsPhoneEnabled(enabled)
    appStore.showSuccess(enabled ? mt('sms.user.resumed') : mt('sms.user.paused'))
  } catch (error) {
    // The box flipped already; put it back to the stored state.
    box.checked = !!status.value?.enabled
    appStore.showError(smsErrorMessage(error, mt, mt('sms.user.updateFailed')))
  } finally {
    busy.value = false
  }
}

async function remove(): Promise<void> {
  if (busy.value) return
  busy.value = true
  try {
    status.value = await removeSmsPhone()
    appStore.showSuccess(mt('sms.user.removed'))
  } catch (error) {
    appStore.showError(smsErrorMessage(error, mt, mt('sms.user.updateFailed')))
  } finally {
    busy.value = false
  }
}

onMounted(load)
onUnmounted(stopCountdown)
</script>
