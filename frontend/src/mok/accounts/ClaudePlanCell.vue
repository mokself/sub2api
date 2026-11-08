<template>
  <span v-if="!applicable" class="text-xs text-gray-400 dark:text-gray-500">-</span>
  <div v-else class="group flex min-w-0 items-center gap-1" data-testid="mok-claude-plan">
    <svg
      v-if="loadingFirst"
      class="h-3 w-3 animate-spin text-gray-400"
      fill="none"
      viewBox="0 0 24 24"
      aria-hidden="true"
    >
      <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
      <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
    </svg>
    <template v-else>
      <span
        v-if="badge"
        data-testid="mok-claude-plan-badge"
        :class="['inline-block whitespace-nowrap rounded px-1.5 py-0.5 text-[10px] font-medium', badge.className]"
        :title="detailTitle"
      >
        {{ badge.label }}
      </span>
      <span
        v-if="subscriptionNote"
        class="whitespace-nowrap text-[10px] text-amber-600 dark:text-amber-400"
        :title="detailTitle"
      >
        {{ subscriptionNote }}
      </span>
      <span
        v-if="errorText"
        role="alert"
        data-testid="mok-claude-plan-error"
        class="whitespace-nowrap text-[10px] font-medium text-red-600 dark:text-red-400"
        :title="errorTitle"
      >
        {{ errorText }}
      </span>
      <span v-if="unsupportedTitle" class="text-xs text-gray-400 dark:text-gray-500" :title="unsupportedTitle">-</span>
      <button
        type="button"
        data-testid="mok-claude-plan-refresh"
        class="rounded p-0.5 text-gray-400 transition hover:bg-gray-100 hover:text-gray-600 focus:opacity-100 disabled:cursor-not-allowed dark:hover:bg-dark-700 dark:hover:text-gray-300"
        :class="errorText ? 'opacity-100' : 'opacity-0 group-hover:opacity-100'"
        :disabled="entry?.loading"
        :title="mt('claudePlan.refresh')"
        :aria-label="mt('claudePlan.refresh')"
        @click="refresh"
      >
        <svg
          class="h-3 w-3"
          :class="{ 'animate-spin': entry?.loading }"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
          aria-hidden="true"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
          />
        </svg>
      </button>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, watch } from 'vue'
import { useMokI18n } from '../i18n'
import { getClaudePlanEntry, requestClaudePlan, resolveClaudePlan } from './claudePlan'

const props = defineProps<{
  account: { id: number; platform: string; type: string }
}>()

const { mt } = useMokI18n()

// Only Claude OAuth accounts can read /api/oauth/profile; setup-token accounts lack the scope.
const applicable = computed(() => props.account.platform === 'anthropic' && props.account.type === 'oauth')
const entry = computed(() => getClaudePlanEntry(props.account.id))
const item = computed(() => entry.value?.item ?? null)

watch(
  () => [props.account.id, applicable.value] as const,
  ([id, ok]) => {
    if (ok) {
      requestClaudePlan(id)
    }
  },
  { immediate: true }
)

const loadingFirst = computed(() => !!entry.value?.loading && !item.value)

const badge = computed(() => {
  const plan = item.value
  if (!plan || (!plan.organization_type && !plan.rate_limit_tier)) {
    return null
  }
  return resolveClaudePlan(plan.organization_type, plan.rate_limit_tier)
})

const subscriptionNote = computed(() => {
  const status = item.value?.subscription_status?.trim() ?? ''
  if (!badge.value || !status || ['active', 'trialing'].includes(status.toLowerCase())) {
    return ''
  }
  return mt('claudePlan.subscriptionStatus', { status })
})

const errorText = computed(() => {
  if (entry.value?.loading || !(entry.value?.failed || item.value?.status === 'error')) {
    return ''
  }
  return badge.value ? '!' : mt('claudePlan.failed')
})

const unsupportedTitle = computed(() => {
  if (item.value?.status !== 'unsupported') {
    return ''
  }
  return item.value.reason === 'missing_profile_scope'
    ? mt('claudePlan.unsupportedScope')
    : mt('claudePlan.unsupportedOther')
})

function formatTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return new Intl.DateTimeFormat(undefined, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  }).format(date)
}

const detailTitle = computed(() => {
  const plan = item.value
  if (!plan) {
    return ''
  }
  const lines = [
    `organization_type: ${plan.organization_type || '-'}`,
    `rate_limit_tier: ${plan.rate_limit_tier || '-'}`,
    `seat_tier: ${plan.seat_tier || '-'}`,
    `subscription_status: ${plan.subscription_status || '-'}`
  ]
  if (plan.fetched_at) {
    lines.push(mt('claudePlan.updatedAt', { time: formatTime(plan.fetched_at) }))
  }
  return lines.join('\n')
})

const errorTitle = computed(() => {
  const lines: string[] = []
  if (entry.value?.failed) {
    lines.push(mt('claudePlan.requestFailed'))
  }
  const plan = item.value
  if (plan?.status === 'error') {
    lines.push(mt('claudePlan.lastError', { message: plan.message || plan.reason || '-' }))
    if (plan.attempted_at) {
      lines.push(mt('claudePlan.attemptedAt', { time: formatTime(plan.attempted_at) }))
    }
  }
  if (badge.value) {
    lines.push('', detailTitle.value)
  }
  return lines.join('\n')
})

function refresh(): void {
  requestClaudePlan(props.account.id, true)
}
</script>
