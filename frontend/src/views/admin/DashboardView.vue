<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Loading State -->
      <div v-if="loading" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>

      <template v-else-if="stats">
        <!-- Row 1: Core Stats -->
        <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
          <!-- Total API Keys -->
          <div class="card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-blue-100 p-2 dark:bg-blue-900/30">
                <Icon name="key" size="md" class="text-blue-600 dark:text-blue-400" :stroke-width="2" />
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.dashboard.apiKeys') }}
                </p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">
                  {{ stats.total_api_keys }}
                </p>
                <p class="text-xs text-green-600 dark:text-green-400">
                  {{ stats.active_api_keys }} {{ t('common.active') }}
                </p>
              </div>
            </div>
          </div>

          <!-- Service Accounts -->
          <div class="card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-purple-100 p-2 dark:bg-purple-900/30">
                <Icon name="server" size="md" class="text-purple-600 dark:text-purple-400" :stroke-width="2" />
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.dashboard.accounts') }}
                </p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">
                  {{ stats.total_accounts }}
                </p>
                <p class="text-xs">
                  <span class="text-green-600 dark:text-green-400"
                    >{{ stats.normal_accounts }} {{ t('common.active') }}</span
                  >
                  <span v-if="stats.error_accounts > 0" class="ml-1 text-red-500"
                    >{{ stats.error_accounts }} {{ t('common.error') }}</span
                  >
                </p>
              </div>
            </div>
          </div>

          <!-- Today Requests -->
          <div class="card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-green-100 p-2 dark:bg-green-900/30">
                <Icon name="chart" size="md" class="text-green-600 dark:text-green-400" :stroke-width="2" />
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.dashboard.todayRequests') }}
                </p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">
                  {{ stats.today_requests }}
                </p>
                <p class="text-xs text-gray-500 dark:text-gray-400">
                  {{ t('common.total') }}: {{ formatNumber(stats.total_requests) }}
                </p>
              </div>
            </div>
          </div>

          <!-- New Users Today -->
          <div class="card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-emerald-100 p-2 dark:bg-emerald-900/30">
                <Icon name="userPlus" size="md" class="text-emerald-600 dark:text-emerald-400" :stroke-width="2" />
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.dashboard.users') }}
                </p>
                <p class="text-xl font-bold text-emerald-600 dark:text-emerald-400">
                  +{{ stats.today_new_users }}
                </p>
                <p class="text-xs text-gray-500 dark:text-gray-400">
                  {{ t('common.total') }}: {{ formatNumber(stats.total_users) }}
                </p>
              </div>
            </div>
          </div>
        </div>

        <!-- Row 2: Token Stats -->
        <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
          <!-- Today Tokens -->
          <div class="card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-amber-100 p-2 dark:bg-amber-900/30">
                <Icon name="cube" size="md" class="text-amber-600 dark:text-amber-400" :stroke-width="2" />
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.dashboard.todayTokens') }}
                </p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">
                  {{ formatTokens(stats.today_tokens) }}
                </p>
                <p class="text-xs">
                  <span
                    class="text-green-600 dark:text-green-400"
                    :title="t('admin.dashboard.actual')"
                    >${{ formatCost(stats.today_actual_cost) }}</span
                  >
                  <span class="text-gray-400 dark:text-gray-500"> / </span>
                  <span
                    class="text-orange-500 dark:text-orange-400"
                    :title="t('admin.dashboard.accountCost')"
                    >${{ formatCost(stats.today_account_cost) }}</span
                  >
                  <span class="text-gray-400 dark:text-gray-500"> / </span>
                  <span
                    class="text-gray-400 dark:text-gray-500"
                    :title="t('admin.dashboard.standard')"
                    >${{ formatCost(stats.today_cost) }}</span
                  >
                </p>
              </div>
            </div>
          </div>

          <!-- Total Tokens -->
          <div class="card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-indigo-100 p-2 dark:bg-indigo-900/30">
                <Icon name="database" size="md" class="text-indigo-600 dark:text-indigo-400" :stroke-width="2" />
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.dashboard.totalTokens') }}
                </p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">
                  {{ formatTokens(stats.total_tokens) }}
                </p>
                <p class="text-xs">
                  <span
                    class="text-green-600 dark:text-green-400"
                    :title="t('admin.dashboard.actual')"
                    >${{ formatCost(stats.total_actual_cost) }}</span
                  >
                  <span class="text-gray-400 dark:text-gray-500"> / </span>
                  <span
                    class="text-orange-500 dark:text-orange-400"
                    :title="t('admin.dashboard.accountCost')"
                    >${{ formatCost(stats.total_account_cost) }}</span
                  >
                  <span class="text-gray-400 dark:text-gray-500"> / </span>
                  <span
                    class="text-gray-400 dark:text-gray-500"
                    :title="t('admin.dashboard.standard')"
                    >${{ formatCost(stats.total_cost) }}</span
                  >
                </p>
              </div>
            </div>
          </div>

          <!-- Performance (RPM/TPM) -->
          <div class="card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-violet-100 p-2 dark:bg-violet-900/30">
                <Icon name="bolt" size="md" class="text-violet-600 dark:text-violet-400" :stroke-width="2" />
              </div>
              <div class="flex-1">
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.dashboard.performance') }}
                </p>
                <div class="flex items-baseline gap-2">
                  <p class="text-xl font-bold text-gray-900 dark:text-white">
                    {{ formatTokens(stats.rpm) }}
                  </p>
                  <span class="text-xs text-gray-500 dark:text-gray-400">RPM</span>
                </div>
                <div class="flex items-baseline gap-2">
                  <p class="text-sm font-semibold text-violet-600 dark:text-violet-400">
                    {{ formatTokens(stats.tpm) }}
                  </p>
                  <span class="text-xs text-gray-500 dark:text-gray-400">TPM</span>
                </div>
              </div>
            </div>
          </div>

          <!-- Avg Response Time -->
          <div class="card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-rose-100 p-2 dark:bg-rose-900/30">
                <Icon name="clock" size="md" class="text-rose-600 dark:text-rose-400" :stroke-width="2" />
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.dashboard.avgResponse') }}
                </p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">
                  {{ formatDuration(stats.average_duration_ms) }}
                </p>
                <p class="text-xs text-gray-500 dark:text-gray-400">
                  {{ stats.active_users }} {{ t('admin.dashboard.activeUsers') }}
                </p>
              </div>
            </div>
          </div>
        </div>

        <!-- Charts Section -->
        <div class="space-y-6">
          <!-- Cache Status -->
          <div class="card p-4">
            <div class="mb-4 flex flex-wrap items-center gap-3">
              <div>
                <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                  {{ t('admin.dashboard.cacheStatus.title') }}
                </h3>
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                  {{ t('admin.dashboard.cacheStatus.subtitle') }}
                </p>
              </div>
              <div class="ml-auto flex flex-wrap items-center gap-2">
                <div class="w-32">
                  <Select
                    v-model="cacheStatusWindow"
                    :options="cacheStatusWindowOptions"
                    @change="loadCacheStatus"
                  />
                </div>
                <div class="w-40">
                  <Select
                    v-model="cacheStatusPlatform"
                    :options="cacheStatusPlatformOptions"
                    @change="loadCacheStatus"
                  />
                </div>
                <button @click="loadCacheStatus" :disabled="cacheStatusLoading" class="btn btn-secondary">
                  {{ t('common.refresh') }}
                </button>
              </div>
            </div>

            <div v-if="cacheStatusLoading && !cacheStatus" class="flex h-44 items-center justify-center">
              <LoadingSpinner size="md" />
            </div>
            <div v-else-if="cacheStatus" class="space-y-4">
              <div class="grid grid-cols-2 gap-3 lg:grid-cols-5">
                <div class="rounded-lg border border-gray-200 p-3 dark:border-gray-700">
                  <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                    {{ t('admin.dashboard.cacheStatus.readRate') }}
                  </p>
                  <p class="mt-1 text-xl font-bold text-emerald-600 dark:text-emerald-400">
                    {{ formatPercent(cacheStatus.summary.cache_read_rate) }}
                  </p>
                  <p class="text-xs text-gray-500 dark:text-gray-400">
                    {{ formatTokens(cacheStatus.summary.cache_read_tokens) }}
                  </p>
                </div>
                <div class="rounded-lg border border-gray-200 p-3 dark:border-gray-700">
                  <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                    {{ t('admin.dashboard.cacheStatus.creationRate') }}
                  </p>
                  <p class="mt-1 text-xl font-bold text-blue-600 dark:text-blue-400">
                    {{ formatPercent(cacheStatus.summary.cache_creation_rate) }}
                  </p>
                  <p class="text-xs text-gray-500 dark:text-gray-400">
                    {{ formatTokens(cacheStatus.summary.cache_creation_tokens) }}
                  </p>
                </div>
                <div class="rounded-lg border border-gray-200 p-3 dark:border-gray-700">
                  <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                    {{ t('admin.dashboard.cacheStatus.hitRequests') }}
                  </p>
                  <p class="mt-1 text-xl font-bold text-gray-900 dark:text-white">
                    {{ formatPercent(cacheStatus.summary.request_hit_rate) }}
                  </p>
                  <p class="text-xs text-gray-500 dark:text-gray-400">
                    {{ cacheStatus.summary.cache_hit_requests }} / {{ cacheStatus.summary.requests }}
                  </p>
                </div>
                <div class="rounded-lg border border-gray-200 p-3 dark:border-gray-700">
                  <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                    {{ t('admin.dashboard.cacheStatus.promptTokens') }}
                  </p>
                  <p class="mt-1 text-xl font-bold text-gray-900 dark:text-white">
                    {{ formatTokens(cacheStatus.summary.prompt_total_tokens) }}
                  </p>
                  <p class="text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.dashboard.cacheStatus.input') }} {{ formatTokens(cacheStatus.summary.input_tokens) }}
                  </p>
                </div>
                <div class="rounded-lg border border-gray-200 p-3 dark:border-gray-700">
                  <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                    {{ t('common.status') }}
                  </p>
                  <p class="mt-2">
                    <span
                      class="rounded-full px-2 py-1 text-xs font-semibold"
                      :class="cacheStatusClass(cacheStatus.summary.status)"
                    >
                      {{ cacheStatusLabel(cacheStatus.summary.status) }}
                    </span>
                  </p>
                  <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
                    {{ cacheStatus.window }} · {{ cacheStatus.platform }}
                  </p>
                </div>
              </div>

              <div class="grid grid-cols-1 gap-4 xl:grid-cols-5">
                <div class="xl:col-span-2">
                  <p class="mb-3 text-xs font-semibold uppercase text-gray-500 dark:text-gray-400">
                    {{ t('admin.dashboard.cacheStatus.trend') }}
                  </p>
                  <div class="flex h-40 items-end gap-1 rounded-lg border border-gray-200 p-3 dark:border-gray-700">
                    <div
                      v-for="point in cacheStatus.trend"
                      :key="point.bucket"
                      class="group relative flex min-w-0 flex-1 items-end"
                    >
                      <div
                        class="w-full rounded-t bg-emerald-500/80 transition-colors group-hover:bg-emerald-500"
                        :style="{ height: cacheTrendBarHeight(point.cache_read_rate) }"
                      ></div>
                      <div
                        class="pointer-events-none absolute bottom-full left-1/2 z-10 mb-2 hidden w-44 -translate-x-1/2 rounded-md bg-gray-900 px-2 py-1 text-xs text-white shadow-lg group-hover:block"
                      >
                        <div>{{ formatCacheBucket(point.bucket) }}</div>
                        <div>{{ t('admin.dashboard.cacheStatus.readRate') }}: {{ formatPercent(point.cache_read_rate) }}</div>
                        <div>{{ t('admin.dashboard.cacheStatus.requests') }}: {{ point.requests }}</div>
                      </div>
                    </div>
                    <div
                      v-if="!cacheStatus.trend.length"
                      class="flex h-full w-full items-center justify-center text-sm text-gray-500 dark:text-gray-400"
                    >
                      {{ t('admin.dashboard.noDataAvailable') }}
                    </div>
                  </div>
                </div>

                <div class="overflow-hidden xl:col-span-3">
                  <p class="mb-3 text-xs font-semibold uppercase text-gray-500 dark:text-gray-400">
                    {{ t('admin.dashboard.cacheStatus.byModel') }}
                  </p>
                  <div class="overflow-x-auto rounded-lg border border-gray-200 dark:border-gray-700">
                    <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-gray-700">
                      <thead class="bg-gray-50 dark:bg-gray-800/60">
                        <tr>
                          <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">
                            {{ t('admin.dashboard.model') }}
                          </th>
                          <th class="px-3 py-2 text-right text-xs font-medium text-gray-500 dark:text-gray-400">
                            {{ t('admin.dashboard.cacheStatus.readRate') }}
                          </th>
                          <th class="px-3 py-2 text-right text-xs font-medium text-gray-500 dark:text-gray-400">
                            {{ t('admin.dashboard.cacheStatus.creationRate') }}
                          </th>
                          <th class="px-3 py-2 text-right text-xs font-medium text-gray-500 dark:text-gray-400">
                            {{ t('admin.dashboard.requests') }}
                          </th>
                          <th class="px-3 py-2 text-right text-xs font-medium text-gray-500 dark:text-gray-400">
                            {{ t('common.status') }}
                          </th>
                        </tr>
                      </thead>
                      <tbody class="divide-y divide-gray-100 dark:divide-gray-800">
                        <tr v-for="model in cacheStatus.models" :key="`${model.requested_model}:${model.upstream_model}`">
                          <td class="max-w-72 px-3 py-2">
                            <div class="truncate font-medium text-gray-900 dark:text-white">
                              {{ model.requested_model || t('common.unknown') }}
                            </div>
                            <div class="truncate text-xs text-gray-500 dark:text-gray-400">
                              {{ model.upstream_model || '-' }}
                            </div>
                          </td>
                          <td class="px-3 py-2 text-right font-semibold text-emerald-600 dark:text-emerald-400">
                            {{ formatPercent(model.cache_read_rate) }}
                          </td>
                          <td class="px-3 py-2 text-right text-gray-700 dark:text-gray-300">
                            {{ formatPercent(model.cache_creation_rate) }}
                          </td>
                          <td class="px-3 py-2 text-right text-gray-700 dark:text-gray-300">
                            {{ model.requests }}
                          </td>
                          <td class="px-3 py-2 text-right">
                            <span
                              class="rounded-full px-2 py-1 text-xs font-semibold"
                              :class="cacheStatusClass(model.status)"
                            >
                              {{ cacheStatusLabel(model.status) }}
                            </span>
                          </td>
                        </tr>
                        <tr v-if="!cacheStatus.models.length">
                          <td colspan="5" class="px-3 py-6 text-center text-sm text-gray-500 dark:text-gray-400">
                            {{ t('admin.dashboard.noDataAvailable') }}
                          </td>
                        </tr>
                      </tbody>
                    </table>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- Date Range Filter -->
          <div class="card p-4">
            <div class="flex flex-wrap items-center gap-4">
              <div class="flex items-center gap-2">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300"
                  >{{ t('admin.dashboard.timeRange') }}:</span
                >
                <DateRangePicker
                  v-model:start-date="startDate"
                  v-model:end-date="endDate"
                  @change="onDateRangeChange"
                />
              </div>
              <button @click="loadDashboardStats" :disabled="chartsLoading" class="btn btn-secondary">
                {{ t('common.refresh') }}
              </button>
              <div class="ml-auto flex items-center gap-2">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300"
                  >{{ t('admin.dashboard.granularity') }}:</span
                >
                <div class="w-28">
                  <Select
                    v-model="granularity"
                    :options="granularityOptions"
                    @change="loadChartData"
                  />
                </div>
              </div>
            </div>
          </div>

          <!-- Charts Grid -->
          <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
            <ModelDistributionChart
              :model-stats="modelStats"
              :enable-ranking-view="true"
              :ranking-items="rankingItems"
              :ranking-total-actual-cost="rankingTotalActualCost"
              :ranking-total-requests="rankingTotalRequests"
              :ranking-total-tokens="rankingTotalTokens"
              :loading="chartsLoading"
              :ranking-loading="rankingLoading"
              :ranking-error="rankingError"
              :start-date="startDate"
              :end-date="endDate"
              @ranking-click="goToUserUsage"
            />
            <TokenUsageTrend :trend-data="trendData" :loading="chartsLoading" />
          </div>

          <!-- User Usage Trend (Full Width) -->
          <div class="card p-4">
            <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('admin.dashboard.recentUsage') }} (Top 12)
            </h3>
            <div class="h-64">
              <div v-if="userTrendLoading" class="flex h-full items-center justify-center">
                <LoadingSpinner size="md" />
              </div>
              <Line v-else-if="userTrendChartData" :data="userTrendChartData" :options="lineOptions" />
              <div
                v-else
                class="flex h-full items-center justify-center text-sm text-gray-500 dark:text-gray-400"
              >
                {{ t('admin.dashboard.noDataAvailable') }}
              </div>
            </div>
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
import { adminAPI } from '@/api/admin'
import type {
  DashboardStats,
  TrendDataPoint,
  ModelStat,
  UserUsageTrendPoint,
  UserSpendingRankingItem
} from '@/types'
import type { CacheStatusResponse, CacheStatusWindow } from '@/api/admin/dashboard'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Select from '@/components/common/Select.vue'
import ModelDistributionChart from '@/components/charts/ModelDistributionChart.vue'
import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'

import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import { Line } from 'vue-chartjs'

// Register Chart.js components
ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Tooltip,
  Legend,
  Filler
)

const appStore = useAppStore()
const router = useRouter()
const stats = ref<DashboardStats | null>(null)
const loading = ref(false)
const chartsLoading = ref(false)
const userTrendLoading = ref(false)
const rankingLoading = ref(false)
const rankingError = ref(false)
const cacheStatusLoading = ref(false)

// Chart data
const trendData = ref<TrendDataPoint[]>([])
const modelStats = ref<ModelStat[]>([])
const userTrend = ref<UserUsageTrendPoint[]>([])
const rankingItems = ref<UserSpendingRankingItem[]>([])
const rankingTotalActualCost = ref(0)
const rankingTotalRequests = ref(0)
const rankingTotalTokens = ref(0)
const cacheStatus = ref<CacheStatusResponse | null>(null)
const cacheStatusWindow = ref<CacheStatusWindow>('24h')
const cacheStatusPlatform = ref('antigravity')
let chartLoadSeq = 0
let usersTrendLoadSeq = 0
let rankingLoadSeq = 0
let cacheStatusLoadSeq = 0
const rankingLimit = 12

// Helper function to format date in local timezone
const formatLocalDate = (date: Date): string => {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

const getLast24HoursRangeDates = (): { start: string; end: string } => {
  const end = new Date()
  const start = new Date(end.getTime() - 24 * 60 * 60 * 1000)
  return {
    start: formatLocalDate(start),
    end: formatLocalDate(end)
  }
}

// Date range
const granularity = ref<'day' | 'hour'>('hour')
const defaultRange = getLast24HoursRangeDates()
const startDate = ref(defaultRange.start)
const endDate = ref(defaultRange.end)

// Granularity options for Select component
const granularityOptions = computed(() => [
  { value: 'day', label: t('admin.dashboard.day') },
  { value: 'hour', label: t('admin.dashboard.hour') }
])

const cacheStatusWindowOptions = computed(() => [
  { value: '1h', label: t('admin.dashboard.cacheStatus.window1h') },
  { value: '6h', label: t('admin.dashboard.cacheStatus.window6h') },
  { value: '24h', label: t('admin.dashboard.cacheStatus.window24h') },
  { value: '7d', label: t('admin.dashboard.cacheStatus.window7d') }
])

const cacheStatusPlatformOptions = computed(() => [
  { value: 'antigravity', label: 'Antigravity' },
  { value: 'all', label: t('common.all') }
])

// Dark mode detection
const isDarkMode = computed(() => {
  return document.documentElement.classList.contains('dark')
})

// Chart colors
const chartColors = computed(() => ({
  text: isDarkMode.value ? '#e5e7eb' : '#374151',
  grid: isDarkMode.value ? '#374151' : '#e5e7eb'
}))

// Line chart options (for user trend chart)
const lineOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: {
    intersect: false,
    mode: 'index' as const
  },
  plugins: {
    legend: {
      position: 'top' as const,
      labels: {
        color: chartColors.value.text,
        usePointStyle: true,
        pointStyle: 'circle',
        padding: 15,
        font: {
          size: 11
        }
      }
    },
    tooltip: {
      itemSort: (a: any, b: any) => {
        const aValue = typeof a?.raw === 'number' ? a.raw : Number(a?.parsed?.y ?? 0)
        const bValue = typeof b?.raw === 'number' ? b.raw : Number(b?.parsed?.y ?? 0)
        return bValue - aValue
      },
      callbacks: {
        label: (context: any) => {
          return `${context.dataset.label}: ${formatTokens(context.raw)}`
        }
      }
    }
  },
  scales: {
    x: {
      grid: {
        color: chartColors.value.grid
      },
      ticks: {
        color: chartColors.value.text,
        font: {
          size: 10
        }
      }
    },
    y: {
      grid: {
        color: chartColors.value.grid
      },
      ticks: {
        color: chartColors.value.text,
        font: {
          size: 10
        },
        callback: (value: string | number) => formatTokens(Number(value))
      }
    }
  }
}))

// User trend chart data
const userTrendChartData = computed(() => {
  if (!userTrend.value?.length) return null

  const getDisplayName = (point: UserUsageTrendPoint): string => {
    const username = point.username?.trim()
    if (username) {
      return username
    }

    const email = point.email?.trim()
    if (email) {
      return email
    }

    return t('admin.redeem.userPrefix', { id: point.user_id })
  }

  // Group by user_id to avoid merging different users with the same display name
  const userGroups = new Map<number, { name: string; data: Map<string, number> }>()
  const allDates = new Set<string>()

  userTrend.value.forEach((point) => {
    allDates.add(point.date)
    const key = point.user_id
    if (!userGroups.has(key)) {
      userGroups.set(key, { name: getDisplayName(point), data: new Map() })
    }
    userGroups.get(key)!.data.set(point.date, point.tokens)
  })

  const sortedDates = Array.from(allDates).sort()
  const colors = [
    '#3b82f6',
    '#10b981',
    '#f59e0b',
    '#ef4444',
    '#8b5cf6',
    '#ec4899',
    '#14b8a6',
    '#f97316',
    '#6366f1',
    '#84cc16',
    '#06b6d4',
    '#a855f7'
  ]

  const datasets = Array.from(userGroups.values()).map((group, idx) => ({
    label: group.name,
    data: sortedDates.map((date) => group.data.get(date) || 0),
    borderColor: colors[idx % colors.length],
    backgroundColor: `${colors[idx % colors.length]}20`,
    fill: false,
    tension: 0.3
  }))

  return {
    labels: sortedDates,
    datasets
  }
})

const cacheTrendMaxReadRate = computed(() => {
  const rates = cacheStatus.value?.trend?.map((point) => point.cache_read_rate) || []
  return Math.max(0.01, ...rates)
})

// Format helpers
const formatTokens = (value: number | undefined): string => {
  if (value === undefined || value === null) return '0'
  if (value >= 1_000_000_000) {
    return `${(value / 1_000_000_000).toFixed(2)}B`
  } else if (value >= 1_000_000) {
    return `${(value / 1_000_000).toFixed(2)}M`
  } else if (value >= 1_000) {
    return `${(value / 1_000).toFixed(2)}K`
  }
  return value.toLocaleString()
}

const formatNumber = (value: number): string => {
  return value.toLocaleString()
}

const formatCost = (value: number): string => {
  if (value >= 1000) {
    return (value / 1000).toFixed(2) + 'K'
  } else if (value >= 1) {
    return value.toFixed(2)
  } else if (value >= 0.01) {
    return value.toFixed(3)
  }
  return value.toFixed(4)
}

const formatDuration = (ms: number): string => {
  if (ms >= 1000) {
    return `${(ms / 1000).toFixed(2)}s`
  }
  return `${Math.round(ms)}ms`
}

const formatPercent = (value: number | undefined): string => {
  if (value === undefined || value === null || Number.isNaN(value)) return '0%'
  return `${(value * 100).toFixed(1)}%`
}

const formatCacheBucket = (value: string): string => {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

const cacheTrendBarHeight = (rate: number): string => {
  const height = Math.max(6, Math.round((rate / cacheTrendMaxReadRate.value) * 100))
  return `${Math.min(100, height)}%`
}

const cacheStatusLabel = (status: string): string => {
  const labels: Record<string, string> = {
    healthy: t('admin.dashboard.cacheStatus.status.healthy'),
    watch: t('admin.dashboard.cacheStatus.status.watch'),
    unhealthy: t('admin.dashboard.cacheStatus.status.unhealthy'),
    insufficient: t('admin.dashboard.cacheStatus.status.insufficient')
  }
  return labels[status] || status
}

const cacheStatusClass = (status: string): string => {
  switch (status) {
    case 'healthy':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300'
    case 'watch':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300'
    case 'unhealthy':
      return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
    default:
      return 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300'
  }
}

const goToUserUsage = (item: UserSpendingRankingItem) => {
  void router.push({
    path: '/admin/usage',
    query: {
      user_id: String(item.user_id),
      start_date: startDate.value,
      end_date: endDate.value
    }
  })
}

// Date range change handler
const onDateRangeChange = (range: {
  startDate: string
  endDate: string
  preset: string | null
}) => {
  // Auto-select granularity based on date range
  const start = new Date(range.startDate)
  const end = new Date(range.endDate)
  const daysDiff = Math.ceil((end.getTime() - start.getTime()) / (1000 * 60 * 60 * 24))

  // If range is 1 day, use hourly granularity
  if (daysDiff <= 1) {
    granularity.value = 'hour'
  } else {
    granularity.value = 'day'
  }

  loadChartData()
}

// Load data
const loadDashboardSnapshot = async (includeStats: boolean) => {
  const currentSeq = ++chartLoadSeq
  if (includeStats && !stats.value) {
    loading.value = true
  }
  chartsLoading.value = true
  try {
    const response = await adminAPI.dashboard.getSnapshotV2({
      start_date: startDate.value,
      end_date: endDate.value,
      granularity: granularity.value,
      include_stats: includeStats,
      include_trend: true,
      include_model_stats: true,
      include_group_stats: false,
      include_users_trend: false
    })
    if (currentSeq !== chartLoadSeq) return
    if (includeStats && response.stats) {
      stats.value = response.stats
    }
    trendData.value = response.trend || []
    modelStats.value = response.models || []
  } catch (error) {
    if (currentSeq !== chartLoadSeq) return
    appStore.showError(t('admin.dashboard.failedToLoad'))
    console.error('Error loading dashboard snapshot:', error)
  } finally {
    if (currentSeq === chartLoadSeq) {
      loading.value = false
      chartsLoading.value = false
    }
  }
}

const loadUsersTrend = async () => {
  const currentSeq = ++usersTrendLoadSeq
  userTrendLoading.value = true
  try {
    const response = await adminAPI.dashboard.getUserUsageTrend({
      start_date: startDate.value,
      end_date: endDate.value,
      granularity: granularity.value,
      limit: 12
    })
    if (currentSeq !== usersTrendLoadSeq) return
    userTrend.value = response.trend || []
  } catch (error) {
    if (currentSeq !== usersTrendLoadSeq) return
    console.error('Error loading users trend:', error)
    userTrend.value = []
  } finally {
    if (currentSeq === usersTrendLoadSeq) {
      userTrendLoading.value = false
    }
  }
}

const loadUserSpendingRanking = async () => {
  const currentSeq = ++rankingLoadSeq
  rankingLoading.value = true
  rankingError.value = false
  try {
    const response = await adminAPI.dashboard.getUserSpendingRanking({
      start_date: startDate.value,
      end_date: endDate.value,
      limit: rankingLimit
    })
    if (currentSeq !== rankingLoadSeq) return
    rankingItems.value = response.ranking || []
    rankingTotalActualCost.value = response.total_actual_cost || 0
    rankingTotalRequests.value = response.total_requests || 0
    rankingTotalTokens.value = response.total_tokens || 0
  } catch (error) {
    if (currentSeq !== rankingLoadSeq) return
    console.error('Error loading user spending ranking:', error)
    rankingItems.value = []
    rankingTotalActualCost.value = 0
    rankingTotalRequests.value = 0
    rankingTotalTokens.value = 0
    rankingError.value = true
  } finally {
    if (currentSeq === rankingLoadSeq) {
      rankingLoading.value = false
    }
  }
}

const loadCacheStatus = async () => {
  const currentSeq = ++cacheStatusLoadSeq
  cacheStatusLoading.value = true
  try {
    const response = await adminAPI.dashboard.getCacheStatus({
      window: cacheStatusWindow.value,
      platform: cacheStatusPlatform.value
    })
    if (currentSeq !== cacheStatusLoadSeq) return
    cacheStatus.value = response
  } catch (error) {
    if (currentSeq !== cacheStatusLoadSeq) return
    console.error('Error loading cache status:', error)
    cacheStatus.value = null
  } finally {
    if (currentSeq === cacheStatusLoadSeq) {
      cacheStatusLoading.value = false
    }
  }
}

const loadDashboardStats = async () => {
  await Promise.all([
    loadDashboardSnapshot(true),
    loadCacheStatus(),
    loadUsersTrend(),
    loadUserSpendingRanking()
  ])
}

const loadChartData = async () => {
  await Promise.all([
    loadDashboardSnapshot(false),
    loadUsersTrend(),
    loadUserSpendingRanking()
  ])
}

onMounted(() => {
  loadDashboardStats()
})
</script>

<style scoped>
</style>
