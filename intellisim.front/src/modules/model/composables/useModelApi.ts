import apiClient from '@/shared/api/client'
import type {
  CreateModelRequest,
  ListModelsResponse,
  ListModelOptionsResponse,
  Model,
  ModelOption,
  ModelResponse,
  UpdateModelRequest,
  ValidationResult,
} from '@/shared/types'

const BASE = '/models'

// Thin REST wrapper over the models module endpoints (mirrors useChatApi).
export function useModelApi() {
  async function listModels(
    page = 1,
    pageSize = 20,
    sortBy?: string,
    sortOrder?: 'asc' | 'desc'
  ): Promise<ListModelsResponse> {
    const params: Record<string, string | number> = { page, page_size: pageSize }
    if (sortBy) params.sort_by = sortBy
    if (sortOrder) params.sort_order = sortOrder
    const { data } = await apiClient.get<ListModelsResponse>(BASE, { params })
    return data
  }

  // Lightweight, unpaginated id+name list of all the caller's models (admin:
  // all) — backs selector UIs like the simulations model filter.
  async function listModelOptions(): Promise<ModelOption[]> {
    const { data } = await apiClient.get<ListModelOptionsResponse>(`${BASE}/options`)
    return data.options ?? []
  }

  async function createModel(req: CreateModelRequest): Promise<ModelResponse> {
    const { data } = await apiClient.post<ModelResponse>(BASE, req)
    return data
  }

  // Duplicate a model (backend also clones its companion chat + messages, if any).
  async function duplicateModel(id: string, name?: string): Promise<ModelResponse> {
    const { data } = await apiClient.post<ModelResponse>(`${BASE}/${id}/duplicate`, { name })
    return data
  }

  async function getModel(id: string): Promise<ModelResponse> {
    const { data } = await apiClient.get<ModelResponse>(`${BASE}/${id}`)
    return data
  }

  /** Full manual replace of the editable model body. */
  async function updateModel(id: string, model: Model): Promise<ModelResponse> {
    const body: UpdateModelRequest = { model }
    const { data } = await apiClient.put<ModelResponse>(`${BASE}/${id}`, body)
    return data
  }

  async function deleteModel(id: string): Promise<void> {
    await apiClient.delete(`${BASE}/${id}`)
  }

  async function getValidation(id: string): Promise<ValidationResult> {
    const { data } = await apiClient.get<ValidationResult>(`${BASE}/${id}/validation`)
    return data
  }

  return {
    listModels,
    listModelOptions,
    createModel,
    duplicateModel,
    getModel,
    updateModel,
    deleteModel,
    getValidation,
  }
}
