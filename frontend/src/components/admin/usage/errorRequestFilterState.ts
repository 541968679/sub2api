export type ErrorRequestFilterState = {
  user_id?: number
  api_key_id?: number
  model: string | null
  account_id?: number
  group_id: number | null
  platform: string
  bridge: string
  upstream_model: string
  q: string
  status_codes: number[]
  include_recovered?: boolean
}

export function emptyErrorRequestFilters(): ErrorRequestFilterState {
  return {
    user_id: undefined,
    api_key_id: undefined,
    model: null,
    account_id: undefined,
    group_id: null,
    platform: '',
    bridge: 'all',
    upstream_model: '',
    q: '',
    status_codes: [],
    include_recovered: true
  }
}
