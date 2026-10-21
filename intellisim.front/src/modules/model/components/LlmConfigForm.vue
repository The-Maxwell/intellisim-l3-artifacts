<template>
  <div class="space-y-3">
    <!-- Execution target (capability-filtered per variant). For the per-agent
         (archetype) variant the target is an OPTIONAL override that inherits the
         model/simulation target field-wise when left empty. -->
    <div>
      <label class="text-[11px] font-semibold uppercase tracking-wide text-zinc-400 mb-1 block">
        {{ variant === 'agent' ? 'Target override' : 'Execution target' }}
      </label>
      <p v-if="variant === 'agent'" class="text-[11px] text-zinc-400 mb-1 leading-snug">
        Inherits the model / simulation target (merged field-wise) when left empty.
        Decision-type strategies may use any target — the backend validates capability at run start.
      </p>
      <LlmExecutionConfig
        :modelValue="modelValue.target ?? null"
        :capability="targetCapability"
        @update:modelValue="(t) => patch({ target: t })"
      />
    </div>

    <!-- Prompt context -->
    <div>
      <label class="text-[11px] text-zinc-500 mb-0.5 block">Prompt context</label>
      <Textarea
        :modelValue="modelValue.prompt_context ?? ''"
        class="w-full text-xs"
        rows="2"
        autoResize
        @update:modelValue="(v) => patch({ prompt_context: v || undefined })"
      />
    </div>

    <!-- Strategy (collapsible) -->
    <Fieldset legend="Strategy" :toggleable="true" :collapsed="true" class="text-sm">
      <div class="space-y-2">
        <div class="flex gap-2">
          <div class="w-40">
            <label class="text-[11px] text-zinc-500 mb-0.5 block">Type</label>
            <Select
              :modelValue="strategy.type"
              :options="LLM_STRATEGY_TYPES as unknown as string[]"
              class="w-full"
              size="small"
              @update:modelValue="(v) => patchStrategy({ type: v })"
            />
          </div>
          <div class="w-32">
            <label class="text-[11px] text-zinc-500 mb-0.5 block">Batch size</label>
            <InputNumber
              :modelValue="strategy.batch_size ?? null"
              class="w-full"
              size="small"
              :min="0"
              @update:modelValue="(v) => patchStrategy({ batch_size: v ?? 0 })"
            />
          </div>
          <div class="w-36">
            <label class="text-[11px] text-zinc-500 mb-0.5 block">Decision frequency</label>
            <InputNumber
              :modelValue="strategy.decision_frequency ?? null"
              class="w-full"
              size="small"
              :min="0"
              @update:modelValue="(v) => patchStrategy({ decision_frequency: v ?? 0 })"
            />
          </div>
        </div>

        <p class="text-[11px] text-zinc-400">{{ strategyHint }}</p>

        <!-- Archetype result application — archetype agents only (hidden for shapes). -->
        <div v-if="variant !== 'shape'" class="space-y-2">
          <div class="flex gap-2">
            <div class="w-44">
              <label class="text-[11px] text-zinc-500 mb-0.5 block">Archetype result mode</label>
              <Select
                :modelValue="archetypeMode"
                :options="LLM_ARCHETYPE_MODES as unknown as string[]"
                class="w-full"
                size="small"
                data-testid="archetype-mode"
                @update:modelValue="(v) => patchStrategy({ archetype_mode: v as LLMArchetypeMode })"
              />
            </div>
            <div class="w-36">
              <label class="text-[11px] text-zinc-500 mb-0.5 block">Variation strength</label>
              <InputNumber
                :modelValue="strategy.variation_strength ?? null"
                class="w-full"
                size="small"
                :min="0"
                :max="1"
                :step="0.05"
                showButtons
                placeholder="default"
                :minFractionDigits="0"
                :maxFractionDigits="2"
                data-testid="variation-strength"
                @update:modelValue="(v) => patchStrategy({ variation_strength: v ?? undefined })"
              />
            </div>
          </div>
          <p class="text-[11px] text-zinc-400">{{ archetypeHint }}</p>
        </div>

        <div class="grid grid-cols-2 gap-x-4 gap-y-1">
          <label
            v-for="f in strategyToggles"
            :key="f.key"
            class="flex items-center gap-2 text-xs"
            :class="flagsEditable ? 'text-zinc-600 cursor-pointer' : 'text-zinc-400 cursor-not-allowed'"
          >
            <ToggleSwitch
              :modelValue="displayFlags[f.key]"
              :disabled="!flagsEditable"
              @update:modelValue="(v) => patchStrategy({ [f.key]: !!v })"
            />
            <span>{{ f.label }}</span>
          </label>
        </div>
      </div>
    </Fieldset>

    <!-- Simulation config (collapsible) — execution + agent variants only -->
    <Fieldset
      v-if="variant !== 'shape'"
      legend="Simulation config"
      :toggleable="true"
      :collapsed="true"
      class="text-sm"
    >
      <div class="space-y-2">
        <!-- Agent variant: optional nested simulation_config behind a toggle.
             Execution variant: the runtime knobs are inline + always present. -->
        <label
          v-if="variant === 'agent'"
          class="flex items-center gap-2 text-xs text-zinc-600 cursor-pointer"
        >
          <ToggleSwitch :modelValue="hasSimConfig" @update:modelValue="onToggleSimConfig" />
          <span>Define simulation config</span>
        </label>

        <template v-if="hasSimConfig">
          <div class="flex gap-2">
            <div class="flex-1">
              <label class="text-[11px] text-zinc-500 mb-0.5 block">Perception radius</label>
              <InputNumber
                :modelValue="simConfig?.perception_radius ?? null"
                class="w-full"
                size="small"
                :min="0"
                @update:modelValue="(v) => patchSim({ perception_radius: v ?? 0 })"
              />
            </div>
            <div class="flex-1">
              <label class="text-[11px] text-zinc-500 mb-0.5 block">Interaction radius</label>
              <InputNumber
                :modelValue="simConfig?.interaction_radius ?? null"
                class="w-full"
                size="small"
                :min="0"
                @update:modelValue="(v) => patchSim({ interaction_radius: v ?? 0 })"
              />
            </div>
            <div class="flex-1">
              <label class="text-[11px] text-zinc-500 mb-0.5 block">Memory size</label>
              <InputNumber
                :modelValue="simConfig?.memory_size ?? null"
                class="w-full"
                size="small"
                :min="0"
                @update:modelValue="(v) => patchSim({ memory_size: v ?? 0 })"
              />
            </div>
          </div>

          <!-- Runner-only fields: execution variant only (inline on the
               execution config). -->
          <div v-if="variant === 'execution'" class="flex flex-wrap gap-2">
            <div class="w-36">
              <label class="text-[11px] text-zinc-500 mb-0.5 block">Parallel workers</label>
              <InputNumber
                :modelValue="(simConfig as LLMSimulationExecutionConfigDTO)?.parallel_workers ?? null"
                class="w-full"
                size="small"
                :min="0"
                @update:modelValue="(v) => patchSim({ parallel_workers: v ?? 0 })"
              />
            </div>
            <div class="w-36">
              <label class="text-[11px] text-zinc-500 mb-0.5 block">Retry timeout (ms)</label>
              <InputNumber
                :modelValue="(simConfig as LLMSimulationExecutionConfigDTO)?.retry_timeout_ms ?? null"
                class="w-full"
                size="small"
                :min="0"
                @update:modelValue="(v) => patchSim({ retry_timeout_ms: v ?? 0 })"
              />
            </div>
            <div class="w-32">
              <label class="text-[11px] text-zinc-500 mb-0.5 block">Max retries</label>
              <InputNumber
                :modelValue="(simConfig as LLMSimulationExecutionConfigDTO)?.max_retries ?? null"
                class="w-full"
                size="small"
                :min="0"
                @update:modelValue="(v) => patchSim({ max_retries: v ?? 0 })"
              />
            </div>
            <div class="w-36">
              <label class="text-[11px] text-zinc-500 mb-0.5 block">Stream batch size</label>
              <InputNumber
                :modelValue="(simConfig as LLMSimulationExecutionConfigDTO)?.stream_batch_size ?? null"
                class="w-full"
                size="small"
                :min="0"
                @update:modelValue="(v) => patchSim({ stream_batch_size: v ?? 0 })"
              />
            </div>
          </div>

          <!-- Fallback policy steps — empty list emits undefined so "unset =
               inherit the model default" is preserved. -->
          <FallbackPolicyEditor
            :modelValue="simConfig?.fallback_policy ?? null"
            @update:modelValue="setFallbackPolicy"
          />
        </template>
      </div>
    </Fieldset>
  </div>
</template>

<script setup lang="ts">
// Edits an LLM config object — the simulation-execution config
// (LLMSimulationExecutionConfigDTO), per-agent (LLMAgentConfigDTO), or per-shape
// (LLMShapeConfigDTO). The shapes differ:
//   • execution: strategy lives at `default_strategy`; the runtime knobs
//     (perception/interaction/memory/fallback + parallel_workers etc.) are
//     INLINE on the config root.
//   • agent/shape: strategy lives at `strategy`; the agent variant has an
//     OPTIONAL nested `simulation_config` (the agent-runtime subset only).
// A `variant` prop normalizes access. These are user-only fields (the manual
// editor). Always emits the full updated config.
import LlmExecutionConfig from '@/shared/components/LlmExecutionConfig.vue'
import FallbackPolicyEditor from './FallbackPolicyEditor.vue'
import Textarea from 'primevue/textarea'
import Select from 'primevue/select'
import InputNumber from 'primevue/inputnumber'
import ToggleSwitch from 'primevue/toggleswitch'
import Fieldset from 'primevue/fieldset'
import { computed } from 'vue'
import type {
  ExecutionLlmTarget,
  LLMSimulationExecutionConfigDTO,
  LLMAgentConfigDTO,
  LLMShapeConfigDTO,
  LLMStrategyDTO,
  LLMArchetypeMode,
  LLMSimulationAgentConfigDTO,
  FallbackPolicyDTO,
} from '@/shared/types'
import {
  LLM_STRATEGY_TYPES,
  LLM_ARCHETYPE_MODES,
} from '@/shared/validation/modelRules'

type AnyLlmConfig = LLMSimulationExecutionConfigDTO | LLMAgentConfigDTO | LLMShapeConfigDTO

const props = defineProps<{
  modelValue: AnyLlmConfig
  variant: 'execution' | 'agent' | 'shape'
}>()
const emit = defineEmits<{ 'update:modelValue': [v: AnyLlmConfig] }>()

// All remaining variants (execution / per-agent / per-shape) drive the run
// engine, so the target picker filters by simulation_execution capability.
const targetCapability = 'simulation_execution'

const strategyKey = computed(() => (props.variant === 'execution' ? 'default_strategy' : 'strategy'))

// Only the boolean `use_for_*` flags are rendered as ToggleSwitches; narrowing
// the key type keeps `strategy[f.key]` boolean (not the type/number fields too).
// Required<> strips the optional modifier on archetype_mode/variation_strength so
// those keys don't leak `undefined` into the union via the homomorphic map.
type StrategyBoolKey = {
  [K in keyof Required<LLMStrategyDTO>]: Required<LLMStrategyDTO>[K] extends boolean ? K : never
}[keyof LLMStrategyDTO]

const strategyToggles: { key: StrategyBoolKey; label: string }[] = [
  { key: 'use_for_movement', label: 'Movement' },
  { key: 'use_for_interaction', label: 'Interaction' },
  { key: 'use_for_properties', label: 'Properties' },
  { key: 'use_for_decisions', label: 'Decisions' },
]

type StrategyFlags = Record<StrategyBoolKey, boolean>

// The four per-step purpose flags are CANONICAL for the strategy Type — only
// "hybrid" lets the user pick a free combination. Mirrors the backend
// LLMStrategy.Canonical (data/models/llm_strategy.go); the backend re-normalizes
// on save regardless, so this is for immediate, correct UX.
function canonicalFlagsFor(type: string): StrategyFlags {
  switch (type) {
    case 'full':
      return {
        use_for_movement: true,
        use_for_interaction: true,
        use_for_properties: true,
        use_for_decisions: true,
      }
    case 'decision':
      return {
        use_for_movement: false,
        use_for_interaction: false,
        use_for_properties: false,
        use_for_decisions: true,
      }
    default: // none / empty
      return {
        use_for_movement: false,
        use_for_interaction: false,
        use_for_properties: false,
        use_for_decisions: false,
      }
  }
}

function emitConfig(partial: Record<string, unknown>) {
  emit('update:modelValue', { ...props.modelValue, ...partial } as AnyLlmConfig)
}

function patch(partial: { target?: ExecutionLlmTarget; prompt_context?: string }) {
  emitConfig(partial)
}

// --- strategy (normalized across variants) ---
const strategy = computed<LLMStrategyDTO>(
  () => (props.modelValue as unknown as Record<string, unknown>)[strategyKey.value] as LLMStrategyDTO
)
function patchStrategy(partial: Partial<LLMStrategyDTO>) {
  let next = { ...strategy.value, ...partial }
  // Changing the Type to a non-hybrid value re-imposes its canonical flags so the
  // emitted model + validation (modelUsesLLMForExecution) stay consistent.
  if (partial.type !== undefined && partial.type !== 'hybrid') {
    next = { ...next, ...canonicalFlagsFor(partial.type) }
  }
  emitConfig({ [strategyKey.value]: next })
}

// Only "hybrid" exposes the four flags for editing; every other Type forces them.
const flagsEditable = computed(() => strategy.value.type === 'hybrid')

// Values shown in the toggles: canonical for non-hybrid (correct even if stored
// flags are stale), the stored flags for hybrid.
const displayFlags = computed<StrategyFlags>(() => {
  if (strategy.value.type === 'hybrid') {
    return {
      use_for_movement: !!strategy.value.use_for_movement,
      use_for_interaction: !!strategy.value.use_for_interaction,
      use_for_properties: !!strategy.value.use_for_properties,
      use_for_decisions: !!strategy.value.use_for_decisions,
    }
  }
  return canonicalFlagsFor(strategy.value.type)
})

const strategyHint = computed(() => {
  switch (strategy.value.type) {
    case 'full':
      return 'Full: the LLM handles movement, interaction, properties and decisions (fixed).'
    case 'decision':
      return 'Decision: the LLM handles decisions only (fixed).'
    case 'hybrid':
      return 'Hybrid: choose which purposes the LLM handles.'
    default:
      return 'None: no LLM is used at runtime.'
  }
})

// Archetype result mode — empty (existing models) reads as 'identical'. This is a
// pure read; it triggers no save (the wire value stays undefined until edited).
const archetypeMode = computed<LLMArchetypeMode>(
  () => (strategy.value.archetype_mode as LLMArchetypeMode) || 'identical',
)
const archetypeHint = computed(() =>
  archetypeMode.value === 'distribution'
    ? 'Distribution: one archetype-level LLM result is sampled per-agent (weighted), so members diversify. Archetype agents only.'
    : 'Identical: every archetype member is an exact clone of the single archetype LLM result. Archetype agents only.',
)

// --- simulation config ---
// For the execution variant the runtime knobs are INLINE on the config root, so
// simConfig IS the config itself (always present). For the agent variant they
// live under an OPTIONAL nested `simulation_config` key.
const simConfig = computed<LLMSimulationAgentConfigDTO | undefined>(() => {
  if (props.variant === 'execution') {
    return props.modelValue as unknown as LLMSimulationAgentConfigDTO
  }
  return (props.modelValue as LLMAgentConfigDTO).simulation_config
})
const hasSimConfig = computed(() => props.variant === 'execution' || !!simConfig.value)

function emptyAgentSimConfig(): LLMSimulationAgentConfigDTO {
  return { perception_radius: 0, interaction_radius: 0, memory_size: 0 }
}
function onToggleSimConfig(on: unknown) {
  // Only the agent variant toggles the nested config; execution is always inline.
  if (props.variant !== 'agent') return
  emitConfig({ simulation_config: on ? emptyAgentSimConfig() : undefined })
}
function patchSim(partial: Partial<LLMSimulationExecutionConfigDTO>) {
  if (props.variant === 'execution') {
    // Inline: patch the config root directly.
    emitConfig(partial)
    return
  }
  if (!simConfig.value) return
  emitConfig({ simulation_config: { ...simConfig.value, ...partial } })
}

// --- fallback policy ---
// The FallbackPolicyEditor emits `undefined` for an empty step list so the
// pointer-unset "inherit the model default" is preserved (never { steps: [] }).
function setFallbackPolicy(policy: FallbackPolicyDTO | undefined) {
  if (props.variant === 'execution') {
    emitConfig({ fallback_policy: policy })
    return
  }
  if (!simConfig.value) return
  emitConfig({ simulation_config: { ...simConfig.value, fallback_policy: policy } })
}
</script>
