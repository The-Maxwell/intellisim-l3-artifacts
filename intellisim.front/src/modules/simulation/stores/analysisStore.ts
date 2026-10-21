import { defineStore } from 'pinia'
import { ref } from 'vue'
import { useAnalysisApi } from '../composables/useAnalysisApi'
import { useSimulationEvents } from '../composables/useSimulationEvents'
import type { Analysis, AnalysisSummary, AnalyzeRequest } from '@/shared/types'

// Owns the analysis list + the currently-open analysis for one simulation, and
// the live SSE subscription that updates them as the pipeline runs.
export const useAnalysisStore = defineStore('analysis', () => {
  const api = useAnalysisApi()
  const events = useSimulationEvents()

  const simId = ref<string | null>(null)
  const list = ref<AnalysisSummary[]>([])
  const current = ref<Analysis | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)
  // analysisId -> {stage, percent} for in-flight analyses.
  const progress = ref<Record<string, { stage: string; percent: number }>>({})

  async function loadList(id: string): Promise<void> {
    loading.value = true
    error.value = null
    try {
      const res = await api.list(id)
      list.value = res.items
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to load analyses'
    } finally {
      loading.value = false
    }
  }

  async function open(id: string): Promise<void> {
    simId.value = id
    await loadList(id)
    // Subscribe once per simulation for analysis lifecycle events.
    events.subscribe(id, {
      onAnalysisProgress: (p) => {
        progress.value = { ...progress.value, [p.analysis_id]: { stage: p.stage, percent: p.percent } }
      },
      onAnalysisCompleted: (p) => {
        delete progress.value[p.analysis_id]
        progress.value = { ...progress.value }
        // Only refresh the open analysis — refreshing unconditionally would
        // replace `current` with whichever analysis happened to finish.
        if (current.value?.id === p.analysis_id) void refresh(p.analysis_id)
        void loadList(id)
      },
      onAnalysisFailed: (p) => {
        delete progress.value[p.analysis_id]
        progress.value = { ...progress.value }
        if (current.value?.id === p.analysis_id) void refresh(p.analysis_id)
        void loadList(id)
      },
    })
  }

  function close(): void {
    events.unsubscribe()
    simId.value = null
    list.value = []
    current.value = null
    progress.value = {}
  }

  async function create(req: AnalyzeRequest): Promise<Analysis | null> {
    if (!simId.value) return null
    const a = await api.create(simId.value, req)
    progress.value = { ...progress.value, [a.id]: { stage: 'queued', percent: 0 } }
    await loadList(simId.value)
    return a
  }

  async function refresh(analysisId: string): Promise<void> {
    if (!simId.value) return
    try {
      current.value = await api.get(simId.value, analysisId)
    } catch {
      /* ignore — may have been deleted */
    }
  }

  async function openAnalysis(analysisId: string): Promise<void> {
    if (!simId.value) return
    loading.value = true
    try {
      current.value = await api.get(simId.value, analysisId)
    } finally {
      loading.value = false
    }
  }

  // Close the opened analysis (back to the full-width list) WITHOUT tearing down
  // the list or the SSE subscription (unlike close()).
  function clearCurrent(): void {
    current.value = null
  }

  async function remove(analysisId: string): Promise<void> {
    if (!simId.value) return
    await api.remove(simId.value, analysisId)
    if (current.value?.id === analysisId) current.value = null
    await loadList(simId.value)
  }

  return { simId, list, current, loading, error, progress, open, close, create, refresh, openAnalysis, clearCurrent, remove }
})
