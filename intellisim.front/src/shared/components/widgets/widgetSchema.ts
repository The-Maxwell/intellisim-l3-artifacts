// Shared schema context + client-side validation for the TYPED widget editor
// (Task 17). One WidgetSchemaContext describes the agents, their property
// UIDs, the environment global property UIDs and the per-agent state lists —
// derived once from either a Model (model editor) or a ModelSnapshot (simulation
// editor) so the SAME WidgetEditor works against both.
//
// validateWidget() mirrors the backend ValidateWidget matrix in
// intellisim.core/data/common/widgets.go so the editor can refuse to save an
// invalid widget. It returns issues keyed loosely (no path indices needed in the
// editor — the editor validates ONE widget at a time and surfaces issues inline).

import type {
  WidgetConfig,
  WidgetSource,
  WidgetSourceType,
  WidgetType,
  WidgetAggregation,
  WidgetLogKind,
  WidgetMetric,
} from '@/shared/types/widget'
import { WIDGET_SOURCE_MATRIX } from '@/shared/types/widget'
import type {
  Model,
  Agent,
  AgentPropertyDTO,
} from '@/shared/types/model'
import type {
  ModelSnapshot,
  SnapshotAgent,
  SnapshotProperty,
} from '@/shared/types/simulation'

// One agent as the widget editor needs to see it: a uid + display label, the
// list of property UIDs (with their display names + numeric flag for aggregation
// hints) and the list of states.
export interface SchemaProperty {
  uid: string
  name: string
  /** true for `number` properties — aggregations/histograms only make sense here. */
  numeric: boolean
}

export interface SchemaAgent {
  uid: string
  name: string
  properties: SchemaProperty[]
  states: string[]
}

// The unified schema context the WidgetEditor binds to. Reusable across the model
// editor (built from a Model) and the simulation editor (built from a
// ModelSnapshot). It carries ONLY the structural lists the editor's pickers need.
export interface WidgetSchemaContext {
  agents: SchemaAgent[]
  /** Environment global property definitions (uid + name + numeric). */
  globals: SchemaProperty[]
}

const KNOWN_AGGREGATIONS: WidgetAggregation[] = ['avg', 'sum', 'min', 'max', 'count', 'median']
const KNOWN_LOG_KINDS: WidgetLogKind[] = ['events', 'state_transitions', 'llm_decisions']
// KNOWN_METRICS mirrors allWidgetMetrics in data/common/widgets.go.
const KNOWN_METRICS: WidgetMetric[] = [
  'total_agents',
  'agents_spawned',
  'agents_removed',
  'interactions',
  'events',
  'rule_executions',
  'movement_distance',
]
// KNOWN_SOURCE_TYPES mirrors allWidgetSourceTypes in data/common/widgets.go.
const KNOWN_SOURCE_TYPES: WidgetSourceType[] = ['agent_property', 'global_property', 'population', 'state_distribution', 'sim_metric']
// KNOWN_WIDGET_TYPES mirrors allWidgetTypes in data/common/widgets.go.
const KNOWN_WIDGET_TYPES: WidgetType[] = ['monitor', 'line_chart', 'bar_chart', 'pie_chart', 'histogram', 'agent_table', 'text_log']

function propIsNumeric(type: string | undefined): boolean {
  return type === 'number'
}

// --- builders -------------------------------------------------------------

function toSchemaProperties(props: { uid: string; name: string; type: string }[]): SchemaProperty[] {
  return props.map((p) => ({ uid: p.uid, name: p.name || p.uid, numeric: propIsNumeric(p.type) }))
}

// Build a schema context from an editable Model (model editor).
export function schemaFromModel(model: Model | null | undefined): WidgetSchemaContext {
  if (!model) return { agents: [], globals: [] }
  const agents: SchemaAgent[] = (model.agents ?? []).map((a: Agent) => ({
    uid: a.uid,
    name: a.name || a.uid,
    properties: toSchemaProperties(
      (a.properties ?? []).map((p: AgentPropertyDTO) => ({ uid: p.uid, name: p.name, type: p.type })),
    ),
    states: a.states ?? [],
  }))
  const globals = toSchemaProperties(
    (model.environment?.global_properties ?? []).map((p: AgentPropertyDTO) => ({
      uid: p.uid,
      name: p.name,
      type: p.type,
    })),
  )
  return { agents, globals }
}

// Build a schema context from a simulation's ModelSnapshot (simulation editor).
export function schemaFromSnapshot(snapshot: ModelSnapshot | null | undefined): WidgetSchemaContext {
  if (!snapshot) return { agents: [], globals: [] }
  const agents: SchemaAgent[] = (snapshot.agents ?? []).map((a: SnapshotAgent) => ({
    uid: a.uid,
    name: a.name || a.uid,
    properties: toSchemaProperties(
      (a.properties ?? []).map((p: SnapshotProperty) => ({ uid: p.uid, name: p.name, type: p.type })),
    ),
    states: a.states ?? [],
  }))
  const globals = toSchemaProperties(
    (snapshot.environment?.global_properties ?? []).map((p: SnapshotProperty) => ({
      uid: p.uid,
      name: p.name,
      type: p.type,
    })),
  )
  return { agents, globals }
}

// --- helpers for the editor pickers --------------------------------------

// The source types the editor may offer for a widget type (from the matrix).
export function allowedSourceTypes(type: WidgetType): WidgetSourceType[] {
  return WIDGET_SOURCE_MATRIX[type] ?? []
}

export function agentByUid(ctx: WidgetSchemaContext, uid: string): SchemaAgent | undefined {
  return ctx.agents.find((a) => a.uid === uid)
}

// Property options for a given source across a set of agents. For
// agent_property we offer the union of the selected agents' properties (or
// all agents' when none selected). For global_property we offer the globals.
export function propertyOptions(
  ctx: WidgetSchemaContext,
  sourceType: WidgetSourceType,
  agentUids: string[] | undefined,
): SchemaProperty[] {
  if (sourceType === 'global_property') return ctx.globals
  if (sourceType !== 'agent_property') return []
  const agents =
    agentUids && agentUids.length
      ? ctx.agents.filter((a) => agentUids.includes(a.uid))
      : ctx.agents
  const seen = new Map<string, SchemaProperty>()
  for (const a of agents) {
    for (const p of a.properties) {
      if (!seen.has(p.uid)) seen.set(p.uid, p)
    }
  }
  return [...seen.values()]
}

// --- client-side validation (mirror of backend ValidateWidget) ------------

export interface WidgetIssue {
  path: string
  reason: string
}

function validateSourceFields(
  src: WidgetSource | undefined,
  prefix: string,
  allowed: WidgetSourceType[],
  add: (path: string, reason: string) => void,
): void {
  if (!src) {
    add(prefix, 'a source is required')
    return
  }
  if (!KNOWN_SOURCE_TYPES.includes(src.type)) {
    add(`${prefix}.type`, `unknown source type "${src.type}"`)
    return
  }
  if (!allowed.includes(src.type)) {
    add(`${prefix}.type`, `source type "${src.type}" is not allowed for this widget`)
    return
  }
  switch (src.type) {
    case 'agent_property':
      if (!src.property_uid) add(`${prefix}.property_uid`, 'agent_property source requires a property')
      if (!src.aggregation) add(`${prefix}.aggregation`, 'agent_property source requires an aggregation')
      else if (!KNOWN_AGGREGATIONS.includes(src.aggregation))
        add(`${prefix}.aggregation`, `unknown aggregation "${src.aggregation}"`)
      if (src.metric) add(`${prefix}.metric`, 'source.metric is only valid for sim_metric sources')
      break
    case 'global_property':
      if (!src.property_uid) add(`${prefix}.property_uid`, 'global_property source requires a property')
      if (src.aggregation) add(`${prefix}.aggregation`, 'global_property source must not have an aggregation')
      if (src.agent_uids && src.agent_uids.length)
        add(`${prefix}.agent_uids`, 'global_property source must not specify agents')
      if (src.metric) add(`${prefix}.metric`, 'source.metric is only valid for sim_metric sources')
      break
    case 'population':
      if (src.property_uid) add(`${prefix}.property_uid`, 'population source must not specify a property')
      if (src.aggregation) add(`${prefix}.aggregation`, 'population source must not have an aggregation')
      if (src.metric) add(`${prefix}.metric`, 'source.metric is only valid for sim_metric sources')
      break
    case 'state_distribution':
      if (src.property_uid) add(`${prefix}.property_uid`, 'state_distribution source must not specify a property')
      if (src.aggregation) add(`${prefix}.aggregation`, 'state_distribution source must not have an aggregation')
      if (!src.agent_uids || src.agent_uids.length !== 1)
        add(`${prefix}.agent_uids`, 'state_distribution requires exactly one agent')
      if (src.metric) add(`${prefix}.metric`, 'source.metric is only valid for sim_metric sources')
      break
    case 'sim_metric':
      if (!src.metric) add(`${prefix}.metric`, 'sim_metric source requires metric')
      else if (!KNOWN_METRICS.includes(src.metric)) add(`${prefix}.metric`, `unknown metric "${src.metric}"`)
      if (src.property_uid) add(`${prefix}.property_uid`, 'sim_metric source must not specify property_uid')
      if (src.agent_uids && src.agent_uids.length)
        add(`${prefix}.agent_uids`, 'sim_metric source must not specify agent_uids')
      if (src.aggregation) add(`${prefix}.aggregation`, 'sim_metric source must not specify aggregation')
      break
  }
}

// validateWidget mirrors the backend matrix for ONE WidgetConfig. Returns the
// list of issues; an empty list means the widget is savable.
export function validateWidget(w: WidgetConfig): WidgetIssue[] {
  const issues: WidgetIssue[] = []
  const add = (path: string, reason: string) => issues.push({ path, reason })

  // ── universal checks (mirrors ValidateWidget universal section) ──────────
  if (!w.id) add('id', 'widget id is required')
  if (!w.title) add('title', 'title is required')
  if (!KNOWN_WIDGET_TYPES.includes(w.type)) {
    add('type', `unknown widget type "${w.type}"`)
    // Can't proceed with per-type checks — source matrix would be nonsense.
    return issues
  }

  switch (w.type) {
    case 'monitor': {
      if (w.series && w.series.length) add('series', 'monitor does not support series')
      if (w.columns && w.columns.length) add('columns', 'monitor does not support columns')
      if (w.log_kinds && w.log_kinds.length) add('log_kinds', 'monitor does not support log_kinds')
      validateSourceFields(w.source, 'source', ['agent_property', 'global_property', 'population', 'sim_metric'], add)
      break
    }
    case 'line_chart': {
      if (w.source) add('source', 'line_chart uses series, not a single source')
      if (!w.series || w.series.length === 0) {
        add('series', 'line_chart requires at least one series')
        break
      }
      w.series.forEach((s, i) => {
        if (!s.label) add(`series[${i}].label`, 'series label is required')
        validateSourceFields(s.source, `series[${i}].source`, ['agent_property', 'global_property', 'population', 'sim_metric'], add)
      })
      break
    }
    case 'bar_chart': {
      const hasSeries = !!w.series && w.series.length > 0
      const hasSource = !!w.source
      if (hasSeries && hasSource) {
        add('source', 'bar_chart must use either series or a single source, not both')
        break
      }
      if (!hasSeries && !hasSource) {
        add('source', 'bar_chart requires either series or a single source')
        break
      }
      if (hasSeries) {
        w.series!.forEach((s, i) => {
          if (!s.label) add(`series[${i}].label`, 'series label is required')
          validateSourceFields(s.source, `series[${i}].source`, ['agent_property', 'global_property', 'population'], add)
        })
      } else {
        validateSourceFields(w.source, 'source', ['population', 'state_distribution'], add)
      }
      break
    }
    case 'pie_chart': {
      if (w.series && w.series.length) add('series', 'pie_chart does not support series')
      validateSourceFields(w.source, 'source', ['population', 'state_distribution'], add)
      break
    }
    case 'histogram': {
      if (w.series && w.series.length) add('series', 'histogram does not support series')
      if (!w.source) {
        add('source', 'histogram requires a source')
        break
      }
      if (w.source.type !== 'agent_property') add('source.type', 'histogram source must be agent_property')
      else {
        if (!w.source.property_uid) add('source.property_uid', 'histogram requires a property')
        if (w.source.aggregation) add('source.aggregation', 'histogram uses raw values — no aggregation')
      }
      if (w.buckets != null && w.buckets < 2) add('buckets', 'buckets must be ≥ 2')
      break
    }
    case 'agent_table': {
      if (w.series && w.series.length) add('series', 'agent_table does not support series')
      if (!w.source) {
        add('source', 'agent_table requires a source')
        break
      }
      if (w.source.type !== 'agent_property') add('source.type', 'agent_table source must be agent_property')
      else {
        if (w.source.property_uid) add('source.property_uid', 'agent_table uses columns, not a property')
        if (w.source.aggregation) add('source.aggregation', 'agent_table source must not have an aggregation')
        if (!w.source.agent_uids || w.source.agent_uids.length !== 1)
          add('source.agent_uids', 'agent_table requires exactly one agent')
      }
      if (!w.columns || w.columns.length === 0) add('columns', 'agent_table requires at least one column')
      break
    }
    case 'text_log': {
      if (w.source) add('source', 'text_log does not support a source')
      if (w.series && w.series.length) add('series', 'text_log does not support series')
      if (w.columns && w.columns.length) add('columns', 'text_log does not support columns')
      if (!w.log_kinds || w.log_kinds.length === 0) {
        add('log_kinds', 'text_log requires at least one log kind')
        break
      }
      const seen = new Set<WidgetLogKind>()
      let dup = false
      for (const k of w.log_kinds) {
        if (!KNOWN_LOG_KINDS.includes(k)) add('log_kinds', `unknown log kind "${k}"`)
        if (seen.has(k)) dup = true
        seen.add(k)
      }
      if (dup) add('log_kinds', 'log kinds must not contain duplicates')
      break
    }
  }

  return issues
}
