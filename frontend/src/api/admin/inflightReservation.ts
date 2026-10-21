import { apiClient } from '../client'

export type InflightInputPricing = 'full' | 'cache_tail'

export interface InflightReservationSettings {
  enabled: boolean
  ttl_seconds: number
  bytes_per_token: number
  default_max_tokens: number
  max_output_tokens: number
  max_input_tokens: number
  input_pricing: InflightInputPricing
  full_price_input_tokens: number
  max_reservation_amount: number
  fail_closed_on_unpriced: boolean
  source?: 'config' | 'settings' | string
  simple_mode?: boolean
}

export const inflightReservationAPI = {
  async get(): Promise<InflightReservationSettings> {
    const { data } = await apiClient.get<InflightReservationSettings>('/admin/inflight-reservation')
    return data
  },

  async update(payload: InflightReservationSettings): Promise<InflightReservationSettings> {
    const { data } = await apiClient.put<InflightReservationSettings>('/admin/inflight-reservation', {
      enabled: payload.enabled,
      ttl_seconds: payload.ttl_seconds,
      bytes_per_token: payload.bytes_per_token,
      default_max_tokens: payload.default_max_tokens,
      max_output_tokens: payload.max_output_tokens,
      max_input_tokens: payload.max_input_tokens,
      input_pricing: payload.input_pricing,
      full_price_input_tokens: payload.full_price_input_tokens,
      max_reservation_amount: payload.max_reservation_amount,
      fail_closed_on_unpriced: payload.fail_closed_on_unpriced,
    })
    return data
  },
}

export default inflightReservationAPI
