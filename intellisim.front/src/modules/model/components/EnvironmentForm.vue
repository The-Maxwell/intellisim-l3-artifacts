<template>
  <div class="space-y-3">
    <!-- width / height / torus / collision — spatial models only -->
    <div v-if="isSpatial" class="flex flex-wrap gap-2 items-end">
      <div class="w-32">
        <label class="text-xs text-zinc-500 mb-1 block">Width</label>
        <InputNumber
          :modelValue="env.width ?? null"
          class="w-full"
          :min="0"
          @update:modelValue="(v) => patch({ width: v ?? 0 })"
        />
      </div>
      <div class="w-32">
        <label class="text-xs text-zinc-500 mb-1 block">Height</label>
        <InputNumber
          :modelValue="env.height ?? null"
          class="w-full"
          :min="0"
          @update:modelValue="(v) => patch({ height: v ?? 0 })"
        />
      </div>
      <label class="flex items-center gap-2 text-sm text-zinc-700 cursor-pointer pb-1">
        <ToggleSwitch :modelValue="env.torus" @update:modelValue="(v) => patch({ torus: !!v })" />
        <span>Torus (wrap edges)</span>
      </label>
      <div class="w-40">
        <label class="text-xs text-zinc-500 mb-1 block">Collision mode</label>
        <Select
          :modelValue="collisionMode"
          :options="COLLISION_MODE_OPTIONS"
          optionLabel="label"
          optionValue="value"
          class="w-full"
          size="small"
          @update:modelValue="(v) => onCollisionMode(v)"
        />
      </div>
    </div>

    <!-- environment-level prompt context -->
    <div>
      <label class="text-xs text-zinc-500 mb-1 block">LLM prompt context</label>
      <Textarea
        :modelValue="env.llm_prompt_context ?? ''"
        class="w-full text-xs"
        rows="2"
        autoResize
        @update:modelValue="(v) => patch({ llm_prompt_context: v || undefined })"
      />
    </div>

    <!-- global properties -->
    <Fieldset legend="Global properties" :toggleable="true" :collapsed="isSpatial" class="text-sm">
      <PropertyEditor
        :modelValue="env.global_properties ?? []"
        @update:modelValue="(p) => patch({ global_properties: p })"
      />
    </Fieldset>

    <!-- Shapes: canvas preview + list + full ShapeForm — spatial models only -->
    <Fieldset v-if="isSpatial" legend="Shapes" :toggleable="true" :collapsed="false" class="text-sm">
      <!-- Canvas preview. Selection is shared with the list/form below via
           v-model:selectedUid; strategy-placed shapes show as ghost markers. -->
      <div class="h-80 rounded-md border border-zinc-200 overflow-hidden">
        <EnvironmentCanvas
          ref="canvasRef"
          :modelValue="env"
          :cellSize="cellSize"
          :locked="false"
          :model="model"
          v-model:selectedUid="selectedShapeUid"
          @update:modelValue="(v) => emit('update:modelValue', v)"
        />
      </div>

      <!-- Image-upload → environment (spatial only). The env update arrives via
           SSE model.updated → store → this canvas re-renders. -->
      <ImageToEnvironment v-if="modelId" :modelId="modelId" />

      <!-- Shape manager: list + full editing form -->
      <div class="mt-3 space-y-2">
        <div class="flex items-center justify-between">
          <span class="text-[11px] font-semibold uppercase tracking-wide text-zinc-400">
            Manage shapes
          </span>
          <Button
            label="Add shape"
            icon="pi pi-plus"
            size="small"
            severity="secondary"
            outlined
            data-testid="add-shape"
            @click="toggleAddPicker"
          />
        </div>

        <!-- Add-shape picker: choose type + placement, no drawing needed. -->
        <div
          v-if="showAddPicker"
          class="flex flex-wrap items-end gap-2 rounded-md border border-violet-200 bg-violet-50 p-2"
          data-testid="add-shape-picker"
        >
          <div class="w-32">
            <label class="text-[11px] text-zinc-500 mb-0.5 block">Type</label>
            <Select
              v-model="pickerType"
              :options="SHAPE_TYPES as unknown as string[]"
              class="w-full"
              size="small"
            />
          </div>
          <div class="w-40">
            <label class="text-[11px] text-zinc-500 mb-0.5 block">Placement</label>
            <Select
              v-model="pickerPlacement"
              :options="ADD_PLACEMENT_OPTIONS"
              optionLabel="label"
              optionValue="value"
              class="w-full"
              size="small"
            />
          </div>
          <Button label="Add" icon="pi pi-check" size="small" data-testid="add-shape-confirm" @click="addShape" />
          <Button label="Cancel" size="small" text @click="showAddPicker = false" />
        </div>

        <MasterDetailList
          :items="shapes"
          v-model:selectedIndex="selectedIndex"
          emptyText="No shapes yet — draw one on the canvas or add one above."
        >
          <template #item="{ item, index }">
            <span
              class="w-3 h-3 rounded-sm flex-none border border-black/10"
              :style="{ backgroundColor: shapeColor(item) }"
            />
            <span class="truncate flex-1">{{ item.name || item.uid || `Shape ${index + 1}` }}</span>
            <i
              v-if="isStrategyPlaced(item)"
              class="pi pi-map-marker text-[10px] text-zinc-400 flex-none"
              title="placed by strategy"
            />
            <i v-if="item.locked" class="pi pi-lock text-[10px] text-amber-500 flex-none" title="locked" />
            <i
              v-if="hasIssues(item)"
              class="pi pi-exclamation-triangle text-[10px] text-red-500 flex-none"
              title="has validation issues"
            />
          </template>
          <template #detail="{ item, index }">
            <ShapeForm
              :modelValue="item"
              :env="envDims"
              :shapeOptions="otherShapeOptions(index)"
              @update:modelValue="(s) => updateShape(index, s)"
              @remove="removeShape(index)"
              @draw-figure="onDrawFigure"
            />
          </template>
        </MasterDetailList>
      </div>
    </Fieldset>
  </div>
</template>

<script setup lang="ts">
// Edits the EnvironmentDTO for BOTH spatial and non-spatial models. Global
// properties + LLM prompt context are always editable; geometry (width/height/
// torus/collision) and the Shapes manager render only when isSpatial.
//
// The Shapes manager is the advanced shape editor: a canvas PREVIEW (drawing +
// selection + drag/resize, editing removed) above a shape LIST + a full ShapeForm
// detail. Every shape appears in the list — including strategy-placed shapes that
// have no fixed canvas position. Selection is bidirectional: the canvas and the
// list share selectedShapeUid (mapped to MasterDetailList's index). "+ Add shape"
// creates a shape (fixed or strategy-placed) form-first, no drawing required.
// Emits the full updated environment.
import InputNumber from 'primevue/inputnumber'
import Textarea from 'primevue/textarea'
import ToggleSwitch from 'primevue/toggleswitch'
import Select from 'primevue/select'
import Fieldset from 'primevue/fieldset'
import Button from 'primevue/button'
import PropertyEditor from './PropertyEditor.vue'
import EnvironmentCanvas from './EnvironmentCanvas.vue'
import ImageToEnvironment from './ImageToEnvironment.vue'
import ShapeForm from './ShapeForm.vue'
import MasterDetailList from '@/shared/components/MasterDetailList.vue'
import * as geo from './environmentCanvasGeometry'
import { computed, nextTick, ref } from 'vue'
import type { CollisionMode, EnvironmentDTO, Model, ShapeDTO, ShapeType, PlacementType } from '@/shared/types'
import { SHAPE_TYPES, validateShape } from '@/shared/validation/modelRules'

const { SHAPE_PALETTE, DEFAULT_SHAPE_COLOR } = geo

// cellSize comes from the model's spatial_cell_size (grid spacing on the canvas).
// model is passed through to the canvas + uid generation so shape uids stay
// model-unique. modelId enables the image→environment generation sub-panel.
const props = withDefaults(
  defineProps<{
    modelValue: EnvironmentDTO
    cellSize?: number
    model?: Model | null
    modelId?: string
    isSpatial?: boolean
  }>(),
  { cellSize: 0, model: null, modelId: '', isSpatial: true }
)
const emit = defineEmits<{ 'update:modelValue': [v: EnvironmentDTO] }>()

const env = computed(() => props.modelValue)
const shapes = computed<ShapeDTO[]>(() => props.modelValue.shapes ?? [])
const envDims = computed(() => ({ width: env.value.width, height: env.value.height }))

// Collision mode: store '' (none, the default) or 'block'. We normalise an
// absent value to '' for the Select binding.
const COLLISION_MODE_OPTIONS: { value: CollisionMode; label: string }[] = [
  { value: '', label: 'None' },
  { value: 'block', label: 'Block (no overlap)' },
]
const collisionMode = computed<CollisionMode>(() => (env.value.collision_mode === 'block' ? 'block' : ''))
function onCollisionMode(v: CollisionMode | null) {
  patch({ collision_mode: v === 'block' ? 'block' : undefined })
}

function patch(partial: Partial<EnvironmentDTO>) {
  emit('update:modelValue', { ...props.modelValue, ...partial })
}

// --- selection (shared canvas ⇄ list) ---
const canvasRef = ref<InstanceType<typeof EnvironmentCanvas> | null>(null)
const selectedShapeUid = ref<string | null>(null)
const selectedIndex = computed<number>({
  get: () => shapes.value.findIndex((s) => s.uid === selectedShapeUid.value),
  set: (i) => { selectedShapeUid.value = shapes.value[i]?.uid ?? null },
})

// --- shape list helpers ---
function shapeColor(shape: ShapeDTO): string {
  return geo.shapeColor(shape) ?? DEFAULT_SHAPE_COLOR
}
const isStrategyPlaced = geo.isStrategyPlaced
function hasIssues(shape: ShapeDTO): boolean {
  return validateShape(shape, 'shape').length > 0
}
function otherShapeOptions(index: number): { uid: string; name: string }[] {
  return shapes.value
    .filter((_, i) => i !== index)
    .map((s) => ({ uid: s.uid, name: s.name || s.uid }))
}

// --- add / update / remove (immutable; emit the whole env) ---
function updateShape(index: number, updated: ShapeDTO) {
  // Selection is keyed by uid; if the edit renamed the selected shape's uid,
  // follow it so the form/canvas don't lose the selection mid-edit.
  const prev = shapes.value[index]
  if (prev && prev.uid === selectedShapeUid.value && updated.uid !== prev.uid) {
    selectedShapeUid.value = updated.uid
  }
  patch({ shapes: shapes.value.map((s, i) => (i === index ? updated : s)) })
}
function removeShape(index: number) {
  const removed = shapes.value[index]
  patch({ shapes: shapes.value.filter((_, i) => i !== index) })
  if (removed && removed.uid === selectedShapeUid.value) selectedShapeUid.value = null
}

// --- add-shape picker (type + placement, no drawing) ---
const showAddPicker = ref(false)
const pickerType = ref<ShapeType>('rectangle')
const pickerPlacement = ref<'fixed' | PlacementType>('fixed')
const ADD_PLACEMENT_OPTIONS: { value: 'fixed' | PlacementType; label: string }[] = [
  { value: 'fixed', label: 'Fixed (single)' },
  { value: 'random', label: 'Random' },
  { value: 'uniform', label: 'Uniform' },
  { value: 'clustered', label: 'Clustered' },
  { value: 'at_shape', label: 'At shape' },
  { value: 'llm', label: 'LLM' },
]
function toggleAddPicker() {
  showAddPicker.value = !showAddPicker.value
}
function addShape() {
  const taken = geo.collectModelUids(props.model, env.value)
  const uid = geo.nextShapeUid(taken)
  const name = geo.nextShapeName(pickerType.value, shapes.value)
  const colorHex = SHAPE_PALETTE[shapes.value.length % SHAPE_PALETTE.length] ?? DEFAULT_SHAPE_COLOR
  const placement = pickerPlacement.value === 'fixed' ? undefined : pickerPlacement.value
  const shape = geo.makeShape({ type: pickerType.value, uid, name, colorHex, env: envDims.value, placement })
  patch({ shapes: [...shapes.value, shape] })
  selectedShapeUid.value = shape.uid
  showAddPicker.value = false
}

// --- figure-draw: ShapeForm asks the canvas to draw a display figure ---
function onDrawFigure(uid: string) {
  selectedShapeUid.value = uid
  nextTick(() => canvasRef.value?.startFigureDraw(uid))
}
</script>
