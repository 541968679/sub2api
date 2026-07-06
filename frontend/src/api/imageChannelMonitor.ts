/**
 * User-facing Image Channel Monitor API endpoints
 * Read-only status views for public (admin-whitelisted) image channels.
 */

import { apiClient } from './client'
import type { MonitorStatus } from './admin/channelMonitor'

export interface ImageMonitorPublicTimelinePoint {
  status: MonitorStatus
  latency_ms: number | null
  checked_at: string
}

export interface ImageMonitorPublicView {
  id: number
  name: string
  model: string
  latest_status: MonitorStatus | 'empty'
  latest_api_ms: number | null
  latest_download_ms: number | null
  availability_7d: number
  availability_15d: number
  availability_30d: number
  timeline: ImageMonitorPublicTimelinePoint[]
}

export interface ImageMonitorPublicListResponse {
  items: ImageMonitorPublicView[]
}

export interface ImageMonitorPublicWindowStat {
  window_days: 7 | 15 | 30
  availability: number
  avg_api_total_ms: number | null
}

export interface ImageMonitorPublicDetail {
  id: number
  name: string
  model: string
  windows: ImageMonitorPublicWindowStat[]
}

export async function list(options?: {
  signal?: AbortSignal
}): Promise<ImageMonitorPublicListResponse> {
  const { data } = await apiClient.get<ImageMonitorPublicListResponse>('/image-channel-monitors', {
    signal: options?.signal,
  })
  return data
}

export async function status(id: number): Promise<ImageMonitorPublicDetail> {
  const { data } = await apiClient.get<ImageMonitorPublicDetail>(
    `/image-channel-monitors/${id}/status`
  )
  return data
}

export const imageChannelMonitorUserAPI = { list, status }

export default imageChannelMonitorUserAPI
