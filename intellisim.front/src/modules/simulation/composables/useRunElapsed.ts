import { computed, onScopeDispose, ref, watch } from 'vue'
import type { useRunStore } from '@/modules/simulation/stores/runStore'
import type { ExecutionRun, StatusChange } from '@/shared/types'
import { formatDuration } from '@/shared/utils/time'

// useRunElapsed derives the "active run time" for the run view — the wall-clock
// time the run spent RUNNING, excluding paused intervals. The number that
// matters (the frozen total shown once a run ends) is the backend-authoritative
// ExecutionRun.active_duration_ms; while a run is live we tick a client-side
// approximation (SSE frames carry no timestamps) that FREEZES on pause and
// resumes on resume. Opening / reloading an already-active run seeds the
// accumulator from the record's started_at + status_history so the value is
// sensible without having watched the run from step 0.
//
// Live value is intentionally approximate (subject to client/server clock skew
// on the open segment); the authoritative backend value replaces it on finish.

const TERMINAL = new Set(['finished', 'stopped', 'failed'])

// activeMsBetween mirrors the backend activeRunDuration over a [startMs, endMs]
// window: total span minus the paused gaps found in the history within it.
function activeMsBetween(startMs: number, endMs: number, history: StatusChange[]): number {
  const total = endMs - startMs
  if (total <= 0) return 0
  const entries = history
    .map((h) => ({ status: h.status, at: Date.parse(h.at) }))
    .filter((e) => !Number.isNaN(e.at))
    .sort((a, b) => a.at - b.at)
  let paused = 0
  for (let i = 0; i < entries.length; i++) {
    const e = entries[i]!
    if (e.status !== 'paused') continue
    if (e.at < startMs || e.at > endMs) continue
    let next = endMs
    const nextEntry = entries[i + 1]
    if (nextEntry && nextEntry.at <= endMs) next = nextEntry.at
    const gap = next - e.at
    if (gap > 0) paused += gap
  }
  const active = total - paused
  return active > 0 ? active : 0
}

export function useRunElapsed(store: ReturnType<typeof useRunStore>) {
  // Ticking client clock (updated every second while running).
  const now = ref(Date.now())
  // Completed running segments (ms) + the open segment's start (client epoch).
  const accumulatedMs = ref(0)
  const segmentStart = ref<number | null>(null)
  // The started_at (ms) of the run we already seeded, so we seed exactly once
  // when we FIRST observe an already-active run; null status has never fired.
  let seededRun: number | null = null
  let lastStatus: string | null = null
  let timer: ReturnType<typeof setInterval> | null = null

  const latestRun = computed<ExecutionRun | null>(() => {
    const runs = store.simulation?.execution_runs
    return runs && runs.length ? (runs[runs.length - 1] ?? null) : null
  })

  function stopTicker(): void {
    if (timer) {
      clearInterval(timer)
      timer = null
    }
  }
  function startTicker(): void {
    now.value = Date.now()
    if (!timer) timer = setInterval(() => (now.value = Date.now()), 1000)
  }

  function closeSegment(): void {
    if (segmentStart.value != null) {
      accumulatedMs.value += Math.max(0, Date.now() - segmentStart.value)
      segmentStart.value = null
    }
  }

  // Seed the accumulator from a persisted, already-active run.
  function seed(startMs: number, status: string): void {
    const history = store.simulation?.status_history ?? []
    const entries = history
      .map((h) => ({ status: h.status, at: Date.parse(h.at) }))
      .filter((e) => !Number.isNaN(e.at) && e.at >= startMs)
      .sort((a, b) => a.at - b.at)
    if (status === 'running') {
      const lastRunning = [...entries].reverse().find((e) => e.status === 'running')
      const openStart = lastRunning ? lastRunning.at : startMs
      accumulatedMs.value = activeMsBetween(startMs, openStart, history)
      // Server-epoch start compared against the client clock — small skew only.
      segmentStart.value = openStart
    } else {
      const lastPaused = [...entries].reverse().find((e) => e.status === 'paused')
      const pauseAt = lastPaused ? lastPaused.at : startMs
      accumulatedMs.value = activeMsBetween(startMs, pauseAt, history)
      segmentStart.value = null
    }
  }

  function sync(): void {
    const status = store.runStatus
    const startISO = store.simulation?.started_at ?? latestRun.value?.started_at
    const startMs = startISO ? Date.parse(startISO) : NaN

    // First observation of an already-active run → seed from the record.
    if (
      (status === 'running' || status === 'paused') &&
      !Number.isNaN(startMs) &&
      seededRun !== startMs &&
      lastStatus === null
    ) {
      seed(startMs, status)
      seededRun = startMs
      lastStatus = status
      if (status === 'running') startTicker()
      else stopTicker()
      return
    }

    if (status !== lastStatus) {
      if (status === 'running') {
        // Fresh in-session start or resume: open a new segment now.
        if (segmentStart.value === null) segmentStart.value = Date.now()
      } else if (status === 'paused' || TERMINAL.has(status ?? '')) {
        closeSegment()
      } else {
        // ready / queued / generating: reset for a future run.
        accumulatedMs.value = 0
        segmentStart.value = null
        seededRun = null
      }
      lastStatus = status
    }

    if (status === 'running') startTicker()
    else stopTicker()
  }

  watch(
    () => [store.runStatus, store.simulation?.started_at, latestRun.value?.started_at] as const,
    () => sync(),
    { immediate: true }
  )

  onScopeDispose(stopTicker)

  const elapsedMs = computed(() => {
    const status = store.runStatus
    if (TERMINAL.has(status ?? '')) {
      return latestRun.value?.active_duration_ms ?? accumulatedMs.value
    }
    if (status === 'running' && segmentStart.value != null) {
      return accumulatedMs.value + Math.max(0, now.value - segmentStart.value)
    }
    if (status === 'paused') return accumulatedMs.value
    return 0
  })

  const elapsedLabel = computed(() => formatDuration(elapsedMs.value))
  // True whenever there is a duration worth showing (a run has started).
  const hasElapsed = computed(
    () => store.runStatus === 'running' || store.runStatus === 'paused' || TERMINAL.has(store.runStatus ?? '')
  )

  return { elapsedMs, elapsedLabel, hasElapsed }
}
