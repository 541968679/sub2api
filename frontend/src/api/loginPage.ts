/**
 * Admin Login Page Content API
 *
 * 管理员编辑登录页文案（8 个字段）。空字段保存后前端自动回落到 i18n `auth.login.*` 默认值。
 */

import { apiClient } from './client'

export interface LoginPageContent {
  badge: string
  heading_line1: string
  heading_line2: string
  description: string
  supported_models_title: string
  models_desc: string
  form_title: string
  form_subtitle: string
}

const EMPTY: LoginPageContent = {
  badge: '',
  heading_line1: '',
  heading_line2: '',
  description: '',
  supported_models_title: '',
  models_desc: '',
  form_title: '',
  form_subtitle: ''
}

/** 管理员读取已保存的 8 段文案。空字段代表未覆盖，前端回落 i18n。 */
export async function getAdminLoginPageContent(): Promise<LoginPageContent> {
  const { data } = await apiClient.get<LoginPageContent>('/admin/login-page/content')
  return { ...EMPTY, ...data }
}

/** 管理员保存 8 段文案。任何字段超长会 400。 */
export async function updateAdminLoginPageContent(
  payload: LoginPageContent
): Promise<LoginPageContent> {
  const { data } = await apiClient.put<LoginPageContent>('/admin/login-page/content', payload)
  return { ...EMPTY, ...data }
}

/** 重置所有字段到空（即全部回落 i18n）。 */
export async function resetAdminLoginPageContent(): Promise<LoginPageContent> {
  return updateAdminLoginPageContent(EMPTY)
}

export const loginPageAPI = {
  getAdminLoginPageContent,
  updateAdminLoginPageContent,
  resetAdminLoginPageContent
}

export default loginPageAPI
