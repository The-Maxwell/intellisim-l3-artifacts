import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type {
  CreateSimulationRequest,
  GenerateRequest,
  GenerationProgressPayload,
  GenerationScope,
  LLMTarget,
  Simulation,
  SimulationSummary,
  SimulationValidationResult,
  UpdateSimulationRequest,
} from '@/shared/types'
import { useSimulationApi } from '@/modules/simulation/composables/useSimulationApi'
import { useSimulationEvents } from '@/modules/simulation/composables/useSimulationEvents'

// Owns the list of simulation summaries (sidebar/list view) and the single
// currently open simulation (editor view) with its validation + dirty tracking.
// Mirrors modelStore's setup-style Pinia convention: a single adoption sink
// (applyUpdate), a stale-echo + dirty-guarded external-update router
// (applyExternalUpdate) that parks conflicting updates in pendingExternal, and
// array/map normalization on adoption. The events composable is wired on open()
// so simulation.updated / generation.progress flow into the store live.
export const useSimulationStore = defineStore('simulation', () => {
  const api = useSimulationApi()
  const events = useSimulationEvents()

  const list = ref<SimulationSummary[]>([])
  const hasMore = ref(false)
  // Server-side pagination + sort + model filter for the list view (PrimeVue
  // lazy DataTable).
  const total = ref(0)
  const page = ref(1)
  const pageSize = ref(20)
  const sortField = ref<string>('updated_at')
  const sortOrder = ref<'asc' | 'desc'>('desc')
  const modelIds = ref<string[]>([])
  const current = ref<Simulation | null>(null)
  const validation = ref<SimulationValidationResult | null>(null)
  const dirty = ref(false)
  const loading = ref(false)
  const error = ref<string | null>(null)
  // The live generation-progress detail (run/stage/detail), mirrored from the
  // events composable. Null when no run is in progress.
  const progress = ref<GenerationProgressPayload | null>(null)
  // A simulation.updated arriving while the user has unsaved edits is parked
  // here (latest wins) instead of clobbering `current` — the editor surfaces an
  // accept/dismiss banner. Null when there is nothing pending.
  const pendingExternal = ref<Simulation | null>(null)

  // generating: the backend is running a generation pass on the open simulation.
  const generating = computed(() => current.value?.status === 'generating')
  // editable: the simulation may be (re)generated or manually edited. Mirrors
  // backend SimulationStatus.Editable() — only `ready` and `generation_failed`.
  // All other statuses (running/paused/finished/failed/stopped/queued) lock the
  // record, so the backend rejects generation with 409 not_editable.
  const editable = computed(
    () => current.value?.status === 'ready' || current.value?.status === 'generation_failed',
  )
  // startReady: validation says the simulation is ready to start a run.
  const startReady = computed(() => validation.value?.start_ready ?? false)
  // terminal: the run has ended, so a persisted world + widget history exists to
  // view. Drives the editor's "View results" entry point into the run screen's
  // replay mode. Mirrors the run store's `finished` grouping.
  const terminal = computed(
    () =>
      current.value?.status === 'finished' ||
      current.value?.status === 'stopped' ||
      current.value?.status === 'failed',
  )

  function asMessage(e: unknown, fallback: string): string {
    return e instanceof Error ? e.message : fallback
  }

  // Normalize null arrays/maps the backend emits for empty Go slices/maps to
  // real arrays/objects so the editor (v-for / .length / property maps) never
  // crashes on null. Mirrors modelStore.normalizeModel — shallow-but-sufficient.
  function normalizeSimulation(sim: Simulation): Simulation {
    return {
      ...sim,
      agents: (sim.agents ?? []).map((a) => ({
        ...a,
        properties: a.properties ?? {},
      })),
      environment: sim.environment
        ? {
            ...sim.environment,
            global_properties: sim.environment.global_properties ?? {},
            shapes: (sim.environment.shapes ?? []).map((s) => ({
              ...s,
              properties: s.properties ?? {},
            })),
          }
        : sim.environment,
      widgets: sim.widgets ?? [],
      status_history: sim.status_history ?? [],
      generation_runs: sim.generation_runs ?? [],
    }
  }

  // Single adoption sink for create/open/save/applyUpdate: normalize, store as
  // current, refresh validation, clear dirty.
  function applyUpdate(sim: Simulation): void {
    const normalized = normalizeSimulation(sim)
    current.value = normalized
    validation.value = normalized.validation
    dirty.value = false
  }

  // Route an UNTRUSTED simulation.updated SSE event (fires for LLM generation
  // AND as an echo of the user's own PUT) through here rather than directly to
  // applyUpdate. Two guards protect in-progress local edits:
  //   1. Stale-echo guard: ignore any response not strictly newer than the one
  //      we already hold (the own-save echo carries the SAME updated_at the PUT
  //      response already adopted, so it is a no-op).
  //   2. Dirty guard: with unsaved edits, park the (normalized) response in
  //      pendingExternal (latest wins) and leave current/dirty untouched so the
  //      editor can offer accept/dismiss; otherwise adopt it like applyUpdate.
  function applyExternalUpdate(sim: Simulation): void {
    if (current.value && Date.parse(sim.updated_at) <= Date.parse(current.value.updated_at)) {
      return
    }
    if (!dirty.value) {
      applyUpdate(sim)
      return
    }
    pendingExternal.value = normalizeSimulation(sim)
  }

  // Discard local edits and adopt the parked external update via the normal sink.
  function acceptExternalUpdate(): void {
    if (!pendingExternal.value) return
    applyUpdate(pendingExternal.value)
    pendingExternal.value = null
  }

  // Keep local edits/dirty; drop the parked external update.
  function dismissExternalUpdate(): void {
    pendingExternal.value = null
  }

  function markDirty(): void {
    dirty.value = true
  }

  // Server-side paged/sorted/filtered load. Any field left undefined keeps its
  // current value, so callers can update just the page, just the model filter,
  // etc. A null sortField resets to the canonical default (updated_at desc). A
  // no-arg reload preserves the current page/sort/filter (used after
  // create/duplicate/delete).
  async function loadList(opts: {
    page?: number
    pageSize?: number
    modelIds?: string[]
    sortField?: string | null
    sortOrder?: 'asc' | 'desc'
  } = {}): Promise<void> {
    if (opts.page !== undefined) page.value = opts.page
    if (opts.pageSize !== undefined) pageSize.value = opts.pageSize
    if (opts.modelIds !== undefined) modelIds.value = opts.modelIds
    if (opts.sortField !== undefined) sortField.value = opts.sortField ?? 'updated_at'
    if (opts.sortOrder !== undefined) sortOrder.value = opts.sortOrder
    loading.value = true
    error.value = null
    try {
      const resp = await api.list(
        page.value,
        pageSize.value,
        modelIds.value,
        sortField.value,
        sortOrder.value
      )
      list.value = resp.items ?? []
      total.value = resp.total ?? 0
      hasMore.value = resp.has_more
      // Clamp: if we paged past the end (e.g. rows were deleted, the model
      // filter shrank the result set, or a stale deep-link) snap to the last
      // valid page and refetch once. Bounded — the clamped page returns rows.
      const lastPage = Math.max(1, Math.ceil(total.value / pageSize.value))
      if (page.value > lastPage) {
        page.value = lastPage
        const clamped = await api.list(
          page.value,
          pageSize.value,
          modelIds.value,
          sortField.value,
          sortOrder.value
        )
        list.value = clamped.items ?? []
        total.value = clamped.total ?? 0
        hasMore.value = clamped.has_more
      }
    } catch (e) {
      error.value = asMessage(e, 'Failed to load simulations')
    } finally {
      loading.value = false
    }
  }

  async function create(req: CreateSimulationRequest): Promise<Simulation> {
    const sim = await api.create(req)
    applyUpdate(sim)
    return sim
  }

  // Open a simulation: fetch it, adopt it, and subscribe to its SSE stream so
  // generation progress + external updates flow live. The view supplies optional
  // toast handlers for generation completed/failed (toast needs setup context).
  async function open(
    id: string,
    toast?: {
      onCompleted?: () => void
      onFailed?: (message: string) => void
    }
  ): Promise<void> {
    loading.value = true
    error.value = null
    pendingExternal.value = null
    try {
      const sim = await api.get(id)
      applyUpdate(sim)
    } catch (e) {
      error.value = asMessage(e, 'Failed to open simulation')
      current.value = null
      validation.value = null
      throw e
    } finally {
      loading.value = false
    }

    events.subscribe(id, {
      onSimulationUpdated: (sim) => applyExternalUpdate(sim),
      onGenerationProgress: (p) => {
        progress.value = p
      },
      onGenerationCompleted: () => {
        progress.value = null
        // Adopt the freshest server state (the simulation.updated event already
        // carried it, but refetch defends against a dropped/overflowed stream).
        void refreshCurrent()
        toast?.onCompleted?.()
      },
      onGenerationFailed: (p) => {
        progress.value = null
        void refreshCurrent()
        toast?.onFailed?.(p.error)
      },
      onSimulationDeleted: () => {
        if (current.value?.id === id) {
          current.value = null
          validation.value = null
          dirty.value = false
          pendingExternal.value = null
        }
        list.value = list.value.filter((s) => s.id !== id)
      },
      onReconnect: () => {
        void refreshCurrent()
      },
    })
  }

  // Refetch the open simulation and route it through the external-update guards
  // (so it never clobbers unsaved edits).
  async function refreshCurrent(): Promise<void> {
    const id = current.value?.id
    if (!id) return
    try {
      const sim = await api.get(id)
      applyExternalUpdate(sim)
    } catch {
      /* transient — the next event/refresh recovers */
    }
  }

  function close(): void {
    events.unsubscribe()
    progress.value = null
    current.value = null
    validation.value = null
    dirty.value = false
    pendingExternal.value = null
  }

  // Extract the editable subset of the current simulation as an update request.
  // Mirrors modelStore's editable-body extraction used by saveCurrent.
  function toUpdateRequest(sim: Simulation): UpdateSimulationRequest {
    return {
      name: sim.name,
      description: sim.description ?? '',
      steps: sim.steps,
      seed: sim.seed,
      llm_generation_target: sim.llm_generation_target,
      llm_execution_target: sim.llm_execution_target,
      agents: sim.agents ?? [],
      environment: sim.environment ?? null,
      widgets: sim.widgets ?? [],
    }
  }

  async function saveCurrent(): Promise<Simulation | null> {
    if (!current.value) return null
    const sim = await api.update(current.value.id, toUpdateRequest(current.value))
    applyUpdate(sim)
    // A save makes the just-persisted local state authoritative; drop any parked
    // external update (the PUT response above carries the freshest updated_at).
    pendingExternal.value = null
    return sim
  }

  // generate triggers a (re)generation run. The generation target override
  // defaults to the simulation's persisted llm_generation_target but the caller
  // (the editor's Generate-model picker) may pass an explicit one.
  async function generate(
    scope: GenerationScope,
    llmGenerationTarget?: LLMTarget | null
  ): Promise<Simulation | null> {
    if (!current.value) return null
    const req: GenerateRequest = {
      scope,
      llm_generation_target:
        llmGenerationTarget !== undefined ? llmGenerationTarget : current.value.llm_generation_target,
    }
    const sim = await api.generate(current.value.id, req)
    applyUpdate(sim)
    return sim
  }

  async function duplicate(id: string, name?: string): Promise<Simulation> {
    const sim = await api.duplicate(id, { name })
    return sim
  }

  async function remove(id: string): Promise<void> {
    await api.remove(id)
    list.value = list.value.filter((s) => s.id !== id)
    if (current.value?.id === id) {
      close()
      current.value = null
      validation.value = null
      dirty.value = false
      pendingExternal.value = null
    }
  }

  return {
    list,
    hasMore,
    total,
    page,
    pageSize,
    sortField,
    sortOrder,
    modelIds,
    current,
    validation,
    dirty,
    loading,
    error,
    progress,
    pendingExternal,
    generating,
    editable,
    startReady,
    terminal,
    loadList,
    create,
    open,
    close,
    saveCurrent,
    generate,
    duplicate,
    remove,
    refreshCurrent,
    applyUpdate,
    applyExternalUpdate,
    acceptExternalUpdate,
    dismissExternalUpdate,
    markDirty,
  }
})
