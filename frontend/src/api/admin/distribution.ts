import { apiClient } from '../client'
import type {
  DistributionAgentApplication,
  DistributionAsset,
  DistributionAssetStatus,
  DistributionAssetType,
  DistributionSettings,
  DistributionWallet,
  DistributionWalletLedgerEntry,
  PaginatedResponse
} from '@/types'

export interface ListDistributionApplicationsParams {
  page?: number
  page_size?: number
  search?: string
}

export interface ReviewDistributionApplicationRequest {
  approved: boolean
  note?: string
}

export interface ListDistributionWalletsParams {
  page?: number
  page_size?: number
  search?: string
}

export interface ListDistributionLedgerParams {
  page?: number
  page_size?: number
  user_id?: number
}

export interface ListDistributionAssetsParams {
  page?: number
  page_size?: number
  user_id?: number
  asset_type?: DistributionAssetType | ''
  status?: DistributionAssetStatus | ''
  search?: string
}

export interface AdjustDistributionWalletRequest {
  amount: number
  note?: string
}

export interface UpdateDistributionWalletStatusRequest {
  frozen: boolean
}

export interface UpdateDistributionAgentRatesRequest {
  rmb_per_usd_override?: number | null
  subscription_discount_override?: number | null
}

export interface UpdateDistributionSettingsRequest {
  rmb_per_usd: number
  subscription_discount: number
  api_key_group_ids: number[]
}

export interface VoidDistributionAssetResponse {
  asset: DistributionAsset
  wallet?: DistributionWallet
  cost_rmb?: number
  refund_rmb: number
}

export type DistributionAssetOperationResponse = VoidDistributionAssetResponse

export interface RechargeDistributionApiKeyRequest {
  quota_usd: number
}

export async function listApplications(
  params: ListDistributionApplicationsParams = {},
): Promise<PaginatedResponse<DistributionAgentApplication>> {
  const { data } = await apiClient.get<PaginatedResponse<DistributionAgentApplication>>(
    '/admin/distribution/applications',
    { params },
  )
  return data
}

export async function reviewApplication(
  userId: number,
  payload: ReviewDistributionApplicationRequest,
): Promise<DistributionAgentApplication> {
  const { data } = await apiClient.post<DistributionAgentApplication>(
    `/admin/distribution/applications/${userId}/review`,
    payload,
  )
  return data
}

export async function getSettings(): Promise<DistributionSettings> {
  const { data } = await apiClient.get<DistributionSettings>('/admin/distribution/settings')
  return data
}

export async function updateSettings(payload: UpdateDistributionSettingsRequest): Promise<DistributionSettings> {
  const { data } = await apiClient.put<DistributionSettings>('/admin/distribution/settings', payload)
  return data
}

export async function updateAgentRates(
  userId: number,
  payload: UpdateDistributionAgentRatesRequest,
): Promise<DistributionAgentApplication> {
  const { data } = await apiClient.put<DistributionAgentApplication>(
    `/admin/distribution/agents/${userId}/rates`,
    payload,
  )
  return data
}

export async function listWallets(
  params: ListDistributionWalletsParams = {},
): Promise<PaginatedResponse<DistributionWallet>> {
  const { data } = await apiClient.get<PaginatedResponse<DistributionWallet>>(
    '/admin/distribution/wallets',
    { params },
  )
  return data
}

export async function listLedger(
  params: ListDistributionLedgerParams = {},
): Promise<PaginatedResponse<DistributionWalletLedgerEntry>> {
  const { data } = await apiClient.get<PaginatedResponse<DistributionWalletLedgerEntry>>(
    '/admin/distribution/ledger',
    { params },
  )
  return data
}

export async function listAssets(
  params: ListDistributionAssetsParams = {},
): Promise<PaginatedResponse<DistributionAsset>> {
  const { data } = await apiClient.get<PaginatedResponse<DistributionAsset>>(
    '/admin/distribution/assets',
    { params },
  )
  return data
}

export async function adjustWallet(
  userId: number,
  payload: AdjustDistributionWalletRequest,
): Promise<DistributionWallet> {
  const { data } = await apiClient.post<DistributionWallet>(
    `/admin/distribution/wallets/${userId}/adjust`,
    payload,
  )
  return data
}

export async function updateWalletStatus(
  userId: number,
  payload: UpdateDistributionWalletStatusRequest,
): Promise<DistributionWallet> {
  const { data } = await apiClient.put<DistributionWallet>(
    `/admin/distribution/wallets/${userId}/status`,
    payload,
  )
  return data
}

export async function voidAsset(id: number): Promise<VoidDistributionAssetResponse> {
  const { data } = await apiClient.post<VoidDistributionAssetResponse>(`/admin/distribution/assets/${id}/void`)
  return data
}

export async function rechargeAsset(
  id: number,
  payload: RechargeDistributionApiKeyRequest,
): Promise<DistributionAssetOperationResponse> {
  const { data } = await apiClient.post<DistributionAssetOperationResponse>(`/admin/distribution/assets/${id}/recharge`, payload)
  return data
}

export async function disableAsset(id: number): Promise<DistributionAssetOperationResponse> {
  const { data } = await apiClient.post<DistributionAssetOperationResponse>(`/admin/distribution/assets/${id}/disable`)
  return data
}

export async function enableAsset(id: number): Promise<DistributionAssetOperationResponse> {
  const { data } = await apiClient.post<DistributionAssetOperationResponse>(`/admin/distribution/assets/${id}/enable`)
  return data
}

export async function refundAsset(id: number): Promise<DistributionAssetOperationResponse> {
  const { data } = await apiClient.post<DistributionAssetOperationResponse>(`/admin/distribution/assets/${id}/refund`)
  return data
}

export const distributionAdminAPI = {
  listApplications,
  reviewApplication,
  getSettings,
  updateSettings,
  updateAgentRates,
  listWallets,
  listLedger,
  listAssets,
  adjustWallet,
  updateWalletStatus,
  voidAsset,
  rechargeAsset,
  disableAsset,
  enableAsset,
  refundAsset,
}

export default distributionAdminAPI
