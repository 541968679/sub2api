import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { announcementsAPI } from '@/api'
import type { AnnouncementSurface, UserAnnouncement } from '@/types'

const THROTTLE_MS = 20 * 60 * 1000 // 20 minutes

export const useAnnouncementStore = defineStore('announcements', () => {
  // State
  const announcements = ref<UserAnnouncement[]>([])
  const loading = ref(false)
  const lastFetchTime = ref(0)
  const popupQueue = ref<UserAnnouncement[]>([])
  const currentPopup = ref<UserAnnouncement | null>(null)

  // Session-scoped dedup set — not reactive, used as plain lookup only
  let shownPopupIds = new Set<number>()

  // Getters
  const unreadCount = computed(() =>
    announcements.value.filter((a) => !a.read_at).length
  )

  // Actions
  async function fetchAnnouncements(force = false) {
    const now = Date.now()
    if (!force && lastFetchTime.value > 0 && now - lastFetchTime.value < THROTTLE_MS) {
      return
    }

    // Set immediately to prevent concurrent duplicate requests
    lastFetchTime.value = now

    try {
      loading.value = true
      const all = await announcementsAPI.list({ surface: 'general' })
      announcements.value = all.slice(0, 20)
      enqueueNewPopups()
    } catch (err: any) {
      // Revert throttle timestamp on failure so retry is allowed
      lastFetchTime.value = 0
      console.error('Failed to fetch announcements:', err)
    } finally {
      loading.value = false
    }
  }

  function enqueueNewPopups() {
    const newPopups = announcements.value.filter(
      (a) => a.should_popup && !shownPopupIds.has(a.id)
    )
    if (newPopups.length === 0) return

    for (const p of newPopups) {
      if (!popupQueue.value.some((q) => q.id === p.id)) {
        popupQueue.value.push(p)
      }
    }

    if (!currentPopup.value) {
      showNextPopup()
    }
  }

  function showNextPopup() {
    if (popupQueue.value.length === 0) {
      currentPopup.value = null
      return
    }
    currentPopup.value = popupQueue.value.shift()!
    shownPopupIds.add(currentPopup.value.id)
  }

  async function dismissPopup() {
    if (!currentPopup.value) return
    const id = currentPopup.value.id
    currentPopup.value = null

    try {
      await announcementsAPI.dismissPopup(id)
      const ann = announcements.value.find((a) => a.id === id)
      if (ann) {
        const now = new Date().toISOString()
        ann.last_popup_dismissed_at = now
        ann.read_at = ann.read_at || now
        ann.should_popup = false
      }
    } catch (err: any) {
      console.error('Failed to dismiss announcement popup:', err)
    }

    // Show next popup after a short delay
    if (popupQueue.value.length > 0) {
      setTimeout(() => showNextPopup(), 300)
    }
  }

  async function markAsRead(id: number) {
    try {
      await announcementsAPI.markRead(id)
      const ann = announcements.value.find((a) => a.id === id)
      if (ann) {
        ann.read_at = new Date().toISOString()
      }
    } catch (err: any) {
      console.error('Failed to mark announcement as read:', err)
    }
  }

  async function markAllAsRead() {
    const unread = announcements.value.filter((a) => !a.read_at)
    if (unread.length === 0) return

    try {
      loading.value = true
      await Promise.all(unread.map((a) => announcementsAPI.markRead(a.id)))
      announcements.value.forEach((a) => {
        if (!a.read_at) {
          a.read_at = new Date().toISOString()
        }
      })
    } catch (err: any) {
      console.error('Failed to mark all as read:', err)
      throw err
    } finally {
      loading.value = false
    }
  }

  async function fetchLatestBySurface(surface: AnnouncementSurface): Promise<UserAnnouncement | null> {
    try {
      const items = await announcementsAPI.list({ surface })
      return items[0] || null
    } catch (err: any) {
      console.error(`Failed to fetch ${surface} announcement:`, err)
      return null
    }
  }

  async function dismissBanner(id: number) {
    try {
      await announcementsAPI.dismissBanner(id)
      const now = new Date().toISOString()
      const ann = announcements.value.find((a) => a.id === id)
      if (ann) {
        ann.banner_dismissed_at = now
      }
    } catch (err: any) {
      console.error('Failed to dismiss announcement banner:', err)
      throw err
    }
  }

  function reset() {
    announcements.value = []
    lastFetchTime.value = 0
    shownPopupIds = new Set()
    popupQueue.value = []
    currentPopup.value = null
    loading.value = false
  }

  return {
    // State
    announcements,
    loading,
    currentPopup,
    // Getters
    unreadCount,
    // Actions
    fetchAnnouncements,
    fetchLatestBySurface,
    dismissPopup,
    dismissBanner,
    markAsRead,
    markAllAsRead,
    reset,
  }
})
