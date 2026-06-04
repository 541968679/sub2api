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

      <button @click="router.push('/distribution')" class="agent-card group">
        <div class="agent-card-glow"></div>
        <div class="agent-card-content">
          <div class="agent-card-icon">
            <svg class="h-7 w-7 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 6v12m-3.5-7.5h7M5.25 5.25h13.5a1.5 1.5 0 011.5 1.5v10.5a1.5 1.5 0 01-1.5 1.5H5.25a1.5 1.5 0 01-1.5-1.5V6.75a1.5 1.5 0 011.5-1.5z" />
            </svg>
          </div>
          <div class="min-w-0 flex-1 text-left">
            <p class="agent-card-eyebrow">{{ t('dashboard.agentEntryEyebrow') }}</p>
            <p class="agent-card-title">{{ t('dashboard.becomeAgent') }}</p>
            <p class="agent-card-desc">{{ t('dashboard.becomeAgentHint') }}</p>
          </div>
          <span class="agent-card-cta">{{ t('dashboard.becomeAgentAction') }}</span>
        </div>
      </button>

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

      <SupportContactBar />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import SupportContactBar from '@/components/common/SupportContactBar.vue'
import Icon from '@/components/icons/Icon.vue'
const router = useRouter()
const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const showTopUp = computed(() => appStore.cachedPublicSettings?.payment_enabled && !authStore.isSimpleMode)
</script>

<style scoped>
.agent-card {
  position: relative;
  isolation: isolate;
  display: block;
  width: 100%;
  overflow: hidden;
  border-radius: 1rem;
  border: 1px solid rgba(20, 184, 166, 0.28);
  background:
    radial-gradient(circle at 14% 20%, rgba(251, 191, 36, 0.26), transparent 30%),
    linear-gradient(135deg, #ecfdf5 0%, #fef3c7 52%, #e0f2fe 100%);
  padding: 1px;
  cursor: pointer;
  transition:
    transform 0.2s ease,
    box-shadow 0.2s ease,
    border-color 0.2s ease;
  box-shadow: 0 14px 34px rgba(20, 184, 166, 0.13);
}

.agent-card:hover {
  transform: translateY(-2px);
  border-color: rgba(245, 158, 11, 0.5);
  box-shadow: 0 18px 42px rgba(245, 158, 11, 0.18);
}

.agent-card-glow {
  position: absolute;
  inset: 0;
  z-index: -1;
  background: linear-gradient(110deg, transparent 0%, rgba(255, 255, 255, 0.82) 45%, transparent 70%);
  transform: translateX(-120%);
  animation: agent-card-sheen 4s ease-in-out infinite;
}

.agent-card-content {
  display: flex;
  align-items: center;
  gap: 1rem;
  min-height: 6rem;
  border-radius: calc(1rem - 1px);
  padding: 1rem;
  background: rgba(255, 255, 255, 0.58);
}

.agent-card-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 3.5rem;
  height: 3.5rem;
  flex-shrink: 0;
  border-radius: 1rem;
  background: linear-gradient(135deg, #f59e0b, #14b8a6);
  box-shadow: 0 12px 24px rgba(20, 184, 166, 0.22);
  transition: transform 0.2s ease;
}

.agent-card:hover .agent-card-icon {
  transform: scale(1.06) rotate(-2deg);
}

.agent-card-eyebrow {
  margin: 0 0 0.25rem;
  font-size: 0.6875rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: #0f766e;
}

.agent-card-title {
  margin: 0;
  font-size: 1rem;
  font-weight: 700;
  color: #111827;
}

.agent-card-desc {
  margin: 0.25rem 0 0;
  max-width: 42rem;
  font-size: 0.8125rem;
  line-height: 1.45;
  color: #4b5563;
}

.agent-card-cta {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: max-content;
  border-radius: 9999px;
  padding: 0.5rem 0.875rem;
  background: #111827;
  color: #fff;
  font-size: 0.8125rem;
  font-weight: 600;
  box-shadow: 0 10px 20px rgba(17, 24, 39, 0.14);
}

:root.dark .agent-card {
  border-color: rgba(20, 184, 166, 0.26);
  background:
    radial-gradient(circle at 14% 20%, rgba(245, 158, 11, 0.2), transparent 30%),
    linear-gradient(135deg, rgba(6, 78, 59, 0.64), rgba(120, 53, 15, 0.44), rgba(30, 64, 175, 0.36));
  box-shadow: 0 14px 34px rgba(0, 0, 0, 0.32);
}

:root.dark .agent-card-content {
  background: rgba(17, 24, 39, 0.66);
}

:root.dark .agent-card-eyebrow {
  color: #5eead4;
}

:root.dark .agent-card-title {
  color: #f9fafb;
}

:root.dark .agent-card-desc {
  color: #d1d5db;
}

:root.dark .agent-card-cta {
  background: rgba(255, 255, 255, 0.92);
  color: #111827;
}

@media (max-width: 640px) {
  .agent-card-content {
    align-items: flex-start;
    flex-direction: column;
  }

  .agent-card-cta {
    width: 100%;
  }
}

@keyframes agent-card-sheen {
  0%,
  56% {
    transform: translateX(-120%);
  }
  82%,
  100% {
    transform: translateX(120%);
  }
}

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
