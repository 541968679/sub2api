import { onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { AdminUser } from '@/types'
import { getAdminUserToggleStatusTarget } from '@/composables/adminUserListRow'

export function useAdminUserRowActions(options: {
  getUser: () => AdminUser
  onChanged: () => void
  onDeleted?: () => void
}) {
  const { t } = useI18n()
  const appStore = useAppStore()

  const showEditModal = ref(false)
  const showDeleteDialog = ref(false)
  const showApiKeysModal = ref(false)
  const showAllowedGroupsModal = ref(false)
  const showModelPricingModal = ref(false)
  const showPlatformQuotaModal = ref(false)
  const showBalanceModal = ref(false)
  const showBalanceHistoryModal = ref(false)
  const showBalanceHistoryManageModal = ref(false)
  const balanceOperation = ref<'add' | 'subtract'>('add')
  const inspectOpen = ref(false)
  const inspectTab = ref<'usage' | 'errors'>('usage')
  const menuOpen = ref(false)
  const menuPosition = ref<{ top: number; left: number } | null>(null)

  function currentUser() {
    return options.getUser()
  }

  function closeActionMenu() {
    menuOpen.value = false
    menuPosition.value = null
  }

  function openActionMenu(event: MouseEvent) {
    if (menuOpen.value) {
      closeActionMenu()
      return
    }
    const padding = 8
    const menuWidth = 200
    const menuHeight = 240
    const viewportWidth = window.innerWidth
    const viewportHeight = window.innerHeight
    const left = Math.max(padding, Math.min(event.clientX - menuWidth, viewportWidth - menuWidth - padding))
    let top = event.clientY
    if (top + menuHeight > viewportHeight - padding) {
      top = Math.max(padding, event.clientY - menuHeight)
    }
    menuPosition.value = { top, left }
    menuOpen.value = true
  }

  function handleClickOutside(event: MouseEvent) {
    const target = event.target as HTMLElement
    if (!target.closest('.action-menu-trigger') && !target.closest('.action-menu-content')) {
      closeActionMenu()
    }
  }

  function handleScroll() {
    closeActionMenu()
  }

  function handleEdit() {
    showEditModal.value = true
  }

  function closeEditModal() {
    showEditModal.value = false
  }

  function handleViewUsage() {
    inspectTab.value = 'usage'
    inspectOpen.value = true
  }

  function handleViewErrorRequests() {
    inspectTab.value = 'errors'
    inspectOpen.value = true
  }

  async function handleToggleStatus() {
    const user = currentUser()
    const newStatus = getAdminUserToggleStatusTarget(user.status)
    try {
      await adminAPI.users.toggleStatus(user.id, newStatus)
      appStore.showSuccess(
        user.status === 'pending_approval'
          ? t('admin.users.userApproved')
          : newStatus === 'active'
            ? t('admin.users.userEnabled')
            : t('admin.users.userDisabled')
      )
      options.onChanged()
    } catch (error: unknown) {
      appStore.showError(extractApiErrorMessage(error, t('admin.users.failedToToggle')))
    }
  }

  function handleViewApiKeys() {
    showApiKeysModal.value = true
  }

  function handleAllowedGroups() {
    showAllowedGroupsModal.value = true
  }

  function handleModelPricing() {
    showModelPricingModal.value = true
  }

  function handlePlatformQuota() {
    showPlatformQuotaModal.value = true
  }

  function handleDeposit() {
    balanceOperation.value = 'add'
    showBalanceModal.value = true
  }

  function handleWithdraw() {
    balanceOperation.value = 'subtract'
    showBalanceModal.value = true
  }

  function handleBalanceHistory() {
    showBalanceHistoryModal.value = true
  }

  function handleBalanceHistoryManage() {
    showBalanceHistoryManageModal.value = true
  }

  function handleDelete() {
    showDeleteDialog.value = true
  }

  async function confirmDelete() {
    const user = currentUser()
    try {
      await adminAPI.users.delete(user.id)
      appStore.showSuccess(t('common.success'))
      showDeleteDialog.value = false
      options.onDeleted?.()
    } catch (error: unknown) {
      appStore.showError(extractApiErrorMessage(error, t('admin.users.failedToDelete')))
    }
  }

  function handleDepositFromHistory() {
    handleDeposit()
  }

  function handleWithdrawFromHistory() {
    handleWithdraw()
  }

  onMounted(() => {
    document.addEventListener('click', handleClickOutside)
    window.addEventListener('scroll', handleScroll, true)
  })

  onUnmounted(() => {
    document.removeEventListener('click', handleClickOutside)
    window.removeEventListener('scroll', handleScroll, true)
  })

  return {
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
  }
}
