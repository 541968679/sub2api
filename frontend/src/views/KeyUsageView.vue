<template>
  <div class="min-h-screen bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white">
    <header class="border-b border-gray-200 bg-white/90 px-4 py-3 dark:border-dark-800 dark:bg-dark-900/90">
      <div class="mx-auto flex max-w-7xl items-center justify-between gap-3">
        <div class="min-w-0">
          <h1 class="truncate text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('keyUsage.title') }}
          </h1>
          <p class="hidden text-sm text-gray-500 dark:text-gray-400 sm:block">
            {{ t('keyUsage.subtitle') }}
          </p>
        </div>
        <div class="flex items-center gap-2">
          <LocaleSwitcher />
          <button
            type="button"
            class="rounded-lg p-2 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="isDark ? t('keyUsage.switchToLight') : t('keyUsage.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>
        </div>
      </div>
    </header>

    <main class="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
      <TablePageLayout scroll-mode="page">
        <template #actions>
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-5">
            <div class="card p-4">
              <div class="flex items-center gap-3">
                <div class="rounded-lg bg-emerald-100 p-2 dark:bg-emerald-900/30">
                  <Icon name="creditCard" size="md" class="text-emerald-600 dark:text-emerald-400" />
                </div>
                <div class="min-w-0 flex-1">
                  <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                    {{ balanceCardTitle }}
                  </p>
                  <p class="text-xl font-bold text-emerald-600 dark:text-emerald-400">
                    {{ balanceCardValue }}
                  </p>
                  <p class="truncate text-xs text-gray-500 dark:text-gray-400">
                    {{ balanceCardDetail }}
                  </p>
                </div>
              </div>
            </div>

            <div class="card p-4">
              <div class="flex items-center gap-3">
                <div class="rounded-lg bg-blue-100 p-2 dark:bg-blue-900/30">
                  <Icon name="document" size="md" class="text-blue-600 dark:text-blue-400" />
                </div>
                <div class="min-w-0">
                  <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                    {{ t('usage.totalRequests') }}
                  </p>
                  <p class="text-xl font-bold text-gray-900 dark:text-white">
                    {{ usageStats?.total_requests?.toLocaleString() || '0' }}
                  </p>
                  <p class="text-xs text-gray-500 dark:text-gray-400">
                    {{ queried ? t('usage.inSelectedRange') : t('keyUsage.awaitingQuery') }}
                  </p>
                </div>
              </div>
            </div>

            <div class="card p-4">
              <div class="flex items-center gap-3">
                <div class="rounded-lg bg-amber-100 p-2 dark:bg-amber-900/30">
                  <Icon name="cube" size="md" class="text-amber-600 dark:text-amber-400" />
                </div>
                <div class="min-w-0">
                  <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                    {{ t('usage.totalTokens') }}
                  </p>
                  <p class="text-xl font-bold text-gray-900 dark:text-white">
                    {{ formatTokens(usageStats?.total_tokens || 0) }}
                  </p>
                  <p class="truncate text-xs text-gray-500 dark:text-gray-400">
                    {{ t('usage.in') }}: {{ formatTokens(usageStats?.total_input_tokens || 0) }} /
                    {{ t('usage.out') }}: {{ formatTokens(usageStats?.total_output_tokens || 0) }}
                  </p>
                </div>
              </div>
            </div>

            <div class="card p-4">
              <div class="flex items-center gap-3">
                <div class="rounded-lg bg-green-100 p-2 dark:bg-green-900/30">
                  <Icon name="dollar" size="md" class="text-green-600 dark:text-green-400" />
                </div>
                <div class="min-w-0 flex-1">
                  <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                    {{ t('usage.totalCost') }}
                  </p>
                  <p class="text-xl font-bold text-green-600 dark:text-green-400">
                    ${{ (usageStats?.total_actual_cost || 0).toFixed(4) }}
                  </p>
                  <p class="truncate text-xs text-gray-500 dark:text-gray-400">
                    {{ t('usage.actualCost') }} /
                    <span class="line-through">${{ (usageStats?.total_cost || 0).toFixed(4) }}</span>
                    {{ t('usage.standardCost') }}
                  </p>
                </div>
              </div>
            </div>

            <div class="card p-4">
              <div class="flex items-center gap-3">
                <div class="rounded-lg bg-purple-100 p-2 dark:bg-purple-900/30">
                  <Icon name="clock" size="md" class="text-purple-600 dark:text-purple-400" />
                </div>
                <div class="min-w-0">
                  <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                    {{ t('usage.avgDuration') }}
                  </p>
                  <p class="text-xl font-bold text-gray-900 dark:text-white">
                    {{ formatDuration(usageStats?.average_duration_ms || 0) }}
                  </p>
                  <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('usage.perRequest') }}</p>
                </div>
              </div>
            </div>
          </div>
        </template>

        <template #filters>
          <div class="card">
            <div class="px-6 py-4">
              <div class="flex flex-wrap items-end gap-4">
                <div class="min-w-[280px] flex-1">
                  <label class="input-label">{{ t('keyUsage.apiKeyLabel') }}</label>
                  <div class="relative">
                    <Icon
                      name="key"
                      size="sm"
                      class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 dark:text-dark-400"
                    />
                    <input
                      v-model="apiKey"
                      :type="keyVisible ? 'text' : 'password'"
                      class="input pl-10 pr-11"
                      :placeholder="t('keyUsage.placeholder')"
                      autocomplete="off"
                      spellcheck="false"
                      @keydown.enter="applyFilters"
                    />
                    <button
                      type="button"
                      class="absolute right-2 top-1/2 -translate-y-1/2 rounded-md p-1.5 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-700 dark:hover:text-white"
                      :title="keyVisible ? t('keyUsage.hideKey') : t('keyUsage.showKey')"
                      @click="keyVisible = !keyVisible"
                    >
                      <Icon :name="keyVisible ? 'eyeOff' : 'eye'" size="sm" />
                    </button>
                  </div>
                </div>

                <div>
                  <label class="input-label">{{ t('usage.timeRange') }}</label>
                  <DateRangePicker
                    v-model:start-date="startDate"
                    v-model:end-date="endDate"
                    @change="onDateRangeChange"
                  />
                </div>

                <div class="ml-auto flex items-center gap-3">
                  <button type="button" class="btn btn-secondary" :disabled="loading" @click="applyFilters">
                    <Icon name="search" size="sm" />
                    {{ loading ? t('keyUsage.querying') : t('keyUsage.query') }}
                  </button>
                  <button type="button" class="btn btn-secondary" @click="resetFilters">
                    <Icon name="refresh" size="sm" />
                    {{ t('common.reset') }}
                  </button>
                </div>
              </div>
              <p class="mt-3 text-xs text-gray-500 dark:text-gray-400">
                {{ t('keyUsage.privacyNote') }}
              </p>
            </div>
          </div>
        </template>

        <template #table>
          <UsageMetricTrendChart :trend-data="usageTrend" :loading="trendLoading" />

          <DataTable
            :columns="columns"
            :data="usageLogs"
            :loading="loading"
            :server-side-sort="true"
            :virtual-scroll="false"
            default-sort-key="created_at"
            default-sort-order="desc"
            @sort="handleSort"
          >
            <template #cell-model="{ value }">
              <span class="font-medium text-gray-900 dark:text-white">{{ value }}</span>
            </template>

            <template #cell-reasoning_effort="{ row }">
              <span class="text-sm text-gray-900 dark:text-white">
                {{ formatReasoningEffort(row.reasoning_effort) }}
              </span>
            </template>

            <template #cell-endpoint="{ row }">
              <span class="block max-w-[320px] whitespace-normal break-all text-sm text-gray-600 dark:text-gray-300">
                {{ formatUsageEndpoints(row) }}
              </span>
            </template>

            <template #cell-stream="{ row }">
              <span
                class="inline-flex items-center rounded px-2 py-0.5 text-xs font-medium"
                :class="getRequestTypeBadgeClass(row)"
              >
                {{ getRequestTypeLabel(row) }}
              </span>
            </template>

            <template #cell-billing_mode="{ row }">
              <span
                class="inline-flex items-center rounded px-1.5 py-0.5 text-xs font-medium"
                :class="getBillingModeBadgeClass(row.billing_mode)"
              >
                {{ getBillingModeLabel(row.billing_mode, t) }}
              </span>
            </template>

            <template #cell-tokens="{ row }">
              <div v-if="row.image_count > 0 && row.billing_mode === 'image'" class="flex items-center gap-1.5">
                <div class="min-w-0 space-y-0.5">
                  <div class="font-medium text-gray-900 dark:text-white">
                    {{ row.image_count }}{{ t('usage.imageUnit') }}
                  </div>
                  <div class="flex flex-wrap items-center gap-x-2 gap-y-0.5 text-xs text-gray-500 dark:text-gray-400">
                    <span>{{ t('usage.imageSize') }}: {{ formatImageMeta(row.image_size) }}</span>
                    <span>{{ t('usage.imageQuality') }}: {{ formatImageMeta(row.image_quality) }}</span>
                  </div>
                </div>
              </div>
              <div v-else class="flex items-center gap-1.5">
                <div class="space-y-1.5 text-sm">
                  <div class="flex items-center gap-2">
                    <div class="inline-flex items-center gap-1">
                      <Icon name="arrowDown" size="sm" class="text-emerald-500" />
                      <span class="font-medium text-gray-900 dark:text-white">
                        {{ row.input_tokens.toLocaleString() }}
                      </span>
                    </div>
                    <div class="inline-flex items-center gap-1">
                      <Icon name="arrowUp" size="sm" class="text-violet-500" />
                      <span class="font-medium text-gray-900 dark:text-white">
                        {{ row.output_tokens.toLocaleString() }}
                      </span>
                    </div>
                  </div>
                  <div v-if="row.cache_read_tokens > 0 || row.cache_creation_tokens > 0" class="flex items-center gap-2">
                    <div v-if="row.cache_read_tokens > 0" class="inline-flex items-center gap-1">
                      <Icon name="inbox" size="sm" class="text-sky-500" />
                      <span class="font-medium text-sky-600 dark:text-sky-400">
                        {{ formatCacheTokens(row.cache_read_tokens) }}
                      </span>
                    </div>
                    <div v-if="row.cache_creation_tokens > 0" class="inline-flex items-center gap-1">
                      <svg class="h-3.5 w-3.5 text-amber-500" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" /></svg>
                      <span class="font-medium text-amber-600 dark:text-amber-400">
                        {{ formatCacheTokens(row.cache_creation_tokens) }}
                      </span>
                      <span v-if="row.cache_creation_1h_tokens > 0" class="inline-flex items-center rounded px-1 py-px text-[10px] font-medium leading-tight bg-orange-100 text-orange-600 ring-1 ring-inset ring-orange-200 dark:bg-orange-500/20 dark:text-orange-400 dark:ring-orange-500/30">1h</span>
                    </div>
                  </div>
                </div>
                <div
                  class="group relative"
                  @mouseenter="showTokenTooltip($event, row)"
                  @mouseleave="hideTokenTooltip"
                >
                  <div
                    class="flex h-4 w-4 cursor-help items-center justify-center rounded-full bg-gray-100 transition-colors group-hover:bg-blue-100 dark:bg-gray-700 dark:group-hover:bg-blue-900/50"
                  >
                    <Icon name="infoCircle" size="xs" class="text-gray-400 group-hover:text-blue-500 dark:text-gray-500 dark:group-hover:text-blue-400" />
                  </div>
                </div>
              </div>
            </template>

            <template #cell-cost="{ row }">
              <div class="flex items-center gap-1.5 text-sm">
                <span class="font-medium text-green-600 dark:text-green-400">
                  ${{ row.actual_cost.toFixed(6) }}
                </span>
                <div
                  class="group relative"
                  @mouseenter="showTooltip($event, row)"
                  @mouseleave="hideTooltip"
                >
                  <div
                    class="flex h-4 w-4 cursor-help items-center justify-center rounded-full bg-gray-100 transition-colors group-hover:bg-blue-100 dark:bg-gray-700 dark:group-hover:bg-blue-900/50"
                  >
                    <Icon name="infoCircle" size="xs" class="text-gray-400 group-hover:text-blue-500 dark:text-gray-500 dark:group-hover:text-blue-400" />
                  </div>
                </div>
              </div>
            </template>

            <template #cell-first_token="{ row }">
              <span v-if="row.first_token_ms != null" class="text-sm text-gray-600 dark:text-gray-400">
                {{ formatDuration(row.first_token_ms) }}
              </span>
              <span v-else class="text-sm text-gray-400 dark:text-gray-500">-</span>
            </template>

            <template #cell-duration="{ row }">
              <span class="text-sm text-gray-600 dark:text-gray-400">
                {{ formatDuration(row.duration_ms) }}
              </span>
            </template>

            <template #cell-created_at="{ value }">
              <span class="text-sm text-gray-600 dark:text-gray-400">
                {{ formatDateTime(value) }}
              </span>
            </template>

            <template #cell-user_agent="{ row }">
              <span
                v-if="row.user_agent"
                class="block max-w-[320px] whitespace-normal break-all text-sm text-gray-600 dark:text-gray-400"
                :title="row.user_agent"
              >
                {{ row.user_agent }}
              </span>
              <span v-else class="text-sm text-gray-400 dark:text-gray-500">-</span>
            </template>

            <template #empty>
              <EmptyState :message="queried ? t('usage.noRecords') : t('keyUsage.emptyBeforeQuery')" />
            </template>
          </DataTable>
        </template>

        <template #pagination>
          <Pagination
            v-if="pagination.total > 0"
            :page="pagination.page"
            :total="pagination.total"
            :page-size="pagination.page_size"
            @update:page="handlePageChange"
            @update:pageSize="handlePageSizeChange"
          />
        </template>
      </TablePageLayout>
    </main>
  </div>

  <Teleport to="body">
    <div
      v-if="tokenTooltipVisible"
      class="pointer-events-none fixed z-[9999] -translate-y-1/2"
      :style="{ left: tokenTooltipPosition.x + 'px', top: tokenTooltipPosition.y + 'px' }"
    >
      <div class="whitespace-nowrap rounded-lg border border-gray-700 bg-gray-900 px-3 py-2.5 text-xs text-white shadow-xl dark:border-gray-600 dark:bg-gray-800">
        <div class="space-y-1.5">
          <div class="text-xs font-semibold text-gray-300">{{ t('usage.tokenDetails') }}</div>
          <div v-if="tokenTooltipData && tokenTooltipData.input_tokens > 0" class="flex items-center justify-between gap-4">
            <span class="text-gray-400">{{ t('admin.usage.inputTokens') }}</span>
            <span class="font-medium text-white">{{ tokenTooltipData.input_tokens.toLocaleString() }}</span>
          </div>
          <div v-if="tokenTooltipData && tokenTooltipData.output_tokens > 0" class="flex items-center justify-between gap-4">
            <span class="text-gray-400">{{ t('admin.usage.outputTokens') }}</span>
            <span class="font-medium text-white">{{ tokenTooltipData.output_tokens.toLocaleString() }}</span>
          </div>
          <div v-if="tokenTooltipData && tokenTooltipData.cache_creation_tokens > 0" class="flex items-center justify-between gap-4">
            <span class="text-gray-400">{{ t('admin.usage.cacheCreationTokens') }}</span>
            <span class="font-medium text-white">{{ tokenTooltipData.cache_creation_tokens.toLocaleString() }}</span>
          </div>
          <div v-if="tokenTooltipData && tokenTooltipData.cache_read_tokens > 0" class="flex items-center justify-between gap-4">
            <span class="text-gray-400">{{ t('admin.usage.cacheReadTokens') }}</span>
            <span class="font-medium text-white">{{ tokenTooltipData.cache_read_tokens.toLocaleString() }}</span>
          </div>
          <div class="flex items-center justify-between gap-6 border-t border-gray-700 pt-1.5">
            <span class="text-gray-400">{{ t('usage.totalTokens') }}</span>
            <span class="font-semibold text-blue-400">{{ visibleTokenTooltipTotal.toLocaleString() }}</span>
          </div>
        </div>
        <div class="absolute right-full top-1/2 h-0 w-0 -translate-y-1/2 border-b-[6px] border-r-[6px] border-t-[6px] border-b-transparent border-r-gray-900 border-t-transparent dark:border-r-gray-800"></div>
      </div>
    </div>
  </Teleport>

  <Teleport to="body">
    <div
      v-if="tooltipVisible"
      class="pointer-events-none fixed z-[9999] -translate-y-1/2"
      :style="{ left: tooltipPosition.x + 'px', top: tooltipPosition.y + 'px' }"
    >
      <div class="whitespace-nowrap rounded-lg border border-gray-700 bg-gray-900 px-3 py-2.5 text-xs text-white shadow-xl dark:border-gray-600 dark:bg-gray-800">
        <div class="space-y-1.5">
          <div class="mb-2 border-b border-gray-700 pb-1.5">
            <div class="mb-1 text-xs font-semibold text-gray-300">{{ t('usage.costDetails') }}</div>
            <div v-if="tooltipData && tooltipData.input_cost > 0" class="flex items-center justify-between gap-4">
              <span class="text-gray-400">{{ t('admin.usage.inputCost') }}</span>
              <span class="font-medium text-white">${{ tooltipData.input_cost.toFixed(6) }}</span>
            </div>
            <div v-if="tooltipData && tooltipData.output_cost > 0" class="flex items-center justify-between gap-4">
              <span class="text-gray-400">{{ t('admin.usage.outputCost') }}</span>
              <span class="font-medium text-white">${{ tooltipData.output_cost.toFixed(6) }}</span>
            </div>
            <template v-if="!tooltipData?.billing_mode || tooltipData.billing_mode === 'token'">
              <div v-if="tooltipData && tooltipData.input_tokens > 0" class="flex items-center justify-between gap-4">
                <span class="text-gray-400">{{ t('usage.inputTokenPrice') }}</span>
                <span class="font-medium text-sky-300">{{ formatTokenPricePerMillion(tooltipData.input_cost, tooltipData.input_tokens) }} {{ t('usage.perMillionTokens') }}</span>
              </div>
              <div v-if="tooltipData && tooltipData.output_tokens > 0" class="flex items-center justify-between gap-4">
                <span class="text-gray-400">{{ t('usage.outputTokenPrice') }}</span>
                <span class="font-medium text-violet-300">{{ formatTokenPricePerMillion(tooltipData.output_cost, tooltipData.output_tokens) }} {{ t('usage.perMillionTokens') }}</span>
              </div>
            </template>
            <div v-else class="flex items-center justify-between gap-4">
              <span class="text-gray-400">{{ tooltipData.billing_mode === 'image' ? t('usage.imageUnitPrice') : t('usage.unitPrice') }}</span>
              <span class="font-medium text-sky-300">${{ tooltipData.total_cost?.toFixed(6) || '0.000000' }}</span>
            </div>
            <div v-if="tooltipData && tooltipData.cache_creation_cost > 0" class="flex items-center justify-between gap-4">
              <span class="text-gray-400">{{ t('admin.usage.cacheCreationCost') }}</span>
              <span class="font-medium text-white">${{ tooltipData.cache_creation_cost.toFixed(6) }}</span>
            </div>
            <div v-if="tooltipData && tooltipData.cache_read_cost > 0" class="flex items-center justify-between gap-4">
              <span class="text-gray-400">{{ t('admin.usage.cacheReadCost') }}</span>
              <span class="font-medium text-white">${{ tooltipData.cache_read_cost.toFixed(6) }}</span>
            </div>
          </div>
          <div class="flex items-center justify-between gap-6">
            <span class="text-gray-400">{{ t('usage.serviceTier') }}</span>
            <span class="font-semibold text-cyan-300">{{ getUsageServiceTierLabel(tooltipData?.service_tier, t) }}</span>
          </div>
          <div class="flex items-center justify-between gap-6">
            <span class="text-gray-400">{{ t('usage.rate') }}</span>
            <span class="font-semibold text-blue-400">{{ formatMultiplier(tooltipData?.rate_multiplier || 1) }}x</span>
          </div>
          <div class="flex items-center justify-between gap-6">
            <span class="text-gray-400">{{ t('usage.original') }}</span>
            <span class="font-medium text-white">${{ tooltipData?.total_cost.toFixed(6) }}</span>
          </div>
          <div class="flex items-center justify-between gap-6 border-t border-gray-700 pt-1.5">
            <span class="text-gray-400">{{ t('usage.billed') }}</span>
            <span class="font-semibold text-green-400">${{ tooltipData?.actual_cost.toFixed(6) }}</span>
          </div>
        </div>
        <div class="absolute right-full top-1/2 h-0 w-0 -translate-y-1/2 border-b-[6px] border-r-[6px] border-t-[6px] border-b-transparent border-r-gray-900 border-t-transparent dark:border-r-gray-800"></div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import UsageMetricTrendChart from '@/components/user/usage/UsageMetricTrendChart.vue'
import type { UsageLog, UsageStatsResponse, TrendDataPoint, PaginatedResponse } from '@/types'
import type { Column } from '@/components/common/types'
import { formatDateTime, formatReasoningEffort } from '@/utils/format'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { formatCacheTokens, formatMultiplier } from '@/utils/formatters'
import { formatTokenPricePerMillion } from '@/utils/usagePricing'
import { getUsageServiceTierLabel } from '@/utils/usageServiceTier'
import { resolveUsageRequestType } from '@/utils/usageRequestType'
import { getBillingModeLabel, getBillingModeBadgeClass } from '@/utils/billingMode'

const { t } = useI18n()
const appStore = useAppStore()

let recordsAbortController: AbortController | null = null
let statsAbortController: AbortController | null = null
let trendAbortController: AbortController | null = null
let summaryAbortController: AbortController | null = null

const apiKey = ref('')
const keyVisible = ref(false)
const queried = ref(false)
const usageStats = ref<UsageStatsResponse | null>(null)
const usageTrend = ref<TrendDataPoint[]>([])
const usageLogs = ref<UsageLog[]>([])
const usageSummary = ref<PublicUsageSummary | null>(null)
const loading = ref(false)
const trendLoading = ref(false)
const isDark = ref(document.documentElement.classList.contains('dark'))

interface PublicUsageQuota {
  limit: number
  used: number
  remaining: number
  unit?: string
}

interface PublicUsageRateLimit {
  window: string
  limit: number
  used: number
  remaining: number
  reset_at?: string | null
  window_start?: string | null
}

interface PublicUsageSubscription {
  daily_usage_usd?: number
  weekly_usage_usd?: number
  monthly_usage_usd?: number
  daily_limit_usd?: number
  weekly_limit_usd?: number
  monthly_limit_usd?: number
  expires_at?: string | null
}

interface PublicUsageSummary {
  mode: 'quota_limited' | 'unrestricted'
  isValid?: boolean
  status?: string
  planName?: string
  unit?: string
  remaining?: number
  balance?: number
  quota?: PublicUsageQuota
  rate_limits?: PublicUsageRateLimit[]
  expires_at?: string | null
  days_until_expiry?: number
  subscription?: PublicUsageSubscription
}

const tooltipVisible = ref(false)
const tooltipPosition = ref({ x: 0, y: 0 })
const tooltipData = ref<UsageLog | null>(null)

const tokenTooltipVisible = ref(false)
const tokenTooltipPosition = ref({ x: 0, y: 0 })
const tokenTooltipData = ref<UsageLog | null>(null)
const visibleTokenTooltipTotal = computed(() => {
  const data = tokenTooltipData.value
  if (!data) return 0
  return data.input_tokens + data.output_tokens + data.cache_creation_tokens + data.cache_read_tokens
})

const formatLocalDate = (date: Date): string => {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

const now = new Date()
const weekAgo = new Date(now)
weekAgo.setDate(weekAgo.getDate() - 6)

const startDate = ref(formatLocalDate(weekAgo))
const endDate = ref(formatLocalDate(now))

const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0
})

const sortState = reactive({
  sort_by: 'created_at',
  sort_order: 'desc' as 'asc' | 'desc'
})

const columns = computed<Column[]>(() => [
  { key: 'model', label: t('usage.model'), sortable: true },
  { key: 'reasoning_effort', label: t('usage.reasoningEffort'), sortable: false },
  { key: 'endpoint', label: t('usage.endpoint'), sortable: false },
  { key: 'stream', label: t('usage.type'), sortable: false },
  { key: 'billing_mode', label: t('admin.usage.billingMode'), sortable: false },
  { key: 'tokens', label: t('usage.tokens'), sortable: false },
  { key: 'cost', label: t('usage.cost'), sortable: false },
  { key: 'first_token', label: t('usage.firstToken'), sortable: false },
  { key: 'duration', label: t('usage.duration'), sortable: false },
  { key: 'created_at', label: t('usage.time'), sortable: true },
  { key: 'user_agent', label: t('usage.userAgent'), sortable: false }
])

const firstRateLimit = computed(() => usageSummary.value?.rate_limits?.[0] || null)

const visibleBalanceAmount = computed<number | null>(() => {
  const summary = usageSummary.value
  if (!summary) return null
  if (Number.isFinite(summary.quota?.remaining)) return summary.quota!.remaining
  if (Number.isFinite(summary.balance)) return summary.balance!
  if (Number.isFinite(summary.remaining)) return summary.remaining!
  if (Number.isFinite(firstRateLimit.value?.remaining)) return firstRateLimit.value!.remaining
  return null
})

const balanceCardTitle = computed(() => {
  const summary = usageSummary.value
  if (!summary) return t('keyUsage.availableBalance')
  if (summary.quota) return t('keyUsage.remainingQuota')
  if (summary.subscription) return t('keyUsage.subscriptionRemaining')
  if (Number.isFinite(summary.balance)) return t('keyUsage.walletBalance')
  if (firstRateLimit.value) return t('keyUsage.rateWindowRemaining')
  return t('keyUsage.availableBalance')
})

const balanceCardValue = computed(() => {
  if (!queried.value) return '-'
  const amount = visibleBalanceAmount.value
  if (amount == null) return '-'
  return formatUSD(amount)
})

const balanceCardDetail = computed(() => {
  if (!queried.value) return t('keyUsage.awaitingQuery')
  const summary = usageSummary.value
  if (!summary) return t('keyUsage.balanceUnavailable')
  if (summary.quota) {
    return t('keyUsage.quotaDetail', {
      limit: formatUSD(summary.quota.limit),
      used: formatUSD(summary.quota.used)
    })
  }
  if (summary.subscription) {
    return t('keyUsage.subscriptionDetail', {
      plan: summary.planName || t('keyUsage.subscriptionType'),
      expires: summary.subscription.expires_at ? formatDateTime(summary.subscription.expires_at) : t('keyUsage.noExpiry')
    })
  }
  if (Number.isFinite(summary.balance)) {
    return summary.planName || t('keyUsage.walletBalance')
  }
  if (firstRateLimit.value) {
    return t('keyUsage.rateLimitDetail', {
      window: formatRateLimitWindow(firstRateLimit.value.window),
      limit: formatUSD(firstRateLimit.value.limit)
    })
  }
  return t('keyUsage.balanceUnavailable')
})

const timezone = (): string => {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone
  } catch {
    return 'UTC'
  }
}

const getTrendGranularity = (): 'day' | 'hour' => {
  const start = new Date(`${startDate.value}T00:00:00`)
  const end = new Date(`${endDate.value}T00:00:00`)
  if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime())) return 'day'
  const days = Math.floor((end.getTime() - start.getTime()) / 86_400_000) + 1
  return days <= 2 ? 'hour' : 'day'
}

const queryParams = (extra: Record<string, unknown> = {}) => {
  const params = new URLSearchParams({
    start_date: startDate.value,
    end_date: endDate.value,
    timezone: timezone()
  })
  Object.entries(extra).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== '') {
      params.set(key, String(value))
    }
  })
  return params
}

async function publicUsageFetch<T>(path: string, params: URLSearchParams, signal?: AbortSignal): Promise<T> {
  const key = apiKey.value.trim()
  const response = await fetch(`${path}?${params.toString()}`, {
    signal,
    headers: {
      Authorization: `Bearer ${key}`
    }
  })
  const body = await response.json().catch(() => null)
  if (!response.ok) {
    const message = body?.error?.message || body?.message || `${t('keyUsage.queryFailed')} (${response.status})`
    throw new Error(message)
  }
  if (body && typeof body === 'object' && 'code' in body) {
    if (body.code === 0) return body.data as T
    throw new Error(body.message || t('keyUsage.queryFailedRetry'))
  }
  return body as T
}

const loadUsageSummary = async () => {
  if (!apiKey.value.trim()) return
  summaryAbortController?.abort()
  const current = new AbortController()
  summaryAbortController = current
  try {
    const summary = await publicUsageFetch<PublicUsageSummary>(
      '/v1/usage',
      queryParams(),
      current.signal
    )
    if (current.signal.aborted) return
    usageSummary.value = summary
  } catch {
    if (current.signal.aborted) return
    usageSummary.value = null
  }
}

const buildRecordsParams = () =>
  queryParams({
    page: pagination.page,
    page_size: pagination.page_size,
    sort_by: sortState.sort_by,
    sort_order: sortState.sort_order
  })

const loadUsageLogs = async () => {
  if (!apiKey.value.trim()) return
  recordsAbortController?.abort()
  tooltipVisible.value = false
  tooltipData.value = null
  tokenTooltipVisible.value = false
  tokenTooltipData.value = null

  const current = new AbortController()
  recordsAbortController = current
  loading.value = true
  try {
    const response = await publicUsageFetch<PaginatedResponse<UsageLog>>(
      '/v1/usage/records',
      buildRecordsParams(),
      current.signal
    )
    if (current.signal.aborted) return
    usageLogs.value = response.items
    pagination.total = response.total
    pagination.pages = response.pages
  } catch (error) {
    if (current.signal.aborted) return
    appStore.showError((error as Error).message || t('usage.failedToLoad'))
  } finally {
    if (recordsAbortController === current) {
      loading.value = false
    }
  }
}

const loadUsageStats = async () => {
  if (!apiKey.value.trim()) return
  statsAbortController?.abort()
  const current = new AbortController()
  statsAbortController = current
  try {
    const stats = await publicUsageFetch<UsageStatsResponse>(
      '/v1/usage/stats',
      queryParams(),
      current.signal
    )
    if (current.signal.aborted) return
    usageStats.value = stats
  } catch (error) {
    if (current.signal.aborted) return
    appStore.showError((error as Error).message || t('keyUsage.queryFailedRetry'))
  }
}

const loadUsageTrend = async () => {
  if (!apiKey.value.trim()) return
  trendAbortController?.abort()
  const current = new AbortController()
  trendAbortController = current
  trendLoading.value = true
  try {
    const response = await publicUsageFetch<{ trend: TrendDataPoint[] }>(
      '/v1/usage/trend',
      queryParams({ granularity: getTrendGranularity() }),
      current.signal
    )
    if (current.signal.aborted) return
    usageTrend.value = response.trend || []
  } catch (error) {
    if (current.signal.aborted) return
    usageTrend.value = []
  } finally {
    if (trendAbortController === current) {
      trendLoading.value = false
    }
  }
}

const loadAll = async () => {
  await Promise.all([loadUsageSummary(), loadUsageLogs(), loadUsageStats(), loadUsageTrend()])
}

const applyFilters = () => {
  if (!apiKey.value.trim()) {
    appStore.showInfo(t('keyUsage.enterApiKey'))
    return
  }
  queried.value = true
  pagination.page = 1
  loadAll()
}

const onDateRangeChange = (range: { startDate: string; endDate: string; preset: string | null }) => {
  startDate.value = range.startDate
  endDate.value = range.endDate
  if (queried.value) applyFilters()
}

const resetFilters = () => {
  const nextNow = new Date()
  const nextWeekAgo = new Date(nextNow)
  nextWeekAgo.setDate(nextWeekAgo.getDate() - 6)
  startDate.value = formatLocalDate(nextWeekAgo)
  endDate.value = formatLocalDate(nextNow)
  pagination.page = 1
  if (queried.value) loadAll()
}

const handlePageChange = (page: number) => {
  pagination.page = page
  loadUsageLogs()
}

const handlePageSizeChange = (pageSize: number) => {
  pagination.page_size = pageSize
  pagination.page = 1
  loadUsageLogs()
}

const handleSort = (key: string, order: 'asc' | 'desc') => {
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  loadUsageLogs()
}

const toggleTheme = () => {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

const formatDuration = (ms: number | null | undefined): string => {
  if (ms == null) return '-'
  if (ms < 1000) return `${ms.toFixed(0)}ms`
  return `${(ms / 1000).toFixed(2)}s`
}

const formatTokens = (value: number): string => {
  if (value >= 1_000_000_000) return `${(value / 1_000_000_000).toFixed(2)}B`
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(2)}M`
  if (value >= 1_000) return `${(value / 1_000).toFixed(2)}K`
  return value.toLocaleString()
}

const formatUSD = (value: number): string => {
  if (!Number.isFinite(value)) return '-'
  return `$${value.toFixed(4)}`
}

const formatRateLimitWindow = (window: string): string => {
  if (window === '5h') return t('keyUsage.limit5h')
  if (window === '1d') return t('keyUsage.limitDaily')
  if (window === '7d') return t('keyUsage.limit7d')
  return window
}

const formatImageMeta = (value: string | null | undefined): string => {
  return value?.trim() || '-'
}

const formatUsageEndpoints = (log: UsageLog): string => {
  const inbound = log.inbound_endpoint?.trim()
  return inbound || '-'
}

const getRequestTypeLabel = (log: UsageLog): string => {
  const requestType = resolveUsageRequestType(log)
  if (requestType === 'ws_v2') return t('usage.ws')
  if (requestType === 'stream') return t('usage.stream')
  if (requestType === 'sync') return t('usage.sync')
  return t('usage.unknown')
}

const getRequestTypeBadgeClass = (log: UsageLog): string => {
  const requestType = resolveUsageRequestType(log)
  if (requestType === 'ws_v2') return 'bg-violet-100 text-violet-800 dark:bg-violet-900 dark:text-violet-200'
  if (requestType === 'stream') return 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200'
  if (requestType === 'sync') return 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200'
  return 'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200'
}

const showTooltip = (event: MouseEvent, row: UsageLog) => {
  const target = event.currentTarget as HTMLElement
  const rect = target.getBoundingClientRect()
  tooltipData.value = row
  tooltipPosition.value.x = rect.right + 8
  tooltipPosition.value.y = rect.top + rect.height / 2
  tooltipVisible.value = true
}

const hideTooltip = () => {
  tooltipVisible.value = false
  tooltipData.value = null
}

const showTokenTooltip = (event: MouseEvent, row: UsageLog) => {
  const target = event.currentTarget as HTMLElement
  const rect = target.getBoundingClientRect()
  tokenTooltipData.value = row
  tokenTooltipPosition.value.x = rect.right + 8
  tokenTooltipPosition.value.y = rect.top + rect.height / 2
  tokenTooltipVisible.value = true
}

const hideTokenTooltip = () => {
  tokenTooltipVisible.value = false
  tokenTooltipData.value = null
}

onBeforeUnmount(() => {
  recordsAbortController?.abort()
  statsAbortController?.abort()
  trendAbortController?.abort()
  summaryAbortController?.abort()
  hideTooltip()
  hideTokenTooltip()
})
</script>
