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

    <!-- Billing Basis Explainer (collapsible) -->
    <details class="rounded-lg border border-gray-200 bg-gray-50/50 px-4 py-2 text-sm dark:border-gray-700 dark:bg-gray-800/50">
      <summary class="cursor-pointer select-none text-gray-600 hover:text-gray-900 dark:text-gray-300 dark:hover:text-white">
        <svg class="mr-1 inline-block h-3.5 w-3.5 -translate-y-px" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        {{ t('admin.modelPricing.billingBasisTitle') }}
      </summary>
      <div class="mt-2 space-y-1 pl-5 text-xs text-gray-500 dark:text-gray-400">
        <p>{{ t('admin.modelPricing.billingBasisIntro') }}</p>
        <ul class="ml-3 list-disc space-y-0.5">
          <li>{{ t('admin.modelPricing.billingBasisRequested') }}</li>
          <li>{{ t('admin.modelPricing.billingBasisUpstream') }}</li>
          <li>{{ t('admin.modelPricing.billingBasisChannelMapped') }}</li>
        </ul>
        <p class="text-gray-400 dark:text-gray-500">{{ t('admin.modelPricing.billingBasisNoChannel') }}</p>
        <p class="mt-1 border-l-2 border-amber-300 pl-2 text-gray-500 dark:border-amber-700 dark:text-gray-400">
          {{ t('admin.modelPricing.billingBasisColumnNote') }}
        </p>
      </div>
    </details>

    <!-- Filters -->
    <div class="space-y-3">
      <!-- Row 1: 搜索 + 刷新 -->
      <div class="flex flex-wrap items-center gap-3">
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
        <div class="sm:ml-auto flex gap-2">
          <button
            ref="addMappingAnchor"
            @click="openAddMapping"
            class="btn btn-secondary text-sm"
            :title="t('admin.modelPricing.addMapping')"
          >
            <svg class="mr-1 inline-block h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4" />
            </svg>
            {{ t('admin.modelPricing.addMapping') }}
          </button>
          <button @click="loadData" :disabled="loading" class="btn btn-secondary text-sm">
            <svg class="h-4 w-4" :class="loading ? 'animate-spin' : ''" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
          </button>
        </div>
      </div>

      <!-- Row 2: Provider tabs -->
      <div class="flex flex-wrap items-end gap-x-5 gap-y-0 border-b border-gray-200 dark:border-gray-700">
        <span class="pb-3 text-xs font-medium text-gray-500 shrink-0 dark:text-gray-400">
          {{ t('admin.modelPricing.providerLabel') }}
        </span>
        <button
          v-for="tab in providerTabs"
          :key="'p-' + tab.value"
          @click="setProvider(tab.value)"
          class="-mb-px whitespace-nowrap border-b-2 px-1 pb-3 text-sm font-medium transition-colors"
          :class="providerFilter === tab.value
            ? 'border-primary-500 text-primary-600 dark:text-primary-400'
            : 'border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'"
        >
          {{ tab.label }}
        </button>
      </div>

      <!-- Row 3: Source tabs -->
      <div class="flex flex-wrap items-end gap-x-5 gap-y-0 border-b border-gray-200 dark:border-gray-700">
        <span class="pb-3 text-xs font-medium text-gray-500 shrink-0 dark:text-gray-400 inline-flex items-center gap-1">
          {{ t('admin.modelPricing.sourceLabel') }}
          <svg
            class="h-3.5 w-3.5 cursor-help text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
            :title="t('admin.modelPricing.sourceHierarchyTooltip')"
            fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"
          >
            <path stroke-linecap="round" stroke-linejoin="round" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        </span>
        <button
          v-for="tab in sourceTabs"
          :key="'s-' + tab.value"
          @click="setSource(tab.value)"
          class="-mb-px whitespace-nowrap border-b-2 px-1 pb-3 text-sm font-medium transition-colors"
          :class="sourceFilter === tab.value
            ? 'border-primary-500 text-primary-600 dark:text-primary-400'
            : 'border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'"
        >
          {{ tab.label }}
        </button>
      </div>
    </div>

    <!-- Inline edit hint -->
    <p class="text-xs text-gray-400 dark:text-gray-500">
      {{ t('admin.modelPricing.inlineEditHint') }}
    </p>

    <!-- Table -->
    <div class="overflow-x-auto rounded-lg border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800">
      <table class="w-full text-sm">
        <thead>
          <tr class="border-b border-gray-200 bg-gray-50 dark:border-gray-700 dark:bg-gray-800/50">
            <th class="px-4 py-3 text-left font-medium text-gray-500 dark:text-gray-400">{{ t('admin.modelPricing.requestedModelName') }}</th>
            <th class="px-4 py-3 text-left font-medium text-gray-500 dark:text-gray-400">{{ t('admin.modelPricing.upstreamModelName') }}</th>
            <th class="px-4 py-3 text-center font-medium text-gray-500 dark:text-gray-400">{{ t('admin.modelPricing.billingMode') }}</th>
            <th class="hidden px-4 py-3 text-left font-medium text-gray-500 dark:text-gray-400 xl:table-cell">Provider</th>
            <th class="px-4 py-3 text-right font-medium text-gray-500 dark:text-gray-400">{{ t('admin.modelPricing.inputPrice') }}</th>
            <th class="px-4 py-3 text-right font-medium text-gray-500 dark:text-gray-400">{{ t('admin.modelPricing.outputPrice') }}</th>
            <th class="hidden px-4 py-3 text-right font-medium text-gray-500 dark:text-gray-400 lg:table-cell">{{ t('admin.modelPricing.cacheWritePrice') }}</th>
            <th class="hidden px-4 py-3 text-right font-medium text-gray-500 dark:text-gray-400 lg:table-cell">{{ t('admin.modelPricing.cacheReadPrice') }}</th>
            <th class="px-4 py-3 text-center font-medium text-gray-500 dark:text-gray-400">{{ t('admin.modelPricing.source') }}</th>
            <th class="hidden px-4 py-3 text-center font-medium text-gray-500 dark:text-gray-400 xl:table-cell">{{ t('admin.modelPricing.channels') }}</th>
            <th class="px-4 py-3 text-right font-medium text-gray-500 dark:text-gray-400">{{ t('admin.modelPricing.actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading" class="border-b">
            <td colspan="11" class="py-12 text-center">
              <span class="inline-block h-5 w-5 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></span>
            </td>
          </tr>
          <tr v-else-if="items.length === 0" class="border-b">
            <td colspan="11" class="py-12 text-center text-gray-400">{{ t('admin.modelPricing.noModels') }}</td>
          </tr>
          <tr
            v-for="row in displayRows"
            :key="row.item.model"
            class="border-b border-gray-100 last:border-0 hover:bg-gray-50 dark:border-gray-700/50 dark:hover:bg-gray-700/30"
          >
            <td class="px-4 py-2.5">
              <span class="font-mono text-xs text-gray-900 dark:text-white">{{ row.requestedDisplay.primary }}</span>
              <span
                v-if="row.requestedDisplay.moreCount > 0"
                class="ml-1 inline-block rounded bg-gray-100 px-1 py-0.5 text-[10px] font-medium text-gray-600 dark:bg-gray-700 dark:text-gray-300"
                :title="row.requestedDisplay.moreTooltip"
              >
                +{{ row.requestedDisplay.moreCount }}
              </span>
            </td>
            <td class="px-4 py-2.5">
              <span class="font-mono text-xs text-gray-700 dark:text-gray-300">{{ row.upstreamDisplay }}</span>
            </td>
            <td class="px-4 py-2.5 text-center">
              <span :class="row.billingModeClass" class="inline-block rounded-full px-2 py-0.5 text-xs font-medium">
                {{ row.billingModeLabel }}
              </span>
            </td>
            <td class="hidden px-4 py-2.5 xl:table-cell">
              <span class="inline-block rounded bg-gray-100 px-1.5 py-0.5 text-xs text-gray-600 dark:bg-gray-700 dark:text-gray-300">
                {{ row.item.provider || '-' }}
              </span>
            </td>
            <td
              class="cursor-pointer px-4 py-2.5 text-right font-mono text-xs hover:bg-primary-50/50 dark:hover:bg-primary-900/20"
              @click="openInlinePopover($event, row.item, 'input')"
            >
              <span :class="row.deltas.input.className" :title="row.deltas.input.tooltip">{{ row.deltas.input.text }}</span>
            </td>
            <td
              class="cursor-pointer px-4 py-2.5 text-right font-mono text-xs hover:bg-primary-50/50 dark:hover:bg-primary-900/20"
              @click="openInlinePopover($event, row.item, 'output')"
            >
              <span :class="row.deltas.output.className" :title="row.deltas.output.tooltip">{{ row.deltas.output.text }}</span>
            </td>
            <td
              class="hidden cursor-pointer px-4 py-2.5 text-right font-mono text-xs hover:bg-primary-50/50 lg:table-cell dark:hover:bg-primary-900/20"
              @click="openInlinePopover($event, row.item, 'cache_write')"
            >
              <span :class="row.deltas.cache_write.className" :title="row.deltas.cache_write.tooltip">{{ row.deltas.cache_write.text }}</span>
            </td>
            <td
              class="hidden cursor-pointer px-4 py-2.5 text-right font-mono text-xs hover:bg-primary-50/50 lg:table-cell dark:hover:bg-primary-900/20"
              @click="openInlinePopover($event, row.item, 'cache_read')"
            >
              <span :class="row.deltas.cache_read.className" :title="row.deltas.cache_read.tooltip">{{ row.deltas.cache_read.text }}</span>
            </td>
            <td class="px-4 py-2.5 text-center">
              <span :class="sourceBadgeClass(row.item.effective_source)" class="inline-block rounded-full px-2 py-0.5 text-xs font-medium">
                {{ row.item.effective_source }}
              </span>
            </td>
            <td class="hidden px-4 py-2.5 text-center xl:table-cell">
              <span v-if="row.item.channel_override_count > 0" class="inline-block rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-700 dark:bg-amber-900/30 dark:text-amber-400">
                {{ row.item.channel_override_count }}
              </span>
              <span v-else class="text-xs text-gray-300">-</span>
            </td>
            <td class="px-4 py-2.5 text-right">
              <div class="flex justify-end gap-0.5">
                <button
                  v-if="row.canEditMapping"
                  @click="openEditMapping($event, row)"
                  class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-gray-700 dark:hover:text-primary-400"
                  :title="t('admin.modelPricing.editMapping')"
                >
                  <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4" />
                  </svg>
                </button>
                <button
                  v-if="row.canTest"
                  @click="openTestDialog(row.item.model)"
                  class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-emerald-600 dark:hover:bg-gray-700 dark:hover:text-emerald-400"
                  :title="t('admin.modelPricing.testModel')"
                >
                  <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
                    <path stroke-linecap="round" stroke-linejoin="round" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                </button>
                <button
                  @click="openDetail(row.item.model)"
                  class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-gray-700 dark:hover:text-gray-300"
                  :title="row.stub ? t('admin.modelPricing.createPricing') : t('admin.modelPricing.viewDetail')"
                >
                  <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M16.862 4.487l1.687-1.688a1.875 1.875 0 112.652 2.652L10.582 16.07a4.5 4.5 0 01-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 011.13-1.897l8.932-8.931zm0 0L19.5 7.125M18 14v4.75A2.25 2.25 0 0115.75 21H5.25A2.25 2.25 0 013 18.75V8.25A2.25 2.25 0 015.25 6H10" />
                  </svg>
                </button>
              </div>
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

    <!-- Inline Popover (quick edit) -->
    <ModelPricingInlinePopover
      :show="popoverState.show"
      :anchor="popoverState.anchor"
      :model="popoverState.model"
      :existing-override="popoverState.existingOverride"
      :litellm-baseline="popoverState.litellmBaseline"
      :focus-field="popoverState.focusField"
      @close="closeInlinePopover"
      @saved="handleInlineSaved"
      @open-full-dialog="popoverFallbackToDialog"
    />

    <!-- Mapping CRUD Popover -->
    <ModelMappingInlinePopover
      :show="mappingPopoverState.show"
      :anchor="mappingPopoverState.anchor"
      :mode="mappingPopoverState.mode"
      :original-from="mappingPopoverState.originalFrom"
      :original-to="mappingPopoverState.originalTo"
      @close="closeMappingPopover"
      @saved="handleMappingSaved"
    />

    <!-- Model Test Dialog -->
    <ModelTestDialog
      :show="testDialogState.show"
      :model="testDialogState.model"
      @close="testDialogState.show = false"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { ModelPricingItem, ModelPricingStats, GlobalOverride, LiteLLMPrices } from '@/api/admin/modelPricing'
import { perTokenToMTok } from '@/components/admin/channel/types'
import ModelPricingDetailDialog from './ModelPricingDetailDialog.vue'
import ModelPricingInlinePopover from './ModelPricingInlinePopover.vue'
import ModelMappingInlinePopover from './ModelMappingInlinePopover.vue'
import ModelTestDialog from './ModelTestDialog.vue'

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

// 内联 popover 状态
type PopoverField = 'input' | 'output' | 'cache_write' | 'cache_read'
interface PopoverState {
  show: boolean
  anchor: HTMLElement | null
  model: string
  existingOverride: GlobalOverride | null
  litellmBaseline: LiteLLMPrices | null
  focusField: PopoverField
}
const popoverState = reactive<PopoverState>({
  show: false,
  anchor: null,
  model: '',
  existingOverride: null,
  litellmBaseline: null,
  focusField: 'input',
})

// 移动端判断：< lg 断点直接回退到 dialog
function isNarrowViewport(): boolean {
  return typeof window !== 'undefined' && window.matchMedia('(max-width: 1023px)').matches
}

function openInlinePopover(event: MouseEvent, item: ModelPricingItem, field: PopoverField) {
  // 移动端：直接打开原 dialog
  if (isNarrowViewport()) {
    openDetail(item.model)
    return
  }
  // Stub 模型（无 LiteLLM 数据且无 global_override）走完整 dialog 以获取建议价
  // 和填入 provider / notes 等上下文字段；popover 只负责快速调参
  if (isStubRow(item)) {
    openDetail(item.model)
    return
  }
  const target = event.currentTarget as HTMLElement | null
  if (!target) return
  popoverState.show = true
  popoverState.anchor = target
  popoverState.model = item.model
  popoverState.existingOverride = item.global_override ?? null
  popoverState.litellmBaseline = item.litellm_prices ?? null
  popoverState.focusField = field
}

function closeInlinePopover() {
  popoverState.show = false
  popoverState.anchor = null
}

function handleInlineSaved(payload: {
  model: string
  override: GlobalOverride | null
  wasCreate: boolean
  wasDelete: boolean
}) {
  // 本地就地替换：避免整表 reload 导致滚动/光标丢失
  const idx = items.value.findIndex((i) => i.model === payload.model)
  if (idx >= 0) {
    const cloned = { ...items.value[idx], global_override: payload.override }
    // 更新 effective_source：有启用的 override 就是 global，否则回退到 litellm/fallback
    if (payload.override && payload.override.enabled) {
      cloned.effective_source = 'global'
    } else if (cloned.litellm_prices) {
      cloned.effective_source = 'litellm'
    } else {
      cloned.effective_source = 'fallback'
    }
    items.value.splice(idx, 1, cloned)
  }
  // stats 差量更新
  if (payload.wasCreate) stats.global_override_count += 1
  if (payload.wasDelete) stats.global_override_count = Math.max(0, stats.global_override_count - 1)
}

function popoverFallbackToDialog() {
  const model = popoverState.model
  closeInlinePopover()
  if (model) openDetail(model)
}

// 映射编辑 popover 状态
const addMappingAnchor = ref<HTMLElement | null>(null)
interface MappingPopoverState {
  show: boolean
  anchor: HTMLElement | null
  mode: 'add' | 'edit'
  originalFrom: string
  originalTo: string
}
const mappingPopoverState = reactive<MappingPopoverState>({
  show: false,
  anchor: null,
  mode: 'add',
  originalFrom: '',
  originalTo: '',
})

function openAddMapping() {
  mappingPopoverState.show = true
  mappingPopoverState.anchor = addMappingAnchor.value
  mappingPopoverState.mode = 'add'
  mappingPopoverState.originalFrom = ''
  mappingPopoverState.originalTo = ''
}

function openEditMapping(event: MouseEvent, row: RowDisplay) {
  const target = event.currentTarget as HTMLElement | null
  if (!target) return
  mappingPopoverState.show = true
  mappingPopoverState.anchor = target
  mappingPopoverState.mode = 'edit'
  mappingPopoverState.originalFrom = row.item.model
  // 对 requested_only 类型，row.upstreamDisplay 就是映射目标；
  // 对 requested_equals_upstream，上游名 == 模型本身（同名映射的 value）
  mappingPopoverState.originalTo = row.upstreamDisplay
}

function closeMappingPopover() {
  mappingPopoverState.show = false
  mappingPopoverState.anchor = null
}

function handleMappingSaved(_payload: { mode: 'add' | 'edit' | 'delete'; from: string; to?: string }) {
  // 映射变化会影响所有徽标和 related_models，必须整表 reload
  loadData()
}

// 模型测试 dialog 状态
const testDialogState = reactive<{ show: boolean; model: string }>({
  show: false,
  model: '',
})

function openTestDialog(model: string) {
  testDialogState.model = model
  testDialogState.show = true
}

let searchTimeout: ReturnType<typeof setTimeout>

function handleSearch() {
  clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => {
    pagination.page = 1
    loadData()
  }, 300)
}

// Provider / Source 的下划线 tab 选项。Anthropic/OpenAI/Gemini/Antigravity 是品牌名不做翻译。
const providerTabs = computed(() => [
  { value: '', label: t('common.all') },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'openai', label: 'OpenAI' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'antigravity', label: 'Antigravity' },
])

// 顺序按实际计费优先级（高 → 低）：渠道 > 全局 > 仅 LiteLLM
const sourceTabs = computed(() => [
  { value: '', label: t('common.all') },
  { value: 'has_channel_override', label: t('admin.modelPricing.hasChannelOverride') },
  { value: 'has_global_override', label: t('admin.modelPricing.hasGlobalOverride') },
  { value: 'litellm_only', label: t('admin.modelPricing.litellmOnly') },
])

function setProvider(value: string) {
  if (providerFilter.value === value) return
  providerFilter.value = value
  pagination.page = 1
  loadData()
}

function setSource(value: string) {
  if (sourceFilter.value === value) return
  sourceFilter.value = value
  pagination.page = 1
  loadData()
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

type PriceField = 'input' | 'output' | 'cache_write' | 'cache_read'

interface PriceDelta {
  text: string          // 展示的 $X 字符串或 '-'
  className: string     // 数字的颜色 class
  tooltip: string       // hover tooltip 文本
  effectiveSource: 'global' | 'litellm' | 'none' // 该字段当前生效来源
}

/**
 * 为某个价格字段计算展示内容 + 差异高亮 class + tooltip。
 *
 * 生效优先级：global override (enabled) > litellm > none
 * 差异计算：仅当 override 和 litellm 都有值时才比较；
 *   |delta| ≤ 1% 视作等同（避免浮点噪声），仍用 primary 色标记"被覆盖"
 *   delta > +1% 涨价 → rose 色
 *   delta < -1% 跌价 → emerald 色
 *   无 litellm 基准的 override → primary 色（不染涨跌）
 *   纯 litellm → 默认灰色
 */
function computePriceDelta(item: ModelPricingItem, type: PriceField): PriceDelta {
  const key = `${type}_price` as const
  const go = item.global_override
  const overrideVal = go && go.enabled ? (go[key] as number | null) : null
  const litellmVal = item.litellm_prices ? (item.litellm_prices[key] as number) : null
  const validLitellm = typeof litellmVal === 'number' && litellmVal > 0 ? litellmVal : null

  // 1) 有全局覆盖且启用
  if (typeof overrideVal === 'number') {
    const mtok = perTokenToMTok(overrideVal)
    if (mtok === null) return { text: '-', className: 'text-gray-400', tooltip: '', effectiveSource: 'global' }
    const text = `$${mtok}`

    if (validLitellm === null) {
      return {
        text,
        className: 'text-primary-600 dark:text-primary-400 font-semibold',
        tooltip: t('admin.modelPricing.noBaseline'),
        effectiveSource: 'global',
      }
    }

    const delta = (overrideVal - validLitellm) / validLitellm
    const baselineMtok = perTokenToMTok(validLitellm)
    const baselineText = baselineMtok !== null ? `$${baselineMtok}` : '-'
    const deltaPct = `${delta >= 0 ? '+' : ''}${(delta * 100).toFixed(1)}%`
    const tooltip = t('admin.modelPricing.priceDeltaTooltip', { baseline: baselineText, delta: deltaPct })

    if (Math.abs(delta) <= 0.01) {
      return { text, className: 'text-primary-600 dark:text-primary-400 font-semibold', tooltip, effectiveSource: 'global' }
    }
    if (delta > 0) {
      return { text, className: 'text-rose-600 dark:text-rose-400 font-semibold', tooltip, effectiveSource: 'global' }
    }
    return { text, className: 'text-emerald-600 dark:text-emerald-400 font-semibold', tooltip, effectiveSource: 'global' }
  }

  // 2) 无全局覆盖但有 litellm
  if (validLitellm !== null) {
    const mtok = perTokenToMTok(validLitellm)
    if (mtok === null) return { text: '-', className: 'text-gray-400', tooltip: '', effectiveSource: 'none' }
    return {
      text: `$${mtok}`,
      className: 'text-gray-600 dark:text-gray-400',
      tooltip: '',
      effectiveSource: 'litellm',
    }
  }

  // 3) 都没有
  return { text: '-', className: 'text-gray-400', tooltip: '', effectiveSource: 'none' }
}

// 判断一行是否为 stub（无 LiteLLM 数据且无全局覆盖——典型是 Antigravity 补的专有模型）
function isStubRow(item: ModelPricingItem): boolean {
  return !item.litellm_prices && !item.global_override
}

// 预计算每行四个价格字段的 delta，以及基于 billing_basis_hint 推导的
// 请求名/上游名双列展示 + 计费模式标签，避免模板内多次计算。
interface RowDisplay {
  item: ModelPricingItem
  stub: boolean
  deltas: Record<PriceField, PriceDelta>
  // 请求名列展示：可能多对一，primary 是首个，moreCount > 0 时展示 +N 并在 tooltip 里列全
  requestedDisplay: { primary: string; moreCount: number; moreTooltip: string }
  // 上游名列展示：单一值
  upstreamDisplay: string
  billingModeLabel: string
  billingModeClass: string
  // 行操作是否可用
  canEditMapping: boolean
  canTest: boolean
}

function deriveNameColumns(item: ModelPricingItem): {
  requestedDisplay: RowDisplay['requestedDisplay']
  upstreamDisplay: string
  billingModeLabel: string
  billingModeClass: string
} {
  const hint = item.billing_basis_hint
  const model = item.model
  // 默认（无徽标或同名映射）：请求 = 上游 = 模型本身
  if (!hint || hint.type === 'requested_equals_upstream') {
    // Antigravity 场景下，同名映射的模型常常同时也是其他请求名的映射目标
    // （如 claude-opus-4-6-thinking 既是同名 key 又被 claude-opus-4-6 指向）。
    // 此时后端在 related_models 里额外塞入那些映射源请求名，让用户看到关联。
    const related = hint?.related_models ?? []
    const moreCount = related.length
    const moreTooltip = moreCount > 0
      ? t('admin.modelPricing.mappedFromMultipleTooltip', {
          count: moreCount,
          list: related.join(', '),
        })
      : ''
    return {
      requestedDisplay: { primary: model, moreCount, moreTooltip },
      upstreamDisplay: model,
      billingModeLabel: t('admin.modelPricing.billingModeRequestEqualsUpstream'),
      billingModeClass: 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300',
    }
  }

  if (hint.type === 'requested_only') {
    // 模型是映射 key，上游名 = related_models[0]
    const upstream = hint.related_models?.[0] ?? model
    return {
      requestedDisplay: { primary: model, moreCount: 0, moreTooltip: '' },
      upstreamDisplay: upstream,
      billingModeLabel: t('admin.modelPricing.billingModeByRequested'),
      billingModeClass: 'bg-sky-100 text-sky-700 dark:bg-sky-900/30 dark:text-sky-400',
    }
  }

  // upstream_only：模型是映射 value，请求名 = 所有 related_models
  const related = hint.related_models ?? []
  const primary = related[0] ?? model
  const moreCount = Math.max(0, related.length - 1)
  let moreTooltip = ''
  if (moreCount > 0) {
    moreTooltip = t('admin.modelPricing.mappedFromMultipleTooltip', {
      count: moreCount,
      list: related.slice(1).join(', '),
    })
  }
  return {
    requestedDisplay: { primary, moreCount, moreTooltip },
    upstreamDisplay: model,
    billingModeLabel: t('admin.modelPricing.billingModeByUpstream'),
    billingModeClass: 'bg-violet-100 text-violet-700 dark:bg-violet-900/30 dark:text-violet-400',
  }
}

const displayRows = computed<RowDisplay[]>(() =>
  items.value.map((item) => {
    const nameCols = deriveNameColumns(item)
    const hintType = item.billing_basis_hint?.type
    // 编辑映射按钮：仅对 Antigravity 映射的 key 行显示
    // （requested_only 是 key != value 的 key；requested_equals_upstream 是同名 key）
    // upstream_only 行只作为 value 不是 key，不显示编辑入口（要改要去对应 key 行）
    const canEditMapping = hintType === 'requested_only' || hintType === 'requested_equals_upstream'
    // 测试按钮：所有 Antigravity 认可的模型（有 hint 即在映射里），以及 provider=antigravity 的 stub
    const canTest = !!hintType || item.provider === 'antigravity'
    return {
      item,
      stub: isStubRow(item),
      deltas: {
        input: computePriceDelta(item, 'input'),
        output: computePriceDelta(item, 'output'),
        cache_write: computePriceDelta(item, 'cache_write'),
        cache_read: computePriceDelta(item, 'cache_read'),
      },
      canEditMapping,
      canTest,
      ...nameCols,
    }
  })
)

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
