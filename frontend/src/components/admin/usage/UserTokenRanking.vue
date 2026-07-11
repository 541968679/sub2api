<template>
  <div class="card overflow-hidden">
    <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 px-4 py-3 dark:border-dark-700 sm:px-6">
      <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.usage.tokenRanking.subtitle') }}</p>
      <div class="w-28"><Select v-model="limit" :options="limitOptions" @change="load" /></div>
    </div>
    <div class="overflow-x-auto">
      <table class="w-full min-w-max divide-y divide-gray-200 text-sm dark:divide-dark-700">
        <thead class="bg-gray-50 dark:bg-dark-800">
          <tr>
            <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 sm:px-6">#</th>
            <th class="px-4 py-3 text-left text-xs font-medium text-gray-500">{{ t('admin.usage.tokenRanking.columns.user') }}</th>
            <th v-for="column in columns" :key="column.key" class="cursor-pointer whitespace-nowrap px-4 py-3 text-right text-xs font-medium text-gray-500 hover:text-primary-500" @click="setSort(column.key)">
              {{ t(column.label) }}<span v-if="sortBy === column.key" class="ml-1">↓</span>
            </th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-700 dark:bg-dark-900">
          <tr v-if="loading"><td :colspan="columns.length + 2" class="py-10 text-center"><LoadingSpinner /></td></tr>
          <tr v-else-if="items.length === 0"><td :colspan="columns.length + 2" class="py-10 text-center text-gray-400">{{ t('admin.dashboard.noDataAvailable') }}</td></tr>
          <tr v-for="(item, index) in items" v-else :key="item.user_id" class="cursor-pointer hover:bg-gray-50 dark:hover:bg-dark-700/40" @click="$emit('select-user', item.user_id, item.email)">
            <td class="px-4 py-3 text-gray-400 sm:px-6">{{ index + 1 }}</td>
            <td class="max-w-[260px] truncate px-4 py-3 font-medium text-gray-700 dark:text-gray-200" :title="item.email">{{ item.email || `User #${item.user_id}` }}</td>
            <td class="px-4 py-3 text-right tabular-nums">{{ item.requests.toLocaleString() }}</td>
            <td class="px-4 py-3 text-right tabular-nums">{{ formatCompactNumber(item.input_tokens) }}</td>
            <td class="px-4 py-3 text-right tabular-nums">{{ formatCompactNumber(item.output_tokens) }}</td>
            <td class="px-4 py-3 text-right tabular-nums">{{ formatCompactNumber(item.cache_creation_tokens) }}</td>
            <td class="px-4 py-3 text-right tabular-nums">{{ formatCompactNumber(item.cache_read_tokens) }}</td>
            <td class="px-4 py-3 text-right font-medium tabular-nums">{{ formatCompactNumber(item.total_tokens) }}</td>
            <td class="px-4 py-3 text-right font-medium tabular-nums text-green-600 dark:text-green-400">${{ formatCostFixed(item.actual_cost, 4) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getUserBreakdown, type UserBreakdownParams } from '@/api/admin/dashboard'
import Select from '@/components/common/Select.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { formatCompactNumber, formatCostFixed } from '@/utils/format'
import type { UserBreakdownItem } from '@/types'

const props = defineProps<{ startDate: string; endDate: string; filters: Record<string, unknown>; model?: string }>()
defineEmits<{ (event: 'select-user', userId: number, email: string): void }>()
const { t } = useI18n()
type SortKey = NonNullable<UserBreakdownParams['sort_by']>
const columns: Array<{ key: SortKey; label: string }> = [
  { key: 'requests', label: 'admin.usage.tokenRanking.columns.requests' },
  { key: 'input_tokens', label: 'admin.usage.tokenRanking.columns.inputTokens' },
  { key: 'output_tokens', label: 'admin.usage.tokenRanking.columns.outputTokens' },
  { key: 'cache_creation_tokens', label: 'admin.usage.tokenRanking.columns.cacheCreationTokens' },
  { key: 'cache_read_tokens', label: 'admin.usage.tokenRanking.columns.cacheReadTokens' },
  { key: 'total_tokens', label: 'admin.usage.tokenRanking.columns.totalTokens' },
  { key: 'actual_cost', label: 'admin.usage.tokenRanking.columns.actualCost' },
]
const limitOptions = [20, 50, 100, 200].map((value) => ({ value, label: `Top ${value}` }))
const items = ref<UserBreakdownItem[]>([])
const limit = ref(50)
const sortBy = ref<SortKey>('total_tokens')
const loading = ref(false)
let requestSequence = 0

const load = async () => {
  const sequence = ++requestSequence
  loading.value = true
  try {
    const params: UserBreakdownParams = {
      ...props.filters,
      start_date: props.startDate,
      end_date: props.endDate,
      model: props.model,
      sort_by: sortBy.value,
      limit: limit.value,
    }
    const response = await getUserBreakdown(params)
    if (sequence === requestSequence) items.value = response.users || []
  } catch {
    if (sequence === requestSequence) items.value = []
  } finally {
    if (sequence === requestSequence) loading.value = false
  }
}
const setSort = (key: SortKey) => { if (sortBy.value !== key) { sortBy.value = key; void load() } }
watch(() => [props.startDate, props.endDate, props.model, JSON.stringify(props.filters)], load, { immediate: true })
defineExpose({ reload: load })
</script>
