<template>
  <AppLayout>
    <div class="space-y-6">
    <!-- Header -->
    <div>
      <h1 class="text-xl font-semibold text-gray-900 dark:text-gray-100">
        {{ t('costAnalysis.subscriptionProfit.title') }}
      </h1>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{ t('costAnalysis.subscriptionProfit.description') }}
      </p>
      <p class="mt-1 text-xs text-gray-400 dark:text-gray-500">
        {{ t('costAnalysis.subscriptionProfit.onlyPaidNote') }}
      </p>
    </div>

    <!-- Controls -->
    <div
      class="flex flex-wrap items-end gap-4 rounded-lg border border-gray-200 bg-white p-4 dark:border-gray-700 dark:bg-gray-800"
    >
      <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
        <input v-model="activeOnly" type="checkbox" class="h-4 w-4" />
        {{ t('costAnalysis.subscriptionProfit.activeOnly') }}
      </label>

      <div class="flex flex-col">
        <label class="mb-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('costAnalysis.subscriptionProfit.costMode') }}
        </label>
        <select
          v-model="costMode"
          class="rounded border border-gray-300 bg-white px-2 py-1 text-sm dark:border-gray-600 dark:bg-gray-900 dark:text-gray-100"
        >
          <option value="per_mtok">{{ t('costAnalysis.subscriptionProfit.unitPerMtok') }}</option>
          <option value="per_dollar">{{ t('costAnalysis.subscriptionProfit.unitPerDollar') }}</option>
        </select>
      </div>

      <div class="flex flex-col">
        <label class="mb-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('costAnalysis.subscriptionProfit.purchasePrice') }}
        </label>
        <div class="flex items-center gap-2">
          <input
            v-model.number="purchasePrice"
            type="number"
            min="0"
            step="0.01"
            class="w-28 rounded border border-gray-300 bg-white px-2 py-1 text-sm dark:border-gray-600 dark:bg-gray-900 dark:text-gray-100"
          />
          <span class="text-xs text-gray-400">{{ unitLabel }}</span>
        </div>
        <span class="mt-1 text-[11px] text-gray-400">
          {{ t('costAnalysis.subscriptionProfit.purchasePriceHint') }}
        </span>
      </div>

      <template v-if="!activeOnly">
        <div class="flex flex-col">
          <label class="mb-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('costAnalysis.subscriptionProfit.startDate') }}
          </label>
          <input
            v-model="startDate"
            type="date"
            class="rounded border border-gray-300 bg-white px-2 py-1 text-sm dark:border-gray-600 dark:bg-gray-900 dark:text-gray-100"
          />
        </div>
        <div class="flex flex-col">
          <label class="mb-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('costAnalysis.subscriptionProfit.endDate') }}
          </label>
          <input
            v-model="endDate"
            type="date"
            class="rounded border border-gray-300 bg-white px-2 py-1 text-sm dark:border-gray-600 dark:bg-gray-900 dark:text-gray-100"
          />
        </div>
      </template>

      <button
        class="rounded bg-blue-600 px-4 py-1.5 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-60"
        :disabled="loading"
        @click="load"
      >
        {{ loading ? t('costAnalysis.subscriptionProfit.loading') : t('costAnalysis.subscriptionProfit.refresh') }}
      </button>

      <span class="text-[11px] text-gray-400">
        {{ activeOnly ? t('costAnalysis.subscriptionProfit.activeHint') : t('costAnalysis.subscriptionProfit.rangeHint') }}
      </span>
    </div>

    <p v-if="error" class="text-sm text-red-600 dark:text-red-400">{{ error }}</p>

    <!-- Summary cards -->
    <div v-if="data" class="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-6">
      <div class="rounded-lg border border-gray-200 bg-white p-3 dark:border-gray-700 dark:bg-gray-800">
        <div class="text-xs text-gray-500 dark:text-gray-400">
          {{ t('costAnalysis.subscriptionProfit.summary.subscriptions') }}
        </div>
        <div class="mt-1 text-lg font-semibold text-gray-900 dark:text-gray-100">
          {{ summary.subscription_count }}
        </div>
      </div>
      <div class="rounded-lg border border-gray-200 bg-white p-3 dark:border-gray-700 dark:bg-gray-800">
        <div class="text-xs text-gray-500 dark:text-gray-400">
          {{ t('costAnalysis.subscriptionProfit.summary.revenue') }}
        </div>
        <div class="mt-1 text-lg font-semibold text-gray-900 dark:text-gray-100">
          ¥{{ fmtMoney(summary.total_revenue_rmb) }}
        </div>
      </div>
      <div class="rounded-lg border border-gray-200 bg-white p-3 dark:border-gray-700 dark:bg-gray-800">
        <div class="text-xs text-gray-500 dark:text-gray-400">
          {{ t('costAnalysis.subscriptionProfit.summary.realCost') }}
        </div>
        <div class="mt-1 text-lg font-semibold text-gray-900 dark:text-gray-100">
          ¥{{ fmtMoney(summary.total_real_cost_rmb) }}
        </div>
      </div>
      <div class="rounded-lg border border-gray-200 bg-white p-3 dark:border-gray-700 dark:bg-gray-800">
        <div class="text-xs text-gray-500 dark:text-gray-400">
          {{ t('costAnalysis.subscriptionProfit.summary.grossProfit') }}
        </div>
        <div
          class="mt-1 text-lg font-semibold"
          :class="summary.total_gross_profit_rmb >= 0 ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'"
        >
          ¥{{ fmtMoney(summary.total_gross_profit_rmb) }}
        </div>
      </div>
      <div class="rounded-lg border border-gray-200 bg-white p-3 dark:border-gray-700 dark:bg-gray-800">
        <div class="text-xs text-gray-500 dark:text-gray-400">
          {{ t('costAnalysis.subscriptionProfit.summary.avgMultiple') }}
        </div>
        <div
          class="mt-1 text-lg font-semibold"
          :class="multipleClass(summary.avg_profit_multiple, summary.total_real_cost_rmb)"
        >
          {{ fmtMultiple(summary.avg_profit_multiple, summary.total_real_cost_rmb) }}
        </div>
      </div>
      <div class="rounded-lg border border-gray-200 bg-white p-3 dark:border-gray-700 dark:bg-gray-800">
        <div class="text-xs text-gray-500 dark:text-gray-400">
          {{ t('costAnalysis.subscriptionProfit.summary.lossCount') }} /
          {{ t('costAnalysis.subscriptionProfit.summary.belowTwoCount') }}
        </div>
        <div class="mt-1 text-lg font-semibold">
          <span class="text-red-600 dark:text-red-400">{{ summary.loss_count }}</span>
          <span class="text-gray-400"> / </span>
          <span class="text-amber-600 dark:text-amber-400">{{ summary.below_two_count }}</span>
        </div>
      </div>
    </div>

    <!-- By plan -->
    <div
      v-if="byPlan.length"
      class="overflow-x-auto rounded-lg border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800"
    >
      <div class="border-b border-gray-200 px-4 py-2 text-sm font-medium text-gray-700 dark:border-gray-700 dark:text-gray-200">
        {{ t('costAnalysis.subscriptionProfit.byPlan.title') }}
      </div>
      <table class="w-full min-w-[860px] text-sm">
        <thead>
          <tr class="text-left text-xs text-gray-500 dark:text-gray-400">
            <th class="px-4 py-2">{{ t('costAnalysis.subscriptionProfit.byPlan.plan') }}</th>
            <th class="px-4 py-2 text-right">{{ t('costAnalysis.subscriptionProfit.byPlan.count') }}</th>
            <th class="px-4 py-2 text-right">{{ t('costAnalysis.subscriptionProfit.byPlan.revenue') }}</th>
            <th class="px-4 py-2 text-right">{{ t('costAnalysis.subscriptionProfit.byPlan.realCost') }}</th>
            <th class="px-4 py-2 text-right">{{ t('costAnalysis.subscriptionProfit.byPlan.avgMultiple') }}</th>
            <th class="px-4 py-2 text-right">{{ t('costAnalysis.subscriptionProfit.byPlan.avgFullDays') }}</th>
            <th class="px-4 py-2 text-right">{{ t('costAnalysis.subscriptionProfit.byPlan.avgCacheRate') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="p in byPlan"
            :key="p.plan_id"
            class="border-t border-gray-100 text-gray-800 dark:border-gray-700/60 dark:text-gray-200"
          >
            <td class="px-4 py-2">
              <div class="max-w-[280px] truncate font-medium" :title="p.plan_name">{{ p.plan_name }}</div>
              <div class="text-xs text-gray-400">{{ p.plan_price > 0 ? '¥' + fmtMoney(p.plan_price) : t('costAnalysis.subscriptionProfit.noRevenue') }}</div>
            </td>
            <td class="px-4 py-2 text-right">{{ p.count }}</td>
            <td class="px-4 py-2 text-right">¥{{ fmtMoney(p.total_revenue_rmb) }}</td>
            <td class="px-4 py-2 text-right">¥{{ fmtMoney(p.total_real_cost_rmb) }}</td>
            <td class="px-4 py-2 text-right" :class="multipleClass(p.avg_profit_multiple, p.total_real_cost_rmb)">
              {{ fmtMultiple(p.avg_profit_multiple, p.total_real_cost_rmb) }}
            </td>
            <td class="px-4 py-2 text-right">{{ fmtDays(p.avg_equivalent_full_days) }}</td>
            <td class="px-4 py-2 text-right">{{ fmtPct(p.avg_cache_rate) }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Detail -->
    <div class="overflow-x-auto rounded-lg border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800">
      <div class="border-b border-gray-200 px-4 py-2 text-sm font-medium text-gray-700 dark:border-gray-700 dark:text-gray-200">
        {{ t('costAnalysis.subscriptionProfit.detail.title') }}
      </div>
      <table class="w-full min-w-[1040px] text-sm">
        <thead>
          <tr class="text-left text-xs text-gray-500 dark:text-gray-400">
            <th class="w-[220px] px-3 py-2">{{ t('costAnalysis.subscriptionProfit.detail.user') }}</th>
            <th class="w-[300px] px-3 py-2">{{ t('costAnalysis.subscriptionProfit.detail.subscription') }}</th>
            <th class="px-3 py-2 text-right">{{ t('costAnalysis.subscriptionProfit.detail.usage') }}</th>
            <th class="px-3 py-2 text-right">{{ t('costAnalysis.subscriptionProfit.detail.cost') }}</th>
            <th class="px-3 py-2 text-right">{{ t('costAnalysis.subscriptionProfit.detail.cache') }}</th>
            <th class="px-3 py-2 text-right">{{ t('costAnalysis.subscriptionProfit.detail.profit') }}</th>
            <th class="w-[160px] px-3 py-2">{{ t('costAnalysis.subscriptionProfit.detail.period') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!rows.length">
            <td colspan="7" class="px-3 py-6 text-center text-sm text-gray-400">
              {{ loading ? t('costAnalysis.subscriptionProfit.loading') : t('costAnalysis.subscriptionProfit.empty') }}
            </td>
          </tr>
          <tr
            v-for="r in rows"
            :key="r.subscription_id"
            class="border-t border-gray-100 text-gray-800 dark:border-gray-700/60 dark:text-gray-200"
          >
            <td class="px-3 py-3 align-top">
              <div class="max-w-[210px] truncate font-medium" :title="r.user_email || ('#' + r.user_id)">
                {{ r.user_email || ('#' + r.user_id) }}
              </div>
              <div class="text-xs text-gray-400">#{{ r.user_id }}</div>
            </td>
            <td class="px-3 py-3 align-top">
              <div class="flex min-w-0 items-center gap-2">
                <span class="max-w-[190px] truncate font-medium" :title="r.plan_name">{{ r.plan_name }}</span>
                <span
                  class="shrink-0 rounded-full px-2 py-0.5 text-[11px] font-medium"
                  :class="sourceClass(r.source)"
                >
                  {{ sourceLabel(r.source) }}
                </span>
              </div>
              <div class="mt-1 flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-xs text-gray-500 dark:text-gray-400">
                <span class="max-w-[170px] truncate" :title="r.group_name">{{ r.group_name || ('#' + r.group_id) }}</span>
                <span>{{ r.plan_price > 0 ? '¥' + fmtMoney(r.plan_price) : t('costAnalysis.subscriptionProfit.noRevenue') }}</span>
                <span>#{{ r.subscription_id }}</span>
              </div>
            </td>
            <td class="px-3 py-3 text-right align-top">
              <div class="font-medium">${{ fmtMoney(r.consumed_usd) }}</div>
              <div class="text-xs text-gray-400">{{ fmtNumber(r.request_count) }} {{ t('costAnalysis.subscriptionProfit.detail.requests') }}</div>
              <div class="text-xs text-gray-400">{{ fmtNumber(r.total_tokens) }} tokens</div>
            </td>
            <td class="px-3 py-3 text-right align-top">
              <div class="font-medium">¥{{ fmtMoney(r.real_cost_rmb) }}</div>
              <div class="text-xs text-gray-400">
                {{ r.consumed_usd > 0 ? '¥' + r.real_cost_per_dollar.toFixed(3) + ' / $' : '—' }}
              </div>
              <div class="text-xs text-gray-400">
                {{ r.consumed_usd > 0 ? '¥' + r.avg_price_per_dollar.toFixed(3) + ' / $ ' + t('costAnalysis.subscriptionProfit.detail.revenue') : '—' }}
              </div>
            </td>
            <td class="px-3 py-3 text-right align-top">
              <div>{{ fmtPct(r.cache_rate) }}</div>
              <div class="text-xs text-gray-400">{{ fmtDays(r.equivalent_full_days) }} {{ t('costAnalysis.subscriptionProfit.detail.fullDays') }}</div>
            </td>
            <td class="px-3 py-3 text-right align-top">
              <div :class="multipleClass(r.profit_multiple, r.real_cost_rmb)">
                {{ fmtMultiple(r.profit_multiple, r.real_cost_rmb) }}
              </div>
              <div
                class="text-xs"
                :class="r.gross_profit_rmb >= 0 ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'"
              >
                ¥{{ fmtMoney(r.gross_profit_rmb) }}
              </div>
            </td>
            <td class="px-3 py-3 align-top">
              <span class="rounded-full px-2 py-0.5 text-[11px] font-medium" :class="statusClass(r.status)">
                {{ r.status }}
              </span>
              <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ fmtDate(r.starts_at) }}</div>
              <div class="text-xs text-gray-400">{{ fmtDate(r.expires_at) }}</div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { getSubscriptionProfit } from '@/api/admin/costAnalysis'
import type {
  CostMode,
  SubscriptionProfitResponse,
  SubscriptionProfitSummary
} from '@/api/admin/costAnalysis'

const { t } = useI18n()

const LS_PRICE = 'cost_analysis_purchase_price'
const LS_MODE = 'cost_analysis_cost_mode'

const EMPTY_SUMMARY: SubscriptionProfitSummary = {
  subscription_count: 0,
  total_revenue_rmb: 0,
  total_real_cost_rmb: 0,
  total_gross_profit_rmb: 0,
  total_consumed_usd: 0,
  avg_profit_multiple: 0,
  loss_count: 0,
  below_two_count: 0,
  cost_mode: 'per_mtok',
  purchase_price: 0
}

const loading = ref(false)
const error = ref('')
const data = ref<SubscriptionProfitResponse | null>(null)

const storedPrice = Number(localStorage.getItem(LS_PRICE) || '')
const purchasePrice = ref<number>(storedPrice > 0 ? storedPrice : 0.25)
const costMode = ref<CostMode>(localStorage.getItem(LS_MODE) === 'per_dollar' ? 'per_dollar' : 'per_mtok')
const activeOnly = ref(true)
const startDate = ref('')
const endDate = ref('')

const summary = computed<SubscriptionProfitSummary>(() => data.value?.summary ?? EMPTY_SUMMARY)
const unitLabel = computed(() =>
  costMode.value === 'per_dollar'
    ? t('costAnalysis.subscriptionProfit.unitPerDollar')
    : t('costAnalysis.subscriptionProfit.unitPerMtok')
)
const byPlan = computed(() => data.value?.by_plan ?? [])
const rows = computed(() => data.value?.rows ?? [])

function fmtMoney(v: number | undefined | null): string {
  if (v === undefined || v === null) return '—'
  return v.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}
function fmtMultiple(m: number, realCost: number): string {
  if (!realCost || realCost <= 0) return '—'
  return m.toFixed(2) + 'x'
}
function fmtPct(v: number): string {
  return (v * 100).toFixed(0) + '%'
}
function fmtDays(v: number): string {
  return v.toFixed(1)
}
function fmtDate(s: string): string {
  if (!s) return '—'
  return s.slice(0, 10)
}
function fmtNumber(v: number | undefined | null): string {
  if (v === undefined || v === null) return '—'
  return v.toLocaleString()
}
function multipleClass(m: number, realCost: number): string {
  if (!realCost || realCost <= 0) return 'text-gray-400'
  if (m < 1) return 'text-red-600 dark:text-red-400 font-semibold'
  if (m < 2) return 'text-amber-600 dark:text-amber-400 font-semibold'
  if (m < 5) return 'text-blue-600 dark:text-blue-400'
  return 'text-green-600 dark:text-green-400 font-semibold'
}
function sourceLabel(source: string): string {
  const key = `costAnalysis.subscriptionProfit.sources.${source || 'system'}`
  const translated = t(key)
  return translated === key ? source || t('costAnalysis.subscriptionProfit.sources.system') : translated
}
function sourceClass(source: string): string {
  switch (source) {
    case 'paid':
      return 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300'
    case 'redeem':
      return 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300'
    case 'admin':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300'
    case 'default':
      return 'bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300'
    default:
      return 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300'
  }
}
function statusClass(status: string): string {
  switch (status) {
    case 'active':
      return 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300'
    case 'expired':
      return 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300'
    case 'suspended':
      return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
    default:
      return 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300'
  }
}

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const price = purchasePrice.value > 0 ? purchasePrice.value : 0
    localStorage.setItem(LS_PRICE, String(price))
    localStorage.setItem(LS_MODE, costMode.value)
    data.value = await getSubscriptionProfit({
      active_only: activeOnly.value,
      start_date: activeOnly.value ? undefined : startDate.value || undefined,
      end_date: activeOnly.value ? undefined : endDate.value || undefined,
      cost_mode: costMode.value,
      purchase_price: price
    })
  } catch (e) {
    error.value = (e as Error)?.message || 'Error'
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>
