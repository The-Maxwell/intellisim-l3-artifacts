<template>
  <div class="space-y-4">
    <!-- Global-properties value form (typed editors per snapshot definition) -->
    <div v-if="globalDefs.length" class="space-y-2">
      <p class="text-[11px] font-semibold uppercase tracking-wide text-zinc-400">
        Global properties
      </p>
      <div
        v-for="def in globalDefs"
        :key="def.uid"
        class="flex items-center gap-3"
      >
        <label class="text-xs text-zinc-600 w-44 shrink-0 truncate" :title="def.uid">
          {{ def.name || def.uid }}
        </label>
        <!-- number -->
        <InputNumber
          v-if="def.type === 'number'"
          :modelValue="asNumber(def)"
          class="w-48"
          size="small"
          :min="def.min_value ?? undefined"
          :max="def.max_value ?? undefined"
          @update:modelValue="(v) => setGlobal(def, v ?? undefined)"
        />
        <!-- boolean -->
        <Checkbox
          v-else-if="def.type === 'boolean'"
          :modelValue="asBool(def)"
          :binary="true"
          @update:modelValue="(v) => setGlobal(def, !!v)"
        />
        <!-- enum -->
        <Select
          v-else-if="def.type === 'enum'"
          :modelValue="globals[def.uid] ?? null"
          :options="(def.allowed_values ?? []) as unknown[]"
          class="w-48"
          size="small"
          showClear
          @update:modelValue="(v) => setGlobal(def, v ?? undefined)"
        />
        <!-- color -->
        <ColorPicker
          v-else-if="def.type === 'color'"
          :modelValue="asColor(def)"
          format="hex"
          @update:modelValue="(v) => setGlobal(def, normalizeHex(v))"
        />
        <!-- string (default) -->
        <InputText
          v-else
          :modelValue="asString(def)"
          class="w-48"
          size="small"
          @update:modelValue="(v) => setGlobal(def, v ?? '')"
        />
      </div>
    </div>

    <!-- Spatial world canvas (Task 16): fully-editable PixiJS world + side panel -->
    <div v-if="spatial" class="flex gap-3 items-stretch">
      <div class="flex-1 min-w-0 h-[480px] rounded-lg border border-zinc-200 overflow-hidden">
        <SimulationWorldCanvas
          :environment="environment"
          :agents="agents"
          :snapshot="snapshot"
          :readonly="readonly"
          @update:environment="(e) => emit('update:environment', e)"
          @update:agents="(a) => emit('update:agents', a)"
          @selection-change="(s) => (selection = s)"
        />
      </div>
      <div class="w-72 shrink-0 h-[480px] overflow-auto rounded-lg border border-zinc-200 bg-white p-3">
        <WorldSidePanel
          :selection="selection"
          :agents="agents"
          :shapes="environment?.shapes ?? []"
          :snapshot="snapshot"
          :readonly="readonly"
          :worldWidth="environment?.width ?? 0"
          :worldHeight="environment?.height ?? 0"
          @update:agent="onAgentEdited"
          @update:shape="onShapeEdited"
          @delete="onDeleteSelection"
        />
      </div>
    </div>

    <p v-if="!globalDefs.length && !spatial" class="text-xs text-zinc-400">
      This model has no environment to configure.
    </p>
  </div>
</template>

<script setup lang="ts">
// Environment section. Renders (1) a typed global-properties value form driven
// by the model snapshot's environment.global_properties definitions, and (2) for
// spatial models the fully-editable PixiJS SimulationWorldCanvas + WorldSidePanel
// (Task 16). The canvas/panel operate on the props passed down (ONE source of
// truth — the editor's store) and emit updated environment / agents slices
// upward; the parent merges them into store.current + markDirty.
import { computed, ref } from 'vue'
import InputNumber from 'primevue/inputnumber'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import Checkbox from 'primevue/checkbox'
import ColorPicker from 'primevue/colorpicker'
import { normalizeHex } from '@/modules/simulation/colorUtil'
import SimulationWorldCanvas from './SimulationWorldCanvas.vue'
import WorldSidePanel from './WorldSidePanel.vue'
import type { SelRef } from './worldCanvasInteractions'
import type {
  SnapshotEnvironment,
  SnapshotProperty,
  SimulationEnvironment,
  SimulationAgent,
  SimulationShape,
  ModelSnapshot,
} from '@/shared/types'

const props = defineProps<{
  snapshotEnvironment?: SnapshotEnvironment | null
  globals: Record<string, unknown>
  spatial: boolean
  environment?: SimulationEnvironment | null
  agents?: SimulationAgent[]
  snapshot?: ModelSnapshot | null
  readonly?: boolean
}>()
const emit = defineEmits<{
  'update:globals': [v: Record<string, unknown>]
  'update:environment': [v: SimulationEnvironment]
  'update:agents': [v: SimulationAgent[]]
}>()

const globalDefs = computed<SnapshotProperty[]>(
  () => props.snapshotEnvironment?.global_properties ?? []
)

const environment = computed<SimulationEnvironment | null>(() => props.environment ?? null)
const agents = computed<SimulationAgent[]>(() => props.agents ?? [])
const snapshot = computed<ModelSnapshot | null>(() => props.snapshot ?? null)
const readonly = computed(() => props.readonly ?? false)

// Selection mirrored from the canvas → fed to the side panel.
const selection = ref<SelRef[]>([])

function asNumber(def: SnapshotProperty): number | null {
  const v = props.globals[def.uid]
  return typeof v === 'number' ? v : null
}
function asBool(def: SnapshotProperty): boolean {
  return props.globals[def.uid] === true
}
function asString(def: SnapshotProperty): string {
  const v = props.globals[def.uid]
  return v == null ? '' : String(v)
}
function asColor(def: SnapshotProperty): string {
  const v = props.globals[def.uid]
  return typeof v === 'string' && v ? v : '#888888'
}

function setGlobal(def: SnapshotProperty, value: unknown) {
  const next = { ...props.globals }
  if (value === undefined) {
    delete next[def.uid]
  } else {
    next[def.uid] = value
  }
  emit('update:globals', next)
}

// --- side-panel edits → resolve into environment / agents updates ---
function onAgentEdited(updated: SimulationAgent) {
  emit(
    'update:agents',
    agents.value.map((a) => (a.id === updated.id ? updated : a))
  )
}
function onShapeEdited(updated: SimulationShape) {
  if (!environment.value) return
  emit('update:environment', {
    ...environment.value,
    shapes: (environment.value.shapes ?? []).map((s) => (s.id === updated.id ? updated : s)),
  })
}
function onDeleteSelection() {
  const shapeIds = new Set(selection.value.filter((r) => r.kind === 'shape').map((r) => r.id))
  const agentIds = new Set(selection.value.filter((r) => r.kind === 'agent').map((r) => r.id))
  if (agentIds.size) {
    emit('update:agents', agents.value.filter((a) => !agentIds.has(a.id)))
  }
  if (shapeIds.size && environment.value) {
    emit('update:environment', {
      ...environment.value,
      shapes: (environment.value.shapes ?? []).filter((s) => !shapeIds.has(s.id)),
    })
  }
  selection.value = []
}
</script>
