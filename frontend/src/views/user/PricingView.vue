<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-6xl space-y-6">
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
        <!-- 本站计价模式 -->
        <section class="card p-6">
          <h2 class="mb-3 flex items-center gap-2 text-lg font-semibold text-gray-900 dark:text-white">
            <span class="inline-block h-2 w-2 rounded-full bg-primary-500"></span>
            {{ t('pricing.introTitle') }}
          </h2>
          <div class="markdown-body prose prose-sm max-w-none dark:prose-invert" v-html="renderedIntro"></div>
        </section>

        <!-- 计价模式科普 -->
        <section class="card p-6">
          <h2 class="mb-3 flex items-center gap-2 text-lg font-semibold text-gray-900 dark:text-white">
            <span class="inline-block h-2 w-2 rounded-full bg-amber-500"></span>
            {{ t('pricing.educationTitle') }}
          </h2>
          <div class="markdown-body prose prose-sm max-w-none dark:prose-invert" v-html="renderedEducation"></div>
        </section>

        <!-- 模型价格一览表 -->
        <section class="card p-6">
          <h2 class="mb-4 flex items-center gap-2 text-lg font-semibold text-gray-900 dark:text-white">
            <span class="inline-block h-2 w-2 rounded-full bg-green-500"></span>
            {{ t('pricing.tableTitle') }}
          </h2>

          <!-- USD→CNY 换算率 banner：只在管理员配置过 payment_cny_per_usd 时显示 -->
          <div
            v-if="cnyRate > 0"
            class="mb-4 inline-flex items-center gap-2 rounded-full bg-primary-50 px-3 py-1.5 text-xs font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300"
          >
            <span aria-hidden="true">💱</span>
            <span>{{ t('pricing.cnyBanner', { rate: cnyRate.toFixed(2) }) }}</span>
          </div>

          <div v-if="!data.platforms.length" class="py-10 text-center text-sm text-gray-500 dark:text-gray-400">
            {{ t('pricing.emptyState') }}
          </div>

          <div v-else class="space-y-6">
            <div v-for="platform in data.platforms" :key="platform.provider" class="overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700">
              <div class="flex items-center justify-between bg-gray-50 px-4 py-2 text-sm font-medium text-gray-700 dark:bg-dark-800 dark:text-gray-200">
                <span class="uppercase tracking-wide">{{ platform.provider }}</span>
                <span class="text-xs text-gray-500 dark:text-gray-400">{{ platform.models.length }} {{ t('pricing.modelsSuffix') }}</span>
              </div>
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
                    v-for="model in platform.models"
                    :key="model.model"
                    class="border-t border-gray-200 dark:border-dark-700"
                  >
                    <td class="px-4 py-2 font-mono text-gray-900 dark:text-white">{{ model.model }}</td>
                    <td class="px-4 py-2 text-gray-600 dark:text-gray-300">{{ billingModeLabel(model.billing_mode) }}</td>
                    <template v-if="model.billing_mode === 'per_request'">
                      <td class="px-4 py-2 text-right text-gray-900 dark:text-white" colspan="3">
                        <span class="font-semibold">{{ perRequestPrimary(model.per_request_price) }}</span>
                        <span v-if="perRequestSecondary(model.per_request_price)" class="ml-1 text-xs text-gray-500 dark:text-gray-400">
                          ({{ perRequestSecondary(model.per_request_price) }})
                        </span>
                        <span class="ml-1 text-xs text-gray-500 dark:text-gray-400">/ {{ t('pricing.perRequestUnit') }}</span>
                      </td>
                    </template>
                    <template v-else>
                      <td class="px-4 py-2 text-right text-gray-900 dark:text-white">
                        <span class="font-semibold">{{ tokenPrimary(model.display_input_price) }}</span>
                        <span v-if="tokenSecondary(model.display_input_price)" class="ml-1 text-xs text-gray-500 dark:text-gray-400">
                          ({{ tokenSecondary(model.display_input_price) }})
                        </span>
                      </td>
                      <td class="px-4 py-2 text-right text-gray-900 dark:text-white">
                        <span class="font-semibold">{{ tokenPrimary(model.display_output_price) }}</span>
                        <span v-if="tokenSecondary(model.display_output_price)" class="ml-1 text-xs text-gray-500 dark:text-gray-400">
                          ({{ tokenSecondary(model.display_output_price) }})
                        </span>
                      </td>
                      <td class="px-4 py-2 text-right text-gray-900 dark:text-white">
                        <span class="font-semibold">{{ tokenPrimary(model.display_cache_read_price) }}</span>
                        <span v-if="tokenSecondary(model.display_cache_read_price)" class="ml-1 text-xs text-gray-500 dark:text-gray-400">
                          ({{ tokenSecondary(model.display_cache_read_price) }})
                        </span>
                      </td>
                    </template>
                  </tr>
                </tbody>
              </table>
            </div>

            <p class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('pricing.unitHint') }}
            </p>
          </div>
        </section>
      </template>

      <div v-else class="card p-6 text-sm text-red-600 dark:text-red-400">
        {{ errorMessage }}
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import AppLayout from '@/components/layout/AppLayout.vue'
import { pricingPageAPI, type PricingPageData } from '@/api/pricingPage'
import { useAppStore } from '@/stores'

const { t } = useI18n()
const appStore = useAppStore()

// USD→CNY 换算率，来自管理员在「充值管理」配置（payment_cny_per_usd 公开设置）。
// 0 / 未配置 → 隐藏 banner，单元格只显示美元，不会出现 ¥0 这种诡异显示。
const cnyRate = computed(() => Number(appStore.cachedPublicSettings?.payment_cny_per_usd ?? 0))

const loading = ref(true)
const data = ref<PricingPageData | null>(null)
const errorMessage = ref('')

marked.setOptions({ breaks: true, gfm: true })

const renderedIntro = computed(() => renderMarkdown(data.value?.intro ?? ''))
const renderedEducation = computed(() => renderMarkdown(data.value?.education ?? ''))

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
  try {
    data.value = await pricingPageAPI.getUserPricingPage()
  } catch (err) {
    errorMessage.value = err instanceof Error ? err.message : String(err)
  } finally {
    loading.value = false
  }
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
