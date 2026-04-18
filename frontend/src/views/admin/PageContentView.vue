<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-4xl space-y-6">
      <!-- Shared header -->
      <div>
        <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
          {{ t('admin.pageContent.title') }}
        </h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.pageContent.description') }}
        </p>
      </div>

      <!-- Tab bar (same visual pattern as ModelConfigView) -->
      <div class="border-b border-gray-200 dark:border-gray-700">
        <nav class="-mb-px flex gap-6">
          <button
            v-for="tab in tabs"
            :key="tab.key"
            type="button"
            class="whitespace-nowrap border-b-2 px-1 pb-3 text-sm font-medium transition-colors"
            :class="activeTab === tab.key
              ? 'border-primary-500 text-primary-600 dark:text-primary-400'
              : 'border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'"
            @click="selectTab(tab.key)"
          >
            {{ tab.label }}
          </button>
        </nav>
      </div>

      <!-- Tab content: each form keeps its own load/save state; switching tabs does not unmount and remount thanks to keep-alive. -->
      <KeepAlive>
        <PricingContentForm v-if="activeTab === 'pricing'" />
        <LoginContentForm v-else-if="activeTab === 'login'" />
      </KeepAlive>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import PricingContentForm from '@/components/admin/page-content/PricingContentForm.vue'
import LoginContentForm from '@/components/admin/page-content/LoginContentForm.vue'

type TabKey = 'pricing' | 'login'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const tabs = computed(() => [
  { key: 'pricing' as TabKey, label: t('admin.pageContent.tabs.pricing') },
  { key: 'login' as TabKey, label: t('admin.pageContent.tabs.login') }
])

function normalizeTab(v: unknown): TabKey {
  return v === 'login' ? 'login' : 'pricing'
}

const activeTab = ref<TabKey>(normalizeTab(route.query.tab))

// Sync query param → active tab when navigating via back/forward or deeplink.
watch(
  () => route.query.tab,
  (v) => {
    activeTab.value = normalizeTab(v)
  }
)

// Switching tabs rewrites the URL so bookmarks survive refresh.
function selectTab(key: TabKey) {
  if (activeTab.value === key) return
  activeTab.value = key
  router.replace({ query: { ...route.query, tab: key } })
}
</script>
