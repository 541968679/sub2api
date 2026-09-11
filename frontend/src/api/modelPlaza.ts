import { apiClient } from './client'

export interface ModelPlazaPricing {
  input_price?: number | null
  output_price?: number | null
  cache_read_price?: number | null
}

export interface ModelPlazaModel {
  name: string
  platform: string
  pricing: ModelPlazaPricing | null
  official_pricing: ModelPlazaPricing | null
}

export interface ModelPlazaGroup {
  id: number
  name: string
  description: string
  platform: string
  subscription_type: string
  rate_multiplier: number
  is_exclusive: boolean
  models: ModelPlazaModel[]
}

export interface ModelPlazaResponse {
  description: string
  groups: ModelPlazaGroup[]
}

export async function getModelPlaza(): Promise<ModelPlazaResponse> {
  const { data } = await apiClient.get<ModelPlazaResponse>('/model-plaza')
  return data
}
