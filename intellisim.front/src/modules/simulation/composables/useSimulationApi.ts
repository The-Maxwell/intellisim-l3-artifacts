import apiClient from '@/shared/api/client'
import type {
  ControlRequest,
  CreateSimulationRequest,
  DuplicateRequest,
  GenerateRequest,
  ListSimulationsResponse,
  ModelResponse,
  RunRequest,
  Simulation,
  SimulationAgent,
  StepsPageResponse,
  StreamTicketResponse,
  UpdateSimulationRequest,
} from '@/shared/types'

const BASE = '/simulations'

// Thin REST wrapper over the refactored simulations module endpoints (mirrors
// useModelApi) — CRUD plus the run/engine surface (run/control/stream-ticket/
// steps). Live step data streams over the run WebSocket (useSimulationRunSocket).
export function useSimulationApi() {
  async function list(
    page = 1,
    pageSize = 20,
    modelIds?: string[],
    sortBy?: string,
    sortOrder?: 'asc' | 'desc'
  ): Promise<ListSimulationsResponse> {
    const params: Record<string, string | number | string[]> = { page, page_size: pageSize }
    if (modelIds && modelIds.length) params.model_id = modelIds
    if (sortBy) params.sort_by = sortBy
    if (sortOrder) params.sort_order = sortOrder
    const { data } = await apiClient.get<ListSimulationsResponse>(BASE, {
      params,
      // Serialize model_id as repeated keys (model_id=a&model_id=b), which Echo's
      // c.QueryParams()["model_id"] reads — not the bracketed model_id[] default.
      paramsSerializer: { indexes: null },
    })
    return data
  }

  // Create is async on the backend (202 + generation runs in the background); the
  // returned Simulation is the initial (generating) state.
  async function create(req: CreateSimulationRequest): Promise<Simulation> {
    const { data } = await apiClient.post<Simulation>(BASE, req)
    return data
  }

  async function get(id: string): Promise<Simulation> {
    const { data } = await apiClient.get<Simulation>(`${BASE}/${id}`)
    return data
  }

  // Lazily fetches the FULL frozen source-model copy embedded in the simulation
  // (same JSON shape as GET /models/:id — the complete model incl. rules,
  // interactions, events, llm config, widgets, images). Read-only: used by the
  // "Model snapshot" tab. Distinct from the trimmed inline `model_snapshot` on
  // the Simulation response.
  async function getModelSnapshot(id: string): Promise<ModelResponse> {
    const { data } = await apiClient.get<ModelResponse>(`${BASE}/${id}/model-snapshot`)
    return data
  }

  // Full manual replace of all editable fields.
  async function update(id: string, req: UpdateSimulationRequest): Promise<Simulation> {
    const { data } = await apiClient.put<Simulation>(`${BASE}/${id}`, req)
    return data
  }

  // Triggers a (re)generation run (202 + background work; progress over SSE).
  async function generate(id: string, req: GenerateRequest): Promise<Simulation> {
    const { data } = await apiClient.post<Simulation>(`${BASE}/${id}/generate`, req)
    return data
  }

  async function duplicate(id: string, req: DuplicateRequest): Promise<Simulation> {
    const { data } = await apiClient.post<Simulation>(`${BASE}/${id}/duplicate`, req)
    return data
  }

  async function remove(id: string): Promise<void> {
    await apiClient.delete(`${BASE}/${id}`)
  }

  // --- run / engine phase ---

  // Starts an engine run (202 + background work; ready→queued→running). Live
  // step data streams over the run WebSocket; lifecycle over SSE. The returned
  // Simulation reflects the post-start state. The optional body carries an
  // execution-target override for this run.
  async function runSimulation(id: string, body: RunRequest = {}): Promise<Simulation> {
    const { data } = await apiClient.post<Simulation>(`${BASE}/${id}/run`, body)
    return data
  }

  // Signals the live run's control channel (pause/resume/stop + optional speed).
  async function controlSimulation(id: string, body: ControlRequest): Promise<void> {
    await apiClient.post(`${BASE}/${id}/control`, body)
  }

  // Mints a single-use, short-lived ticket used to authenticate the WS upgrade.
  async function streamTicket(id: string): Promise<StreamTicketResponse> {
    const { data } = await apiClient.post<StreamTicketResponse>(`${BASE}/${id}/stream-ticket`)
    return data
  }

  // Fetches persisted step history for replay/scrub (paged, [from,to] inclusive).
  async function listSteps(
    id: string,
    params: { from?: number; to?: number; page?: number; page_size?: number } = {}
  ): Promise<StepsPageResponse> {
    const { data } = await apiClient.get<StepsPageResponse>(`${BASE}/${id}/steps`, { params })
    return data
  }

  // Fetches a single agent's detail. While a run is active (running/paused) the
  // backend returns the LIVE runtime snapshot (fresh memory + state); otherwise
  // the persisted record. The response is wire-identical to SimulationAgent.
  // 404 { code: "agent_not_found" } for an unknown id.
  async function getAgent(id: string, agentId: number | string): Promise<SimulationAgent> {
    const { data } = await apiClient.get<SimulationAgent>(`${BASE}/${id}/agents/${agentId}`)
    return data
  }

  return {
    list,
    create,
    get,
    getModelSnapshot,
    update,
    generate,
    duplicate,
    remove,
    runSimulation,
    controlSimulation,
    streamTicket,
    listSteps,
    getAgent,
  }
}
