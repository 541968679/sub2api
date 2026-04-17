/**
 * User Groups API endpoints (non-admin)
 * Handles group-related operations for regular users
 */

import { apiClient } from './client'
import type { Group, UserGroupRateData } from '@/types'

/**
 * Get available groups that the current user can bind to API keys
 * This returns groups based on user's permissions:
 * - Standard groups: public (non-exclusive) or explicitly allowed
 * - Subscription groups: user has active subscription
 * @returns List of available groups
 */
export async function getAvailable(): Promise<Group[]> {
  const { data } = await apiClient.get<Group[]>('/groups/available')
  return data
}

/**
 * Get current user's custom group rate multipliers (including display rates)
 * @returns Map of group_id to UserGroupRateData
 */
export async function getUserGroupRates(): Promise<Record<number, UserGroupRateData>> {
  const { data } = await apiClient.get<Record<number, UserGroupRateData> | null>('/groups/rates')
  return data || {}
}

export const userGroupsAPI = {
  getAvailable,
  getUserGroupRates
}

export default userGroupsAPI
