/**
 * Redeem page buy-notice API
 * 用户兑换页「去哪买兑换码」说明 + 管理员编辑接口。
 */

import { apiClient } from './client'

export interface RedeemPageContent {
  notice: string
}

/**
 * 获取用户可见的兑换页购买说明
 */
export async function getUserRedeemPageNotice(signal?: AbortSignal): Promise<RedeemPageContent> {
  const { data } = await apiClient.get<RedeemPageContent>('/user/redeem-page', { signal })
  return data
}

/**
 * 管理员读取当前保存的说明文案
 */
export async function getAdminRedeemPageContent(): Promise<RedeemPageContent> {
  const { data } = await apiClient.get<RedeemPageContent>('/admin/redeem-page/content')
  return data
}

/**
 * 管理员保存说明文案（空字符串关闭前端横幅）
 */
export async function updateAdminRedeemPageContent(payload: RedeemPageContent): Promise<RedeemPageContent> {
  const { data } = await apiClient.put<RedeemPageContent>('/admin/redeem-page/content', payload)
  return data
}

export const redeemPageAPI = {
  getUserRedeemPageNotice,
  getAdminRedeemPageContent,
  updateAdminRedeemPageContent
}

export default redeemPageAPI
