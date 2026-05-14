import { apiClient } from '../client'
import type {
  DistributionAgentApplication,
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

export interface AdjustDistributionWalletRequest {
  amount: number
  note?: string
}

export interface UpdateDistributionWalletStatusRequest {
  frozen: boolean
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

export async function updateSettings(payload: DistributionSettings): Promise<DistributionSettings> {
  const { data } = await apiClient.put<DistributionSettings>('/admin/distribution/settings', payload)
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

export const distributionAdminAPI = {
  listApplications,
  reviewApplication,
  getSettings,
  updateSettings,
  listWallets,
  listLedger,
  adjustWallet,
  updateWalletStatus,
}

export default distributionAdminAPI
