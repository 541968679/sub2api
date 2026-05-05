<template>
  <div class="card">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('dashboard.quickActions') }}</h2>
    </div>
    <div class="p-4 space-y-3">
      <!-- Row 1: Primary actions (large cards) -->
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <!-- 充值/订阅 -->
        <button
          v-if="showTopUp"
          @click="router.push('/purchase')"
          class="primary-card group"
          style="--card-from: #fef3c7; --card-to: #ffedd5; --card-from-dark: rgba(245,158,11,0.12); --card-to-dark: rgba(234,88,12,0.12); --card-border: rgba(245,158,11,0.3); --card-border-dark: rgba(245,158,11,0.2);"
        >
          <div class="primary-card-icon bg-gradient-to-br from-amber-400 to-orange-500">
            <svg class="h-7 w-7 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M2.25 18.75a60.07 60.07 0 0115.797 2.101c.727.198 1.453-.342 1.453-1.096V18.75M3.75 4.5v.75A.75.75 0 013 6h-.75m0 0v-.375c0-.621.504-1.125 1.125-1.125H20.25M2.25 6v9m18-10.5v.75c0 .414.336.75.75.75h.75m-1.5-1.5h.375c.621 0 1.125.504 1.125 1.125v9.75c0 .621-.504 1.125-1.125 1.125h-.375m1.5-1.5H21a.75.75 0 00-.75.75v.75m0 0H3.75m0 0h-.375a1.125 1.125 0 01-1.125-1.125V15m1.5 1.5v-.75A.75.75 0 003 15h-.75M15 10.5a3 3 0 11-6 0 3 3 0 016 0zm3 0h.008v.008H18V10.5zm-12 0h.008v.008H6V10.5z" />
            </svg>
          </div>
          <p class="primary-card-title">{{ t('dashboard.topUp') }}</p>
          <p class="primary-card-desc">{{ t('dashboard.topUpHint') }}</p>
        </button>

        <!-- 查看教程 -->
        <button
          @click="router.push('/tutorial')"
          class="primary-card group"
          style="--card-from: #ede9fe; --card-to: #e0e7ff; --card-from-dark: rgba(99,102,241,0.12); --card-to-dark: rgba(79,70,229,0.12); --card-border: rgba(99,102,241,0.3); --card-border-dark: rgba(99,102,241,0.2);"
        >
          <div class="primary-card-icon bg-gradient-to-br from-violet-400 to-indigo-500">
            <svg class="h-7 w-7 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 6.042A8.967 8.967 0 006 3.75c-1.052 0-2.062.18-3 .512v14.25A8.987 8.987 0 016 18c2.305 0 4.408.867 6 2.292m0-14.25a8.966 8.966 0 016-2.292c1.052 0 2.062.18 3 .512v14.25A8.987 8.987 0 0018 18a8.967 8.967 0 00-6 2.292m0-14.25v14.25" />
            </svg>
          </div>
          <p class="primary-card-title">{{ t('dashboard.viewTutorial') }}</p>
          <p class="primary-card-desc">{{ t('dashboard.viewTutorialHint') }}</p>
        </button>

        <!-- 获取 API Key -->
        <button
          @click="router.push('/keys')"
          class="primary-card group"
          style="--card-from: #dbeafe; --card-to: #e0f2fe; --card-from-dark: rgba(59,130,246,0.12); --card-to-dark: rgba(14,165,233,0.12); --card-border: rgba(59,130,246,0.3); --card-border-dark: rgba(59,130,246,0.2);"
        >
          <div class="primary-card-icon bg-gradient-to-br from-blue-400 to-sky-500">
            <svg class="h-7 w-7 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M15.75 5.25a3 3 0 013 3m3 0a6 6 0 01-7.029 5.912c-.563-.097-1.159.026-1.563.43L10.5 17.25H8.25v2.25H6v2.25H2.25v-2.818c0-.597.237-1.17.659-1.591l6.499-6.499c.404-.404.527-1 .43-1.563A6 6 0 1121.75 8.25z" />
            </svg>
          </div>
          <p class="primary-card-title">{{ t('dashboard.getApiKey') }}</p>
          <p class="primary-card-desc">{{ t('dashboard.getApiKeyHint') }}</p>
        </button>
      </div>

      <!-- Row 2: Secondary actions (compact) -->
      <div class="grid grid-cols-2 gap-2 sm:grid-cols-3">
        <button @click="router.push('/usage')" class="secondary-card group">
          <div class="secondary-card-icon bg-emerald-100 dark:bg-emerald-900/30">
            <Icon name="chart" size="md" class="text-emerald-600 dark:text-emerald-400" />
          </div>
          <span class="secondary-card-title">{{ t('dashboard.viewUsage') }}</span>
        </button>

        <button @click="router.push('/redeem')" class="secondary-card group">
          <div class="secondary-card-icon bg-amber-100 dark:bg-amber-900/30">
            <Icon name="gift" size="md" class="text-amber-600 dark:text-amber-400" />
          </div>
          <span class="secondary-card-title">{{ t('dashboard.redeemCode') }}</span>
        </button>

        <button @click="router.push('/pricing')" class="secondary-card group">
          <div class="secondary-card-icon bg-rose-100 dark:bg-rose-900/30">
            <svg class="h-5 w-5 text-rose-600 dark:text-rose-400" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9.568 3H5.25A2.25 2.25 0 003 5.25v4.318c0 .597.237 1.17.659 1.591l9.581 9.581c.699.699 1.78.872 2.607.33a18.095 18.095 0 005.223-5.223c.542-.827.369-1.908-.33-2.607L11.16 3.66A2.25 2.25 0 009.568 3z" />
            </svg>
          </div>
          <span class="secondary-card-title">{{ t('dashboard.viewPricing') }}</span>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import Icon from '@/components/icons/Icon.vue'
const router = useRouter()
const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const showTopUp = computed(() => appStore.cachedPublicSettings?.payment_enabled && !authStore.isSimpleMode)
</script>

<style scoped>
/* Row 1: Primary large cards */
.primary-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
  padding: 1.5rem 1rem;
  border-radius: 1rem;
  border: 1px solid var(--card-border);
  background: linear-gradient(135deg, var(--card-from), var(--card-to));
  text-align: center;
  transition: all 0.2s;
  cursor: pointer;
}
.primary-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 25px rgba(0, 0, 0, 0.08);
}
:root.dark .primary-card {
  border-color: var(--card-border-dark);
  background: linear-gradient(135deg, var(--card-from-dark), var(--card-to-dark));
}
:root.dark .primary-card:hover {
  box-shadow: 0 8px 25px rgba(0, 0, 0, 0.3);
}

.primary-card-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 3.5rem;
  height: 3.5rem;
  border-radius: 1rem;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  transition: transform 0.2s;
}
.primary-card:hover .primary-card-icon {
  transform: scale(1.08);
}

.primary-card-title {
  font-size: 0.9375rem;
  font-weight: 600;
  color: #111827;
  margin: 0;
}
:root.dark .primary-card-title {
  color: #f3f4f6;
}

.primary-card-desc {
  font-size: 0.75rem;
  color: #6b7280;
  margin: 0;
  line-height: 1.4;
}
:root.dark .primary-card-desc {
  color: #9ca3af;
}

/* Row 2: Secondary compact cards */
.secondary-card {
  display: flex;
  align-items: center;
  gap: 0.625rem;
  padding: 0.75rem;
  border-radius: 0.75rem;
  background: #f9fafb;
  transition: all 0.15s;
  cursor: pointer;
  border: none;
  text-align: left;
}
.secondary-card:hover {
  background: #f3f4f6;
}
:root.dark .secondary-card {
  background: rgba(31, 41, 55, 0.5);
}
:root.dark .secondary-card:hover {
  background: #1f2937;
}

.secondary-card-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2.25rem;
  height: 2.25rem;
  border-radius: 0.625rem;
  flex-shrink: 0;
}

.secondary-card-title {
  font-size: 0.8125rem;
  font-weight: 500;
  color: #374151;
}
:root.dark .secondary-card-title {
  color: #d1d5db;
}
</style>
