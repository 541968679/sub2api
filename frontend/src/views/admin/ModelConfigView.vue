<template>
  <AppLayout>
    <div class="px-4 py-6 sm:px-6" :class="activeTab === 'pricing' ? 'mx-auto max-w-full' : 'mx-auto max-w-7xl'">
      <div class="mb-6">
        <h1 class="text-2xl font-bold text-gray-900 dark:text-white">{{ t('admin.modelConfig.title') }}</h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.modelConfig.description') }}</p>
      </div>

      <!-- Tab Bar -->
      <div class="mb-6 border-b border-gray-200 dark:border-gray-700">
        <nav class="-mb-px flex gap-6">
          <button
            v-for="tab in tabs"
            :key="tab.key"
            @click="activeTab = tab.key"
            class="whitespace-nowrap border-b-2 px-1 pb-3 text-sm font-medium transition-colors"
            :class="activeTab === tab.key
              ? 'border-primary-500 text-primary-600 dark:text-primary-400'
              : 'border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'"
          >
            {{ tab.label }}
          </button>
        </nav>
      </div>

      <!-- Tab Content: Pricing（映射 CRUD 和模型测试已合并到此 tab 内） -->
      <ModelPricingTab v-if="activeTab === 'pricing'" />

      <!-- Tab Content: Rate Multipliers -->
      <RateMultiplierOverview v-if="activeTab === 'rate'" />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { AppLayout } from '@/components/layout'
import ModelPricingTab from '@/components/admin/model-pricing/ModelPricingTab.vue'
import RateMultiplierOverview from '@/components/admin/model-pricing/RateMultiplierOverview.vue'

const { t } = useI18n()
const route = useRoute()

const tabs = computed(() => [
  { key: 'pricing', label: t('admin.modelConfig.tabs.pricing', 'Model Pricing') },
  { key: 'rate', label: t('admin.modelConfig.tabs.rate', 'Rate Multipliers') },
])

// 默认进 pricing tab。历史 URL 可能有 ?tab=mapping，统一回退到 pricing。
const initialTab = (route.query.tab as string) || 'pricing'
const activeTab = ref(initialTab === 'mapping' ? 'pricing' : initialTab)
</script>
