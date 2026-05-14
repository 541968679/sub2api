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
            <p class="mt-2 text-lg font-semibold text-gray-900 dark:text-white">{{ statusLabel }}</p>
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
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('distribution.stats.spent') }}</p>
            <p class="mt-2 text-2xl font-semibold text-primary-600 dark:text-primary-400">
              {{ formatCurrency(summary?.wallet?.total_spent ?? 0, 'CNY') }}
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

        <div v-if="isApproved" class="grid gap-4 lg:grid-cols-3">
          <form class="card p-5" @submit.prevent="generateBalanceCode">
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('distribution.generate.balanceTitle') }}</h3>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ balanceCostHint }}</p>
            <div class="mt-4 space-y-3">
              <div>
                <label class="input-label">{{ t('distribution.generate.valueUsd') }}</label>
                <input v-model.number="balanceForm.value_usd" type="number" min="0" step="0.01" class="input" />
              </div>
              <div>
                <label class="input-label">{{ t('distribution.generate.note') }}</label>
                <input v-model.trim="balanceForm.note" class="input" maxlength="200" />
              </div>
              <button class="btn btn-primary w-full" :disabled="generating.balance">
                {{ generating.balance ? t('common.processing') : t('distribution.generate.createCode') }}
              </button>
            </div>
          </form>

          <form class="card p-5" @submit.prevent="generateSubscriptionCode">
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('distribution.generate.subscriptionTitle') }}</h3>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ subscriptionCostHint }}</p>
            <div class="mt-4 space-y-3">
              <div>
                <label class="input-label">{{ t('distribution.generate.faceValueRmb') }}</label>
                <input v-model.number="subscriptionForm.face_value_rmb" type="number" min="0" step="0.01" class="input" />
              </div>
              <div>
                <label class="input-label">{{ t('distribution.generate.group') }}</label>
                <select v-model.number="subscriptionForm.group_id" class="input">
                  <option :value="0">{{ t('distribution.generate.selectGroup') }}</option>
                  <option v-for="group in subscriptionGroups" :key="group.id" :value="group.id">{{ group.name }}</option>
                </select>
              </div>
              <div>
                <label class="input-label">{{ t('distribution.generate.validityDays') }}</label>
                <input v-model.number="subscriptionForm.validity_days" type="number" min="1" step="1" class="input" />
              </div>
              <button class="btn btn-primary w-full" :disabled="generating.subscription">
                {{ generating.subscription ? t('common.processing') : t('distribution.generate.createCode') }}
              </button>
            </div>
          </form>

          <form class="card p-5" @submit.prevent="generateApiKey">
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('distribution.generate.apiTitle') }}</h3>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ apiCostHint }}</p>
            <div class="mt-4 space-y-3">
              <div>
                <label class="input-label">{{ t('common.name') }}</label>
                <input v-model.trim="apiForm.name" class="input" maxlength="100" />
              </div>
              <div>
                <label class="input-label">{{ t('distribution.generate.quotaUsd') }}</label>
                <input v-model.number="apiForm.quota_usd" type="number" min="0" step="0.01" class="input" />
              </div>
              <div>
                <label class="input-label">{{ t('distribution.generate.group') }}</label>
                <select v-model.number="apiForm.group_id" class="input">
                  <option :value="0">{{ t('keys.noGroup') }}</option>
                  <option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }}</option>
                </select>
              </div>
              <div>
                <label class="input-label">{{ t('distribution.generate.expiresInDays') }}</label>
                <input v-model.number="apiForm.expires_in_days" type="number" min="0" step="1" class="input" />
              </div>
              <button class="btn btn-primary w-full" :disabled="generating.api">
                {{ generating.api ? t('common.processing') : t('distribution.generate.createApiKey') }}
              </button>
            </div>
          </form>
        </div>

        <div v-if="generatedItems.length > 0" class="card p-6">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('distribution.generated.title') }}</h3>
          <div class="mt-4 space-y-3">
            <div v-for="item in generatedItems" :key="item.value" class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
              <div class="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                <div class="min-w-0">
                  <p class="text-xs uppercase text-gray-500 dark:text-dark-400">{{ item.label }}</p>
                  <p class="mt-1 break-all font-mono text-sm font-medium text-gray-900 dark:text-white">{{ item.value }}</p>
                  <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ item.meta }}</p>
                </div>
                <button class="btn btn-secondary btn-sm shrink-0" @click="copy(item.value)">{{ t('common.copy') }}</button>
              </div>
            </div>
          </div>
        </div>

        <div class="card p-6">
          <div class="mb-4 flex items-center justify-between">
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('distribution.ledger.title') }}</h3>
            <button class="btn btn-secondary btn-sm" :disabled="ledgerLoading" @click="loadLedger">{{ t('common.refresh') }}</button>
          </div>
          <div v-if="ledger.items.length === 0" class="rounded-lg border border-dashed border-gray-300 p-6 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-dark-400">
            {{ t('distribution.ledger.empty') }}
          </div>
          <div v-else class="overflow-x-auto">
            <table class="w-full min-w-[760px] text-left text-sm">
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
                  <td class="px-3 py-3 text-gray-900 dark:text-white">{{ actionLabel(item.action) }}</td>
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
import userGroupsAPI from '@/api/groups'
import type { DistributionSummary, DistributionWalletLedgerEntry, Group } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatCurrency, formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(true)
const submitting = ref(false)
const ledgerLoading = ref(false)
const summary = ref<DistributionSummary | null>(null)
const groups = ref<Group[]>([])
const generatedItems = ref<Array<{ label: string; value: string; meta: string }>>([])
const ledger = reactive({ items: [] as DistributionWalletLedgerEntry[] })
const applyForm = reactive({ contact: '', reason: '' })
const balanceForm = reactive({ value_usd: 10, note: '' })
const subscriptionForm = reactive({ face_value_rmb: 30, group_id: 0, validity_days: 30 })
const apiForm = reactive({ name: 'Distribution API Key', quota_usd: 10, group_id: 0, expires_in_days: 0 })
const generating = reactive({ balance: false, subscription: false, api: false })

const isApproved = computed(() => summary.value?.application?.status === 'approved' && summary.value?.wallet?.status === 'active')
const settings = computed(() => summary.value?.settings ?? { rmb_per_usd: 0.5, subscription_discount: 0.75 })
const subscriptionGroups = computed(() => groups.value.filter((group) => group.subscription_type === 'subscription'))

const statusLabel = computed(() => {
  const status = summary.value?.application?.status
  if (summary.value?.wallet?.status === 'frozen') return t('distribution.status.frozen')
  return status ? t(`distribution.status.${status}`) : t('distribution.status.notApplied')
})

const statusBadgeClass = computed(() => {
  if (summary.value?.wallet?.status === 'frozen') return 'badge-warning'
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

const balanceCostHint = computed(() => t('distribution.generate.balanceHint', {
  cost: formatCurrency((balanceForm.value_usd || 0) * settings.value.rmb_per_usd, 'CNY'),
}))
const subscriptionCostHint = computed(() => t('distribution.generate.subscriptionHint', {
  cost: formatCurrency((subscriptionForm.face_value_rmb || 0) * settings.value.subscription_discount, 'CNY'),
}))
const apiCostHint = computed(() => t('distribution.generate.apiHint', {
  cost: formatCurrency((apiForm.quota_usd || 0) * settings.value.rmb_per_usd, 'CNY'),
}))

async function loadSummary(): Promise<void> {
  summary.value = await distributionAPI.getSummary()
}

async function loadGroups(): Promise<void> {
  groups.value = await userGroupsAPI.getAvailable()
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

async function generateBalanceCode(): Promise<void> {
  generating.balance = true
  try {
    const result = await distributionAPI.generateBalanceRedeemCode({ ...balanceForm })
    addGenerated(t('distribution.generated.balanceCode'), result.code, `${formatCurrency(result.cost_rmb, 'CNY')} / $${result.value.toFixed(2)}`)
    await refreshAfterGeneration(result.balance_after)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('distribution.generate.failed')))
  } finally {
    generating.balance = false
  }
}

async function generateSubscriptionCode(): Promise<void> {
  generating.subscription = true
  try {
    const result = await distributionAPI.generateSubscriptionRedeemCode({ ...subscriptionForm })
    addGenerated(t('distribution.generated.subscriptionCode'), result.code, `${formatCurrency(result.cost_rmb, 'CNY')} / ${result.validity_days}d`)
    await refreshAfterGeneration(result.balance_after)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('distribution.generate.failed')))
  } finally {
    generating.subscription = false
  }
}

async function generateApiKey(): Promise<void> {
  generating.api = true
  try {
    const payload = {
      name: apiForm.name,
      quota_usd: apiForm.quota_usd,
      group_id: apiForm.group_id > 0 ? apiForm.group_id : null,
      expires_in_days: apiForm.expires_in_days > 0 ? apiForm.expires_in_days : null,
    }
    const result = await distributionAPI.generateApiKey(payload)
    addGenerated(t('distribution.generated.apiKey'), result.key, `${formatCurrency(result.cost_rmb, 'CNY')} / $${result.quota.toFixed(2)}`)
    await refreshAfterGeneration(result.balance_after)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('distribution.generate.failed')))
  } finally {
    generating.api = false
  }
}

async function refreshAfterGeneration(balanceAfter: number): Promise<void> {
  if (summary.value?.wallet) summary.value.wallet.balance = balanceAfter
  appStore.showSuccess(t('distribution.generate.success'))
  await loadLedger()
}

function addGenerated(label: string, value: string, meta: string): void {
  generatedItems.value.unshift({ label, value, meta })
}

async function copy(value: string): Promise<void> {
  await navigator.clipboard.writeText(value)
  appStore.showSuccess(t('common.copiedToClipboard'))
}

function actionLabel(action: string): string {
  return t(`distribution.ledger.actions.${action}`, action)
}

onMounted(async () => {
  try {
    await Promise.all([loadSummary(), loadGroups()])
    await loadLedger()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('distribution.loadFailed')))
  } finally {
    loading.value = false
  }
})
</script>
