import { ref } from 'vue'
import { fetchEventSource } from '@microsoft/fetch-event-source'
import type {
  AnalysisCompletedPayload,
  AnalysisFailedPayload,
  AnalysisProgressPayload,
  GenerationCompletedPayload,
  GenerationFailedPayload,
  GenerationProgressPayload,
  RunCompletionPayload,
  RunProgressPayload,
  Simulation,
  SimulationDeletedPayload,
} from '@/shared/types'

// One live SSE subscription per open simulation. Mirrors useModelChatEvents'
// fetch-based EventSource mechanics EXACTLY (fetch-event-source with a real
// Authorization header — native EventSource cannot send headers — a per-attempt
// fetch so every reconnect carries the CURRENT token, openWhenHidden, infinite
// retry until abort()) and useModelChatSession's per-id session caching (exactly
// ONE connection per simulation id; close() evicts the cache entry).

export interface SimulationEventHandlers {
  /** Full updated simulation arrives on simulation.updated (validation inside). */
  onSimulationUpdated?: (sim: Simulation) => void
  onGenerationProgress?: (p: GenerationProgressPayload) => void
  onGenerationCompleted?: (p: GenerationCompletedPayload) => void
  onGenerationFailed?: (p: GenerationFailedPayload) => void
  /** Live run progress (step counter + status) — the always-on lifecycle channel. */
  onRunProgress?: (p: RunProgressPayload) => void
  /** Terminal run transition (finished/stopped/failed). */
  onRunCompleted?: (p: RunCompletionPayload) => void
  onSimulationDeleted?: (p: SimulationDeletedPayload) => void
  /** Level-3 analysis pipeline progress. */
  onAnalysisProgress?: (p: AnalysisProgressPayload) => void
  onAnalysisCompleted?: (p: AnalysisCompletedPayload) => void
  onAnalysisFailed?: (p: AnalysisFailedPayload) => void
  /** Fired on a successful re-open after a drop — refetch state, no replay. */
  onReconnect?: () => void
}

const RETRY_MS = 3000

// Low-level subscriber: opens GET /simulations/:id/events and routes named
// events to handlers. Returns an unsubscribe (idempotent abort).
function subscribeSimulationEvents(
  simulationId: string,
  handlers: SimulationEventHandlers
): () => void {
  const ctrl = new AbortController()
  const base = import.meta.env.VITE_API_BASE_URL || '/api/v1/intellisim'
  let openedOnce = false

  ;(async () => {
    const { useAuthStore } = await import('@/modules/auth/stores/authStore')
    const auth = useAuthStore()

    await fetchEventSource(`${base}/simulations/${simulationId}/events`, {
      signal: ctrl.signal,
      openWhenHidden: true,
      // Per-attempt fetch so every (re)connect carries the CURRENT token.
      fetch: async (input, init) => {
        const token = await auth.getValidAccessToken()
        const headers = new Headers(init?.headers)
        if (token) headers.set('Authorization', `Bearer ${token}`)
        return window.fetch(input, { ...init, headers })
      },
      async onopen(response) {
        if (response.ok) {
          if (openedOnce) handlers.onReconnect?.()
          openedOnce = true
          return
        }
        if (response.status === 401) await auth.renew() // next retry uses fresh token
        throw new Error(`SSE open failed: ${response.status}`)
      },
      onmessage(ev) {
        if (!ev.event || !ev.data) return
        const data: unknown = JSON.parse(ev.data)
        switch (ev.event) {
          case 'simulation.updated':
            handlers.onSimulationUpdated?.(data as Simulation)
            break
          case 'generation.progress':
            handlers.onGenerationProgress?.(data as GenerationProgressPayload)
            break
          case 'generation.completed':
            handlers.onGenerationCompleted?.(data as GenerationCompletedPayload)
            break
          case 'generation.failed':
            handlers.onGenerationFailed?.(data as GenerationFailedPayload)
            break
          case 'run.progress':
            handlers.onRunProgress?.(data as RunProgressPayload)
            break
          case 'run.completed':
            handlers.onRunCompleted?.(data as RunCompletionPayload)
            break
          case 'simulation.deleted':
            handlers.onSimulationDeleted?.(data as SimulationDeletedPayload)
            break
          case 'analysis.progress':
            handlers.onAnalysisProgress?.(data as AnalysisProgressPayload)
            break
          case 'analysis.completed':
            handlers.onAnalysisCompleted?.(data as AnalysisCompletedPayload)
            break
          case 'analysis.failed':
            handlers.onAnalysisFailed?.(data as AnalysisFailedPayload)
            break
        }
      },
      onerror() {
        return RETRY_MS // never give up; abort() is the only exit
      },
    }).catch(() => {
      /* aborted */
    })
  })()

  return () => ctrl.abort()
}

// useSimulationEvents wraps the subscriber with session-style bookkeeping: ONE
// subscription per id, a reactive `progress` ref the store/UI can bind, and
// subscribe/unsubscribe lifecycle. The store (lifecycle owner) supplies the
// data-routing handlers; the consuming view supplies toast handlers (toast must
// run in component setup context, not in a store action).
export function useSimulationEvents() {
  const progress = ref<GenerationProgressPayload | null>(null)

  let unsub: (() => void) | null = null
  let subscribedId: string | null = null

  function subscribe(id: string, handlers: SimulationEventHandlers = {}): void {
    // Same simulation already subscribed — keep the single live connection.
    if (id === subscribedId && unsub !== null) return
    unsubscribe()
    subscribedId = id
    progress.value = null
    unsub = subscribeSimulationEvents(id, {
      ...handlers,
      onGenerationProgress: (p) => {
        progress.value = p
        handlers.onGenerationProgress?.(p)
      },
      // A completed/failed run ends the progress stream.
      onGenerationCompleted: (p) => {
        progress.value = null
        handlers.onGenerationCompleted?.(p)
      },
      onGenerationFailed: (p) => {
        progress.value = null
        handlers.onGenerationFailed?.(p)
      },
    })
  }

  function unsubscribe(): void {
    unsub?.()
    unsub = null
    subscribedId = null
    progress.value = null
  }

  return { progress, subscribe, unsubscribe }
}
