import { apiClient } from '../client'

export type ImageMonitorSourceType = 'custom' | 'account'
export type ImageMonitorStatus = 'operational' | 'degraded' | 'failed' | 'error'

export interface ImageChannelMonitor {
  id: number
  name: string
  source_type: ImageMonitorSourceType
  endpoint: string
  api_key_masked: string
  api_key_decrypt_failed?: boolean
  account_id: number | null
  account_name: string
  proxy_id: number | null
  proxy_name: string
  model: string
  prompt: string
  size: string
  quality: string
  n: number
  download_image: boolean
  enabled: boolean
  interval_seconds: number
  timeout_seconds: number
  last_checked_at: string | null
  created_by: number
  created_at: string
  updated_at: string
}

export interface ImageChannelMonitorListParams {
  page?: number
  page_size?: number
  source_type?: ImageMonitorSourceType
  enabled?: boolean
  search?: string
}

export interface ImageChannelMonitorListResponse {
  items: ImageChannelMonitor[]
  total: number
  page: number
  page_size: number
  pages: number
}

export interface ImageChannelMonitorCreateParams {
  name: string
  source_type?: ImageMonitorSourceType
  endpoint?: string
  api_key?: string
  account_id?: number | null
  proxy_id?: number | null
  model?: string
  prompt?: string
  size?: string
  quality?: string
  n?: number
  download_image?: boolean
  enabled?: boolean
  interval_seconds?: number
  timeout_seconds?: number
}

export type ImageChannelMonitorUpdateParams = Partial<ImageChannelMonitorCreateParams>

export interface ImageChannelMonitorResult {
  monitor_id: number
  status: ImageMonitorStatus
  http_status: number | null
  api_header_ms: number | null
  api_body_ms: number | null
  api_total_ms: number | null
  json_bytes: number | null
  has_url: boolean
  has_b64_json: boolean
  image_url_host: string
  image_first_byte_ms: number | null
  image_download_ms: number | null
  image_bytes: number | null
  image_content_type: string
  image_width: number | null
  image_height: number | null
  error_stage: string
  message: string
  checked_at: string
  revised_prompt: string
  returned_image_url: string
  returned_image_data: string
}

export interface ImageChannelMonitorRuntimeStatus {
  monitor_id: number
  running: boolean
  stage: string
  message: string
  started_at: string | null
  updated_at: string | null
  completed_at: string | null
  next_check_at: string | null
  seconds_until_next_check: number | null
}

export type ImageChannelMonitorHistoryItem = Omit<
  ImageChannelMonitorResult,
  'revised_prompt' | 'returned_image_url' | 'returned_image_data'
> & {
  id: number
}

export async function list(
  params: ImageChannelMonitorListParams = {},
  options?: { signal?: AbortSignal }
): Promise<ImageChannelMonitorListResponse> {
  const { data } = await apiClient.get<ImageChannelMonitorListResponse>(
    '/admin/image-channel-monitors',
    {
      params,
      signal: options?.signal,
    }
  )
  return data
}

export async function get(id: number): Promise<ImageChannelMonitor> {
  const { data } = await apiClient.get<ImageChannelMonitor>(
    `/admin/image-channel-monitors/${id}`
  )
  return data
}

export async function create(
  params: ImageChannelMonitorCreateParams
): Promise<ImageChannelMonitor> {
  const { data } = await apiClient.post<ImageChannelMonitor>(
    '/admin/image-channel-monitors',
    params
  )
  return data
}

export async function update(
  id: number,
  params: ImageChannelMonitorUpdateParams
): Promise<ImageChannelMonitor> {
  const { data } = await apiClient.put<ImageChannelMonitor>(
    `/admin/image-channel-monitors/${id}`,
    params
  )
  return data
}

export async function del(id: number): Promise<void> {
  await apiClient.delete(`/admin/image-channel-monitors/${id}`)
}

export async function runNow(id: number): Promise<ImageChannelMonitorRuntimeStatus> {
  const { data } = await apiClient.post<ImageChannelMonitorRuntimeStatus>(
    `/admin/image-channel-monitors/${id}/run`
  )
  return data
}

export async function getStatus(id: number): Promise<ImageChannelMonitorRuntimeStatus> {
  const { data } = await apiClient.get<ImageChannelMonitorRuntimeStatus>(
    `/admin/image-channel-monitors/${id}/status`
  )
  return data
}

export async function listHistory(
  id: number,
  params: { limit?: number } = {}
): Promise<ImageChannelMonitorHistoryItem[]> {
  const { data } = await apiClient.get<ImageChannelMonitorHistoryItem[]>(
    `/admin/image-channel-monitors/${id}/history`,
    { params }
  )
  return data
}

export const imageChannelMonitorAPI = {
  list,
  get,
  create,
  update,
  del,
  runNow,
  getStatus,
  listHistory,
}

export default imageChannelMonitorAPI
