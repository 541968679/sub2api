import apiClient from '../client'

export interface UserModelPricingOverride {
  id: number
  user_id: number
  model: string
  input_price: number | null
  output_price: number | null
  cache_write_price: number | null
  cache_read_price: number | null
  display_input_price: number | null
  display_output_price: number | null
  display_rate_multiplier: number | null
  cache_transfer_ratio: number | null
  enabled: boolean
  notes: string
  created_at?: string
  updated_at?: string
}

export async function getUserModelPricing(userId: number) {
  const { data } = await apiClient.get<{ data: UserModelPricingOverride[] }>(
    `/admin/users/${userId}/model-pricing`
  )
  return data.data
}

export async function createUserModelPricing(
  userId: number,
  override: Partial<UserModelPricingOverride>
) {
  const { data } = await apiClient.post<{ data: UserModelPricingOverride }>(
    `/admin/users/${userId}/model-pricing`,
    override
  )
  return data.data
}

export async function updateUserModelPricing(
  userId: number,
  overrideId: number,
  override: Partial<UserModelPricingOverride>
) {
  const { data } = await apiClient.put<{ data: UserModelPricingOverride }>(
    `/admin/users/${userId}/model-pricing/${overrideId}`,
    override
  )
  return data.data
}

export async function deleteUserModelPricing(userId: number, overrideId: number) {
  const { data } = await apiClient.delete(`/admin/users/${userId}/model-pricing/${overrideId}`)
  return data
}

export async function batchUpsertUserModelPricing(
  userId: number,
  overrides: Partial<UserModelPricingOverride>[]
) {
  const { data } = await apiClient.put(`/admin/users/${userId}/model-pricing/batch`, {
    overrides,
  })
  return data
}
