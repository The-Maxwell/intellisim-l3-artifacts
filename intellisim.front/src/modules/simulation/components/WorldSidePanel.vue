<template>
  <div class="text-xs space-y-3">
    <!-- nothing selected -->
    <p v-if="!selection.length" class="text-zinc-400">
      Select an item on the canvas to edit it.
    </p>

    <!-- multi-select -->
    <div v-else-if="selection.length > 1" class="space-y-2">
      <p class="text-zinc-600 font-medium">{{ selection.length }} items selected</p>
      <p class="text-[11px] text-zinc-400">
        {{ shapeCount }} shape(s), {{ agentCount }} agent(s). Drag on the canvas to move the group.
      </p>
      <Button
        label="Delete selected"
        icon="pi pi-trash"
        severity="danger"
        size="small"
        outlined
        :disabled="readonly"
        @click="emit('delete')"
      />
    </div>

    <!-- single AGENT -->
    <div v-else-if="agent" class="space-y-3">
      <div class="flex items-center justify-between">
        <span class="font-medium text-zinc-700 truncate">{{ agentArchetype?.name || agent.agent_uid }}</span>
        <Tag value="agent" severity="secondary" />
      </div>

      <div>
        <label class="text-[10px] text-zinc-400 block mb-0.5">Name</label>
        <InputText
          :modelValue="agent.name ?? ''"
          class="w-full"
          size="small"
          :disabled="readonly"
          @update:modelValue="(v) => patchAgent({ name: v ?? '' })"
        />
      </div>

      <div>
        <label class="text-[10px] text-zinc-400 block mb-0.5">State</label>
        <Select
          :modelValue="agent.state ?? null"
          :options="agentStates"
          class="w-full"
          size="small"
          :placeholder="agentStates.length ? 'state' : 'no states'"
          :disabled="readonly"
          @update:modelValue="(v) => patchAgent({ state: v ?? '' })"
        />
      </div>

      <div class="flex items-center gap-2">
        <div class="flex-1">
          <label class="text-[10px] text-zinc-400 block mb-0.5">X</label>
          <InputNumber
            :modelValue="agent.position?.x ?? null"
            class="w-full"
            size="small"
            :disabled="readonly"
            @update:modelValue="(v) => setAgentPos('x', v)"
          />
        </div>
        <div class="flex-1">
          <label class="text-[10px] text-zinc-400 block mb-0.5">Y</label>
          <InputNumber
            :modelValue="agent.position?.y ?? null"
            class="w-full"
            size="small"
            :disabled="readonly"
            @update:modelValue="(v) => setAgentPos('y', v)"
          />
        </div>
      </div>

      <!-- typed property editors -->
      <div v-for="prop in agentProps" :key="prop.uid" class="space-y-0.5">
        <label class="text-[10px] text-zinc-400 block" :title="prop.uid">{{ prop.name || prop.uid }}</label>
        <InputNumber
          v-if="prop.type === 'number'"
          :modelValue="agentPropNumber(prop)"
          class="w-full"
          size="small"
          :min="prop.min_value ?? undefined"
          :max="prop.max_value ?? undefined"
          :disabled="readonly"
          @update:modelValue="(v) => setAgentProp(prop, v ?? undefined)"
        />
        <Checkbox
          v-else-if="prop.type === 'boolean'"
          :modelValue="agentPropBool(prop)"
          :binary="true"
          :disabled="readonly"
          @update:modelValue="(v) => setAgentProp(prop, !!v)"
        />
        <Select
          v-else-if="prop.type === 'enum'"
          :modelValue="agent.properties?.[prop.uid] ?? null"
          :options="(prop.allowed_values ?? []) as unknown[]"
          class="w-full"
          size="small"
          showClear
          :disabled="readonly"
          @update:modelValue="(v) => setAgentProp(prop, v ?? undefined)"
        />
        <ColorPicker
          v-else-if="prop.type === 'color'"
          :modelValue="agentPropColor(prop)"
          format="hex"
          :disabled="readonly"
          @update:modelValue="(v) => setAgentProp(prop, normalizeHex(v))"
        />
        <InputText
          v-else
          :modelValue="agentPropString(prop)"
          class="w-full"
          size="small"
          :disabled="readonly"
          @update:modelValue="(v) => setAgentProp(prop, v ?? '')"
        />
      </div>
    </div>

    <!-- single SHAPE -->
    <div v-else-if="shape" class="space-y-3">
      <div class="flex items-center justify-between">
        <span class="font-medium text-zinc-700 truncate">{{ shape.name || shape.shape_uid }}</span>
        <Tag :value="shape.type" severity="secondary" />
      </div>

      <div>
        <label class="text-[10px] text-zinc-400 block mb-0.5">Name</label>
        <InputText
          :modelValue="shape.name ?? ''"
          class="w-full"
          size="small"
          :disabled="readonly"
          @update:modelValue="(v) => patchShape({ name: v ?? '' })"
        />
      </div>

      <div class="flex items-center gap-2">
        <div class="w-20">
          <label class="text-[10px] text-zinc-400 block mb-0.5">Layer</label>
          <InputNumber
            :modelValue="shape.layer ?? 0"
            class="w-full"
            size="small"
            :disabled="readonly"
            @update:modelValue="(v) => patchShape({ layer: v ?? 0 })"
          />
        </div>
        <label class="flex items-center gap-1.5 cursor-pointer text-zinc-600 pt-4">
          <Checkbox
            :modelValue="shape.visible !== false"
            :binary="true"
            :disabled="readonly"
            @update:modelValue="(v) => patchShape({ visible: !!v })"
          />
          <span>Visible</span>
        </label>
      </div>

      <!-- position -->
      <div class="flex items-center gap-2">
        <div class="flex-1">
          <label class="text-[10px] text-zinc-400 block mb-0.5">X</label>
          <InputNumber
            :modelValue="shape.position?.x ?? 0"
            class="w-full"
            size="small"
            :disabled="readonly"
            @update:modelValue="(v) => setShapePos('x', v)"
          />
        </div>
        <div class="flex-1">
          <label class="text-[10px] text-zinc-400 block mb-0.5">Y</label>
          <InputNumber
            :modelValue="shape.position?.y ?? 0"
            class="w-full"
            size="small"
            :disabled="readonly"
            @update:modelValue="(v) => setShapePos('y', v)"
          />
        </div>
      </div>

      <!-- geometry per type -->
      <div v-if="shape.type === 'circle'">
        <label class="text-[10px] text-zinc-400 block mb-0.5">Radius</label>
        <InputNumber
          :modelValue="shape.radius ?? 0"
          class="w-full"
          size="small"
          :min="1"
          :disabled="readonly"
          @update:modelValue="(v) => setShapeRadius(v)"
        />
      </div>
      <div v-else-if="shape.type === 'rectangle'" class="flex items-center gap-2">
        <div class="flex-1">
          <label class="text-[10px] text-zinc-400 block mb-0.5">Width</label>
          <InputNumber
            :modelValue="shape.width ?? 0"
            class="w-full"
            size="small"
            :min="1"
            :disabled="readonly"
            @update:modelValue="(v) => setShapeWidth(v)"
          />
        </div>
        <div class="flex-1">
          <label class="text-[10px] text-zinc-400 block mb-0.5">Height</label>
          <InputNumber
            :modelValue="shape.height ?? 0"
            class="w-full"
            size="small"
            :min="1"
            :disabled="readonly"
            @update:modelValue="(v) => setShapeHeight(v)"
          />
        </div>
      </div>
      <div v-else-if="shape.type === 'line' || shape.type === 'polygon'">
        <p class="text-[10px] text-zinc-400">
          {{ shape.points?.length ?? 0 }} point(s) — drag the vertex handles on the canvas to edit.
        </p>
      </div>

      <!-- fill / stroke -->
      <div class="flex items-center gap-4">
        <label class="flex items-center gap-1.5 cursor-pointer text-zinc-600">
          <Checkbox
            :modelValue="shape.fill !== false"
            :binary="true"
            :disabled="readonly"
            @update:modelValue="(v) => patchShape({ fill: !!v })"
          />
          <span>Fill</span>
        </label>
        <label class="flex items-center gap-1.5 cursor-pointer text-zinc-600">
          <Checkbox
            :modelValue="shape.stroke !== false"
            :binary="true"
            :disabled="readonly"
            @update:modelValue="(v) => patchShape({ stroke: !!v })"
          />
          <span>Stroke</span>
        </label>
        <div class="w-20">
          <label class="text-[10px] text-zinc-400 block mb-0.5">Stroke w.</label>
          <InputNumber
            :modelValue="shape.stroke_width ?? null"
            class="w-full"
            size="small"
            :min="0"
            :disabled="readonly"
            @update:modelValue="(v) => patchShape({ stroke_width: v ?? undefined })"
          />
        </div>
      </div>

      <!-- typed property editors (from snapshot shape def) -->
      <div v-for="prop in shapeProps" :key="prop.uid" class="space-y-0.5">
        <label class="text-[10px] text-zinc-400 block" :title="prop.uid">{{ prop.name || prop.uid }}</label>
        <InputNumber
          v-if="prop.type === 'number'"
          :modelValue="shapePropNumber(prop)"
          class="w-full"
          size="small"
          :min="prop.min_value ?? undefined"
          :max="prop.max_value ?? undefined"
          :disabled="readonly"
          @update:modelValue="(v) => setShapeProp(prop, v ?? undefined)"
        />
        <Checkbox
          v-else-if="prop.type === 'boolean'"
          :modelValue="shapePropBool(prop)"
          :binary="true"
          :disabled="readonly"
          @update:modelValue="(v) => setShapeProp(prop, !!v)"
        />
        <Select
          v-else-if="prop.type === 'enum'"
          :modelValue="shape.properties?.[prop.uid] ?? null"
          :options="(prop.allowed_values ?? []) as unknown[]"
          class="w-full"
          size="small"
          showClear
          :disabled="readonly"
          @update:modelValue="(v) => setShapeProp(prop, v ?? undefined)"
        />
        <ColorPicker
          v-else-if="prop.type === 'color'"
          :modelValue="shapePropColor(prop)"
          format="hex"
          :disabled="readonly"
          @update:modelValue="(v) => setShapeProp(prop, normalizeHex(v))"
        />
        <InputText
          v-else
          :modelValue="shapePropString(prop)"
          class="w-full"
          size="small"
          :disabled="readonly"
          @update:modelValue="(v) => setShapeProp(prop, v ?? '')"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// Side panel bound to the SimulationWorldCanvas selection. Edits the SINGLE
// selected agent or shape (or shows a count + delete for multi-select). Every
// change emits a fresh agent / shape upward; the canvas/editor apply it to the
// same source arrays (ONE source of truth — this panel keeps no local copy).
// Reuses the AgentInstanceTable typed-editor approach + colorUtil.normalizeHex.
import { computed } from 'vue'
import InputText from 'primevue/inputtext'
import InputNumber from 'primevue/inputnumber'
import Select from 'primevue/select'
import Checkbox from 'primevue/checkbox'
import ColorPicker from 'primevue/colorpicker'
import Button from 'primevue/button'
import Tag from 'primevue/tag'
import { normalizeHex } from '@/modules/simulation/colorUtil'
import type {
  SimulationAgent,
  SimulationShape,
  ModelSnapshot,
  SnapshotProperty,
} from '@/shared/types'
import type { SelRef } from './worldCanvasInteractions'

const props = defineProps<{
  selection: SelRef[]
  agents: SimulationAgent[]
  shapes: SimulationShape[]
  snapshot: ModelSnapshot | null
  readonly?: boolean
  /** World bounds (env.width / env.height) — used to clamp numeric edits. */
  worldWidth?: number
  worldHeight?: number
}>()
const emit = defineEmits<{
  'update:agent': [v: SimulationAgent]
  'update:shape': [v: SimulationShape]
  delete: []
}>()

const shapeCount = computed(() => props.selection.filter((r) => r.kind === 'shape').length)
const agentCount = computed(() => props.selection.filter((r) => r.kind === 'agent').length)

// --- single selected agent / shape ---
const agent = computed<SimulationAgent | null>(() => {
  if (props.selection.length !== 1 || props.selection[0]!.kind !== 'agent') return null
  return props.agents.find((a) => a.id === props.selection[0]!.id) ?? null
})
const shape = computed<SimulationShape | null>(() => {
  if (props.selection.length !== 1 || props.selection[0]!.kind !== 'shape') return null
  return props.shapes.find((s) => s.id === props.selection[0]!.id) ?? null
})

const agentArchetype = computed(() =>
  agent.value ? props.snapshot?.agents?.find((a) => a.uid === agent.value!.agent_uid) ?? null : null
)
const agentStates = computed<string[]>(() => agentArchetype.value?.states ?? [])
const agentProps = computed<SnapshotProperty[]>(() => agentArchetype.value?.properties ?? [])

const shapeDef = computed(() =>
  shape.value
    ? props.snapshot?.environment?.shapes?.find((d) => d.uid === shape.value!.shape_uid) ?? null
    : null
)
const shapeProps = computed<SnapshotProperty[]>(() => shapeDef.value?.properties ?? [])

// --- bounds clamping (mirrors the canvas drag/resize clamp; keeps panel edits
//     in-bounds even though the panel never sees the canvas geometry helpers) ---
/** Clamp a position coordinate to [0, bound] (rounded, non-negative). */
function clampPos(value: number, bound?: number): number {
  let out = Math.round(value)
  if (out < 0) out = 0
  if (typeof bound === 'number' && bound > 0 && out > bound) out = bound
  return out
}
/** Clamp a size (radius/width/height) to >= 1 (rounded), capped to bound. */
function clampSize(value: number, bound?: number): number {
  let out = Math.max(1, Math.round(value))
  if (typeof bound === 'number' && bound > 0 && out > bound) out = bound
  return out
}

// --- agent mutations ---
function patchAgent(partial: Partial<SimulationAgent>) {
  if (!agent.value || props.readonly) return
  emit('update:agent', { ...agent.value, ...partial })
}
function setAgentPos(axis: 'x' | 'y', value: number | null) {
  if (!agent.value || props.readonly) return
  const position = { x: agent.value.position?.x ?? 0, y: agent.value.position?.y ?? 0 }
  position[axis] = clampPos(value ?? 0, axis === 'x' ? props.worldWidth : props.worldHeight)
  emit('update:agent', { ...agent.value, position })
}
function setAgentProp(prop: SnapshotProperty, value: unknown) {
  if (!agent.value || props.readonly) return
  const properties = { ...(agent.value.properties ?? {}) }
  if (value === undefined) delete properties[prop.uid]
  else properties[prop.uid] = value
  emit('update:agent', { ...agent.value, properties })
}
function agentPropNumber(prop: SnapshotProperty): number | null {
  const v = agent.value?.properties?.[prop.uid]
  return typeof v === 'number' ? v : null
}
function agentPropBool(prop: SnapshotProperty): boolean {
  return agent.value?.properties?.[prop.uid] === true
}
function agentPropString(prop: SnapshotProperty): string {
  const v = agent.value?.properties?.[prop.uid]
  return v == null ? '' : String(v)
}
function agentPropColor(prop: SnapshotProperty): string {
  const v = agent.value?.properties?.[prop.uid]
  return typeof v === 'string' && v ? v : '#888888'
}

// --- shape mutations ---
function patchShape(partial: Partial<SimulationShape>) {
  if (!shape.value || props.readonly) return
  emit('update:shape', { ...shape.value, ...partial })
}
function setShapePos(axis: 'x' | 'y', value: number | null) {
  if (!shape.value || props.readonly) return
  const position = { x: shape.value.position?.x ?? 0, y: shape.value.position?.y ?? 0 }
  position[axis] = clampPos(value ?? 0, axis === 'x' ? props.worldWidth : props.worldHeight)
  emit('update:shape', { ...shape.value, position })
}
/** Set radius / width / height, clamped to >= 1 and capped to the world dim. */
function setShapeRadius(value: number | null) {
  if (!shape.value || props.readonly) return
  // a circle's radius is bounded by the smaller world dimension
  const bound =
    props.worldWidth != null && props.worldHeight != null
      ? Math.min(props.worldWidth, props.worldHeight)
      : (props.worldWidth ?? props.worldHeight)
  patchShape({ radius: clampSize(value ?? 0, bound) })
}
function setShapeWidth(value: number | null) {
  if (!shape.value || props.readonly) return
  patchShape({ width: clampSize(value ?? 0, props.worldWidth) })
}
function setShapeHeight(value: number | null) {
  if (!shape.value || props.readonly) return
  patchShape({ height: clampSize(value ?? 0, props.worldHeight) })
}
function setShapeProp(prop: SnapshotProperty, value: unknown) {
  if (!shape.value || props.readonly) return
  const properties = { ...(shape.value.properties ?? {}) }
  if (value === undefined) delete properties[prop.uid]
  else properties[prop.uid] = value
  emit('update:shape', { ...shape.value, properties })
}
function shapePropNumber(prop: SnapshotProperty): number | null {
  const v = shape.value?.properties?.[prop.uid]
  return typeof v === 'number' ? v : null
}
function shapePropBool(prop: SnapshotProperty): boolean {
  return shape.value?.properties?.[prop.uid] === true
}
function shapePropString(prop: SnapshotProperty): string {
  const v = shape.value?.properties?.[prop.uid]
  return v == null ? '' : String(v)
}
function shapePropColor(prop: SnapshotProperty): string {
  const v = shape.value?.properties?.[prop.uid]
  return typeof v === 'string' && v ? v : '#888888'
}
</script>
