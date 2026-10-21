import apiClient from './client'
import type { LLMProviderListResponse } from '@/shared/types'

export async function fetchLLMProviders(capability?: string, model?: string): Promise<LLMProviderListResponse> {
  const params: Record<string, string> = {}
  if (capability) params.capability = capability
  if (model) params.model = model
  const { data } = await apiClient.get<LLMProviderListResponse>('/llms', { params })
  return data
}
