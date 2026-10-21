<template>
  <div class="space-y-1" data-testid="polygon-draw">
    <div
      class="relative w-full max-w-[260px] aspect-square rounded-md border border-zinc-200 bg-zinc-50 select-none touch-none"
    >
      <svg
        ref="svgRef"
        :viewBox="`0 0 ${SIZE} ${SIZE}`"
        class="absolute inset-0 h-full w-full cursor-crosshair"
        @pointerdown="onBackgroundDown"
      >
        <!-- full-area hit target: transparent SVG regions don't capture pointer
             events on their own, so this rect (pointer-events=all) makes empty
             areas emit events that bubble up to the svg handler below -->
        <rect x="0" y="0" :width="SIZE" :height="SIZE" fill="none" pointer-events="all" />

        <!-- grid -->
        <line
          v-for="(g, i) in gridLines"
          :key="'g' + i"
          :x1="g.x1"
          :y1="g.y1"
          :x2="g.x2"
          :y2="g.y2"
          stroke="#e4e4e7"
          stroke-width="1"
        />

        <!-- origin axes (agent center at 0,0, top-left) -->
        <line :x1="originPx" :y1="originPx" :x2="SIZE" :y2="originPx" stroke="#c4b5fd" stroke-width="1" />
        <line :x1="originPx" :y1="originPx" :x2="originPx" :y2="SIZE" stroke="#c4b5fd" stroke-width="1" />

        <!-- polygon body -->
        <polygon
          v-if="pxPoints.length >= 3"
          :points="polygonAttr"
          fill="#8b5cf6"
          fill-opacity="0.18"
          stroke="#7c3aed"
          stroke-width="1.5"
        />
        <polyline
          v-else-if="pxPoints.length >= 2"
          :points="polygonAttr"
          fill="none"
          stroke="#7c3aed"
          stroke-width="1.5"
        />

        <!-- origin marker (agent center) -->
        <circle :cx="originPx" :cy="originPx" r="3" fill="#a78bfa" />

        <!-- vertex handles -->
        <circle
          v-for="(p, i) in pxPoints"
          :key="'v' + i"
          :cx="p.x"
          :cy="p.y"
          r="5"
          fill="#7c3aed"
          stroke="#ffffff"
          stroke-width="1.5"
          class="cursor-grab"
          data-testid="polygon-vertex"
          @pointerdown.stop="onVertexDown(i, $event)"
          @dblclick.stop="removeVertex(i)"
        />
      </svg>

      <!-- (0,0) = agent center label -->
      <div
        class="pointer-events-none absolute left-1 bottom-1 rounded bg-white/85 px-1.5 py-0.5 text-[9px] text-zinc-400"
      >
        (0,0) = agent center
      </div>

      <!-- size readout (in the draw area only) -->
      <div
        v-if="points.length"
        class="pointer-events-none absolute right-1 top-1 rounded bg-white/85 px-1.5 py-0.5 text-[10px] font-medium text-zinc-600"
        data-testid="polygon-size"
      >
        {{ size.w }} × {{ size.h }} units
      </div>
      <div
        v-else
        class="pointer-events-none absolute inset-0 flex items-center justify-center text-[10px] text-zinc-400"
      >
        Click to add points
      </div>
    </div>

    <div class="flex items-center gap-2">
      <span
        class="text-[10px] leading-snug"
        :class="points.length && points.length < 3 ? 'text-amber-600' : 'text-zinc-400'"
      >
        {{ points.length }} point(s){{ points.length && points.length < 3 ? ' — need at least 3' : '' }}
      </span>
      <Button
        label="Undo"
        icon="pi pi-undo"
        size="small"
        severity="secondary"
        text
        :disabled="!points.length"
        @click="undo"
      />
      <Button
        label="Clear"
        icon="pi pi-trash"
        size="small"
        severity="danger"
        text
        :disabled="!points.length"
        @click="clear"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
// Inline polygon drawing surface for an agent's anchor-relative polygon
// (footprint.points OR display.figure). Agents have no world position in the
// model editor, so — unlike shapes, which draw on the environment canvas — this
// gives a small standalone canvas: click empty space to add a vertex, drag a
// vertex to move it, double-click a vertex to remove it. Undo removes the last
// point; Clear removes all.
//
// Points are stored as NON-NEGATIVE INTEGER offsets from the agent center in
// WORLD units (the coordinate type is unsigned core-wide). The origin (0,0) —
// the agent center — sits at the TOP-LEFT of the drawing area, and the polygon
// extends right (+x) / down (+y) from it, exactly how the engine and renderer
// place it (agentCenter + point). A live bounding-box W×H readout (world units)
// is drawn in the corner. The view auto-fits existing points with a minimum
// extent so a small polygon isn't over-zoomed, and stays fixed during a drag so
// clicks map cleanly.
import { computed, ref } from 'vue'
import Button from 'primevue/button'
import type { PointDTO } from '@/shared/types'

const props = withDefaults(
  defineProps<{
    modelValue: PointDTO[]
    /** Minimum extent (world units) the view shows from the origin. */
    minExtent?: number
  }>(),
  { minExtent: 10 }
)
const emit = defineEmits<{ 'update:modelValue': [v: PointDTO[]] }>()

const SIZE = 240
const MARGIN = 20 // px kept clear around the origin / far edges
const originPx = MARGIN

const svgRef = ref<SVGSVGElement | null>(null)
const dragIndex = ref<number | null>(null)

const points = computed<PointDTO[]>(() => props.modelValue ?? [])

// Extent (world units) the view spans from the origin: grows to fit existing
// points, but never below minExtent. Frozen while dragging so the map is stable.
const frozenExtent = ref<number | null>(null)
const extent = computed(() => {
  if (frozenExtent.value != null) return frozenExtent.value
  let maxC = 0
  for (const p of points.value) maxC = Math.max(maxC, p.x, p.y)
  return Math.max(props.minExtent, Math.ceil(maxC * 1.15))
})

const scale = computed(() => (SIZE - 2 * MARGIN) / extent.value) // px per world unit

function toPx(p: PointDTO): PointDTO {
  return { x: originPx + p.x * scale.value, y: originPx + p.y * scale.value }
}

const pxPoints = computed(() => points.value.map(toPx))
const polygonAttr = computed(() => pxPoints.value.map((p) => `${p.x},${p.y}`).join(' '))

// --- grid ---
function niceStep(ext: number): number {
  const candidates = [1, 2, 5, 10, 20, 50, 100]
  for (const s of candidates) if (ext / s <= 6) return s
  return 100
}
const gridLines = computed(() => {
  const step = niceStep(extent.value)
  const lines: { x1: number; y1: number; x2: number; y2: number }[] = []
  for (let v = step; v <= extent.value; v += step) {
    const off = originPx + v * scale.value
    lines.push({ x1: off, y1: originPx, x2: off, y2: SIZE })
    lines.push({ x1: originPx, y1: off, x2: SIZE, y2: off })
  }
  return lines
})

// --- size readout ---
const size = computed(() => {
  if (!points.value.length) return { w: 0, h: 0 }
  const xs = points.value.map((p) => p.x)
  const ys = points.value.map((p) => p.y)
  return { w: Math.max(...xs) - Math.min(...xs), h: Math.max(...ys) - Math.min(...ys) }
})

// --- pointer → world (non-negative integers) ---
function eventToWorld(ev: PointerEvent): PointDTO {
  const svg = svgRef.value
  if (!svg) return { x: 0, y: 0 }
  const rect = svg.getBoundingClientRect()
  const px = ((ev.clientX - rect.left) / rect.width) * SIZE
  const py = ((ev.clientY - rect.top) / rect.height) * SIZE
  const ext = extent.value
  const toUnit = (screen: number) =>
    Math.max(0, Math.min(ext, Math.round((screen - originPx) / scale.value)))
  return { x: toUnit(px), y: toUnit(py) }
}

// --- interactions ---
function onBackgroundDown(ev: PointerEvent) {
  // A background click adds a vertex (vertex handles stop propagation).
  emit('update:modelValue', [...points.value, eventToWorld(ev)])
}

function onVertexDown(index: number, ev: PointerEvent) {
  dragIndex.value = index
  frozenExtent.value = extent.value
  const svg = svgRef.value
  svg?.setPointerCapture(ev.pointerId)
  svg?.addEventListener('pointermove', onDragMove)
  svg?.addEventListener('pointerup', onDragEnd)
  svg?.addEventListener('pointercancel', onDragEnd)
}
function onDragMove(ev: PointerEvent) {
  if (dragIndex.value == null) return
  const w = eventToWorld(ev)
  emit(
    'update:modelValue',
    points.value.map((p, i) => (i === dragIndex.value ? w : p))
  )
}
function onDragEnd(ev: PointerEvent) {
  const svg = svgRef.value
  svg?.releasePointerCapture?.(ev.pointerId)
  svg?.removeEventListener('pointermove', onDragMove)
  svg?.removeEventListener('pointerup', onDragEnd)
  svg?.removeEventListener('pointercancel', onDragEnd)
  dragIndex.value = null
  frozenExtent.value = null
}

function removeVertex(index: number) {
  emit('update:modelValue', points.value.filter((_, i) => i !== index))
}
function undo() {
  emit('update:modelValue', points.value.slice(0, -1))
}
function clear() {
  emit('update:modelValue', [])
}
</script>
