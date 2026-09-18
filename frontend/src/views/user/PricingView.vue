<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-7xl space-y-6">
      <!-- Page header -->
      <div>
        <h1 class="text-2xl font-bold text-gray-900 dark:text-white">{{ t('pricing.title') }}</h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('pricing.description') }}</p>
      </div>

      <!-- Loading -->
      <div v-if="loading" class="card p-8 text-center text-gray-500 dark:text-gray-400">
        {{ t('common.loading') }}
      </div>

      <template v-else-if="data">
        <!-- 计费说明：intro + education 合并为一块 -->
        <section class="card p-6">
          <h2 class="mb-3 flex items-center gap-2 text-lg font-semibold text-gray-900 dark:text-white">
            <span class="inline-block h-2 w-2 rounded-full bg-primary-500"></span>
            {{ t('pricing.billingExplainerTitle') }}
          </h2>
          <div
            v-if="renderedIntro"
            class="markdown-body prose prose-sm max-w-none dark:prose-invert"
            v-html="renderedIntro"
          ></div>
        </section>

        <!-- 双表：左模型价格 / 右分组倍率 -->
        <div class="grid grid-cols-1 gap-6 lg:grid-cols-3">
          <!-- 模型价格 -->
          <section class="card flex min-w-0 flex-col p-6 lg:col-span-2">
            <h2 class="mb-4 flex items-center gap-2 text-lg font-semibold text-gray-900 dark:text-white">
              <span class="inline-block h-2 w-2 rounded-full bg-green-500"></span>
              {{ t('pricing.modelTableTitle') }}
            </h2>

            <div
              v-if="cnyRate > 0"
              class="mb-4 inline-flex items-center gap-2 self-start rounded-full bg-primary-50 px-3 py-1.5 text-xs font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300"
            >
              <span aria-hidden="true">💱</span>
              <span>{{ t('pricing.cnyBanner', { rate: cnyRate.toFixed(2) }) }}</span>
            </div>

            <div v-if="!displayPlatforms.length" class="py-10 text-center text-sm text-gray-500 dark:text-gray-400">
              {{ t('pricing.emptyState') }}
            </div>

            <template v-else>
              <div class="mb-4 w-full sm:max-w-sm" data-test="pricing-model-search">
                <SearchInput
                  v-model="modelSearch"
                  :placeholder="t('pricing.searchPlaceholder')"
                />
              </div>

              <div
                role="tablist"
                class="mb-4 flex flex-wrap gap-1 border-b border-gray-200 dark:border-dark-700"
                :aria-label="t('pricing.platformTabsLabel')"
              >
                <button
                  v-for="platform in displayPlatforms"
                  :key="platform.provider"
                  type="button"
                  role="tab"
                  class="-mb-px whitespace-nowrap border-b-2 px-3 py-2 text-sm font-medium transition-colors"
                  :class="selectedProvider === platform.provider
                    ? 'border-primary-500 text-primary-600 dark:text-primary-400'
                    : 'border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'"
                  :aria-selected="selectedProvider === platform.provider"
                  :data-test="'pricing-platform-tab-' + platform.provider"
                  @click="selectedProvider = platform.provider"
                >
                  <span :class="platform.provider === DOMESTIC_PRICING_TAB ? 'tracking-wide' : 'uppercase tracking-wide'">
                    {{ platformTabLabel(platform.provider) }}
                  </span>
                  <span class="ml-1.5 text-xs font-normal text-gray-400 dark:text-gray-500">
                    {{ filteredCount(platform) }}
                  </span>
                </button>
              </div>

              <div
                v-if="selectedPlatform && visibleModels.length"
                class="min-w-0 overflow-x-auto rounded-lg border border-gray-200 dark:border-dark-700"
              >
                <table class="w-full border-collapse text-sm">
                  <thead class="bg-gray-100 text-xs uppercase text-gray-500 dark:bg-dark-900 dark:text-gray-400">
                    <tr>
                      <th class="px-4 py-2 text-left font-medium">{{ t('pricing.columns.model') }}</th>
                      <th class="px-4 py-2 text-left font-medium">{{ t('pricing.columns.billingMode') }}</th>
                      <th class="px-4 py-2 text-right font-medium">{{ t('pricing.columns.inputPrice') }}</th>
                      <th class="px-4 py-2 text-right font-medium">{{ t('pricing.columns.outputPrice') }}</th>
                      <th class="px-4 py-2 text-right font-medium">{{ t('pricing.columns.cacheReadPrice') }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr
                      v-for="model in visibleModels"
                      :key="model.model"
                      class="border-t border-gray-200 dark:border-dark-700"
                    >
                      <td class="px-4 py-2 font-mono text-gray-900 dark:text-white">{{ model.model }}</td>
                      <td class="px-4 py-2 text-gray-600 dark:text-gray-300">{{ billingModeLabel(model.billing_mode) }}</td>
                      <template v-if="model.billing_mode === 'per_request'">
                        <td class="px-4 py-2 text-right text-gray-900 dark:text-white" colspan="3">
                          <span class="font-semibold">{{ perRequestPrimary(model.per_request_price) }}</span>
                          <span
                            v-if="perRequestSecondary(model.per_request_price)"
                            class="ml-1 text-xs text-gray-500 dark:text-gray-400"
                          >
                            ({{ perRequestSecondary(model.per_request_price) }})
                          </span>
                          <span class="ml-1 text-xs text-gray-500 dark:text-gray-400">
                            / {{ t('pricing.perRequestUnit') }}
                          </span>
                        </td>
                      </template>
                      <template v-else>
                        <td class="px-4 py-2 text-right text-gray-900 dark:text-white">
                          <span class="font-semibold">{{ tokenPrimary(model.display_input_price) }}</span>
                          <span
                            v-if="tokenSecondary(model.display_input_price)"
                            class="ml-1 text-xs text-gray-500 dark:text-gray-400"
                          >
                            ({{ tokenSecondary(model.display_input_price) }})
                          </span>
                        </td>
                        <td class="px-4 py-2 text-right text-gray-900 dark:text-white">
                          <span class="font-semibold">{{ tokenPrimary(model.display_output_price) }}</span>
                          <span
                            v-if="tokenSecondary(model.display_output_price)"
                            class="ml-1 text-xs text-gray-500 dark:text-gray-400"
                          >
                            ({{ tokenSecondary(model.display_output_price) }})
                          </span>
                        </td>
                        <td class="px-4 py-2 text-right text-gray-900 dark:text-white">
                          <span class="font-semibold">{{ tokenPrimary(model.display_cache_read_price) }}</span>
                          <span
                            v-if="tokenSecondary(model.display_cache_read_price)"
                            class="ml-1 text-xs text-gray-500 dark:text-gray-400"
                          >
                            ({{ tokenSecondary(model.display_cache_read_price) }})
                          </span>
                        </td>
                      </template>
                    </tr>
                  </tbody>
                </table>
              </div>

              <div
                v-else
                class="py-10 text-center text-sm text-gray-500 dark:text-gray-400"
                data-test="pricing-search-empty"
              >
                {{ modelSearch.trim() ? t('pricing.searchEmpty') : t('pricing.emptyState') }}
              </div>

              <p class="mt-3 text-xs text-gray-500 dark:text-gray-400">
                {{ t('pricing.unitHint') }}
              </p>
            </template>
          </section>

          <!-- 分组倍率 -->
          <section class="card flex min-w-0 flex-col p-6" data-test="pricing-group-rates">
            <h2 class="mb-4 flex items-center gap-2 text-lg font-semibold text-gray-900 dark:text-white">
              <span class="inline-block h-2 w-2 rounded-full bg-amber-500"></span>
              {{ t('pricing.groupTableTitle') }}
            </h2>

            <div
              v-if="groupsLoading"
              class="py-10 text-center text-sm text-gray-500 dark:text-gray-400"
            >
              {{ t('common.loading') }}
            </div>
            <div
              v-else-if="groupsError"
              class="py-10 text-center text-sm text-red-600 dark:text-red-400"
              data-test="pricing-groups-error"
            >
              {{ groupsError }}
            </div>
            <div
              v-else-if="!groups.length"
              class="py-10 text-center text-sm text-gray-500 dark:text-gray-400"
              data-test="pricing-groups-empty"
            >
              {{ t('pricing.groupsEmpty') }}
            </div>
            <div
              v-else
              class="min-w-0 overflow-x-auto rounded-lg border border-gray-200 dark:border-dark-700"
            >
              <table class="w-full border-collapse text-sm">
                <thead class="bg-gray-100 text-xs uppercase text-gray-500 dark:bg-dark-900 dark:text-gray-400">
                  <tr>
                    <th class="px-4 py-2 text-left font-medium">{{ t('pricing.groupColumns.name') }}</th>
                    <th class="px-4 py-2 text-left font-medium">{{ t('pricing.groupColumns.platform') }}</th>
                    <th class="px-4 py-2 text-right font-medium">{{ t('pricing.groupColumns.displayRate') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="group in groups"
                    :key="group.id"
                    class="border-t border-gray-200 dark:border-dark-700"
                    :data-test="'pricing-group-row-' + group.id"
                  >
                    <td class="px-4 py-2 text-gray-900 dark:text-white">{{ group.name }}</td>
                    <td class="px-4 py-2 uppercase tracking-wide text-gray-600 dark:text-gray-300">
                      {{ group.platform }}
                    </td>
                    <td class="px-4 py-2 text-right font-semibold text-gray-900 dark:text-white">
                      ×{{ formatRate(group.rate_multiplier) }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>

          </section>
        </div>
      </template>

      <div v-else class="card p-6 text-sm text-red-600 dark:text-red-400">
        {{ errorMessage }}
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import AppLayout from '@/components/layout/AppLayout.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import { pricingPageAPI, type PricingPageData, type PricingPageModel, type PricingPagePlatform } from '@/api/pricingPage'
import { userGroupsAPI } from '@/api/groups'
import type { Group } from '@/types'
import { useAppStore } from '@/stores'
import {
  DOMESTIC_PRICING_TAB,
  filterPricingModels,
  splitDomesticPricingPlatforms
} from '@/utils/pricingPageModels'

const { t } = useI18n()
const appStore = useAppStore()

// USD→CNY 换算率，来自管理员在「充值管理」配置（payment_cny_per_usd 公开设置）。
// 0 / 未配置 → 隐藏 banner，单元格只显示美元，不会出现 ¥0 这种诡异显示。
const cnyRate = computed(() => Number(appStore.cachedPublicSettings?.payment_cny_per_usd ?? 0))

const loading = ref(true)
const data = ref<PricingPageData | null>(null)
const errorMessage = ref('')
const selectedProvider = ref('')
const modelSearch = ref('')

const groups = ref<Group[]>([])
const groupsLoading = ref(false)
const groupsError = ref('')

marked.setOptions({ breaks: true, gfm: true })

const renderedIntro = computed(() => renderMarkdown(data.value?.intro ?? ''))

const displayPlatforms = computed<PricingPagePlatform[]>(() =>
  splitDomesticPricingPlatforms(data.value?.platforms ?? [])
)

const selectedPlatform = computed<PricingPagePlatform | null>(() => {
  if (!displayPlatforms.value.length) return null
  return displayPlatforms.value.find((p) => p.provider === selectedProvider.value) ?? displayPlatforms.value[0] ?? null
})

const visibleModels = computed<PricingPageModel[]>(() => {
  if (!selectedPlatform.value) return []
  return filterPricingModels(selectedPlatform.value.models, modelSearch.value, cnyRate.value)
})

watch(
  displayPlatforms,
  (platforms) => {
    if (!platforms.length) {
      selectedProvider.value = ''
      return
    }
    if (!platforms.some((p) => p.provider === selectedProvider.value)) {
      selectedProvider.value = platforms[0].provider
    }
  },
  { immediate: true }
)

watch(modelSearch, () => {
  if (!normalizeHasQuery() || visibleModels.value.length > 0) return
  const firstHit = displayPlatforms.value.find((platform) => filteredCount(platform) > 0)
  if (firstHit) selectedProvider.value = firstHit.provider
})

function normalizeHasQuery(): boolean {
  return modelSearch.value.trim().length > 0
}

function filteredCount(platform: PricingPagePlatform): number {
  return filterPricingModels(platform.models, modelSearch.value, cnyRate.value).length
}

function platformTabLabel(provider: string): string {
  if (provider === DOMESTIC_PRICING_TAB) return t('pricing.tabs.domestic')
  return provider
}

function renderMarkdown(text: string): string {
  if (!text) return ''
  const html = marked.parse(text) as string
  return DOMPurify.sanitize(html)
}

function billingModeLabel(mode: string): string {
  if (mode === 'per_request') return t('pricing.billingMode.perRequest')
  if (mode === 'image') return t('pricing.billingMode.image')
  return t('pricing.billingMode.perToken')
}

function formatRate(rate: number | null | undefined): string {
  const n = Number(rate ?? 1)
  if (!Number.isFinite(n)) return '1'
  return Number(n.toFixed(4)).toString()
}

// 价格双币种渲染。display_*_price / per_request_price 都是 USD（per token / per call）。
// primary：人民币（按 cnyRate 实时换算）；secondary：USD 原价加括号显示。
// 当未配置 cnyRate 时，primary 退化为美元、secondary 为 null（单币种显示）。

function tokenPrimary(usdPerToken: number | null | undefined): string {
  if (usdPerToken == null) return '—'
  const usdMTok = usdPerToken * 1_000_000
  return cnyRate.value > 0
    ? `¥${(usdMTok * cnyRate.value).toFixed(2)}`
    : `$${usdMTok.toFixed(2)}`
}

function tokenSecondary(usdPerToken: number | null | undefined): string | null {
  if (usdPerToken == null || cnyRate.value <= 0) return null
  return `$${(usdPerToken * 1_000_000).toFixed(2)}`
}

function perRequestPrimary(usd: number | null | undefined): string {
  if (usd == null) return '—'
  return cnyRate.value > 0
    ? `¥${(usd * cnyRate.value).toFixed(4)}`
    : `$${usd.toFixed(4)}`
}

function perRequestSecondary(usd: number | null | undefined): string | null {
  if (usd == null || cnyRate.value <= 0) return null
  return `$${usd.toFixed(4)}`
}

onMounted(async () => {
  groupsLoading.value = true
  const [pricingResult, groupsResult] = await Promise.allSettled([
    pricingPageAPI.getUserPricingPage(),
    userGroupsAPI.getAvailable()
  ])

  if (pricingResult.status === 'fulfilled') {
    data.value = pricingResult.value
  } else {
    const err = pricingResult.reason
    errorMessage.value = err instanceof Error ? err.message : String(err)
  }

  if (groupsResult.status === 'fulfilled') {
    groups.value = groupsResult.value ?? []
  } else {
    const err = groupsResult.reason
    groupsError.value = err instanceof Error ? err.message : t('pricing.groupsLoadFailed')
  }

  groupsLoading.value = false
  loading.value = false
})
</script>

<style scoped>
.markdown-body :deep(table) {
  @apply my-3 w-full border-collapse text-sm;
}
.markdown-body :deep(th),
.markdown-body :deep(td) {
  @apply border border-gray-200 px-3 py-1.5 text-left dark:border-dark-700;
}
.markdown-body :deep(th) {
  @apply bg-gray-50 font-medium dark:bg-dark-800;
}
.markdown-body :deep(h1),
.markdown-body :deep(h2),
.markdown-body :deep(h3) {
  @apply mb-2 mt-4 font-semibold text-gray-900 dark:text-white;
}
.markdown-body :deep(h2) {
  @apply text-base;
}
.markdown-body :deep(ul),
.markdown-body :deep(ol) {
  @apply ml-5 list-disc;
}
.markdown-body :deep(p) {
  @apply my-2 leading-relaxed text-gray-700 dark:text-gray-300;
}
.markdown-body :deep(strong) {
  @apply text-gray-900 dark:text-white;
}
.markdown-body :deep(code) {
  @apply rounded bg-gray-100 px-1 py-0.5 font-mono text-xs dark:bg-dark-800;
}
</style>
