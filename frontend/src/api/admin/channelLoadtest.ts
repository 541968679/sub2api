import { apiClient } from '../client'

export type LoadtestProfile = 'user363' | 'user363-stream' | 'user363-sync' | 'user363-sla' | 'smoke'
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
  want_value?: number
  got_value?: number
  samples?: number
}

export interface LoadtestResult {
  seq: number
  started_at: string
  class: string
  bucket: string
  stream: boolean
  requested_model?: string
  target_input_tokens: number
  body_bytes: number
  status_code: number
  outcome: string
  error_category?: string
  error_message?: string
  duration_ms: number
  header_ms?: number
  first_sse_ms?: number
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
  estimated_input_tokens: number
  sla: LoadtestSLAVerdict[]
  sla_pass: boolean
  model_missing_requests: number
  strict_contract_hits: number
  results?: LoadtestResult[]
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

const channelLoadtestAPI = { start, get, latest, stop }
export default channelLoadtestAPI
