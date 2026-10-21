<template>
  <div class="space-y-3">
    <!-- Position: only meaningful for a fixed single instance. For strategy/llm
         placement the position is computed at simulation time, so it is hidden. -->
    <div v-if="placementMode === 'fixed'" class="flex flex-wrap items-end gap-2">
      <div class="w-28">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Position X</label>
        <InputNumber
          :modelValue="position.x"
          class="w-full"
          size="small"
          :min="0"
          :max="env.width"
          data-testid="geom-pos-x"
          @update:modelValue="(v) => setPosition({ x: clampX(v ?? 0) })"
        />
      </div>
      <div class="w-28">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Position Y</label>
        <InputNumber
          :modelValue="position.y"
          class="w-full"
          size="small"
          :min="0"
          :max="env.height"
          data-testid="geom-pos-y"
          @update:modelValue="(v) => setPosition({ y: clampY(v ?? 0) })"
        />
      </div>
      <p class="text-[11px] text-zinc-400 pb-1.5 leading-snug">
        {{ anchorHint }}
      </p>
    </div>
    <p v-else class="text-[11px] text-sky-600 leading-snug" data-testid="geom-strategy-note">
      Position is computed at simulation time from the placement strategy.
    </p>

    <!-- rectangle: width + height -->
    <template v-if="type === 'rectangle'">
      <ScalarSpecEditor
        :modelValue="geom.width"
        label="Width"
        testid="geom-width"
        @update:modelValue="(s) => patchGeom({ width: s })"
      />
      <ScalarSpecEditor
        :modelValue="geom.height"
        label="Height"
        testid="geom-height"
        @update:modelValue="(s) => patchGeom({ height: s })"
      />
    </template>

    <!-- circle: radius -->
    <template v-else-if="type === 'circle'">
      <ScalarSpecEditor
        :modelValue="geom.radius"
        label="Radius"
        testid="geom-radius"
        @update:modelValue="(s) => patchGeom({ radius: s })"
      />
    </template>

    <!-- line / polygon: point list -->
    <template v-else>
      <div class="space-y-1">
        <div class="flex items-center justify-between">
          <label class="text-[11px] text-zinc-500 block">
            Points ({{ points.length }})
            <span class="text-zinc-400">{{ type === 'line' ? '— exactly 2' : '— at least 3' }}</span>
          </label>
          <Button
            v-if="type === 'polygon'"
            label="Add point"
            icon="pi pi-plus"
            text
            size="small"
            @click="addPoint"
          />
        </div>
        <div
          v-for="(pt, i) in points"
          :key="i"
          class="flex items-center gap-2"
          :data-testid="`geom-point-${i}`"
        >
          <span class="text-[11px] text-zinc-400 w-5">{{ i + 1 }}</span>
          <InputNumber
            :modelValue="pt.x"
            class="w-24"
            size="small"
            :min="0"
            :max="env.width"
            @update:modelValue="(v) => setPoint(i, { x: clampX(v ?? 0) })"
          />
          <InputNumber
            :modelValue="pt.y"
            class="w-24"
            size="small"
            :min="0"
            :max="env.height"
            @update:modelValue="(v) => setPoint(i, { y: clampY(v ?? 0) })"
          />
          <Button
            v-if="type === 'polygon' && points.length > 3"
            icon="pi pi-trash"
            text
            rounded
            severity="danger"
            size="small"
            aria-label="Remove point"
            @click="removePoint(i)"
          />
        </div>
      </div>
    </template>

    <!-- render styling -->
    <div class="flex flex-wrap items-end gap-4">
      <label v-if="type !== 'line'" class="flex items-center gap-1.5 text-xs text-zinc-600 cursor-pointer">
        <ToggleSwitch :modelValue="fillOn" @update:modelValue="(v) => patchGeom({ fill: !!v })" />
        <span>Fill</span>
      </label>
      <label class="flex items-center gap-1.5 text-xs text-zinc-600 cursor-pointer">
        <ToggleSwitch :modelValue="strokeOn" @update:modelValue="(v) => patchGeom({ stroke: !!v })" />
        <span>Stroke</span>
      </label>
      <div v-if="strokeOn && type !== 'line'" class="w-28">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Stroke width</label>
        <InputNumber
          :modelValue="geom.stroke_width ?? null"
          class="w-full"
          size="small"
          :min="0"
          placeholder="1"
          showClear
          @update:modelValue="(v) => patchGeom({ stroke_width: v ?? undefined })"
        />
      </div>
      <div v-if="type === 'line'" class="w-28">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Thickness</label>
        <InputNumber
          :modelValue="geom.thickness ?? null"
          class="w-full"
          size="small"
          :min="0"
          placeholder="1"
          showClear
          @update:modelValue="(v) => patchGeom({ thickness: v ?? undefined })"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// Edits a shape's ShapeGeometryDTO. Type-conditional: rectangle → width+height,
// circle → radius (both via ScalarSpecEditor for fixed|range), line/polygon → a
// numeric point list. Position is shown ONLY in fixed placement mode (strategy/
// llm placement resolves position at simulation time). fill/stroke/stroke_width
// and line thickness are exposed. Always emits a fresh geometry object.
import InputNumber from 'primevue/inputnumber'
import ToggleSwitch from 'primevue/toggleswitch'
import Button from 'primevue/button'
import ScalarSpecEditor from './ScalarSpecEditor.vue'
import { computed } from 'vue'
import type { ShapeGeometryDTO, ShapeType, PointDTO } from '@/shared/types'

const props = defineProps<{
  modelValue: ShapeGeometryDTO | undefined
  type: ShapeType
  placementMode: 'fixed' | 'strategy' | 'llm'
  env: { width: number; height: number }
}>()
const emit = defineEmits<{ 'update:modelValue': [v: ShapeGeometryDTO] }>()

const geom = computed<ShapeGeometryDTO>(() => props.modelValue ?? {})
const position = computed<PointDTO>(() => geom.value.position ?? { x: 0, y: 0 })
const points = computed<PointDTO[]>(() => geom.value.points ?? [])

// Fill/stroke default ON (renderer + engine treat undefined as enabled).
const fillOn = computed(() => geom.value.fill !== false)
const strokeOn = computed(() => geom.value.stroke !== false)

const anchorHint = computed(() => {
  switch (props.type) {
    case 'rectangle':
      return 'Top-left corner.'
    case 'circle':
      return 'Circle centre.'
    default:
      return 'Anchor (first vertex).'
  }
})

function clampX(v: number): number {
  return Math.max(0, Math.min(props.env.width, Math.round(v)))
}
function clampY(v: number): number {
  return Math.max(0, Math.min(props.env.height, Math.round(v)))
}

function patchGeom(partial: Partial<ShapeGeometryDTO>) {
  emit('update:modelValue', { ...geom.value, ...partial })
}

function setPosition(partial: Partial<PointDTO>) {
  patchGeom({ position: { ...position.value, ...partial } })
}

function setPoint(i: number, partial: Partial<PointDTO>) {
  const next = points.value.map((p, idx) => (idx === i ? { ...p, ...partial } : p))
  const patch: Partial<ShapeGeometryDTO> = { points: next }
  // Keep the anchor position aligned with the first vertex — but ONLY for a fixed
  // shape. A strategy/llm-placed line/polygon has no position (placement computes
  // its anchor at sim time); writing one here would flip it to fixed and trip the
  // position⇒population-1 rule.
  if (props.placementMode === 'fixed' && i === 0 && next[0]) patch.position = { ...next[0] }
  patchGeom(patch)
}

function addPoint() {
  const last = points.value[points.value.length - 1]
  const seed = last ? { x: last.x, y: last.y } : { x: 0, y: 0 }
  patchGeom({ points: [...points.value, seed] })
}

function removePoint(i: number) {
  const next = points.value.filter((_, idx) => idx !== i)
  const patch: Partial<ShapeGeometryDTO> = { points: next }
  if (props.placementMode === 'fixed' && next[0]) patch.position = { ...next[0] }
  patchGeom(patch)
}
</script>
