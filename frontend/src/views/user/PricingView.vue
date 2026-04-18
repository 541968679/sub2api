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
                        <span class="font-semibold">{{ formatPerRequest(model.per_request_price) }}</span>
                        <span class="ml-1 text-xs text-gray-500 dark:text-gray-400">/ {{ t('pricing.perRequestUnit') }}</span>
                      </td>
                    </template>
                    <template v-else>
                      <td class="px-4 py-2 text-right text-gray-900 dark:text-white">
                        {{ formatTokenPrice(model.display_input_price) }}
                      </td>
                      <td class="px-4 py-2 text-right text-gray-900 dark:text-white">
                        {{ formatTokenPrice(model.display_output_price) }}
                      </td>
                      <td class="px-4 py-2 text-right text-gray-900 dark:text-white">
                        {{ formatTokenPrice(model.display_cache_read_price) }}
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

const { t } = useI18n()

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

// display_*_price is USD per single token; convention across the project is
// to show "per 1M tokens" so users can compare with upstream marketing prices.
function formatTokenPrice(price: number | null | undefined): string {
  if (price == null) return '—'
  const perMillion = price * 1_000_000
  return `$${perMillion.toFixed(2)}`
}

function formatPerRequest(price: number | null | undefined): string {
  if (price == null) return '—'
  return `$${price.toFixed(4)}`
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
