<template>
  <!--
    Sits on the user's affiliate page (views/user/AffiliateView.vue). Rewords the upstream
    sentences there in usage mode and shows two cards: the tier card, hidden without tiers
    or for exclusive rates, and the usage rebate card, shown in usage mode and to inviters
    who have usage rebate records.
  -->
  <div v-if="status && status.enabled && !status.exclusive" class="card p-6" data-testid="mok-affiliate-tier-progress">
    <div class="flex flex-wrap items-start justify-between gap-4">
      <div class="min-w-0">
        <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ mt('affiliateTiers.user.title') }}</h3>
        <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ mt('affiliateTiers.user.teamVolumeHint') }}</p>
        <p
          v-if="status.rebate_mode === 'usage'"
          class="mt-1 text-sm text-primary-700 dark:text-primary-300"
          data-testid="mok-affiliate-tier-usage-mode"
        >
          {{ mt('affiliateTiers.user.usageMode') }}
        </p>
      </div>
      <div class="text-right">
        <p class="text-xs text-gray-500 dark:text-dark-400">{{ mt('affiliateTiers.user.teamVolume') }}</p>
        <p class="mt-1 text-xl font-semibold text-gray-900 dark:text-white" data-testid="mok-affiliate-tier-volume">
          {{ formatCurrency(status.team_volume) }}
        </p>
      </div>
    </div>

    <div class="mt-4">
      <div class="flex flex-wrap items-center justify-between gap-2 text-sm">
        <span class="text-gray-600 dark:text-gray-300" data-testid="mok-affiliate-tier-next">
          {{
            status.next_tier
              ? mt('affiliateTiers.user.nextTier', {
                  amount: formatCurrency(status.next_tier_remaining),
                  rate: formatRate(status.next_tier.rate_percent)
                })
              : mt('affiliateTiers.user.topTier')
          }}
        </span>
        <span class="font-medium text-primary-600 dark:text-primary-400">
          {{ mt('affiliateTiers.user.currentRate') }} {{ formatRate(status.current_rate_percent) }}%
        </span>
      </div>
      <div class="mt-2 h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
        <div class="h-full rounded-full bg-primary-500 transition-all" :style="{ width: `${progress}%` }"></div>
      </div>
    </div>

    <div class="mt-5">
      <p class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ mt('affiliateTiers.user.ladder') }}</p>
      <ul class="mt-2 divide-y divide-gray-100 overflow-hidden rounded-xl border border-gray-200 dark:divide-dark-700 dark:border-dark-700">
        <li
          v-for="row in ladder"
          :key="row.key"
          :class="[
            'flex items-center justify-between gap-3 px-4 py-2.5 text-sm',
            row.current ? 'bg-primary-50 dark:bg-primary-900/20' : ''
          ]"
          data-testid="mok-affiliate-tier-row"
        >
          <span class="text-gray-600 dark:text-gray-300">{{ row.label }}</span>
          <span class="flex items-center gap-2">
            <span
              v-if="row.current"
              class="rounded-full bg-primary-100 px-2 py-0.5 text-xs font-medium text-primary-700 dark:bg-primary-900/40 dark:text-primary-300"
            >
              {{ mt('affiliateTiers.user.current') }}
            </span>
            <span class="font-semibold text-gray-900 dark:text-white">{{ formatRate(row.rate) }}%</span>
          </span>
        </li>
      </ul>
    </div>
  </div>
  <div v-if="showUsageCard" class="card p-6" data-testid="mok-affiliate-usage-card">
    <div class="flex flex-wrap items-center justify-between gap-4">
      <div class="min-w-0">
        <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ mt('affiliateUsage.user.title') }}</h3>
        <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
          {{ status?.rebate_mode === 'usage' ? mt('affiliateUsage.user.usageOn') : mt('affiliateUsage.user.usageOff') }}
        </p>
        <p class="mt-1 text-sm text-gray-600 dark:text-gray-300" data-testid="mok-affiliate-usage-latest">
          {{
            latestRecord
              ? mt('affiliateUsage.user.latest', {
                  period: formatRebatePeriod(latestRecord.period_start, latestRecord.period_end),
                  amount: formatCurrency(latestRecord.rebate_amount)
                })
              : mt('affiliateUsage.user.none')
          }}
        </p>
      </div>
      <button
        type="button"
        class="btn btn-secondary"
        :disabled="recordTotal === 0"
        data-testid="mok-affiliate-usage-open"
        @click="recordsOpen = true"
      >
        {{ mt('affiliateUsage.user.view', { count: recordTotal }) }}
      </button>
    </div>
  </div>
  <AffiliateUsageRebateRecords :show="recordsOpen" scope="user" @close="recordsOpen = false" />
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { formatCurrency } from '@/utils/format'
import { getAffiliateTierStatus, type AffiliateTierStatus } from '../api/affiliateTiers'
import { listMyAffiliateUsageRebates, type AffiliateUsageRebateRecord } from '../api/affiliateUsageRebates'
import { setAffiliateUsageWording, useMokI18n } from '../i18n'
import AffiliateUsageRebateRecords from './AffiliateUsageRebateRecords.vue'
import { progressToNextTier } from './tiers'
import { formatRebatePeriod, formatRebateRate as formatRate } from './usageRebate'

const { mt } = useMokI18n()
const status = ref<AffiliateTierStatus | null>(null)
const latestRecord = ref<AffiliateUsageRebateRecord | null>(null)
const recordTotal = ref(0)
const recordsOpen = ref(false)

// After a switch back to recharge mode the card stays for inviters who have records.
const showUsageCard = computed(() => status.value?.rebate_mode === 'usage' || recordTotal.value > 0)

const progress = computed(() => {
  const s = status.value
  return s ? progressToNextTier(s.team_volume, s.tiers, s.next_tier) : 0
})

const ladder = computed(() => {
  const s = status.value
  if (!s || s.tiers.length === 0) return []
  const reached = s.tiers.filter((tier) => tier.min_amount <= s.team_volume).length
  return [
    {
      key: 'base',
      label: mt('affiliateTiers.user.baseRow', { amount: formatCurrency(s.tiers[0].min_amount) }),
      rate: s.base_rate_percent,
      current: reached === 0
    },
    ...s.tiers.map((tier, index) => ({
      key: String(tier.min_amount),
      label: mt('affiliateTiers.user.tierRow', { amount: formatCurrency(tier.min_amount) }),
      rate: tier.rate_percent,
      current: index === reached - 1
    }))
  ]
})

async function loadStatus(): Promise<void> {
  try {
    status.value = await getAffiliateTierStatus()
    setAffiliateUsageWording(status.value.rebate_mode === 'usage')
  } catch (error) {
    console.error('Failed to load affiliate tier status:', error)
  }
}

async function loadLatestRecord(): Promise<void> {
  try {
    const result = await listMyAffiliateUsageRebates({ page: 1, page_size: 1 })
    latestRecord.value = result.items?.[0] ?? null
    recordTotal.value = result.total ?? 0
  } catch (error) {
    console.error('Failed to load affiliate usage rebates:', error)
  }
}

onMounted(() => {
  void loadStatus()
  void loadLatestRecord()
})
</script>
