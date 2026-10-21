// Widgets2 live-data path (Task 25). The monitor / pie / histogram / bar /
// agent_table widgets recompute directly from the latest streamed frame (the
// RunDashboard synthesises a live PreviewContext and binds it to those render
// components). This composable owns the only NET-NEW live-data work: folding the
// two TIME-axis widgets — line_chart and text_log — across steps.
//
// On every new step it:
//   • appends one point per line_chart series (lineSeriesValue at the frame),
//     keyed by step, into the run store's bounded series map; and
//   • derives this step's text_log entries (state transitions / decisions /
//     event-count deltas via deriveLogEntries) against the store's per-agent
//     baseline, then pushes the entries that match each text_log widget's
//     log_kinds into the store's bounded logs map.
//
// The same accumulate() drives both LIVE streaming (called from a watcher on the
// store's currentStep) and REPLAY (the dashboard re-accumulates one step at a
// time as it walks history up to the scrub point) — so a finished run shows the
// IDENTICAL line/log content it showed live ("same results after finish").

import {
  deriveLogEntries,
  lineSeriesValue,
  type PreviewContext,
} from '@/shared/components/widgets/widgetData'
import type { useRunStore } from '@/modules/simulation/stores/runStore'
import type { SimulationStatistics, WidgetConfig } from '@/shared/types'

type RunStore = ReturnType<typeof useRunStore>

export function useRunLiveData(store: RunStore) {
  // Fold ONE step into the line_chart series + text_log entries. ctx is the live
  // PreviewContext for this step (agents/environment/snapshot); stats is the
  // step's statistics (for event-count deltas); step is the step number. Guarded
  // so a chunked step folded twice is a no-op.
  function accumulate(
    widgets: WidgetConfig[],
    ctx: PreviewContext,
    stats: SimulationStatistics | null,
    step: number,
  ): void {
    if (!store.beginAccumulateStep(step)) return

    // --- line_chart: one point per series at this step --------------------
    for (const w of widgets) {
      if (w.type !== 'line_chart' || !w.series) continue
      for (const s of w.series) {
        const v = lineSeriesValue(s.source, ctx)
        if (v == null) continue
        store.pushSeriesPoint(w.id, s.label, step, v)
      }
    }

    // --- text_log: derive this step's lines, route to matching widgets ----
    const baseline = store.getLogBaseline()
    const derived = deriveLogEntries(
      ctx.agents,
      stats,
      baseline.states,
      baseline.decisions,
      baseline.statistics,
    )
    if (derived.length) {
      for (const w of widgets) {
        if (w.type !== 'text_log' || !w.log_kinds || w.log_kinds.length === 0) continue
        const kinds = new Set(w.log_kinds)
        for (const e of derived) {
          if (kinds.has(e.kind)) store.pushLogEntry(w.id, step, e.kind, e.message)
        }
      }
    }

    // --- advance the baseline to THIS step --------------------------------
    const states = new Map<number, string>()
    const decisions = new Map<number, string>()
    for (const a of ctx.agents) {
      if (a.state) states.set(a.id, a.state)
      const p = a.properties
      if (p) {
        const d = p.decision ?? p.last_decision ?? p._decision
        if (typeof d === 'string' && d) decisions.set(a.id, d)
      }
    }
    baseline.states = states
    baseline.decisions = decisions
    baseline.statistics = stats
  }

  return { accumulate }
}
