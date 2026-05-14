<template>
  <AppLayout>
    <div class="space-y-6">
      <div v-if="loading" class="flex justify-center py-12">
        <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
      </div>

      <template v-else>
        <div class="grid gap-4 md:grid-cols-4">
          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('distribution.stats.status') }}</p>
            <p class="mt-2 text-lg font-semibold text-gray-900 dark:text-white">
              {{ statusLabel }}
            </p>
          </div>
          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('distribution.stats.balance') }}</p>
            <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">
              {{ formatCurrency(summary?.wallet?.balance ?? 0, 'CNY') }}
            </p>
          </div>
          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('distribution.stats.recharged') }}</p>
            <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ formatCurrency(summary?.wallet?.total_recharged ?? 0, 'CNY') }}
            </p>
          </div>
          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('distribution.stats.rebate') }}</p>
            <p class="mt-2 text-2xl font-semibold text-primary-600 dark:text-primary-400">
              {{ formatCurrency(summary?.wallet?.total_rebate ?? 0, 'CNY') }}
            </p>
          </div>
        </div>

        <div v-if="!summary?.application" class="card p-6">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('distribution.apply.title') }}</h3>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('distribution.apply.description') }}</p>
          <form class="mt-5 space-y-4" @submit.prevent="submitApplication">
            <div>
              <label class="input-label">{{ t('distribution.apply.contact') }}</label>
              <input v-model.trim="applyForm.contact" class="input" maxlength="200" />
            </div>
            <div>
              <label class="input-label">{{ t('distribution.apply.reason') }}</label>
              <textarea v-model.trim="applyForm.reason" class="input min-h-28" maxlength="1000"></textarea>
            </div>
            <button class="btn btn-primary" :disabled="submitting">
              {{ submitting ? t('common.submitting') : t('distribution.apply.submit') }}
            </button>
          </form>
        </div>

        <div v-else class="card p-6">
          <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('distribution.application.title') }}</h3>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                {{ t('distribution.application.submittedAt') }} {{ formatDateTime(summary.application.created_at) }}
              </p>
              <p v-if="summary.application.admin_note" class="mt-3 text-sm text-gray-700 dark:text-gray-300">
                {{ t('distribution.application.adminNote') }}: {{ summary.application.admin_note }}
              </p>
            </div>
            <span class="badge" :class="statusBadgeClass">{{ statusLabel }}</span>
          </div>
        </div>

        <div class="grid gap-4 md:grid-cols-3">
          <div class="card p-5 opacity-70">
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('distribution.features.recharge.title') }}</h3>
            <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">{{ t('distribution.features.recharge.description') }}</p>
          </div>
          <div class="card p-5 opacity-70">
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('distribution.features.codes.title') }}</h3>
            <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">{{ t('distribution.features.codes.description') }}</p>
          </div>
          <div class="card p-5 opacity-70">
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('distribution.features.api.title') }}</h3>
            <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">{{ t('distribution.features.api.description') }}</p>
          </div>
        </div>

        <div class="card p-6">
          <div class="mb-4 flex items-center justify-between">
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('distribution.ledger.title') }}</h3>
            <button class="btn btn-secondary btn-sm" :disabled="ledgerLoading" @click="loadLedger">
              {{ t('common.refresh') }}
            </button>
          </div>
          <div v-if="ledger.items.length === 0" class="rounded-lg border border-dashed border-gray-300 p-6 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-dark-400">
            {{ t('distribution.ledger.empty') }}
          </div>
          <div v-else class="overflow-x-auto">
            <table class="w-full min-w-[720px] text-left text-sm">
              <thead>
                <tr class="border-b border-gray-200 text-gray-500 dark:border-dark-700 dark:text-dark-400">
                  <th class="px-3 py-2 font-medium">{{ t('distribution.ledger.columns.action') }}</th>
                  <th class="px-3 py-2 font-medium text-right">{{ t('distribution.ledger.columns.amount') }}</th>
                  <th class="px-3 py-2 font-medium text-right">{{ t('distribution.ledger.columns.balanceAfter') }}</th>
                  <th class="px-3 py-2 font-medium">{{ t('distribution.ledger.columns.note') }}</th>
                  <th class="px-3 py-2 font-medium">{{ t('distribution.ledger.columns.createdAt') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item in ledger.items" :key="item.id" class="border-b border-gray-100 last:border-b-0 dark:border-dark-800">
                  <td class="px-3 py-3 text-gray-900 dark:text-white">{{ item.action }}</td>
                  <td class="px-3 py-3 text-right font-medium">{{ formatCurrency(item.amount, 'CNY') }}</td>
                  <td class="px-3 py-3 text-right">{{ formatCurrency(item.balance_after, 'CNY') }}</td>
                  <td class="px-3 py-3 text-gray-600 dark:text-gray-300">{{ item.note || '-' }}</td>
                  <td class="px-3 py-3 text-gray-600 dark:text-gray-300">{{ formatDateTime(item.created_at) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import distributionAPI from '@/api/distribution'
import type { DistributionSummary, DistributionWalletLedgerEntry } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatCurrency, formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(true)
const submitting = ref(false)
const ledgerLoading = ref(false)
const summary = ref<DistributionSummary | null>(null)
const ledger = reactive({ items: [] as DistributionWalletLedgerEntry[] })
const applyForm = reactive({ contact: '', reason: '' })

const statusLabel = computed(() => {
  const status = summary.value?.application?.status
  return status ? t(`distribution.status.${status}`) : t('distribution.status.notApplied')
})

const statusBadgeClass = computed(() => {
  switch (summary.value?.application?.status) {
    case 'approved':
      return 'badge-success'
    case 'rejected':
      return 'badge-danger'
    case 'frozen':
      return 'badge-warning'
    default:
      return 'badge-gray'
  }
})

async function loadSummary(): Promise<void> {
  summary.value = await distributionAPI.getSummary()
}

async function loadLedger(): Promise<void> {
  ledgerLoading.value = true
  try {
    const resp = await distributionAPI.listLedger(1, 20)
    ledger.items = resp.items ?? []
  } catch {
    ledger.items = []
  } finally {
    ledgerLoading.value = false
  }
}

async function submitApplication(): Promise<void> {
  if (submitting.value) return
  submitting.value = true
  try {
    summary.value = await distributionAPI.apply({ ...applyForm })
    appStore.showSuccess(t('distribution.apply.success'))
    await loadLedger()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('distribution.apply.failed')))
  } finally {
    submitting.value = false
  }
}

onMounted(async () => {
  try {
    await loadSummary()
    await loadLedger()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('distribution.loadFailed')))
  } finally {
    loading.value = false
  }
})
</script>
