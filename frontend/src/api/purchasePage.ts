/**
 * Purchase page notice API
 * 用户充值/订阅页顶部公告 + 管理员编辑接口。
 */

import { apiClient } from './client'

export interface PurchasePageContent {
  notice: string
}

/**
 * 获取用户可见的充值/订阅页公告
 */
export async function getUserPurchasePageNotice(signal?: AbortSignal): Promise<PurchasePageContent> {
  const { data } = await apiClient.get<PurchasePageContent>('/user/purchase-page', { signal })
  return data
}

/**
 * 管理员读取当前保存的公告文案
 */
export async function getAdminPurchasePageContent(): Promise<PurchasePageContent> {
  const { data } = await apiClient.get<PurchasePageContent>('/admin/purchase-page/content')
  return data
}

/**
 * 管理员保存公告文案（空字符串关闭前端横幅）
 */
export async function updateAdminPurchasePageContent(payload: PurchasePageContent): Promise<PurchasePageContent> {
  const { data } = await apiClient.put<PurchasePageContent>('/admin/purchase-page/content', payload)
  return data
}

export const purchasePageAPI = {
  getUserPurchasePageNotice,
  getAdminPurchasePageContent,
  updateAdminPurchasePageContent
}

export default purchasePageAPI
