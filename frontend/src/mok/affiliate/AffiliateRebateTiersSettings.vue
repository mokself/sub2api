<template>
  <!-- Part of ./AffiliateSettings.vue; saves on its own. -->
  <div class="border-t border-gray-100 pt-6 dark:border-dark-700" data-testid="mok-affiliate-tiers">
    <div class="mb-3 flex items-start justify-between gap-3">
      <div class="min-w-0">
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
          {{ mt('affiliateTiers.title') }}
        </h3>
        <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
          {{ mt('affiliateTiers.description') }}
        </p>
      </div>
      <button
        type="button"
        class="btn btn-secondary btn-sm shrink-0"
        :disabled="loading || loadFailed || drafts.length >= MAX_AFFILIATE_REBATE_TIERS"
        :title="drafts.length >= MAX_AFFILIATE_REBATE_TIERS ? mt('affiliateTiers.maxTiers', { max: MAX_AFFILIATE_REBATE_TIERS }) : undefined"
        data-testid="mok-affiliate-tiers-add"
        @click="addTier"
      >
        <Icon name="plus" size="sm" class="mr-1" />
        {{ mt('affiliateTiers.addTier') }}
      </button>
    </div>

    <div v-if="loading" class="flex justify-center py-4">
      <div class="h-5 w-5 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
    </div>

    <div
      v-else-if="loadFailed"
      class="flex items-center justify-between gap-3 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-300"
      role="alert"
    >
      <span>{{ mt('affiliateTiers.loadFailed') }}</span>
      <button type="button" class="btn btn-secondary btn-sm" @click="load">{{ mt('affiliateTiers.retry') }}</button>
    </div>

    <template v-else>
      <div
        v-if="drafts.length === 0"
        class="rounded-xl border border-dashed border-gray-300 px-4 py-3 text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400"
        data-testid="mok-affiliate-tiers-empty"
      >
        {{ mt('affiliateTiers.empty') }}
      </div>
      <div v-else class="overflow-hidden rounded-xl border border-gray-200 dark:border-dark-600">
        <div
          class="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_2rem] items-center gap-3 bg-gray-50 px-3 py-2 text-xs font-medium text-gray-500 dark:bg-dark-800 dark:text-gray-400"
        >
          <span>{{ mt('affiliateTiers.minAmountLabel') }}</span>
          <span>{{ mt('affiliateTiers.rateLabel') }}</span>
          <span></span>
        </div>
        <div class="divide-y divide-gray-100 dark:divide-dark-700">
          <div
            v-for="(draft, index) in drafts"
            :key="index"
            class="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_2rem] items-center gap-3 px-3 py-2"
            data-testid="mok-affiliate-tiers-row"
          >
            <div class="relative">
              <span class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-xs text-gray-400">$</span>
              <input
                v-model="draft.min_amount"
                type="number"
                step="0.01"
                min="0"
                class="input py-2 pl-7"
                placeholder="50000"
                data-testid="mok-affiliate-tiers-min"
                @keydown.enter.prevent
              />
            </div>
            <div class="relative">
              <input
                v-model="draft.rate_percent"
                type="number"
                step="0.01"
                min="0"
                max="100"
                class="input py-2 pr-8"
                placeholder="15"
                data-testid="mok-affiliate-tiers-rate"
                @keydown.enter.prevent
              />
              <span class="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3 text-xs text-gray-400">%</span>
            </div>
            <button
              type="button"
              class="flex h-8 w-8 items-center justify-center rounded-lg text-gray-400 hover:bg-red-50 hover:text-red-600 dark:hover:bg-dark-700 dark:hover:text-red-400"
              :title="mt('affiliateTiers.removeTier')"
              :aria-label="mt('affiliateTiers.removeTier')"
              data-testid="mok-affiliate-tiers-remove"
              @click="removeTier(index)"
            >
              <Icon name="x" size="sm" />
            </button>
            <p
              v-if="validation.errors[index]"
              class="col-span-3 -mt-1 text-xs text-red-600 dark:text-red-400"
              data-testid="mok-affiliate-tiers-error"
            >
              {{ mt(`affiliateTiers.errors.${validation.errors[index]}`) }}
            </p>
          </div>
        </div>
      </div>

      <p class="mt-2 text-xs text-gray-400 dark:text-gray-500">
        {{ mt('affiliateTiers.volumeHint') }}
      </p>

      <div class="mt-3 flex items-center justify-end gap-3">
        <span v-if="dirty" class="text-xs text-amber-600 dark:text-amber-400" data-testid="mok-affiliate-tiers-dirty">
          {{ mt('affiliateTiers.unsaved') }}
        </span>
        <button
          type="button"
          class="btn btn-primary btn-sm"
          :disabled="!canSave"
          data-testid="mok-affiliate-tiers-save"
          @click="save"
        >
          {{ saving ? mt('affiliateTiers.saving') : mt('affiliateTiers.save') }}
        </button>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { getAffiliateRebateTiers, saveAffiliateRebateTiers, type AffiliateRebateTier } from '../api/affiliateTiers'
import { useMokI18n } from '../i18n'
import {
  MAX_AFFILIATE_REBATE_TIERS,
  sameTiers,
  toDrafts,
  validateDrafts,
  type AffiliateRebateTierDraft
} from './tiers'

const { mt } = useMokI18n()
const appStore = useAppStore()

const loading = ref(true)
const loadFailed = ref(false)
const saving = ref(false)
const saved = ref<AffiliateRebateTier[]>([])
const drafts = ref<AffiliateRebateTierDraft[]>([])

const validation = computed(() => validateDrafts(drafts.value))
const dirty = computed(() => !sameTiers(saved.value, drafts.value))
const canSave = computed(() => !saving.value && dirty.value && validation.value.tiers !== null)

async function load(): Promise<void> {
  loading.value = true
  loadFailed.value = false
  try {
    saved.value = await getAffiliateRebateTiers()
    drafts.value = toDrafts(saved.value)
  } catch (error) {
    console.error('Failed to load affiliate rebate tiers:', error)
    loadFailed.value = true
  } finally {
    loading.value = false
  }
}

function addTier(): void {
  if (drafts.value.length < MAX_AFFILIATE_REBATE_TIERS) {
    drafts.value.push({ min_amount: '', rate_percent: '' })
  }
}

function removeTier(index: number): void {
  drafts.value.splice(index, 1)
}

async function save(): Promise<void> {
  const tiers = validation.value.tiers
  if (!tiers || saving.value) return
  saving.value = true
  try {
    saved.value = await saveAffiliateRebateTiers(tiers)
    drafts.value = toDrafts(saved.value)
    appStore.showSuccess(mt('affiliateTiers.saved'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, mt('affiliateTiers.saveFailed')))
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>
