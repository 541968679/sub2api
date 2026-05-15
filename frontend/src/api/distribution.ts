import { apiClient } from './client'
import type {
  DistributionAsset,
  DistributionAssetStatus,
  DistributionAssetType,
  DistributionGeneratedApiKey,
  DistributionGeneratedRedeemCode,
  DistributionSummary,
  DistributionWalletLedgerEntry,
  PaginatedResponse
} from '@/types'

export interface ApplyDistributionRequest {
  contact: string
  reason: string
}

export interface GenerateBalanceRedeemCodeRequest {
  value_usd: number
  note?: string
}

export interface GenerateSubscriptionRedeemCodeRequest {
  plan_id: number
  note?: string
}

export interface GenerateDistributionApiKeyRequest {
  name: string
  quota_usd: number
  group_id?: number | null
  expires_in_days?: number | null
}

export interface ListDistributionAssetsParams {
  page?: number
  page_size?: number
  asset_type?: DistributionAssetType | ''
  status?: DistributionAssetStatus | ''
  search?: string
}

export interface VoidDistributionAssetResponse {
  asset: DistributionAsset
  refund_rmb: number
}

export async function getSummary(): Promise<DistributionSummary> {
  const { data } = await apiClient.get<DistributionSummary>('/distribution')
  return data
}

export async function apply(payload: ApplyDistributionRequest): Promise<DistributionSummary> {
  const { data } = await apiClient.post<DistributionSummary>('/distribution/apply', payload)
  return data
}

export async function listLedger(
  page = 1,
  pageSize = 20,
): Promise<PaginatedResponse<DistributionWalletLedgerEntry>> {
  const { data } = await apiClient.get<PaginatedResponse<DistributionWalletLedgerEntry>>('/distribution/ledger', {
    params: { page, page_size: pageSize },
  })
  return data
}

export async function listAssets(
  params: ListDistributionAssetsParams = {},
): Promise<PaginatedResponse<DistributionAsset>> {
  const { data } = await apiClient.get<PaginatedResponse<DistributionAsset>>('/distribution/assets', { params })
  return data
}

export async function voidAsset(id: number): Promise<VoidDistributionAssetResponse> {
  const { data } = await apiClient.post<VoidDistributionAssetResponse>(`/distribution/assets/${id}/void`)
  return data
}

export async function generateBalanceRedeemCode(
  payload: GenerateBalanceRedeemCodeRequest,
): Promise<DistributionGeneratedRedeemCode> {
  const { data } = await apiClient.post<DistributionGeneratedRedeemCode>('/distribution/redeem-codes/balance', payload)
  return data
}

export async function generateSubscriptionRedeemCode(
  payload: GenerateSubscriptionRedeemCodeRequest,
): Promise<DistributionGeneratedRedeemCode> {
  const { data } = await apiClient.post<DistributionGeneratedRedeemCode>('/distribution/redeem-codes/subscription', payload)
  return data
}

export async function generateApiKey(
  payload: GenerateDistributionApiKeyRequest,
): Promise<DistributionGeneratedApiKey> {
  const { data } = await apiClient.post<DistributionGeneratedApiKey>('/distribution/api-keys', payload)
  return data
}

export const distributionAPI = {
  getSummary,
  apply,
  listLedger,
  listAssets,
  voidAsset,
  generateBalanceRedeemCode,
  generateSubscriptionRedeemCode,
  generateApiKey,
}

export default distributionAPI
