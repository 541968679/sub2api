import { apiClient } from '../client'

export type LoadtestProfile = 'user363' | 'user363-stream' | 'user363-sync' | 'user363-sla' | 'smoke' | 'general'
export type LoadtestAPIMode = 'chat_completions' | 'responses'
export type LoadtestStreamMode = 'auto' | 'stream' | 'sync'
export type LoadtestStatus = 'running' | 'stopping' | 'done' | 'failed' | 'stopped'

export interface LoadtestSLAVerdict {
  name: string
  want: string
  got: string
  pass: boolean
  skip: boolean
  metric?: 'ttft' | 'tpot' | string
  band?: string
  want_value?: number
  got_value?: number
  samples?: number
}

export interface LoadtestTokenPercentiles {
  n: number
  p50: number
  p90: number
  p99: number
  avg: number
}

export interface LoadtestDataProfile {
  target_input: LoadtestTokenPercentiles
  usage_input: LoadtestTokenPercentiles
  usage_output: LoadtestTokenPercentiles
}

export interface LoadtestResult {
  seq: number
  started_at: string
  class: string
  bucket: string
  stream: boolean
  requested_model?: string
  target_input_tokens: number
  input_band?: string
  body_bytes: number
  status_code: number
  outcome: string
  error_category?: string
  error_message?: string
  duration_ms: number
  header_ms?: number
  first_sse_ms?: number
  first_token_ms?: number
  first_content_ms?: number
  generation_ms?: number
  tpot_tok_s?: number
  chunks: number
  content_chars: number
  finish_reason?: string
  prompt_tokens?: number
  completion_tokens?: number
  cached_tokens?: number
  response_model?: string
  model_missing_chunks?: number
  model_empty_chunks?: number
  contract_issues?: string[]
  proto?: string
  request_id?: string
}

export interface LoadtestSnapshot {
  id: string
  status: LoadtestStatus
  source: string
  account_id?: number
  account_name?: string
  proxy_id?: number
  proxy_name?: string
  base_url: string
  path: string
  api_mode: LoadtestAPIMode
  stream_mode: LoadtestStreamMode
  models: string[]
  profile: LoadtestProfile
  concurrency: number
  total: number
  duration_sec?: number
  started_at: string
  ended_at?: string
  error?: string
  inflight: number
  peak: number
  done: number
  ok: number
  success_rate: number
  rpm?: number
  rpm_peak?: number
  rpm_avg?: number
  estimated_input_tokens: number
  input_tokens?: number
  tiers?: LoadtestTier[]
  data_profile?: LoadtestDataProfile
  sla: LoadtestSLAVerdict[]
  sla_pass: boolean
  model_missing_requests: number
  strict_contract_hits: number
  results?: LoadtestResult[]
}

export interface LoadtestTier {
  input_tokens: number
  count: number
}

export interface StartLoadtestRequest {
  account_id?: number
  base_url?: string
  api_key?: string
  proxy_id?: number
  path?: string
  api_mode?: LoadtestAPIMode
  stream_mode?: LoadtestStreamMode
  model?: string
  models?: string
  profile?: LoadtestProfile
  concurrency?: number
  total?: number
  duration_sec?: number
  timeout_sec?: number
  max_tokens?: number
  size_cap?: number
  tools?: string
  confirm_cost?: boolean
  abort_after_first?: boolean
  input_tokens?: number
  tiers?: LoadtestTier[]
}

export async function start(req: StartLoadtestRequest): Promise<LoadtestSnapshot> {
  const { data } = await apiClient.post<LoadtestSnapshot>('/admin/channel-loadtest/runs', req)
  return data
}

export async function get(id: string): Promise<LoadtestSnapshot> {
  const { data } = await apiClient.get<LoadtestSnapshot>(`/admin/channel-loadtest/runs/${id}`)
  return data
}

export async function latest(): Promise<LoadtestSnapshot | null> {
  const { data } = await apiClient.get<LoadtestSnapshot | { run: null }>('/admin/channel-loadtest/runs/latest')
  if (!data || (data as { run?: null }).run === null) {
    return null
  }
  return data as LoadtestSnapshot
}

export async function stop(id: string): Promise<LoadtestSnapshot> {
  const { data } = await apiClient.post<LoadtestSnapshot>(`/admin/channel-loadtest/runs/${id}/stop`)
  return data
}

export interface LoadtestExportOptions {
  include_conditions: boolean
  include_overview: boolean
  include_ttft: boolean
  include_duration: boolean
  include_success_rate: boolean
  condition_fields: string[]
}

export const LOADTEST_CONDITION_FIELDS = [
  'run_id',
  'status',
  'started_at',
  'ended_at',
  'duration',
  'source',
  'account',
  'base_url',
  'path',
  'proxy',
  'models',
  'api_mode',
  'stream_mode',
  'profile',
  'tiers',
  'concurrency',
  'total',
  'max_tokens',
  'size_cap',
  'input_tokens',
  'tools',
  'notes'
] as const

export function defaultLoadtestExportOptions(): LoadtestExportOptions {
  return {
    include_conditions: true,
    include_overview: true,
    include_ttft: true,
    include_duration: true,
    include_success_rate: true,
    condition_fields: [...LOADTEST_CONDITION_FIELDS]
  }
}

export async function exportExcel(
  id: string,
  options: LoadtestExportOptions = defaultLoadtestExportOptions()
): Promise<{ blob: Blob; filename: string }> {
  const response = await apiClient.post(
    `/admin/channel-loadtest/runs/${encodeURIComponent(id)}/export`,
    options,
    { responseType: 'blob' }
  )
  const blob = response.data as Blob
  const disposition = String(response.headers?.['content-disposition'] || '')
  const matched = /filename="([^"]+)"/i.exec(disposition)
  const filename = matched?.[1] || `loadtest-${id}.xlsx`
  return { blob, filename }
}

const channelLoadtestAPI = { start, get, latest, stop, exportExcel }
export default channelLoadtestAPI
