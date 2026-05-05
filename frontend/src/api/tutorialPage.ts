/**
 * Admin Tutorial Page Content API
 */

import { apiClient } from './client'

export interface TutorialPageContent {
  content: string
}

export async function getAdminTutorialContent(): Promise<TutorialPageContent> {
  const { data } = await apiClient.get<TutorialPageContent>('/admin/tutorial-page/content')
  return { content: data.content || '' }
}

export async function updateAdminTutorialContent(
  payload: TutorialPageContent
): Promise<TutorialPageContent> {
  const { data } = await apiClient.put<TutorialPageContent>('/admin/tutorial-page/content', payload)
  return { content: data.content || '' }
}

export async function uploadTutorialImage(file: File): Promise<{ url: string; filename: string }> {
  const formData = new FormData()
  formData.append('image', file)
  const { data } = await apiClient.post<{ url: string; filename: string }>(
    '/admin/tutorial-page/upload-image',
    formData,
    { headers: { 'Content-Type': 'multipart/form-data' } }
  )
  return data
}

/** User-facing: read tutorial content */
export async function getUserTutorialContent(): Promise<TutorialPageContent> {
  const { data } = await apiClient.get<TutorialPageContent>('/user/tutorial-page')
  return { content: data.content || '' }
}

export const tutorialPageAPI = {
  getAdminTutorialContent,
  updateAdminTutorialContent,
  uploadTutorialImage,
  getUserTutorialContent
}

export default tutorialPageAPI
