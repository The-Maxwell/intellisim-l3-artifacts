<template>
  <div class="space-y-2">
    <div class="flex flex-wrap gap-2">
      <!-- Count mode + population inputs are hidden when the host owns the count
           (e.g. a fixed-position shape → always a single instance). -->
      <template v-if="!hideCount">
        <div class="w-40" data-testid="init-count-mode">
          <label class="text-[11px] text-zinc-500 mb-0.5 block">Count mode</label>
          <Select
            :modelValue="init.count_mode ?? null"
            :options="POPULATION_COUNT_MODES as unknown as string[]"
            class="w-full"
            size="small"
            showClear
            placeholder="fixed"
            @update:modelValue="(v) => onCountModeChange((v ?? undefined) as PopulationCountMode | undefined)"
          />
        </div>
        <!-- fixed (or unset ⇒ default fixed): a single population count. -->
        <div v-if="showFixedCount" class="w-36" data-testid="init-population-count">
          <label class="text-[11px] text-zinc-500 mb-0.5 block">Population count</label>
          <InputNumber
            :modelValue="init.population_count ?? null"
            class="w-full"
            size="small"
            :min="0"
            @update:modelValue="(v) => patchInit({ population_count: v ?? 0 })"
          />
        </div>
        <!-- random_range: population drawn from [min, max]. -->
        <div v-if="showRange" class="w-32" data-testid="init-min-population">
          <label class="text-[11px] text-zinc-500 mb-0.5 block">Min population</label>
          <InputNumber
            :modelValue="init.min_population ?? null"
            class="w-full"
            size="small"
            :min="0"
            @update:modelValue="(v) => patchInit({ min_population: v ?? undefined })"
          />
        </div>
        <div v-if="showRange" class="w-32" data-testid="init-max-population">
          <label class="text-[11px] text-zinc-500 mb-0.5 block">Max population</label>
          <InputNumber
            :modelValue="init.max_population ?? null"
            class="w-full"
            size="small"
            :min="0"
            @update:modelValue="(v) => patchInit({ max_population: v ?? undefined })"
          />
        </div>
      </template>
      <!-- llm: the LLM decides the count — no numeric inputs; guided by the
           placement prompt context below. -->
      <!-- initial_state: only for hosts that declare states (agents). Shapes have
           no state machine and pass states=undefined ⇒ this is hidden. -->
      <div v-if="states !== undefined" class="w-40">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Initial state</label>
        <Select
          :modelValue="init.initial_state ?? null"
          :options="states ?? []"
          class="w-full"
          size="small"
          showClear
          @update:modelValue="(v) => patchInit({ initial_state: v ?? undefined })"
        />
      </div>
    </div>

    <p v-if="!hideCount && init.count_mode === 'random_range'" class="text-[11px] text-amber-600">
      random_range draws the population from [min, max] — set both min and max population.
    </p>
    <p v-else-if="!hideCount && init.count_mode === 'llm'" class="text-[11px] text-zinc-400">
      llm count mode lets the LLM decide the initial population from the description and
      model context.
    </p>

    <!-- Placement -->
    <div class="rounded-md border border-zinc-200 bg-zinc-50 p-2 space-y-2">
      <label class="text-[11px] font-semibold uppercase tracking-wide text-zinc-400">
        Placement
      </label>
      <Select
        :modelValue="init.placement?.type ?? null"
        :options="placementTypes as unknown as string[]"
        class="w-40"
        size="small"
        showClear
        :placeholder="placementPlaceholder"
        data-testid="placement-type"
        @update:modelValue="(v) => onPlacementType(v as PlacementType | null)"
      />
      <!-- at_shape: a picker when the host supplies candidate shapes, else free text. -->
      <div v-if="init.placement?.type === 'at_shape'">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Shape UID</label>
        <Select
          v-if="shapeOptions"
          :modelValue="init.placement?.shape_uid ?? null"
          :options="shapeOptions"
          optionLabel="name"
          optionValue="uid"
          class="w-full"
          size="small"
          showClear
          placeholder="pick a shape"
          @update:modelValue="(v) => patchPlacement({ shape_uid: (v as string) || undefined })"
        />
        <InputText
          v-else
          :modelValue="init.placement?.shape_uid ?? ''"
          class="w-full"
          size="small"
          @update:modelValue="(v) => patchPlacement({ shape_uid: v || undefined })"
        />
      </div>
      <!-- Clustered: number of cluster centres + spread (blank ⇒ auto-derived). -->
      <div v-if="init.placement?.type === 'clustered'" class="flex gap-2">
        <div class="w-28">
          <label class="text-[11px] text-zinc-500 mb-0.5 block">Clusters</label>
          <InputNumber
            :modelValue="init.placement?.clusters ?? null"
            class="w-full"
            size="small"
            :min="1"
            :step="1"
            placeholder="auto"
            showClear
            @update:modelValue="(v) => patchPlacement({ clusters: v ?? undefined })"
          />
        </div>
        <div class="w-28">
          <label class="text-[11px] text-zinc-500 mb-0.5 block">Cluster radius</label>
          <InputNumber
            :modelValue="init.placement?.cluster_radius ?? null"
            class="w-full"
            size="small"
            :min="1"
            :step="1"
            placeholder="auto"
            showClear
            @update:modelValue="(v) => patchPlacement({ cluster_radius: v ?? undefined })"
          />
        </div>
      </div>
      <!-- Manual: positioned per-simulation before the run (no coordinates on the model).
           Only offered to hosts that include 'manual' in placementTypes (agents). -->
      <p
        v-if="init.placement?.type === 'manual'"
        class="text-[11px] text-zinc-400 leading-snug"
        data-testid="manual-placement-note"
      >
        Instances are placed by hand in the simulation editor before each run — drag them
        into the world there. The run stays blocked until every instance is placed.
      </p>
      <!-- Prompt context guides LLM placement only. -->
      <div v-if="init.placement?.type === 'llm'">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Prompt context</label>
        <Textarea
          :modelValue="init.placement?.prompt_context ?? ''"
          class="w-full text-xs"
          rows="2"
          autoResize
          @update:modelValue="(v) => setPlacementPromptContext(v || undefined)"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// Shared Initialization + Placement editor for the AgentInitializationDTO carried
// by both agents and shapes. Extracted from AgentForm so the two stay in lockstep
// with the backend AgentInitialization/Placement contract. Host differences are
// props, not variants:
//   - states: agents pass their state list (drives the initial_state select);
//     shapes pass undefined ⇒ initial_state hidden (shapes have no state machine).
//   - shapeOptions: when present, `at_shape` uses a shape picker instead of a
//     free-text uid (shapes supply the other shapes; agents omit ⇒ free text).
//   - placementTypes: which placement strategies to offer (agents include 'manual';
//     shapes omit it — the engine scatters 'manual' shapes randomly, so it is not a
//     meaningful shape option).
//   - hideCount: hide the count-mode/population inputs when the host owns the count
//     (e.g. a fixed-position shape is always a single instance).
// Always emits the full initialization object (immutable patch pattern).
import InputText from 'primevue/inputtext'
import InputNumber from 'primevue/inputnumber'
import Textarea from 'primevue/textarea'
import Select from 'primevue/select'
import { computed } from 'vue'
import type {
  AgentInitializationDTO,
  PlacementDTO,
  PlacementType,
  PopulationCountMode,
} from '@/shared/types'
import { PLACEMENT_TYPES, POPULATION_COUNT_MODES } from '@/shared/validation/modelRules'

const props = withDefaults(
  defineProps<{
    modelValue: AgentInitializationDTO
    /** State names for the initial_state select; undefined ⇒ hide (shapes). */
    states?: string[]
    /** Candidate shapes for at_shape; present ⇒ picker, absent ⇒ free-text uid. */
    shapeOptions?: { uid: string; name: string }[]
    /** Placement strategies to offer (default: the full backend set). */
    placementTypes?: readonly PlacementType[]
    /** Hide the count-mode/population inputs (host owns the count). */
    hideCount?: boolean
    /** Placeholder for the empty placement select. */
    placementPlaceholder?: string
  }>(),
  {
    states: undefined,
    shapeOptions: undefined,
    placementTypes: () => PLACEMENT_TYPES,
    hideCount: false,
    placementPlaceholder: 'type',
  }
)
const emit = defineEmits<{ 'update:modelValue': [v: AgentInitializationDTO] }>()

const init = computed<AgentInitializationDTO>(() => props.modelValue ?? { population_count: 0 })

function patchInit(partial: Partial<AgentInitializationDTO>) {
  emit('update:modelValue', { ...init.value, ...partial })
}

// Which numeric count inputs to show depends on the count mode (empty ⇒ fixed).
// fixed → population_count; random_range → min/max; llm → none (LLM decides).
const showFixedCount = computed(
  () => !init.value.count_mode || init.value.count_mode === 'fixed'
)
const showRange = computed(() => init.value.count_mode === 'random_range')

// Switching count mode clears the now-irrelevant numeric fields so the saved
// model carries no stale bounds/count from a previous mode.
function onCountModeChange(mode: PopulationCountMode | undefined) {
  const partial: Partial<AgentInitializationDTO> = { count_mode: mode }
  if (mode === 'random_range') {
    partial.population_count = 0
  } else if (mode === 'llm') {
    partial.population_count = 0
    partial.min_population = undefined
    partial.max_population = undefined
  } else {
    // fixed (or cleared ⇒ default fixed): keep population_count, drop bounds.
    partial.min_population = undefined
    partial.max_population = undefined
  }
  patchInit(partial)
}
function onPlacementType(type: PlacementType | null) {
  if (!type) {
    patchInit({ placement: undefined })
    return
  }
  patchInit({ placement: { ...(init.value.placement ?? {}), type } as PlacementDTO })
}
function patchPlacement(partial: Partial<PlacementDTO>) {
  if (!init.value.placement) return
  patchInit({ placement: { ...init.value.placement, ...partial } })
}
// The prompt context lives on Placement and guides LLM placement; it is only
// shown/settable for placement type 'llm'.
function setPlacementPromptContext(prompt_context: string | undefined) {
  patchInit({
    placement: { ...(init.value.placement ?? {}), prompt_context } as PlacementDTO,
  })
}
</script>
