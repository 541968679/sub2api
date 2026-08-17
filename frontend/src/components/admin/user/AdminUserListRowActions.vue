<template>
  <div class="flex items-center gap-1" data-testid="admin-user-list-row-actions">
    <button
      type="button"
      data-testid="user-row-edit"
      class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400"
      @click="handleEdit"
    >
      <Icon name="edit" size="sm" />
      <span class="text-xs">{{ t('common.edit') }}</span>
    </button>

    <button
      type="button"
      data-testid="user-view-usage"
      class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-blue-50 hover:text-blue-600 dark:hover:bg-blue-900/20 dark:hover:text-blue-400"
      :title="t('admin.users.viewUsage')"
      @click="handleViewUsage"
    >
      <Icon name="chartBar" size="sm" />
      <span class="text-xs">{{ t('admin.users.viewUsageShort') }}</span>
    </button>

    <button
      type="button"
      data-testid="user-view-error-requests"
      class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-amber-50 hover:text-amber-600 dark:hover:bg-amber-900/20 dark:hover:text-amber-400"
      :title="t('admin.users.viewErrorRequests')"
      @click="handleViewErrorRequests"
    >
      <Icon name="exclamationTriangle" size="sm" />
      <span class="text-xs">{{ t('admin.users.viewErrorRequestsShort') }}</span>
    </button>

    <button
      v-if="user.role !== 'admin'"
      type="button"
      data-testid="user-row-toggle-status"
      :class="[
        'flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors',
        toggleStatusButtonClass
      ]"
      @click="handleToggleStatus"
    >
      <Icon v-if="user.status === 'active'" name="ban" size="sm" />
      <Icon v-else name="checkCircle" size="sm" />
      <span class="text-xs">{{ toggleStatusLabel }}</span>
    </button>

    <button
      type="button"
      data-testid="user-row-more"
      class="action-menu-trigger flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-900 dark:hover:bg-dark-700 dark:hover:text-white"
      :class="{ 'bg-gray-100 text-gray-900 dark:bg-dark-700 dark:text-white': menuOpen }"
      @click="openActionMenu($event)"
    >
      <Icon name="more" size="sm" />
      <span class="text-xs">{{ t('common.more') }}</span>
    </button>
  </div>

  <Teleport to="body">
    <div
      v-if="menuOpen && menuPosition"
      class="action-menu-content fixed z-[9999] w-48 overflow-hidden rounded-xl bg-white shadow-lg ring-1 ring-black/5 dark:bg-dark-800 dark:ring-white/10"
      :style="{ top: menuPosition.top + 'px', left: menuPosition.left + 'px' }"
      data-testid="user-row-action-menu"
    >
      <div class="py-1">
        <button
          type="button"
          class="flex w-full items-center gap-2 px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700"
          @click="handleViewApiKeys(); closeActionMenu()"
        >
          <Icon name="key" size="sm" class="text-gray-400" :stroke-width="2" />
          {{ t('admin.users.apiKeys') }}
        </button>
        <button
          type="button"
          class="flex w-full items-center gap-2 px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700"
          @click="handleAllowedGroups(); closeActionMenu()"
        >
          <Icon name="users" size="sm" class="text-gray-400" :stroke-width="2" />
          {{ t('admin.users.groups') }}
        </button>
        <button
          type="button"
          class="flex w-full items-center gap-2 px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700"
          @click="handleModelPricing(); closeActionMenu()"
        >
          <Icon name="dollar" size="sm" class="text-gray-400" :stroke-width="2" />
          {{ t('admin.users.modelPricing') }}
        </button>
        <button
          type="button"
          class="flex w-full items-center gap-2 px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700"
          @click="handlePlatformQuota(); closeActionMenu()"
        >
          <Icon name="chartBar" size="sm" class="text-gray-400" :stroke-width="2" />
          {{ t('admin.users.platformQuota.menuItem') }}
        </button>
        <div class="my-1 border-t border-gray-100 dark:border-dark-700"></div>
        <button
          type="button"
          class="flex w-full items-center gap-2 px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700"
          @click="handleDeposit(); closeActionMenu()"
        >
          <Icon name="plus" size="sm" class="text-emerald-500" :stroke-width="2" />
          {{ t('admin.users.deposit') }}
        </button>
        <button
          type="button"
          class="flex w-full items-center gap-2 px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700"
          @click="handleWithdraw(); closeActionMenu()"
        >
          <svg class="h-4 w-4 text-amber-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 12H4" />
          </svg>
          {{ t('admin.users.withdraw') }}
        </button>
        <button
          type="button"
          class="flex w-full items-center gap-2 px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700"
          @click="handleBalanceHistory(); closeActionMenu()"
        >
          <Icon name="dollar" size="sm" class="text-gray-400" :stroke-width="2" />
          {{ t('admin.users.balanceHistory') }}
        </button>
        <button
          type="button"
          class="flex w-full items-center gap-2 px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700"
          @click="handleBalanceHistoryManage(); closeActionMenu()"
        >
          <Icon name="trash" size="sm" class="text-gray-400" :stroke-width="2" />
          {{ t('admin.users.balanceHistoryManage') }}
        </button>
        <div class="my-1 border-t border-gray-100 dark:border-dark-700"></div>
        <button
          v-if="user.role !== 'admin'"
          type="button"
          class="flex w-full items-center gap-2 px-4 py-2 text-sm text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20"
          @click="handleDelete(); closeActionMenu()"
        >
          <Icon name="trash" size="sm" :stroke-width="2" />
          {{ t('common.delete') }}
        </button>
      </div>
    </div>
  </Teleport>

  <ConfirmDialog
    :show="showDeleteDialog"
    :title="t('admin.users.deleteUser')"
    :message="t('admin.users.deleteConfirm', { email: user.email })"
    :danger="true"
    @confirm="confirmDelete"
    @cancel="showDeleteDialog = false"
  />
  <UserEditModal :show="showEditModal" :user="user" @close="closeEditModal" @success="emit('updated')" />
  <UserApiKeysModal :show="showApiKeysModal" :user="user" @close="showApiKeysModal = false" />
  <UserAllowedGroupsModal
    :show="showAllowedGroupsModal"
    :user="user"
    @close="showAllowedGroupsModal = false"
    @success="emit('updated')"
  />
  <UserModelPricingModal
    :show="showModelPricingModal"
    :user="user"
    @close="showModelPricingModal = false"
    @success="emit('updated')"
  />
  <UserPlatformQuotaModal
    :show="showPlatformQuotaModal"
    :user="user"
    @close="showPlatformQuotaModal = false"
    @success="emit('updated')"
  />
  <UserBalanceModal
    :show="showBalanceModal"
    :user="user"
    :operation="balanceOperation"
    @close="showBalanceModal = false"
    @success="emit('updated')"
  />
  <UserBalanceHistoryModal
    :show="showBalanceHistoryModal"
    :user="user"
    @close="showBalanceHistoryModal = false"
    @deposit="handleDepositFromHistory"
    @withdraw="handleWithdrawFromHistory"
  />
  <UserBalanceHistoryManageModal
    :show="showBalanceHistoryManageModal"
    :user="user"
    @close="showBalanceHistoryManageModal = false"
  />
  <UsageErrorInspectDialog
    :show="inspectOpen"
    scope="user"
    :subject-id="user.id"
    :subject-label="user.email || user.username || ''"
    :initial-tab="inspectTab"
    @close="inspectOpen = false"
  />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AdminUser } from '@/types'
import { useAdminUserRowActions } from '@/composables/useAdminUserRowActions'
import Icon from '@/components/icons/Icon.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import UserEditModal from '@/components/admin/user/UserEditModal.vue'
import UserApiKeysModal from '@/components/admin/user/UserApiKeysModal.vue'
import UserAllowedGroupsModal from '@/components/admin/user/UserAllowedGroupsModal.vue'
import UserModelPricingModal from '@/components/admin/user/UserModelPricingModal.vue'
import UserPlatformQuotaModal from '@/components/admin/user/UserPlatformQuotaModal.vue'
import UserBalanceModal from '@/components/admin/user/UserBalanceModal.vue'
import UserBalanceHistoryModal from '@/components/admin/user/UserBalanceHistoryModal.vue'
import UserBalanceHistoryManageModal from '@/components/admin/user/UserBalanceHistoryManageModal.vue'
import UsageErrorInspectDialog from '@/components/admin/usage/UsageErrorInspectDialog.vue'

const props = defineProps<{
  user: AdminUser
}>()

const emit = defineEmits<{
  updated: []
  deleted: []
}>()

const { t } = useI18n()

const {
  showEditModal,
  showDeleteDialog,
  showApiKeysModal,
  showAllowedGroupsModal,
  showModelPricingModal,
  showPlatformQuotaModal,
  showBalanceModal,
  showBalanceHistoryModal,
  showBalanceHistoryManageModal,
  balanceOperation,
  inspectOpen,
  inspectTab,
  menuOpen,
  menuPosition,
  closeActionMenu,
  openActionMenu,
  handleEdit,
  closeEditModal,
  handleViewUsage,
  handleViewErrorRequests,
  handleToggleStatus,
  handleViewApiKeys,
  handleAllowedGroups,
  handleModelPricing,
  handlePlatformQuota,
  handleDeposit,
  handleWithdraw,
  handleBalanceHistory,
  handleBalanceHistoryManage,
  handleDelete,
  confirmDelete,
  handleDepositFromHistory,
  handleWithdrawFromHistory
} = useAdminUserRowActions({
  getUser: () => props.user,
  onChanged: () => emit('updated'),
  onDeleted: () => emit('deleted')
})

const toggleStatusLabel = computed(() => {
  if (props.user.status === 'pending_approval') return t('admin.users.approve')
  return props.user.status === 'active' ? t('admin.users.disable') : t('admin.users.enable')
})

const toggleStatusButtonClass = computed(() =>
  props.user.status === 'active'
    ? 'hover:bg-orange-50 hover:text-orange-600 dark:hover:bg-orange-900/20 dark:hover:text-orange-400'
    : 'hover:bg-green-50 hover:text-green-600 dark:hover:bg-green-900/20 dark:hover:text-green-400'
)
</script>
