<template>
  <AppLayout>
    <div class="space-y-6">
      <div v-if="loading" class="flex items-center justify-center py-12"><LoadingSpinner /></div>
      <template v-else-if="stats">
        <DashboardAnnouncementBanner />
        <UserDashboardQuickActions />
        <UserDashboardStats :stats="stats" :balance="user?.balance || 0" :is-simple="authStore.isSimpleMode" />
        <section class="space-y-3">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">
            {{ t('dashboard.platformQuota.title') }}
          </h2>
          <UserPlatformQuotaCell :quotas="platformQuotas ?? undefined" />
        </section>
        <UserDashboardCharts v-model:startDate="startDate" v-model:endDate="endDate" v-model:granularity="granularity" :loading="loadingCharts" :trend="trendData" :models="modelStats" @dateRangeChange="loadCharts" @granularityChange="loadCharts" @refresh="refreshAll" />
        <UserDashboardRecentUsage :data="recentUsage" :loading="loadingUsage" />
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { usageAPI, type UserDashboardStats as UserStatsType } from '@/api/usage'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import DashboardAnnouncementBanner from '@/components/user/dashboard/DashboardAnnouncementBanner.vue'
import UserDashboardStats from '@/components/user/dashboard/UserDashboardStats.vue'
import UserDashboardCharts from '@/components/user/dashboard/UserDashboardCharts.vue'
import UserDashboardRecentUsage from '@/components/user/dashboard/UserDashboardRecentUsage.vue'
import UserDashboardQuickActions from '@/components/user/dashboard/UserDashboardQuickActions.vue'
import UserPlatformQuotaCell from '@/components/user/UserPlatformQuotaCell.vue'
import { getMyPlatformQuotas } from '@/api/user'
import type { UsageLog, TrendDataPoint, ModelStat, PlatformQuotaItem } from '@/types'

const { t } = useI18n()
const authStore = useAuthStore()
const user = computed(() => authStore.user)
const stats = ref<UserStatsType | null>(null)
const loading = ref(false)
const loadingUsage = ref(false)
const loadingCharts = ref(false)
const trendData = ref<TrendDataPoint[]>([])
const modelStats = ref<ModelStat[]>([])
const recentUsage = ref<UsageLog[]>([])
const platformQuotas = ref<PlatformQuotaItem[] | null>(null)

const formatLD = (d: Date) => d.toISOString().split('T')[0]
const startDate = ref(formatLD(new Date(Date.now() - 6 * 86400000)))
const endDate = ref(formatLD(new Date()))
const granularity = ref('day')

const loadStats = async () => { loading.value = true; try { await authStore.refreshUser(); stats.value = await usageAPI.getDashboardStats() } catch (error) { console.error('Failed to load dashboard stats:', error) } finally { loading.value = false } }
const loadCharts = async () => { loadingCharts.value = true; try { const res = await Promise.all([usageAPI.getDashboardTrend({ start_date: startDate.value, end_date: endDate.value, granularity: granularity.value as any }), usageAPI.getDashboardModels({ start_date: startDate.value, end_date: endDate.value })]); trendData.value = res[0].trend || []; modelStats.value = res[1].models || [] } catch (error) { console.error('Failed to load charts:', error) } finally { loadingCharts.value = false } }
const loadRecent = async () => { loadingUsage.value = true; try { const res = await usageAPI.getByDateRange(startDate.value, endDate.value); recentUsage.value = res.items.slice(0, 5) } catch (error) { console.error('Failed to load recent usage:', error) } finally { loadingUsage.value = false } }
const loadPlatformQuotas = async () => { try { const response = await getMyPlatformQuotas(); platformQuotas.value = response.platform_quotas } catch (error) { console.error('Failed to load platform quotas:', error); platformQuotas.value = [] } }
const refreshAll = () => { loadStats(); loadCharts(); loadRecent(); loadPlatformQuotas() }

// Keep the summary cards (今日/累计) live, the same way the balance auto-refreshes.
// Without this, a tab left open across midnight keeps showing the previous day's "今日":
// the cards are only fetched in onMounted, while the balance is refreshed by a global
// 60s timer in the auth store — so the balance looks current but the stats are stale.
// Refetch silently (no full-page spinner) on tab focus/visibility and on a light
// visible-only interval, which also corrects the day rollover within ~60s.
const refreshStatsSilently = async () => {
  try { stats.value = await usageAPI.getDashboardStats() } catch (error) { console.error('Failed to refresh dashboard stats:', error) }
}
const onVisible = () => { if (document.visibilityState === 'visible') refreshStatsSilently() }
let statsTimer: ReturnType<typeof setInterval> | null = null

onMounted(() => {
  refreshAll()
  document.addEventListener('visibilitychange', onVisible)
  window.addEventListener('focus', onVisible)
  statsTimer = setInterval(() => { if (document.visibilityState === 'visible') refreshStatsSilently() }, 60000)
})
onBeforeUnmount(() => {
  document.removeEventListener('visibilitychange', onVisible)
  window.removeEventListener('focus', onVisible)
  if (statsTimer) { clearInterval(statsTimer); statsTimer = null }
})
</script>
