<template>
  <div class="overflow-x-auto rounded-xl border border-gray-200 dark:border-dark-700" data-testid="admin-user-list-row">
    <DataTable
      :columns="columns"
      :data="rows"
      :loading="loading"
      :sticky-first-column="true"
      :sticky-actions-column="true"
      :expandable-actions="true"
      :actions-count="5"
      :virtual-scroll="false"
    >
      <template #cell-email="{ value }">
        <div class="flex items-center gap-2">
          <div class="flex h-8 w-8 items-center justify-center rounded-full bg-primary-100 dark:bg-primary-900/30">
            <span class="text-sm font-medium text-primary-700 dark:text-primary-300">
              {{ emailInitial(value) }}
            </span>
          </div>
          <span class="font-medium text-gray-900 dark:text-white">{{ value }}</span>
        </div>
      </template>

      <template #cell-role="{ value }">
        <span :class="['badge', value === 'admin' ? 'badge-purple' : 'badge-gray']">
          {{ t('admin.users.roles.' + value) }}
        </span>
      </template>

      <template #cell-groups>
        <div v-if="groups.length > 0" class="flex flex-col gap-1">
          <span
            v-if="userGroups.exclusive.length > 0"
            class="group/ex relative inline-flex items-center gap-1 whitespace-nowrap text-xs"
          >
            <Icon name="shield" size="xs" class="h-3.5 w-3.5 text-purple-500 dark:text-purple-400" />
            <span class="font-medium text-purple-600 dark:text-purple-400">
              {{ userGroups.exclusive.length }}
            </span>
            <span class="text-gray-500 dark:text-dark-400">{{ t('admin.users.exclusiveLabel') }}</span>
            <div
              class="pointer-events-none absolute left-0 top-full z-50 mt-1.5 rounded bg-gray-900 px-2.5 py-1.5 text-xs text-white opacity-0 shadow-lg transition-opacity duration-75 group-hover/ex:opacity-100 dark:bg-dark-600"
            >
              <div class="absolute bottom-full left-4 border-4 border-transparent border-b-gray-900 dark:border-b-dark-600"></div>
              <div class="flex flex-col gap-0.5 whitespace-nowrap">
                <span v-for="group in userGroups.exclusive" :key="group.id">
                  {{ group.name }}
                </span>
              </div>
            </div>
          </span>
          <span
            v-if="userGroups.publicGroups.length > 0"
            class="group/pub relative inline-flex cursor-default items-center gap-1 whitespace-nowrap text-xs"
          >
            <Icon name="globe" size="xs" class="h-3.5 w-3.5 text-gray-400 dark:text-dark-500" />
            <span class="font-medium text-gray-600 dark:text-dark-300">
              {{ userGroups.publicGroups.length }}
            </span>
            <span class="text-gray-400 dark:text-dark-500">{{ t('admin.users.publicLabel') }}</span>
            <div
              class="pointer-events-none absolute left-0 top-full z-50 mt-1.5 rounded bg-gray-900 px-2.5 py-1.5 text-xs text-white opacity-0 shadow-lg transition-opacity duration-75 group-hover/pub:opacity-100 dark:bg-dark-600"
            >
              <div class="absolute bottom-full left-4 border-4 border-transparent border-b-gray-900 dark:border-b-dark-600"></div>
              <div class="flex flex-col gap-0.5 whitespace-nowrap">
                <span v-for="group in userGroups.publicGroups" :key="group.id">
                  {{ group.name }}
                </span>
              </div>
            </div>
          </span>
          <span
            v-if="userGroups.exclusive.length === 0 && userGroups.publicGroups.length === 0"
            class="text-xs text-gray-400 dark:text-dark-500"
          >
            -
          </span>
        </div>
        <span v-else class="text-xs text-gray-400 dark:text-dark-500">-</span>
      </template>

      <template #cell-subscriptions="{ row }">
        <div
          v-if="row.subscriptions && row.subscriptions.length > 0"
          class="flex flex-wrap gap-1.5"
        >
          <GroupBadge
            v-for="sub in row.subscriptions"
            :key="sub.id"
            :name="sub.group?.name || ''"
            :platform="sub.group?.platform"
            :subscription-type="sub.group?.subscription_type"
            :rate-multiplier="sub.group?.rate_multiplier"
            :days-remaining="sub.expires_at ? getSubscriptionDaysRemaining(sub.expires_at) : null"
            :title="sub.expires_at ? formatDateTime(sub.expires_at) : ''"
          />
        </div>
        <span
          v-else
          class="inline-flex items-center gap-1.5 rounded-md bg-gray-50 px-2 py-1 text-xs text-gray-400 dark:bg-dark-700/50 dark:text-dark-500"
        >
          <Icon name="ban" size="xs" class="h-3.5 w-3.5" />
          <span>{{ t('admin.users.noSubscription') }}</span>
        </span>
      </template>

      <template #cell-balance="{ value }">
        <span class="font-medium text-gray-900 dark:text-white">${{ Number(value ?? 0).toFixed(2) }}</span>
      </template>

      <template #cell-burn_rate>
        <UserBurnRateCell :stats="burnRateStats" />
      </template>

      <template #cell-usage>
        <div class="text-sm">
          <div class="flex items-center gap-1.5">
            <span class="text-gray-500 dark:text-gray-400">{{ t('admin.users.today') }}:</span>
            <span class="font-medium text-gray-900 dark:text-white">
              ${{ (usageStats?.today_actual_cost ?? 0).toFixed(4) }}
            </span>
          </div>
          <div class="mt-0.5 flex items-center gap-1.5">
            <span class="text-gray-500 dark:text-gray-400">{{ t('admin.users.total') }}:</span>
            <span class="font-medium text-gray-900 dark:text-white">
              ${{ (usageStats?.total_actual_cost ?? 0).toFixed(4) }}
            </span>
          </div>
        </div>
      </template>

      <template #cell-concurrency="{ row }">
        <UserConcurrencyCell
          :current="row.current_concurrency ?? 0"
          :max="row.concurrency"
        />
      </template>

      <template #header-smart_schedule="{ column }">
        <div class="flex items-center">
          <span>{{ column.label }}</span>
          <HelpTooltip :content="t('admin.users.smartSchedule.columnHint')" width-class="w-72" />
        </div>
      </template>
      <template #cell-smart_schedule>
        <span
          v-if="smartScheduleLoading"
          class="text-xs text-gray-400 dark:text-gray-500"
        >
          {{ t('common.loading') }}
        </span>
        <span
          v-else-if="!(smartSchedule?.enabled_platforms?.length)"
          class="text-xs text-gray-400 dark:text-gray-500"
        >
          {{ t('admin.users.smartSchedule.off') }}
        </span>
        <span v-else class="flex flex-wrap gap-1">
          <span
            v-for="platform in smartSchedule.enabled_platforms"
            :key="platform"
            class="inline-flex items-center rounded-full bg-emerald-50 px-2 py-0.5 text-xs font-medium text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300"
          >
            {{ t(`admin.groups.platforms.${platform}`) }}
            ·
            {{ smartSchedule.pool_counts?.[platform] ?? 0 }}
          </span>
        </span>
      </template>

      <template #header-schedule_pnl="{ column }">
        <div class="flex items-center">
          <span>{{ column.label }}</span>
          <HelpTooltip :content="t('admin.users.schedulePnl.columnHint')" width-class="w-72" />
        </div>
      </template>
      <template #cell-schedule_pnl>
        <UserSchedulePnlCell
          :summary="schedulePnl"
          :loading="schedulePnlLoading"
          @click="emit('open-schedule-pnl')"
        />
      </template>

      <template #header-quality_ttft="{ column }">
        <div class="flex items-center">
          <span>{{ column.label }}</span>
          <HelpTooltip :content="t('admin.users.quality.ttftHint')" width-class="w-72" />
        </div>
      </template>
      <template #cell-quality_ttft>
        <AccountQualityCell
          mode="ttft"
          :stats="qualityStats"
          :loading="qualityLoading"
          :error="qualityError"
        />
      </template>
      <template #header-quality_success_rate="{ column }">
        <div class="flex items-center">
          <span>{{ column.label }}</span>
          <HelpTooltip :content="t('admin.users.quality.successRateHint')" width-class="w-80" />
        </div>
      </template>
      <template #cell-quality_success_rate>
        <AccountQualityCell
          mode="success_rate"
          :stats="qualityStats"
          :loading="qualityLoading"
          :error="qualityError"
        />
      </template>

      <template #cell-status="{ value }">
        <div class="flex items-center gap-1.5">
          <span
            :class="['inline-block h-2 w-2 rounded-full', getAdminUserStatusDotClass(value)]"
          ></span>
          <span :class="['text-sm', getAdminUserStatusTextClass(value)]">
            {{ getAdminUserStatusLabel(value, t) }}
          </span>
        </div>
      </template>

      <template #cell-created_at="{ value }">
        <span class="text-sm text-gray-500 dark:text-dark-400">{{ value ? formatDateTime(value) : '-' }}</span>
      </template>
      <template #cell-last_used_at="{ value }">
        <span class="text-sm text-gray-500 dark:text-dark-400">
          {{ value ? formatDateTime(value) : '-' }}
        </span>
      </template>
      <template #cell-last_active_at="{ value }">
        <span class="text-sm text-gray-500 dark:text-dark-400">
          {{ value ? formatDateTime(value) : '-' }}
        </span>
      </template>
      <template #cell-actions="{ row }">
        <AdminUserListRowActions :user="row" @updated="emit('updated')" @deleted="emit('deleted')" />
      </template>
    </DataTable>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AccountQualityStats } from '@/api/admin/accounts'
import type { BatchUserBurnRateStats, BatchUserUsageStats } from '@/api/admin/dashboard'
import type { SchedulePnlSummary, SmartScheduleSummary } from '@/api/admin/users'
import type { AdminGroup, AdminUser } from '@/types'
import { formatDateTime } from '@/utils/format'
import {
  buildAdminUserListRowColumns,
  getAdminUserStatusDotClass,
  getAdminUserStatusLabel,
  getAdminUserStatusTextClass,
  getSubscriptionDaysRemaining,
  resolveAdminUserGroups
} from '@/composables/adminUserListRow'
import DataTable from '@/components/common/DataTable.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import Icon from '@/components/icons/Icon.vue'
import AccountQualityCell from '@/components/account/AccountQualityCell.vue'
import UserBurnRateCell from '@/components/user/UserBurnRateCell.vue'
import UserConcurrencyCell from '@/components/user/UserConcurrencyCell.vue'
import AdminUserListRowActions from '@/components/admin/user/AdminUserListRowActions.vue'
import UserSchedulePnlCell from '@/components/admin/user/UserSchedulePnlCell.vue'

const props = defineProps<{
  user: AdminUser
  groups: AdminGroup[]
  loading?: boolean
  qualityStats?: AccountQualityStats | null
  qualityLoading?: boolean
  qualityError?: string | null
  usageStats?: BatchUserUsageStats | null
  burnRateStats?: BatchUserBurnRateStats | null
  smartSchedule?: SmartScheduleSummary | null
  smartScheduleLoading?: boolean
  schedulePnl?: SchedulePnlSummary | null
  schedulePnlLoading?: boolean
}>()

const emit = defineEmits<{
  updated: []
  deleted: []
  'open-schedule-pnl': []
}>()

const { t } = useI18n()
const columns = computed(() => buildAdminUserListRowColumns(t))
const rows = computed(() => [props.user])
const userGroups = computed(() => resolveAdminUserGroups(props.user, props.groups))

function emailInitial(value: unknown) {
  return typeof value === 'string' && value.length > 0 ? value.charAt(0).toUpperCase() : '?'
}
</script>
