import { apiClient } from './client'
import type {
  DistributionSummary,
  DistributionWalletLedgerEntry,
  PaginatedResponse
} from '@/types'

export interface ApplyDistributionRequest {
  contact: string
  reason: string
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

export const distributionAPI = {
  getSummary,
  apply,
  listLedger,
}

export default distributionAPI
