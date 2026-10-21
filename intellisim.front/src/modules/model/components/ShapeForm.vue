<template>
  <div class="rounded-lg border border-zinc-200 bg-white p-3 space-y-3">
    <!-- Header: name + color + lock + remove -->
    <div class="flex items-center gap-2">
      <div class="flex-1 min-w-0">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Name</label>
        <InputText
          :modelValue="modelValue.name"
          class="w-full"
          size="small"
          :maxlength="ENTITY_NAME_MAX"
          placeholder="Shape name"
          @update:modelValue="(v) => patch({ name: v ?? '' })"
        />
      </div>
      <div class="w-16">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Color</label>
        <input
          type="color"
          class="w-full h-8 border border-zinc-200 rounded cursor-pointer"
          :value="colorHex"
          aria-label="Shape color"
          @input="onColor(($event.target as HTMLInputElement).value)"
        />
      </div>
      <div class="flex items-center gap-1 pt-4">
        <span v-tooltip.top="lockTooltip" class="flex items-center gap-1">
          <i :class="['pi text-xs', modelValue.locked ? 'pi-lock text-amber-500' : 'pi-lock-open text-zinc-400']" />
          <ToggleSwitch
            :modelValue="modelValue.locked"
            @update:modelValue="(v) => patch({ locked: !!v })"
          />
        </span>
        <Button
          icon="pi pi-trash"
          severity="danger"
          text
          rounded
          size="small"
          aria-label="Remove shape"
          @click="emit('remove')"
        />
      </div>
    </div>

    <!-- uid / type / layer / visible -->
    <div class="flex flex-wrap gap-2 items-end">
      <div class="flex-1 min-w-32">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">UID</label>
        <InputText
          :modelValue="modelValue.uid"
          class="w-full"
          size="small"
          :maxlength="UID_MAX"
          placeholder="wall"
          @update:modelValue="(v) => patch({ uid: v ?? '' })"
        />
      </div>
      <div class="w-32">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Type</label>
        <Select
          :modelValue="modelValue.type"
          :options="SHAPE_TYPES as unknown as string[]"
          class="w-full"
          size="small"
          data-testid="shape-type"
          @update:modelValue="(v) => onTypeChange(v as ShapeType)"
        />
      </div>
      <div class="w-24">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Layer</label>
        <InputNumber
          :modelValue="modelValue.layer ?? 0"
          class="w-full"
          size="small"
          :min="0"
          @update:modelValue="(v) => patch({ layer: v ?? 0 })"
        />
      </div>
      <label class="flex items-center gap-1.5 text-xs text-zinc-600 cursor-pointer pb-1.5">
        <ToggleSwitch
          :modelValue="modelValue.visible !== false"
          @update:modelValue="(v) => patch({ visible: !!v })"
        />
        <span>Visible</span>
      </label>
    </div>

    <!-- description -->
    <div>
      <label class="text-[11px] text-zinc-500 mb-0.5 block">Description</label>
      <Textarea
        :modelValue="modelValue.description"
        class="w-full text-xs"
        rows="2"
        autoResize
        @update:modelValue="(v) => patch({ description: v ?? '' })"
      />
    </div>

    <!-- Geometry -->
    <Fieldset legend="Geometry" :toggleable="true" :collapsed="false" class="text-sm">
      <ShapeGeometryEditor
        :modelValue="modelValue.geometry"
        :type="modelValue.type"
        :placementMode="placementMode"
        :env="env"
        @update:modelValue="(g) => patch({ geometry: g })"
      />
    </Fieldset>

    <!-- Initialization + placement -->
    <Fieldset legend="Initialization & placement" :toggleable="true" :collapsed="false" class="text-sm">
      <p class="text-[11px] text-zinc-400 leading-snug mb-2">
        Leave placement empty for a single fixed shape you position on the canvas. Pick a strategy
        to auto-place multiple instances at simulation time.
      </p>
      <InitializationForm
        :modelValue="init"
        :shapeOptions="shapeOptions"
        :placementTypes="SHAPE_PLACEMENT_TYPES"
        :hideCount="placementMode === 'fixed'"
        placementPlaceholder="fixed (single)"
        @update:modelValue="onInitUpdate"
      />
    </Fieldset>

    <!-- Missing-colour hint: appears if the user removed the conventional `color`
         property. Rendering then falls back to a palette colour; a one-click add
         restores an explicit, editable colour. -->
    <Message
      v-if="!hasColor"
      severity="warn"
      :closable="false"
      class="text-xs"
    >
      <div class="flex items-center justify-between gap-2">
        <span>No colour set — this shape renders with a default palette colour.</span>
        <Button label="Add colour" icon="pi pi-plus" size="small" text @click="onColor(geo.DEFAULT_SHAPE_COLOR)" />
      </div>
    </Message>

    <!-- Properties -->
    <Fieldset legend="Properties" :toggleable="true" :collapsed="true" class="text-sm">
      <PropertyEditor
        :modelValue="modelValue.properties ?? []"
        @update:modelValue="(p) => patch({ properties: p })"
      />
    </Fieldset>

    <!-- Display (visual) -->
    <Fieldset legend="Display" :toggleable="true" :collapsed="!modelValue.display" class="text-sm">
      <DisplayEditor
        :modelValue="modelValue.display"
        :allowDraw="true"
        @update:modelValue="(d) => patch({ display: d })"
        @draw-figure="emit('draw-figure', modelValue.uid)"
      />
    </Fieldset>

    <!-- Per-shape LLM config -->
    <Fieldset legend="LLM config" :toggleable="true" :collapsed="!hasLlmConfig" class="text-sm">
      <div class="space-y-2">
        <label class="flex items-center gap-2 text-xs text-zinc-600 cursor-pointer">
          <ToggleSwitch :modelValue="hasLlmConfig" @update:modelValue="onToggleLlm" />
          <span>{{ hasLlmConfig ? 'LLM enabled for this shape' : 'No LLM (inherit model default)' }}</span>
        </label>
        <LlmConfigForm
          v-if="hasLlmConfig && modelValue.llm_config"
          :modelValue="modelValue.llm_config"
          variant="shape"
          @update:modelValue="(c) => patch({ llm_config: c as LLMShapeConfigDTO })"
        />
      </div>
    </Fieldset>

    <!-- inline validation issues (client mirror of validation.go) -->
    <ul v-if="issues.length" class="text-[11px] text-red-600 space-y-0.5" data-testid="shape-issues">
      <li v-for="issue in issues" :key="issue.path + issue.code" class="flex items-start gap-1">
        <i class="pi pi-exclamation-triangle text-[10px] mt-0.5" />
        <span>{{ issue.message }}</span>
      </li>
    </ul>
  </div>
</template>

<script setup lang="ts">
// Edits one Shape (used in a list managed by the parent EnvironmentForm). The
// single full editing surface for every shape config the backend supports:
// identity/type, geometry (numeric + ScalarSpec ranges via ShapeGeometryEditor),
// custom properties (PropertyEditor), initialization + placement strategy (shared
// InitializationForm), display (DisplayEditor; figure-draw bubbles up to the
// canvas), and per-shape LLM config. ShapeForm owns the geometry↔placement
// reconciliation: a fixed shape has a position + single instance; switching to a
// strategy clears the position and enables the count controls, and back again
// re-seeds a centred position and forces population 1. Always emits the full shape.
import InputText from 'primevue/inputtext'
import Textarea from 'primevue/textarea'
import Select from 'primevue/select'
import InputNumber from 'primevue/inputnumber'
import ToggleSwitch from 'primevue/toggleswitch'
import Fieldset from 'primevue/fieldset'
import Button from 'primevue/button'
import Message from 'primevue/message'
import Tooltip from 'primevue/tooltip'
import PropertyEditor from './PropertyEditor.vue'
import LlmConfigForm from './LlmConfigForm.vue'
import DisplayEditor from './DisplayEditor.vue'
import InitializationForm from './InitializationForm.vue'
import ShapeGeometryEditor from './ShapeGeometryEditor.vue'
import * as geo from './environmentCanvasGeometry'
import { computed } from 'vue'
import type {
  ShapeDTO,
  ShapeType,
  AgentInitializationDTO,
  LLMShapeConfigDTO,
  ExecutionLlmTarget,
} from '@/shared/types'
import { SHAPE_TYPES, ENTITY_NAME_MAX, UID_MAX, validateShape } from '@/shared/validation/modelRules'

// Local directive registration (PrimeVue Tooltip).
const vTooltip = Tooltip

const props = defineProps<{
  modelValue: ShapeDTO
  /** Environment bounds — for numeric position clamps and centred defaults. */
  env: { width: number; height: number }
  /** Other shapes (self excluded) offered to the at_shape placement picker. */
  shapeOptions?: { uid: string; name: string }[]
}>()
const emit = defineEmits<{
  'update:modelValue': [v: ShapeDTO]
  remove: []
  'draw-figure': [uid: string]
}>()

const lockTooltip = 'Locked: the LLM will not modify this shape'

// Shapes are either fixed on the canvas or auto-placed — 'manual' (agents-only,
// positioned per-run) is excluded: the engine scatters 'manual' shapes randomly.
const SHAPE_PLACEMENT_TYPES = ['random', 'uniform', 'clustered', 'at_shape', 'llm'] as const

function patch(partial: Partial<ShapeDTO>) {
  emit('update:modelValue', { ...props.modelValue, ...partial })
}

// --- color (stored as the conventional `color` property) ---
const colorHex = computed(() => geo.shapeColor(props.modelValue) ?? geo.DEFAULT_SHAPE_COLOR)
const hasColor = computed(() => geo.hasColorProperty(props.modelValue))
function onColor(hex: string) {
  emit('update:modelValue', geo.setShapeColor(props.modelValue, hex))
}

// --- placement mode (drives the geometry position field + count controls) ---
const placementMode = computed<'fixed' | 'strategy' | 'llm'>(() => {
  const t = props.modelValue.initialization?.placement?.type
  if (!t) return 'fixed'
  if (t === 'llm') return 'llm'
  return 'strategy'
})

const init = computed<AgentInitializationDTO>(
  () => props.modelValue.initialization ?? { population_count: 1 }
)

// Reconcile geometry ⇄ placement on every initialization edit. A transition
// between fixed (no placement) and strategy/llm (placement set) must keep the
// data valid for the backend (position ⇒ population 1; strategy ⇒ no position).
function onInitUpdate(nextInit: AgentInitializationDTO) {
  const prevType = props.modelValue.initialization?.placement?.type
  const nextType = nextInit.placement?.type
  let geometry = props.modelValue.geometry
  const nextInitOut: AgentInitializationDTO = { ...nextInit }

  if (nextType && !prevType) {
    // fixed → strategy/llm: drop the fixed position (placement computes it).
    if (geometry?.position) geometry = { ...geometry, position: undefined }
  } else if (!nextType && prevType) {
    // strategy/llm → fixed: a single instance at a centred position.
    nextInitOut.population_count = 1
    nextInitOut.count_mode = 'fixed'
    nextInitOut.min_population = undefined
    nextInitOut.max_population = undefined
    const base = geometry ?? geo.defaultGeometryForType(props.modelValue.type, props.env)
    if (!base.position) {
      geometry = { ...base, position: geo.centerPosition(props.modelValue.type, base, props.env) }
    } else {
      geometry = base
    }
  }
  patch({ initialization: nextInitOut, geometry })
}

// Changing type rebuilds default size geometry (preserving fixed placement).
function onTypeChange(newType: ShapeType) {
  if (newType === props.modelValue.type) return
  const geometry = geo.defaultGeometryForType(newType, props.env)
  if (placementMode.value === 'fixed') {
    geometry.position = geo.centerPosition(newType, geometry, props.env)
  }
  patch({ type: newType, geometry })
}

// --- per-shape LLM config ---
const hasLlmConfig = computed(() => !!props.modelValue.llm_config)
function emptyShapeLlmConfig(): LLMShapeConfigDTO {
  return {
    target: {} as ExecutionLlmTarget,
    prompt_context: undefined,
    strategy: {
      type: 'none',
      batch_size: 0,
      use_for_movement: false,
      use_for_interaction: false,
      use_for_properties: false,
      use_for_decisions: false,
      decision_frequency: 0,
    },
  }
}
function onToggleLlm(on: unknown) {
  patch({ llm_config: on ? emptyShapeLlmConfig() : undefined })
}

// --- inline validation (client mirror; backend remains authoritative) ---
const issues = computed(() => validateShape(props.modelValue, 'shape'))
</script>
