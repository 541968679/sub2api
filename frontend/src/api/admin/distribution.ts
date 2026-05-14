import { apiClient } from '../client'
import type { DistributionAgentApplication, PaginatedResponse } from '@/types'

export interface ListDistributionApplicationsParams {
  page?: number
  page_size?: number
  search?: string
}

export interface ReviewDistributionApplicationRequest {
  approved: boolean
  note?: string
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

export const distributionAdminAPI = {
  listApplications,
  reviewApplication,
}

export default distributionAdminAPI
