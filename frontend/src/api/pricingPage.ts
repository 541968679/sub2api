/**
 * Model Pricing Page API
 * 用户「模型计价」页的聚合数据接口 + 管理员文案编辑接口。
 */

import { apiClient } from './client'

export interface PricingPageModel {
  model: string
  billing_mode: string
  display_input_price: number | null
  display_output_price: number | null
  display_cache_read_price: number | null
  per_request_price: number | null
}

export interface PricingPagePlatform {
  provider: string
  models: PricingPageModel[]
}

export interface PricingPageData {
  intro: string
  education: string
  platforms: PricingPagePlatform[]
}

export interface PricingPageContent {
  intro: string
  education: string
}

/**
 * 获取用户可见的计价页聚合数据
 */
export async function getUserPricingPage(signal?: AbortSignal): Promise<PricingPageData> {
  const { data } = await apiClient.get<PricingPageData>('/user/pricing-page', { signal })
  return data
}

/**
 * 管理员读取当前保存的两段 Markdown 文案
 */
export async function getAdminPricingPageContent(): Promise<PricingPageContent> {
  const { data } = await apiClient.get<PricingPageContent>('/admin/pricing-page/content')
  return data
}

/**
 * 管理员保存两段 Markdown 文案
 */
export async function updateAdminPricingPageContent(payload: PricingPageContent): Promise<PricingPageContent> {
  const { data } = await apiClient.put<PricingPageContent>('/admin/pricing-page/content', payload)
  return data
}

export const pricingPageAPI = {
  getUserPricingPage,
  getAdminPricingPageContent,
  updateAdminPricingPageContent
}

export default pricingPageAPI
