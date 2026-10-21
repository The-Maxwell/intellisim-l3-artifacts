// The editable slice of a Simulation — the subset the editor's form sections
// and JSON panel may change, mirroring the store's toUpdateRequest body. Excludes
// all read-only/server-owned fields (status, token_usage, generation_runs,
// model_snapshot, validation, timestamps, current_step, …). The JSON panel binds
// to THIS shape (not the whole response) so users can't edit read-only fields,
// and the editor view applies an EditableSimulation back onto store.current.
import type {
  LLMTarget,
  Simulation,
  SimulationAgent,
  SimulationEnvironment,
} from '@/shared/types/simulation'
import type { WidgetConfig } from '@/shared/types/widget'

export interface EditableSimulation {
  name: string
  description: string
  steps: number
  seed: number
  // Per-simulation LLM target overrides (generation seeds the world; execution
  // drives the run engine). Null/absent = inherit the model's configured target.
  llm_generation_target?: LLMTarget | null
  llm_execution_target?: LLMTarget | null
  agents: SimulationAgent[]
  environment: SimulationEnvironment | null
  widgets: WidgetConfig[]
}

// Project a full Simulation down to its editable slice. Widgets round-trip here
// even though this task ships no widgets form — they stay in the slice so the
// JSON panel can edit them and save preserves them.
export function toEditable(sim: Simulation): EditableSimulation {
  return {
    name: sim.name,
    description: sim.description ?? '',
    steps: sim.steps,
    seed: sim.seed,
    llm_generation_target: sim.llm_generation_target ?? null,
    llm_execution_target: sim.llm_execution_target ?? null,
    agents: sim.agents ?? [],
    environment: sim.environment ?? null,
    widgets: sim.widgets ?? [],
  }
}

// Merge an EditableSimulation slice back onto a full Simulation, leaving all
// read-only fields untouched.
export function applyEditable(sim: Simulation, edit: EditableSimulation): Simulation {
  return {
    ...sim,
    name: edit.name,
    description: edit.description,
    steps: edit.steps,
    seed: edit.seed,
    llm_generation_target: edit.llm_generation_target ?? null,
    llm_execution_target: edit.llm_execution_target ?? null,
    agents: edit.agents,
    environment: edit.environment,
    widgets: edit.widgets,
  }
}
