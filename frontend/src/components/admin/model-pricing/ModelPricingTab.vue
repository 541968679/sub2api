<template>
  <div class="space-y-4">
    <!-- Stats Cards -->
    <div class="grid grid-cols-3 gap-4">
      <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-gray-700 dark:bg-gray-800">
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.modelPricing.totalModels') }}</p>
        <p class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">{{ stats.total_models }}</p>
      </div>
      <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-gray-700 dark:bg-gray-800">
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.modelPricing.globalOverrides') }}</p>
        <p class="mt-1 text-2xl font-bold text-primary-600 dark:text-primary-400">{{ stats.global_override_count }}</p>
      </div>
      <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-gray-700 dark:bg-gray-800">
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.modelPricing.channelOverrides') }}</p>
        <p class="mt-1 text-2xl font-bold text-amber-600 dark:text-amber-400">{{ stats.channel_override_count }}</p>
      </div>
    </div>

    <!-- Filters -->
    <div class="flex flex-col justify-between gap-3 lg:flex-row lg:items-center">
      <div class="flex flex-1 flex-wrap items-center gap-3">
        <div class="relative w-full sm:w-64">
          <svg class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
          <input
            v-model="searchQuery"
            @input="handleSearch"
            type="text"
            class="input w-full pl-10 text-sm"
            :placeholder="t('admin.modelPricing.searchPlaceholder')"
          />
        </div>
        <select v-model="providerFilter" @change="loadData" class="input text-sm">
          <option value="">{{ t('admin.modelPricing.allProviders') }}</option>
          <option value="anthropic">Anthropic</option>
          <option value="openai">OpenAI</option>
          <option value="vertex_ai">Gemini</option>
          <option value="antigravity">Antigravity</option>
        </select>
        <select v-model="sourceFilter" @change="loadData" class="input text-sm">
          <option value="">{{ t('admin.modelPricing.allSources') }}</option>
          <option value="has_global_override">{{ t('admin.modelPricing.hasGlobalOverride') }}</option>
          <option value="has_channel_override">{{ t('admin.modelPricing.hasChannelOverride') }}</option>
          <option value="litellm_only">{{ t('admin.modelPricing.litellmOnly') }}</option>
        </select>
      </div>
      <div class="flex gap-2">
        <button @click="loadData" :disabled="loading" class="btn btn-secondary text-sm">
          <svg class="h-4 w-4" :class="loading ? 'animate-spin' : ''" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
        </button>
      </div>
    </div>

    <!-- Table -->
    <div class="overflow-x-auto rounded-lg border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800">
      <table class="w-full text-sm">
        <thead>
          <tr class="border-b border-gray-200 bg-gray-50 dark:border-gray-700 dark:bg-gray-800/50">
            <th class="px-4 py-3 text-left font-medium text-gray-500 dark:text-gray-400">{{ t('admin.modelPricing.model') }}</th>
            <th class="px-4 py-3 text-left font-medium text-gray-500 dark:text-gray-400">Provider</th>
            <th class="px-4 py-3 text-right font-medium text-gray-500 dark:text-gray-400">{{ t('admin.modelPricing.inputPrice') }}</th>
            <th class="px-4 py-3 text-right font-medium text-gray-500 dark:text-gray-400">{{ t('admin.modelPricing.outputPrice') }}</th>
            <th class="hidden px-4 py-3 text-right font-medium text-gray-500 dark:text-gray-400 lg:table-cell">{{ t('admin.modelPricing.cacheWritePrice') }}</th>
            <th class="hidden px-4 py-3 text-right font-medium text-gray-500 dark:text-gray-400 lg:table-cell">{{ t('admin.modelPricing.cacheReadPrice') }}</th>
            <th class="px-4 py-3 text-center font-medium text-gray-500 dark:text-gray-400">{{ t('admin.modelPricing.source') }}</th>
            <th class="px-4 py-3 text-center font-medium text-gray-500 dark:text-gray-400">{{ t('admin.modelPricing.channels') }}</th>
            <th class="px-4 py-3 text-right font-medium text-gray-500 dark:text-gray-400">{{ t('admin.modelPricing.actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading" class="border-b">
            <td colspan="9" class="py-12 text-center">
              <span class="inline-block h-5 w-5 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></span>
            </td>
          </tr>
          <tr v-else-if="items.length === 0" class="border-b">
            <td colspan="9" class="py-12 text-center text-gray-400">{{ t('admin.modelPricing.noModels') }}</td>
          </tr>
          <tr
            v-for="item in items"
            :key="item.model"
            class="border-b border-gray-100 last:border-0 hover:bg-gray-50 dark:border-gray-700/50 dark:hover:bg-gray-700/30"
          >
            <td class="px-4 py-2.5">
              <span class="font-mono text-xs text-gray-900 dark:text-white">{{ item.model }}</span>
            </td>
            <td class="px-4 py-2.5">
              <span class="inline-block rounded bg-gray-100 px-1.5 py-0.5 text-xs text-gray-600 dark:bg-gray-700 dark:text-gray-300">
                {{ item.provider || '-' }}
              </span>
            </td>
            <td class="px-4 py-2.5 text-right font-mono text-xs">
              <span :class="item.global_override ? 'text-primary-600 dark:text-primary-400 font-semibold' : 'text-gray-600 dark:text-gray-400'">
                {{ formatPrice(item, 'input') }}
              </span>
            </td>
            <td class="px-4 py-2.5 text-right font-mono text-xs">
              <span :class="item.global_override ? 'text-primary-600 dark:text-primary-400 font-semibold' : 'text-gray-600 dark:text-gray-400'">
                {{ formatPrice(item, 'output') }}
              </span>
            </td>
            <td class="hidden px-4 py-2.5 text-right font-mono text-xs text-gray-500 lg:table-cell">
              {{ formatPrice(item, 'cache_write') }}
            </td>
            <td class="hidden px-4 py-2.5 text-right font-mono text-xs text-gray-500 lg:table-cell">
              {{ formatPrice(item, 'cache_read') }}
            </td>
            <td class="px-4 py-2.5 text-center">
              <span :class="sourceBadgeClass(item.effective_source)" class="inline-block rounded-full px-2 py-0.5 text-xs font-medium">
                {{ item.effective_source }}
              </span>
            </td>
            <td class="px-4 py-2.5 text-center">
              <span v-if="item.channel_override_count > 0" class="inline-block rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-700 dark:bg-amber-900/30 dark:text-amber-400">
                {{ item.channel_override_count }}
              </span>
              <span v-else class="text-xs text-gray-300">-</span>
            </td>
            <td class="px-4 py-2.5 text-right">
              <button
                @click="openDetail(item.model)"
                class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-gray-700 dark:hover:text-gray-300"
                :title="t('admin.modelPricing.viewDetail')"
              >
                <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M16.862 4.487l1.687-1.688a1.875 1.875 0 112.652 2.652L10.582 16.07a4.5 4.5 0 01-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 011.13-1.897l8.932-8.931zm0 0L19.5 7.125M18 14v4.75A2.25 2.25 0 0115.75 21H5.25A2.25 2.25 0 013 18.75V8.25A2.25 2.25 0 015.25 6H10" />
                </svg>
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Pagination -->
    <div v-if="pagination.pages > 1" class="flex items-center justify-between">
      <p class="text-xs text-gray-500">
        {{ t('admin.modelPricing.showing', { from: (pagination.page - 1) * pagination.page_size + 1, to: Math.min(pagination.page * pagination.page_size, pagination.total), total: pagination.total }) }}
      </p>
      <div class="flex gap-1">
        <button
          @click="goToPage(pagination.page - 1)"
          :disabled="pagination.page <= 1"
          class="btn btn-secondary px-2 py-1 text-xs"
        >
          &laquo;
        </button>
        <button
          v-for="p in visiblePages"
          :key="p"
          @click="goToPage(p)"
          class="px-2.5 py-1 text-xs rounded"
          :class="p === pagination.page ? 'bg-primary-600 text-white' : 'btn btn-secondary'"
        >
          {{ p }}
        </button>
        <button
          @click="goToPage(pagination.page + 1)"
          :disabled="pagination.page >= pagination.pages"
          class="btn btn-secondary px-2 py-1 text-xs"
        >
          &raquo;
        </button>
      </div>
    </div>

    <!-- Detail Dialog -->
    <ModelPricingDetailDialog
      :show="showDetailDialog"
      :model="selectedModel"
      @close="showDetailDialog = false"
      @saved="loadData"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { ModelPricingItem, ModelPricingStats } from '@/api/admin/modelPricing'
import { perTokenToMTok } from '@/components/admin/channel/types'
import ModelPricingDetailDialog from './ModelPricingDetailDialog.vue'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const items = ref<ModelPricingItem[]>([])
const stats = reactive<ModelPricingStats>({ total_models: 0, global_override_count: 0, channel_override_count: 0 })
const pagination = reactive({ page: 1, page_size: 50, total: 0, pages: 0 })

const searchQuery = ref('')
const providerFilter = ref('')
const sourceFilter = ref('')

const showDetailDialog = ref(false)
const selectedModel = ref('')

let searchTimeout: ReturnType<typeof setTimeout>

function handleSearch() {
  clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => {
    pagination.page = 1
    loadData()
  }, 300)
}

async function loadData() {
  loading.value = true
  try {
    const result = await adminAPI.modelPricing.list(pagination.page, pagination.page_size, {
      search: searchQuery.value || undefined,
      provider: providerFilter.value || undefined,
      source: sourceFilter.value || undefined,
    })
    items.value = result.items || []
    stats.total_models = result.stats.total_models
    stats.global_override_count = result.stats.global_override_count
    stats.channel_override_count = result.stats.channel_override_count
    pagination.total = result.pagination.total
    pagination.pages = result.pagination.pages
  } catch {
    appStore.showError(t('common.error'))
  } finally {
    loading.value = false
  }
}

function formatPrice(item: ModelPricingItem, type: 'input' | 'output' | 'cache_write' | 'cache_read'): string {
  // Prefer global override price, then litellm
  const go = item.global_override
  if (go && go.enabled) {
    const key = `${type}_price` as keyof typeof go
    const val = go[key]
    if (typeof val === 'number') {
      const mtok = perTokenToMTok(val)
      return mtok !== null ? `$${mtok}` : '-'
    }
  }
  if (item.litellm_prices) {
    const key = `${type}_price` as keyof typeof item.litellm_prices
    const val = item.litellm_prices[key]
    if (typeof val === 'number' && val > 0) {
      const mtok = perTokenToMTok(val)
      return mtok !== null ? `$${mtok}` : '-'
    }
  }
  return '-'
}

function sourceBadgeClass(source: string): string {
  switch (source) {
    case 'global':
      return 'bg-primary-100 text-primary-700 dark:bg-primary-900/30 dark:text-primary-400'
    case 'litellm':
      return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
    case 'channel':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400'
    default:
      return 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400'
  }
}

function openDetail(model: string) {
  selectedModel.value = model
  showDetailDialog.value = true
}

function goToPage(page: number) {
  if (page < 1 || page > pagination.pages) return
  pagination.page = page
  loadData()
}

const visiblePages = computed(() => {
  const current = pagination.page
  const total = pagination.pages
  const pages: number[] = []
  const start = Math.max(1, current - 2)
  const end = Math.min(total, current + 2)
  for (let i = start; i <= end; i++) pages.push(i)
  return pages
})

onMounted(loadData)
</script>
