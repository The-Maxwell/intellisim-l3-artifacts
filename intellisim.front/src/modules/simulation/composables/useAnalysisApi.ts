import apiClient from '@/shared/api/client'
import type {
  Analysis,
  AnalyzeRequest,
  ListAnalysesResponse,
  ResultsSeries,
  ResultsSummary,
} from '@/shared/types'

const BASE = '/simulations'

// REST wrapper for the Level-3 analysis surface (mirrors useSimulationApi).
export function useAnalysisApi() {
  // Create is async on the backend (202 + pipeline runs in background); the
  // returned Analysis is the initial (pending) state.
  async function create(simId: string, req: AnalyzeRequest): Promise<Analysis> {
    const { data } = await apiClient.post<Analysis>(`${BASE}/${simId}/analyses`, req)
    return data
  }

  async function list(simId: string): Promise<ListAnalysesResponse> {
    const { data } = await apiClient.get<ListAnalysesResponse>(`${BASE}/${simId}/analyses`)
    return data
  }

  async function get(simId: string, analysisId: string): Promise<Analysis> {
    const { data } = await apiClient.get<Analysis>(`${BASE}/${simId}/analyses/${analysisId}`)
    return data
  }

  async function remove(simId: string, analysisId: string): Promise<void> {
    await apiClient.delete(`${BASE}/${simId}/analyses/${analysisId}`)
  }

  // Fetches the PDF as a Blob (responseType blob) for in-browser view/download.
  async function fetchFile(simId: string, analysisId: string, download = false): Promise<Blob> {
    const { data } = await apiClient.get<Blob>(`${BASE}/${simId}/analyses/${analysisId}/file`, {
      params: download ? { download: 1 } : {},
      responseType: 'blob',
    })
    return data
  }

  async function resultsSummary(simId: string): Promise<ResultsSummary> {
    const { data } = await apiClient.get<ResultsSummary>(`${BASE}/${simId}/results/summary`)
    return data
  }

  async function resultsSeries(
    simId: string,
    params: { property?: string; agents?: string[]; from?: number; to?: number; aggregation?: string; entity_type?: 'agent' | 'shape' } = {}
  ): Promise<ResultsSeries> {
    const { data } = await apiClient.get<ResultsSeries>(`${BASE}/${simId}/results/series`, { params })
    return data
  }

  return { create, list, get, remove, fetchFile, resultsSummary, resultsSeries }
}
