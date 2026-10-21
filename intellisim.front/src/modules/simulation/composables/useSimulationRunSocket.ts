import { onUnmounted, ref } from 'vue'
import { useSimulationApi } from '@/modules/simulation/composables/useSimulationApi'
import type { StreamMessage } from '@/shared/types'

// One live run-stream subscription per open simulation run. Mirrors
// useWebSocket's native-WebSocket mechanics EXACTLY (reconnect with exponential
// backoff capped at 30s, newline-delimited coalesced-frame parsing — the server
// packs several envelopes into one ws message separated by '\n', see
// modules/simulations/stream.go writePump) but layers in the run-phase
// connect-ticket auth model: every (re)connect first mints a FRESH single-use
// ticket over authenticated REST (POST /:id/stream-ticket) and opens the WS with
// it as a query param (the WS upgrade carries no bearer header — browsers can't
// set one — so the ticket is the only credential). On any drop the composable
// re-fetches a new ticket and reconnects; the caller refetches missed history
// via listSteps on reconnect.

export type RunSocketConnectionStatus = 'disconnected' | 'connecting' | 'connected' | 'reconnecting'

export interface RunSocketHandlers {
  /** A step/keyframe frame carrying agent/global/statistics deltas. */
  onFrame?: (msg: StreamMessage) => void
  /** A status/completion frame carrying a run lifecycle status string. */
  onStatus?: (status: string, completed: boolean) => void
  /** An error frame (or transport failure) — the view surfaces it as a toast. */
  onError?: (message: string) => void
  /** Fired on a successful re-open after a drop — caller refetches history. */
  onReconnect?: () => void
}

export interface UseSimulationRunSocketOptions extends RunSocketHandlers {
  maxRetries?: number
  baseDelay?: number
}

// Build the run WS URL the Vite dev server proxies (/ws → ws://localhost:8080).
// VITE_WS_BASE_URL already carries the /api/v1/intellisim prefix; we fall back to
// the current host (same construction the legacy getStreamUrl used).
function runStreamUrl(id: string, ticket: string): string {
  const wsBase =
    (import.meta.env.VITE_WS_BASE_URL as string | undefined) ||
    `ws://${window.location.host}/api/v1/intellisim`
  return `${wsBase}/simulations/${id}/ws?ticket=${encodeURIComponent(ticket)}`
}

export function useSimulationRunSocket(options: UseSimulationRunSocketOptions = {}) {
  const api = useSimulationApi()

  const connected = ref(false)
  // The transport-level connection state (mirrors useWebSocket.connectionStatus).
  const connectionStatus = ref<RunSocketConnectionStatus>('disconnected')
  // The latest step/keyframe frame routed to onFrame (also exposed reactively so
  // a view can bind directly without a handler).
  const lastFrame = ref<StreamMessage | null>(null)
  // The latest run lifecycle status string carried by a status/completion frame
  // (the contract's `status` reactive — distinct from connectionStatus).
  const status = ref<string | null>(null)
  // The step number of the most recent step/keyframe frame.
  const step = ref(0)

  const maxRetries = options.maxRetries ?? 10
  const baseDelay = options.baseDelay ?? 1000

  let ws: WebSocket | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let intentionalClose = false
  let retryCount = 0
  let openedOnce = false
  let activeId: string | null = null
  // Guards against a late open/close of a stale socket after disconnect()/reconnect.
  let generation = 0

  function routeMessage(raw: unknown): void {
    const msg = raw as StreamMessage
    if (!msg || typeof msg.type !== 'string') return
    switch (msg.type) {
      case 'step':
      case 'keyframe':
        lastFrame.value = msg
        if (typeof msg.step === 'number') step.value = msg.step
        options.onFrame?.(msg)
        break
      case 'status':
        if (msg.status) {
          status.value = msg.status
          options.onStatus?.(msg.status, false)
        }
        break
      case 'completion':
        if (msg.status) {
          status.value = msg.status
        }
        options.onStatus?.(msg.status ?? '', true)
        break
      case 'progress':
        // Progress rides the SSE /:id/events hub; the WS frame is reserved. Treat
        // it like a frame so step counters can advance if it ever carries one.
        if (typeof msg.step === 'number') step.value = msg.step
        options.onFrame?.(msg)
        break
      case 'error':
        options.onError?.(msg.error || 'Run stream error')
        break
    }
  }

  // Open the socket for the active id using a freshly-minted ticket. Each attempt
  // re-fetches a ticket so a reconnect never replays a consumed/expired one.
  async function openSocket(): Promise<void> {
    const id = activeId
    if (id === null) return
    const myGen = generation

    let ticket: string
    try {
      const resp = await api.streamTicket(id)
      ticket = resp.ticket
    } catch (err) {
      if (intentionalClose || myGen !== generation) return
      // A 4xx ticket mint is TERMINAL, not transient: 409 means the run has no
      // stream room (it isn't running/paused — e.g. not started yet, or already
      // finished), 403/404 mean not owned/gone. Looping would just spam the
      // endpoint, so stop. Transient failures (5xx / network) reconnect.
      const status = (err as { status?: number } | null)?.status
      if (typeof status === 'number' && status >= 400 && status < 500) {
        connectionStatus.value = 'disconnected'
        return
      }
      scheduleReconnect()
      return
    }

    // A disconnect()/new connect() may have happened while awaiting the ticket.
    if (intentionalClose || myGen !== generation) return

    const socket = new WebSocket(runStreamUrl(id, ticket))
    ws = socket

    socket.onopen = () => {
      if (myGen !== generation) {
        // Stale socket from a superseded attempt — close it quietly.
        try {
          socket.close()
        } catch {
          /* ignore */
        }
        return
      }
      connected.value = true
      connectionStatus.value = 'connected'
      retryCount = 0
      if (openedOnce) options.onReconnect?.()
      openedOnce = true
    }

    socket.onmessage = (event) => {
      try {
        // The server coalesces frames newline-delimited into one ws message.
        const parts = (event.data as string).split('\n').filter(Boolean)
        for (const part of parts) {
          routeMessage(JSON.parse(part))
        }
      } catch {
        // Non-JSON / partial message — ignore (best-effort delivery).
      }
    }

    socket.onclose = () => {
      if (myGen !== generation) return
      connected.value = false
      ws = null
      if (intentionalClose) {
        connectionStatus.value = 'disconnected'
        return
      }
      scheduleReconnect()
    }

    socket.onerror = () => {
      // onclose follows and drives the reconnect; surface a transient note.
      if (myGen !== generation || intentionalClose) return
      options.onError?.('Run stream connection error')
    }
  }

  function scheduleReconnect(): void {
    if (intentionalClose) return
    if (retryCount >= maxRetries) {
      connectionStatus.value = 'disconnected'
      return
    }
    const delay = Math.min(baseDelay * Math.pow(2, retryCount), 30000)
    retryCount++
    connectionStatus.value = 'reconnecting'
    reconnectTimer = setTimeout(() => {
      void openSocket()
    }, delay)
  }

  // connect(id): mint a ticket and open the run WS for this simulation. Calling
  // connect again (same or different id) tears down the previous socket first.
  function connect(id: string): void {
    disconnect()
    intentionalClose = false
    openedOnce = false
    retryCount = 0
    activeId = id
    generation++
    status.value = null
    lastFrame.value = null
    step.value = 0
    connectionStatus.value = 'connecting'
    void openSocket()
  }

  // disconnect(): abort the socket and cancel any pending reconnect.
  function disconnect(): void {
    intentionalClose = true
    generation++
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    if (ws) {
      const socket = ws
      ws = null
      try {
        socket.close()
      } catch {
        /* ignore */
      }
    }
    connected.value = false
    connectionStatus.value = 'disconnected'
    activeId = null
  }

  onUnmounted(disconnect)

  return {
    connected,
    connectionStatus,
    status,
    lastFrame,
    step,
    connect,
    disconnect,
  }
}
