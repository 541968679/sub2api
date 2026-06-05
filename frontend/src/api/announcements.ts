/**
 * User Announcements API endpoints
 */

import { apiClient } from './client'
import type { AnnouncementSurface, UserAnnouncement } from '@/types'

export interface AnnouncementListOptions {
  unreadOnly?: boolean
  surface?: AnnouncementSurface
  signal?: AbortSignal
}

export async function list(options: AnnouncementListOptions = {}): Promise<UserAnnouncement[]> {
  const params: Record<string, string | number> = {}
  if (options.unreadOnly) {
    params.unread_only = 1
  }
  if (options.surface) {
    params.surface = options.surface
  }

  const { data } = await apiClient.get<UserAnnouncement[]>('/announcements', {
    params,
    signal: options.signal
  })
  return data
}

export async function markRead(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>(`/announcements/${id}/read`)
  return data
}

export async function dismissPopup(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>(`/announcements/${id}/popup-dismiss`)
  return data
}

export async function dismissBanner(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>(`/announcements/${id}/banner-dismiss`)
  return data
}

const announcementsAPI = {
  list,
  markRead,
  dismissPopup,
  dismissBanner
}

export default announcementsAPI
