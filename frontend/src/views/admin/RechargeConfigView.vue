<template>
  <AppLayout>
    <div class="mx-auto max-w-3xl space-y-6">
      <div v-if="loading" class="flex items-center justify-center py-12">
        <div class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary-600"></div>
      </div>
      <template v-else>
        <!-- Card 1: Price per USD -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.recharge.priceTitle') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.recharge.priceDescription') }}</p>
          </div>
          <div class="space-y-4 p-6">
            <div class="grid grid-cols-2 gap-4">
              <div>
                <label class="input-label">{{ t('admin.recharge.cnyPerUsd') }}</label>
                <input
                  :value="form.cny_per_usd || ''"
                  @input="form.cny_per_usd = parseFloat(($event.target as HTMLInputElement).value) || 0"
                  type="number" step="0.01" min="0" class="input" placeholder="0.5"
                />
                <p class="mt-1 text-xs text-gray-400">{{ t('admin.recharge.cnyPerUsdHint') }}</p>
              </div>
              <div v-if="form.cny_per_usd > 0" class="flex items-center">
                <div class="rounded-lg border border-gray-200 bg-gray-50 px-4 py-3 dark:border-dark-600 dark:bg-dark-800">
                  <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.recharge.preview') }}</p>
                  <p class="mt-1 text-base font-semibold text-gray-900 dark:text-white">
                    &yen;100 &rarr; ${{ (100 / form.cny_per_usd).toFixed(2) }}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Card 2: Bonus Tiers -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.recharge.bonusTitle') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.recharge.bonusDescription') }}</p>
          </div>
          <div class="space-y-4 p-6">
            <div v-if="form.bonus_tiers.length === 0" class="py-6 text-center text-sm text-gray-400 dark:text-gray-500">
              {{ t('admin.recharge.noTiers') }}
            </div>
            <div v-else class="space-y-3">
              <div v-for="(tier, idx) in form.bonus_tiers" :key="idx" class="flex items-center gap-3">
                <div class="flex-1">
                  <label v-if="idx === 0" class="input-label">{{ t('admin.recharge.tierMinAmount') }}</label>
                  <div class="relative">
                    <span class="absolute left-3 top-1/2 -translate-y-1/2 text-xs text-gray-400">&yen;</span>
                    <input
                      v-model.number="tier.min_amount"
                      type="number" step="1" min="1" class="input pl-7"
                      :placeholder="t('admin.recharge.tierMinPlaceholder')"
                    />
                  </div>
                </div>
                <div class="flex-1">
                  <label v-if="idx === 0" class="input-label">{{ t('admin.recharge.tierBonusUsd') }}</label>
                  <div class="relative">
                    <span class="absolute left-3 top-1/2 -translate-y-1/2 text-xs text-gray-400">$</span>
                    <input
                      v-model.number="tier.bonus_usd"
                      type="number" step="0.01" min="0" class="input pl-7"
                      :placeholder="t('admin.recharge.tierBonusPlaceholder')"
                    />
                  </div>
                </div>
                <div :class="idx === 0 ? 'mt-6' : ''">
                  <button type="button" class="rounded-lg p-2 text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20" @click="removeTier(idx)">
                    <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" /></svg>
                  </button>
                </div>
              </div>
            </div>
            <button type="button" class="btn-secondary text-sm" @click="addTier">
              + {{ t('admin.recharge.addTier') }}
            </button>

            <!-- Preview -->
            <div v-if="form.bonus_tiers.length > 0 && form.cny_per_usd > 0" class="mt-4 rounded-lg border border-gray-200 bg-gray-50 p-4 dark:border-dark-600 dark:bg-dark-800">
              <p class="mb-2 text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.recharge.previewExample') }}</p>
              <div class="space-y-1 text-sm">
                <div v-for="tier in sortedTiers" :key="tier.min_amount" class="flex justify-between">
                  <span class="text-gray-600 dark:text-gray-300">&yen;{{ tier.min_amount }}</span>
                  <span class="text-green-600 dark:text-green-400">
                    ${{ (tier.min_amount / form.cny_per_usd).toFixed(2) }} + ${{ tier.bonus_usd.toFixed(2) }} = ${{ (tier.min_amount / form.cny_per_usd + tier.bonus_usd).toFixed(2) }}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Save Button -->
        <button class="btn-primary w-full py-3 text-base font-medium" :disabled="saving" @click="save">
          <span v-if="saving" class="flex items-center justify-center gap-2">
            <span class="h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent"></span>
            {{ t('common.saving') }}
          </span>
          <span v-else>{{ t('common.save') }}</span>
        </button>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api'
import { useAppStore } from '@/stores/app'
import AppLayout from '@/components/layout/AppLayout.vue'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(true)
const saving = ref(false)

const form = ref({
  cny_per_usd: 0,
  bonus_tiers: [] as { min_amount: number; bonus_usd: number }[],
})

const sortedTiers = computed(() =>
  [...form.value.bonus_tiers].sort((a, b) => a.min_amount - b.min_amount)
)

function addTier() {
  form.value.bonus_tiers.push({ min_amount: 0, bonus_usd: 0 })
}

function removeTier(idx: number) {
  form.value.bonus_tiers.splice(idx, 1)
}

async function load() {
  loading.value = true
  try {
    const settings = await adminAPI.settings.getSettings()
    form.value.cny_per_usd = settings.payment_cny_per_usd || 0
    form.value.bonus_tiers = Array.isArray(settings.payment_bonus_tiers)
      ? settings.payment_bonus_tiers.map(t => ({ min_amount: t.min_amount, bonus_usd: t.bonus_usd }))
      : []
  } catch {
    appStore.showError(t('admin.recharge.title') + ': ' + t('common.error'))
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  try {
    // Sort tiers before saving
    const tiers = [...form.value.bonus_tiers]
      .filter(t => t.min_amount > 0)
      .sort((a, b) => a.min_amount - b.min_amount)
    await adminAPI.settings.updateSettings({
      payment_cny_per_usd: Number(form.value.cny_per_usd) || 0,
      payment_bonus_tiers: tiers,
    })
    form.value.bonus_tiers = tiers
    appStore.showSuccess(t('admin.recharge.saved'))
  } catch {
    appStore.showError(t('common.error'))
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>
