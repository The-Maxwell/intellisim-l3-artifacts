// Referential-option builders for the Condition/Action/Event builders.
//
// The rule/interaction/event forms author fields that reference other model
// entities by UID/ID (agents, shapes, events, interactions, agent + environment
// properties). These pure helpers turn a Model (+ optional applies_to scope)
// into PrimeVue Select/MultiSelect option lists ({label, value}) so the builders
// stop using free-text inputs. Backend remains authoritative on the wire values.
import type { Model } from '@/shared/types'
import { PseudoPropertyState, PseudoPropertyPosition, EnvironmentUID } from '@/shared/validation/modelRules'

export interface SelectOption {
  label: string
  value: string
}

// Agents as {label:name||uid, value:uid}.
export function agentOptions(model: Model): SelectOption[] {
  return (model.agents ?? []).map((a) => ({ label: a.name || a.uid, value: a.uid }))
}

// Shapes as {label:"name (shape)", value:uid} — prefixed so agents and shapes
// are distinguishable in a combined target list.
export function shapeOptions(model: Model): SelectOption[] {
  return (model.environment?.shapes ?? []).map((s) => ({
    label: `${s.name || s.uid} (shape)`,
    value: s.uid,
  }))
}

// Agents + shapes, for proximity/count/barrier/distance/move/teleport targets
// (the engine's nearestTargetPos matches both agent archetypes and shape UIDs).
export function targetOptions(model: Model): SelectOption[] {
  return [...agentOptions(model), ...shapeOptions(model)]
}

// Declared events as {label:name||uid, value:uid}.
export function eventOptions(model: Model): SelectOption[] {
  return (model.events ?? []).map((e) => ({ label: e.name || e.uid, value: e.uid }))
}

// Declared interactions as {label:name||id, value:id}.
export function interactionOptions(model: Model): SelectOption[] {
  return (model.interactions ?? []).map((it) => ({ label: it.name || it.id, value: it.id }))
}

// Environment global properties. When `mutableOnly` is set, immutable descriptors
// are excluded (set/modify against the environment can only touch mutable globals).
export function globalPropertyOptions(model: Model, mutableOnly = false): SelectOption[] {
  return (model.environment?.global_properties ?? [])
    .filter((p) => !mutableOnly || !p.immutable)
    .map((p) => ({ label: p.name || p.uid, value: p.uid }))
}

// Property UIDs applicable to a rule/interaction scope. When `appliesTo` lists
// agent UIDs, only those agents' properties are offered; otherwise the union of
// all agents' properties. The pseudo-properties `state` and `position` are added
// as extra options (the engine reads them via ConditionType position/state).
export function agentPropertyOptions(model: Model, appliesTo?: string[]): SelectOption[] {
  const scoped = appliesTo && appliesTo.length
    ? (model.agents ?? []).filter((a) => appliesTo.includes(a.uid))
    : (model.agents ?? [])
  const seen = new Set<string>()
  const opts: SelectOption[] = []
  for (const a of scoped) {
    for (const p of a.properties ?? []) {
      if (seen.has(p.uid)) continue
      seen.add(p.uid)
      opts.push({ label: p.name || p.uid, value: p.uid })
    }
  }
  opts.push(
    { label: 'state (pseudo)', value: PseudoPropertyState },
    { label: 'position (pseudo)', value: PseudoPropertyPosition }
  )
  return opts
}

// Declared states for a rule/interaction scope (union of applies_to agents'
// states, or all agents' states when unscoped).
export function stateOptions(model: Model, appliesTo?: string[]): SelectOption[] {
  const scoped = appliesTo && appliesTo.length
    ? (model.agents ?? []).filter((a) => appliesTo.includes(a.uid))
    : (model.agents ?? [])
  const seen = new Set<string>()
  const opts: SelectOption[] = []
  for (const a of scoped) {
    for (const s of a.states ?? []) {
      if (seen.has(s)) continue
      seen.add(s)
      opts.push({ label: s, value: s })
    }
  }
  return opts
}

// The reserved environment target sentinel, re-exported for the builders.
export { EnvironmentUID }
