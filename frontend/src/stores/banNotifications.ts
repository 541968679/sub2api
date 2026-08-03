import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import {
  listBanNotifications,
  markBanNotificationsRead,
  clearBanNotifications,
  unbanUser,
  type AdminRiskBanNotification,
} from '@/api/admin/riskControl'

const POLL_MS = 45_000

export const useBanNotificationStore = defineStore('banNotifications', () => {
  const items = ref<AdminRiskBanNotification[]>([])
  const unreadCount = ref(0)
  const total = ref(0)
  const loading = ref(false)
  const lastFetchTime = ref(0)
  let pollTimer: ReturnType<typeof setInterval> | null = null

  const hasUnread = computed(() => unreadCount.value > 0)

  async function fetchNotifications(force = false) {
    const now = Date.now()
    if (!force && lastFetchTime.value > 0 && now - lastFetchTime.value < 5_000) {
      return
    }
    lastFetchTime.value = now
    try {
      loading.value = true
      const res = await listBanNotifications({ page: 1, page_size: 30 })
      items.value = res.items || []
      unreadCount.value = res.unread_count || 0
      total.value = res.total || 0
    } catch (err) {
      lastFetchTime.value = 0
      console.error('Failed to fetch ban notifications:', err)
    } finally {
      loading.value = false
    }
  }

  async function markRead(ids?: number[], all = false) {
    try {
      await markBanNotificationsRead(all ? { all: true } : { ids: ids || [] })
      if (all) {
        items.value = items.value.map((item) => ({ ...item, read: true }))
        unreadCount.value = 0
      } else if (ids?.length) {
        const idSet = new Set(ids)
        items.value = items.value.map((item) =>
          idSet.has(item.id) ? { ...item, read: true } : item
        )
        unreadCount.value = items.value.filter((i) => !i.read).length
      }
    } catch (err) {
      console.error('Failed to mark ban notifications read:', err)
      throw err
    }
  }

  async function clear(ids?: number[], all = false) {
    try {
      await clearBanNotifications(all || !ids?.length ? { all: true } : { ids })
      if (all || !ids?.length) {
        items.value = []
        unreadCount.value = 0
        total.value = 0
      } else {
        const idSet = new Set(ids)
        items.value = items.value.filter((item) => !idSet.has(item.id))
        unreadCount.value = items.value.filter((i) => !i.read).length
        total.value = Math.max(0, total.value - ids.length)
      }
    } catch (err) {
      console.error('Failed to clear ban notifications:', err)
      throw err
    }
  }

  async function quickUnban(userId: number) {
    const result = await unbanUser(userId)
    items.value = items.value.map((item) =>
      item.user_id === userId ? { ...item, user_status: result.status || 'active' } : item
    )
    return result
  }

  function startPolling() {
    stopPolling()
    void fetchNotifications(true)
    pollTimer = setInterval(() => {
      if (document.visibilityState === 'visible') {
        void fetchNotifications(true)
      }
    }, POLL_MS)
  }

  function stopPolling() {
    if (pollTimer) {
      clearInterval(pollTimer)
      pollTimer = null
    }
  }

  function reset() {
    stopPolling()
    items.value = []
    unreadCount.value = 0
    total.value = 0
    lastFetchTime.value = 0
  }

  return {
    items,
    unreadCount,
    total,
    loading,
    hasUnread,
    fetchNotifications,
    markRead,
    clear,
    quickUnban,
    startPolling,
    stopPolling,
    reset,
  }
})
