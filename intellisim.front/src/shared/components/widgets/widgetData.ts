// Pure compute helpers for the TYPED widget PREVIEW (Task 17). Given a
// WidgetConfig + the CURRENT editable initial-state data (SimulationAgent[],
// SimulationEnvironment, ModelSnapshot for labels), these functions derive the
// datapoints each preview render component shows — BEFORE any run. They compute
// from initial instance values only; there is no time-series/event data yet, so
// line_chart and text_log have no compute path (their components show a
// placeholder).
//
// Every helper guards empties/missing values: no NaN escapes; missing numbers are
// skipped; an empty population yields 0 / an empty list rather than crashing.

import type {
  WidgetAggregation,
  WidgetLogKind,
  WidgetSource,
  WidgetMetric,
} from '@/shared/types/widget'
import type {
  SimulationAgent,
  SimulationEnvironment,
  ModelSnapshot,
  SimulationStatistics,
  SnapshotAgent,
} from '@/shared/types/simulation'

// A simple {label, value} datapoint used by monitor / bar / pie / histogram.
export interface NamedValue {
  label: string
  value: number
}

// The full preview data bundle the render components draw from.
export interface PreviewContext {
  agents: SimulationAgent[]
  environment: SimulationEnvironment | null
  snapshot: ModelSnapshot | null
  // Cumulative run statistics — only present on the LIVE/replay run path
  // (sim_metric sources read from this); absent in the pre-run editor preview.
  statistics?: SimulationStatistics | null
}

// --- label resolution -----------------------------------------------------

function agentTypeLabel(snapshot: ModelSnapshot | null, uid: string): string {
  const a = snapshot?.agents?.find((x: SnapshotAgent) => x.uid === uid)
  return a?.name || uid
}

// Instances of the given agent uids (or all if none specified).
function instancesFor(agents: SimulationAgent[], agentUids: string[] | undefined): SimulationAgent[] {
  if (!agentUids || agentUids.length === 0) return agents
  const set = new Set(agentUids)
  return agents.filter((a) => set.has(a.agent_uid))
}

// Numeric values of one property across a set of instances (non-numbers skipped).
function numericValues(instances: SimulationAgent[], propertyUid: string): number[] {
  const out: number[] = []
  for (const a of instances) {
    const v = a.properties?.[propertyUid]
    if (typeof v === 'number' && Number.isFinite(v)) out.push(v)
  }
  return out
}

// --- aggregation ----------------------------------------------------------

export function aggregate(values: number[], agg: WidgetAggregation | undefined): number {
  if (agg === 'count') return values.length
  if (values.length === 0) return 0
  switch (agg) {
    case 'sum':
      return values.reduce((s, v) => s + v, 0)
    case 'min':
      return Math.min(...values)
    case 'max':
      return Math.max(...values)
    case 'median': {
      const sorted = [...values].sort((a, b) => a - b)
      const mid = Math.floor(sorted.length / 2)
      return sorted.length % 2 === 0 ? (sorted[mid - 1]! + sorted[mid]!) / 2 : sorted[mid]!
    }
    case 'avg':
    default:
      return values.reduce((s, v) => s + v, 0) / values.length
  }
}

// --- sim_metric (cumulative run counter) ----------------------------------

// Resolve a sim_metric source to its cumulative counter off the run statistics
// snapshot. Returns null when the statistics are absent (pre-run preview) or the
// metric is unknown (caller shows "—" / skips the point).
export function simMetricValue(
  metric: WidgetMetric | undefined,
  stats: SimulationStatistics | null | undefined,
): number | null {
  if (!stats || !metric) return null
  switch (metric) {
    case 'total_agents':
      return stats.total_agents
    case 'agents_spawned':
      return stats.agents_spawned
    case 'agents_removed':
      return stats.agents_removed
    case 'interactions':
      return stats.interactions
    case 'events':
      return stats.events
    case 'rule_executions':
      return stats.rule_executions
    case 'movement_distance':
      return stats.movement_distance
    default:
      return null
  }
}

// --- monitor (single scalar) ----------------------------------------------

// Compute the single number a monitor shows from its source. Returns null when
// the source can't be resolved against the current data (caller shows "—").
export function monitorValue(source: WidgetSource | undefined, ctx: PreviewContext): number | null {
  if (!source) return null
  switch (source.type) {
    case 'population':
      return instancesFor(ctx.agents, source.agent_uids).length
    case 'global_property': {
      const v = ctx.environment?.global_properties?.[source.property_uid ?? '']
      return typeof v === 'number' && Number.isFinite(v) ? v : null
    }
    case 'agent_property': {
      if (!source.property_uid) return null
      const insts = instancesFor(ctx.agents, source.agent_uids)
      const vals = numericValues(insts, source.property_uid)
      if (source.aggregation !== 'count' && vals.length === 0) return null
      return aggregate(vals, source.aggregation)
    }
    case 'sim_metric':
      return simMetricValue(source.metric, ctx.statistics)
    default:
      return null
  }
}

// --- population by agent ---------------------------------------------------

// One bar/slice per agent with its instance count. Honors an optional
// agent_uids filter on the source (only those agents are counted).
export function populationByAgent(source: WidgetSource | undefined, ctx: PreviewContext): NamedValue[] {
  const filter = source?.agent_uids
  const agentUids =
    filter && filter.length
      ? filter
      : // distinct agents present in the snapshot (fallback: present in agents)
        (ctx.snapshot?.agents?.map((a) => a.uid) ??
          [...new Set(ctx.agents.map((a) => a.agent_uid))])
  return agentUids.map((uid) => ({
    label: agentTypeLabel(ctx.snapshot, uid),
    value: ctx.agents.filter((a) => a.agent_uid === uid).length,
  }))
}

// --- state distribution (one agent) ---------------------------------------

// One bar/slice per state of the single referenced agent, counting its
// instances currently in that state. Includes the agent's declared states
// even when empty (so 0-count states still appear).
export function stateDistribution(source: WidgetSource | undefined, ctx: PreviewContext): NamedValue[] {
  const uid = source?.agent_uids?.[0]
  if (!uid) return []
  const agentDef = ctx.snapshot?.agents?.find((a) => a.uid === uid)
  const insts = ctx.agents.filter((a) => a.agent_uid === uid)
  const counts = new Map<string, number>()
  for (const s of agentDef?.states ?? []) counts.set(s, 0)
  for (const a of insts) {
    const s = a.state || '—'
    counts.set(s, (counts.get(s) ?? 0) + 1)
  }
  return [...counts.entries()].map(([label, value]) => ({ label, value }))
}

// --- histogram (buckets of an agent_property) -----------------------------

export interface HistogramBins {
  labels: string[]
  counts: number[]
}

// Bucket the initial values of an agent_property into `buckets` equal-width bins.
export function histogramBins(
  source: WidgetSource | undefined,
  buckets: number | undefined,
  ctx: PreviewContext,
): HistogramBins {
  const empty: HistogramBins = { labels: [], counts: [] }
  if (!source || source.type !== 'agent_property' || !source.property_uid) return empty
  const vals = numericValues(instancesFor(ctx.agents, source.agent_uids), source.property_uid)
  if (vals.length === 0) return empty
  const n = Math.max(2, Math.floor(buckets ?? 10))
  const min = Math.min(...vals)
  const max = Math.max(...vals)
  if (min === max) {
    // All identical — a single meaningful bucket; render as one bar.
    return { labels: [String(round(min))], counts: [vals.length] }
  }
  const width = (max - min) / n
  const counts = new Array<number>(n).fill(0)
  for (const v of vals) {
    let idx = Math.floor((v - min) / width)
    if (idx >= n) idx = n - 1 // include the max in the last bucket
    if (idx < 0) idx = 0
    counts[idx]!++
  }
  const labels = counts.map((_, i) => {
    const lo = min + i * width
    const hi = lo + width
    return `${round(lo)}–${round(hi)}`
  })
  return { labels, counts }
}

function round(n: number): number {
  return Math.round(n * 100) / 100
}

// --- agent table (one agent × selected columns) ---------------------------

export interface TableRow {
  [field: string]: unknown
}

// Build the rows for an agent_table: one row per instance of the single
// referenced agent, with the selected columns (property uids, plus the
// pseudo-columns id/name/state which the editor may expose).
export function tableRows(
  source: WidgetSource | undefined,
  columns: string[] | undefined,
  ctx: PreviewContext,
): TableRow[] {
  const uid = source?.agent_uids?.[0]
  if (!uid || !columns || columns.length === 0) return []
  const insts = ctx.agents.filter((a) => a.agent_uid === uid)
  return insts.map((a) => {
    const row: TableRow = {}
    for (const col of columns) {
      if (col === 'id') row[col] = a.id
      else if (col === 'name') row[col] = a.name
      else if (col === 'state') row[col] = a.state
      else row[col] = a.properties?.[col]
    }
    return row
  })
}

// --- multi-series bar chart -----------------------------------------------

// For a series-mode bar chart: each series collapses to one value (so the chart
// shows one bar per series). agent_property → aggregation; global_property → the
// raw global value; population → the filtered instance count.
export function seriesValues(
  series: { label: string; source: WidgetSource }[] | undefined,
  ctx: PreviewContext,
): NamedValue[] {
  if (!series) return []
  return series.map((s) => {
    const src = s.source
    let value = 0
    if (src.type === 'population') value = instancesFor(ctx.agents, src.agent_uids).length
    else if (src.type === 'global_property') {
      const v = ctx.environment?.global_properties?.[src.property_uid ?? '']
      value = typeof v === 'number' && Number.isFinite(v) ? v : 0
    } else if (src.type === 'agent_property' && src.property_uid) {
      value = aggregate(numericValues(instancesFor(ctx.agents, src.agent_uids), src.property_uid), src.aggregation)
    }
    return { label: s.label, value }
  })
}

// ===========================================================================
// LIVE-DATA path (Task 25). monitor/pie/histogram/bar/agent_table recompute
// directly from the PreviewContext above (RunDashboard synthesises a live ctx
// from the latest streamed frame). The two TIME-axis widgets — line_chart and
// text_log — have no pre-run compute path; the helpers below derive their data
// per step from the streamed frame so a run-stream composable can ACCUMULATE
// them across steps (and a replay can re-accumulate up to a scrub point).
// ===========================================================================

// lineSeriesValue resolves one line_chart series source to a SINGLE scalar at
// the current frame (the value to append at this step). Mirrors a single entry
// of seriesValues(): population → filtered instance count; global_property →
// the raw global value; agent_property → its aggregation. Returns null when the
// source can't be resolved against the frame (caller skips the point).
export function lineSeriesValue(source: WidgetSource | undefined, ctx: PreviewContext): number | null {
  if (!source) return null
  switch (source.type) {
    case 'population':
      return instancesFor(ctx.agents, source.agent_uids).length
    case 'global_property': {
      const v = ctx.environment?.global_properties?.[source.property_uid ?? '']
      return typeof v === 'number' && Number.isFinite(v) ? v : null
    }
    case 'agent_property': {
      if (!source.property_uid) return null
      const insts = instancesFor(ctx.agents, source.property_uid ? source.agent_uids : undefined)
      const vals = numericValues(insts, source.property_uid)
      if (source.aggregation !== 'count' && vals.length === 0) return null
      return aggregate(vals, source.aggregation)
    }
    case 'sim_metric':
      return simMetricValue(source.metric, ctx.statistics)
    default:
      return null
  }
}

// One derived text_log line for one step. kind is the WidgetLogKind it satisfies
// so the renderer can colour/tag it; a text_log widget keeps only the entries
// whose kind is in its log_kinds.
export interface DerivedLogEntry {
  kind: WidgetLogKind
  message: string
}

// agentLabel renders a stable per-instance label for a log line.
function agentLabel(a: SimulationAgent): string {
  const name = a.name && a.name !== String(a.id) ? a.name : ''
  return name ? `${name} (#${a.id})` : `#${a.id}`
}

// deriveLogEntries computes the text_log lines produced THIS step, given the
// current frame agents/statistics and the PREVIOUS frame's per-agent state +
// statistics counters (so deltas can be detected):
//   • state_transitions — an agent whose `state` changed since the last frame.
//   • llm_decisions     — the engine stamps the applied LLM decision summary on
//                         the frame (agent.decision, 2026-07-10). It is a per-
//                         step EVENT (present only on decision steps), so every
//                         occurrence is emitted unconditionally — identical
//                         consecutive decisions are distinct decisions. Legacy
//                         fallback for old data: a reserved `decision`/
//                         `last_decision` property, emitted only on change.
//   • events            — increases in the statistics by_event / by_interaction
//                         cumulative counters since the last frame.
// prevStates / prevDecisions map agent id → its prior value; prevStats is the
// statistics snapshot from the previous step (null on the first step). All are
// best-effort: a missing signal simply yields no entry of that kind.
export function deriveLogEntries(
  agents: SimulationAgent[],
  statistics: SimulationStatistics | null | undefined,
  prevStates: Map<number, string>,
  prevDecisions: Map<number, string>,
  prevStats: SimulationStatistics | null | undefined,
): DerivedLogEntry[] {
  const out: DerivedLogEntry[] = []

  for (const a of agents) {
    // state_transitions
    const prevState = prevStates.get(a.id)
    if (prevState !== undefined && a.state && a.state !== prevState) {
      out.push({ kind: 'state_transitions', message: `${agentLabel(a)}: ${prevState} → ${a.state}` })
    }
    // llm_decisions — explicit per-step field first (emit unconditionally),
    // reserved-property probe as the legacy fallback (emit on change only).
    if (a.decision) {
      out.push({ kind: 'llm_decisions', message: `${agentLabel(a)}: ${a.decision}` })
    } else {
      const decision = readDecision(a)
      if (decision) {
        const prevDecision = prevDecisions.get(a.id)
        if (decision !== prevDecision) {
          out.push({ kind: 'llm_decisions', message: `${agentLabel(a)}: ${decision}` })
        }
      }
    }
  }

  // events — count increases in the cumulative by_event / by_interaction tallies.
  if (statistics) {
    for (const e of statistics.by_event ?? []) {
      const before = eventCountBefore(prevStats, e.event_uid)
      const delta = e.emitted - before
      if (delta > 0) {
        out.push({ kind: 'events', message: `${e.event_uid} ×${delta}` })
      }
    }
    for (const i of statistics.by_interaction ?? []) {
      const before = interactionCountBefore(prevStats, i.interaction_id)
      const delta = i.count - before
      if (delta > 0) {
        out.push({ kind: 'events', message: `interaction ${i.interaction_id} ×${delta}` })
      }
    }
  }

  return out
}

// readDecision pulls a best-effort decision label off an agent's properties. The
// engine MAY write the agent's last LLM decision into a reserved property; we
// probe a small set of conventional keys (none → no llm_decisions entry).
function readDecision(a: SimulationAgent): string {
  const p = a.properties
  if (!p) return ''
  for (const key of ['decision', 'last_decision', '_decision']) {
    const v = p[key]
    if (typeof v === 'string' && v) return v
  }
  return ''
}

// eventCountBefore / interactionCountBefore read the cumulative count of one
// keyed stat row from a prior statistics snapshot; 0 when the snapshot is absent
// or has no such row (so a first appearance reports its full count as the delta).
function eventCountBefore(prev: SimulationStatistics | null | undefined, eventUID: string): number {
  for (const r of prev?.by_event ?? []) {
    if (r.event_uid === eventUID) return r.emitted
  }
  return 0
}

function interactionCountBefore(prev: SimulationStatistics | null | undefined, interactionID: string): number {
  for (const r of prev?.by_interaction ?? []) {
    if (r.interaction_id === interactionID) return r.count
  }
  return 0
}
