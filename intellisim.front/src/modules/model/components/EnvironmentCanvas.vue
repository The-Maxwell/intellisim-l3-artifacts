<template>
  <div
    ref="containerRef"
    class="w-full h-full relative bg-zinc-100 overflow-hidden select-none"
    tabindex="0"
    @keydown="onKeyDown"
  >
    <canvas ref="canvasRef" class="block" :class="cursorClass" />

    <!-- Tool palette (Phase 5B). Hidden when the model is read-only (locked). -->
    <div
      v-if="!locked"
      class="absolute top-3 left-3 flex flex-col gap-1"
    >
      <div class="flex items-center gap-1 bg-white/90 rounded-lg shadow border border-zinc-200 p-1">
        <button
          v-for="t in tools"
          :key="t.id"
          type="button"
          class="w-7 h-7 flex items-center justify-center rounded text-xs"
          :class="tool === t.id ? 'bg-violet-600 text-white' : 'text-zinc-600 hover:bg-zinc-100'"
          :title="t.label"
          @click="selectTool(t.id)"
        >
          <i :class="t.icon" />
        </button>
        <span class="w-px h-5 bg-zinc-200 mx-0.5" />
        <button
          type="button"
          class="w-7 h-7 flex items-center justify-center rounded text-xs"
          :class="snap ? 'bg-violet-100 text-violet-700' : 'text-zinc-600 hover:bg-zinc-100'"
          title="Snap to grid"
          @click="snap = !snap"
        >
          <i class="pi pi-th-large" />
        </button>
        <button
          type="button"
          class="w-7 h-7 flex items-center justify-center rounded text-xs"
          :class="canDeleteSelected ? 'text-red-600 hover:bg-red-50' : 'text-zinc-300 cursor-not-allowed'"
          :disabled="!canDeleteSelected"
          title="Delete selected shape"
          @click="deleteSelected"
        >
          <i class="pi pi-trash" />
        </button>
      </div>
      <!-- per-tool hint -->
      <div class="bg-white/90 rounded-md shadow border border-zinc-200 px-2 py-1 text-[10px] text-zinc-500 max-w-[260px]">
        {{ toolHint }}
      </div>
    </div>

    <!-- Read-only banner when the whole model is locked -->
    <div
      v-else
      class="absolute top-3 left-3 flex items-center gap-1 bg-white/90 rounded-lg shadow border border-zinc-200 px-2 py-1 text-[11px] text-zinc-500"
    >
      <i class="pi pi-lock text-amber-500" />
      <span>Read-only</span>
    </div>

    <!-- View controls (zoom / fit) -->
    <div class="absolute top-3 right-3 flex flex-col gap-1 bg-white/90 rounded-lg shadow border border-zinc-200 p-1">
      <button
        type="button"
        class="w-7 h-7 flex items-center justify-center text-zinc-600 hover:bg-zinc-100 rounded"
        title="Zoom in"
        @click="zoomIn"
      >
        <i class="pi pi-plus text-xs" />
      </button>
      <button
        type="button"
        class="w-7 h-7 flex items-center justify-center text-zinc-600 hover:bg-zinc-100 rounded"
        title="Zoom out"
        @click="zoomOut"
      >
        <i class="pi pi-minus text-xs" />
      </button>
      <button
        type="button"
        class="w-7 h-7 flex items-center justify-center text-zinc-600 hover:bg-zinc-100 rounded"
        title="Fit to view"
        @click="fitView"
      >
        <i class="pi pi-expand text-xs" />
      </button>
    </div>

    <!-- Selected-shape hint (editing lives in the ShapeForm beside the canvas). -->
    <div
      v-if="selectedShape && (selectedShape.locked || isStrategyPlaced(selectedShape) || drawingFigure)"
      class="absolute top-3 right-12 max-w-[16rem] bg-white/95 rounded-lg shadow border border-zinc-200 px-2.5 py-1.5 text-[11px] leading-snug"
    >
      <p v-if="drawingFigure" class="text-violet-600">
        Drawing figure — click to add points; double-click or Enter to finish (≥3), Esc to cancel.
      </p>
      <p v-else-if="selectedShape.locked" class="text-amber-600">
        Locked — unlock in the form to edit on canvas.
      </p>
      <p v-else class="text-sky-600">
        Placement is computed at simulation time — configure it in the form.
      </p>
    </div>

    <!-- Empty-environment hint -->
    <div
      v-if="modelValue.width <= 0 || modelValue.height <= 0"
      class="absolute inset-0 flex items-center justify-center text-xs text-zinc-400 pointer-events-none"
    >
      Set a width and height to render the environment.
    </div>

    <!-- Legend -->
    <div
      v-if="legend.length"
      class="absolute bottom-3 left-3 bg-white/90 rounded-lg shadow border border-zinc-200 p-2 text-xs max-w-[60%]"
    >
      <div
        v-for="item in legend"
        :key="item.uid"
        class="flex items-center gap-2 mb-1 last:mb-0"
      >
        <span :style="{ backgroundColor: item.color }" class="w-3 h-3 rounded-sm flex-shrink-0" />
        <span class="text-zinc-600 truncate">{{ item.name }}</span>
        <i v-if="item.ghost" class="pi pi-map-marker text-[10px] text-zinc-400" title="placed by strategy" />
        <i v-if="item.locked" class="pi pi-lock text-[10px] text-amber-500" title="locked" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// Phase 5B — PixiJS environment canvas, full INTERACTIVE editor.
//
// Builds on the 5A render half (grid, pan/zoom/fit, fixed/ghost/locked shape
// rendering, world/screen transforms, layers). Adds:
//   - a tool palette (select / rectangle / circle / line / polygon / pan) plus a
//     snap-to-grid toggle and a delete button;
//   - drawing of the 4 ShapeTypes as FIXED-position instances (population 1, no
//     placement) with coords clamped to [0,w]x[0,h], rounded to int, optionally
//     snapped to cellSize;
//   - select / move / resize / delete with selection outline + handles drawn in
//     overlayLayer; body-drag translates, handle-drag resizes;
//   - a side panel quick-editing the selected shape's name/color/layer/visible/
//     locked + a read-only geometry note.
//
// Locked shapes are selectable (so the panel can unlock them) but never
// drag/resize/delete on canvas. Strategy-placed ghosts are selectable but not
// editable on canvas (their position is computed at sim time). When the model is
// read-only (`locked` prop) the whole canvas is view-only.
//
// All shape mutations are IMMUTABLE and pushed via emit('update:modelValue', ...)
// with a fresh shapes array — never a prop mutation. The store mutation is
// emitted once on pointerup (drags preview locally without spamming the store).
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useResizeObserver } from '@vueuse/core'
import { Application, Container, Graphics, Sprite, Text } from 'pixi.js'
import type { FederatedPointerEvent } from 'pixi.js'
import type { EnvironmentDTO, ShapeDTO, ScalarSpecDTO, PointDTO, Model } from '@/shared/types'
import * as geo from './environmentCanvasGeometry'
import {
  loadTextureFromBase64,
  displayIsImage,
  displayIsFigure,
} from '@/modules/simulation/components/displayRender'

const props = withDefaults(
  defineProps<{
    modelValue: EnvironmentDTO
    /** Grid spacing in world (cell) units; from model.spatial_cell_size. */
    cellSize?: number
    /** Model-level read-only — when true the whole canvas is view-only. */
    locked?: boolean
    /** The owning model, used only to make drawn-shape uids model-unique. */
    model?: Model | null
    /** Controlled selection: uid of the selected shape (shared with the list/form). */
    selectedUid?: string | null
  }>(),
  { cellSize: 0, locked: false, model: null, selectedUid: null }
)

const emit = defineEmits<{
  'update:modelValue': [v: EnvironmentDTO]
  'update:selectedUid': [uid: string | null]
}>()

const containerRef = ref<HTMLElement | null>(null)
const canvasRef = ref<HTMLCanvasElement | null>(null)

// --- palette (shared with the shape list + add-shape factory) ---
const PALETTE = geo.SHAPE_PALETTE
const DEFAULT_GRID = 20 // fallback world-unit grid spacing when cellSize is 0/undefined
const MIN_SCALE = 0.02
const MAX_SCALE = 200
const HANDLE_PX = 7 // on-screen handle half-size / hit radius (px)

// --- tools ---
type Tool = 'select' | 'rectangle' | 'circle' | 'line' | 'polygon' | 'pan'
const tools: { id: Tool; label: string; icon: string }[] = [
  // Icons must be valid PrimeIcons classes (v7). `pi-arrows-alt` is the canonical
  // four-way "move" glyph → used for Pan (drag the view). Select uses a cursor-like
  // arrow so the two tools read distinctly. (Was `pi-arrows-alt-v`, which does not
  // exist in PrimeIcons and rendered as an empty button.)
  { id: 'select', label: 'Select / move / resize', icon: 'pi pi-arrow-up-left' },
  { id: 'rectangle', label: 'Rectangle', icon: 'pi pi-stop' },
  { id: 'circle', label: 'Circle', icon: 'pi pi-circle' },
  { id: 'line', label: 'Line', icon: 'pi pi-minus' },
  { id: 'polygon', label: 'Polygon', icon: 'pi pi-share-alt' },
  { id: 'pan', label: 'Pan (drag to move the view)', icon: 'pi pi-arrows-alt' },
]
const tool = ref<Tool>('select')
const snap = ref(false)
// Selection is CONTROLLED by the parent (list ⇄ canvas share one source of
// truth). Reads use this computed; writes go through setSelected → emit.
const selectedUid = computed<string | null>(() => props.selectedUid ?? null)
function setSelected(uid: string | null) {
  emit('update:selectedUid', uid)
}
// When set, the polygon tool's clicks build a display FIGURE for this shape
// (anchor-relative to the shape position) instead of creating a new shape.
const figureTargetUid = ref<string | null>(null)
const drawingFigure = computed(() => figureTargetUid.value !== null)

const toolHints: Record<Tool, string> = {
  select: 'Click a shape to select. Drag body to move, drag a handle to resize. Delete removes it.',
  rectangle: 'Drag from one corner to the opposite corner.',
  circle: 'Drag from the center outward to set the radius.',
  line: 'Click the start point, then click the end point.',
  polygon: 'Click to add points; double-click or Enter to finish (≥3), Esc to cancel.',
  pan: 'Drag to pan the view. (Middle-drag or space-drag works in any tool.)',
}
const toolHint = computed(() => toolHints[tool.value])

const cursorClass = computed(() => {
  if (props.locked) return 'cursor-default'
  if (tool.value === 'pan' || spaceDown.value) return 'cursor-grab'
  if (tool.value === 'select') return 'cursor-default'
  return 'cursor-crosshair'
})

// --- pixi handles ---
let app: Application | null = null
let worldContainer: Container | null = null // pan/zoom transform target (world units)
let gridLayer: Container | null = null // background + grid (under shapes)
let shapesLayer: Container | null = null // rendered shapes / ghosts
let overlayLayer: Container | null = null // selection handles / draw preview
let mounted = false
// Bumped on every shapes re-render so an async sprite texture that resolves
// after the layer was cleared/redrawn is discarded instead of orphan-added.
let shapesRenderToken = 0

// --- pan state ---
let panning = false
const panStart = { x: 0, y: 0, worldX: 0, worldY: 0 }
const spaceDown = ref(false)

// --- interaction state (drawing / editing) ---
type DragMode = 'none' | 'draw-rect' | 'draw-circle' | 'move' | 'resize'
let dragMode: DragMode = 'none'
const drawStart = ref<PointDTO | null>(null) // world units (anchor for rect/circle)
const drawCurrent = ref<PointDTO | null>(null) // world units (live pointer)
const polyPoints = ref<PointDTO[]>([]) // in-progress polygon/line points (world)
const polyCursor = ref<PointDTO | null>(null) // live pointer for poly/line preview
// move/resize working state on a LOCAL clone of the selected shape (only emitted
// on pointerup, to avoid spamming the store every mouse move).
let editGeometry: ShapeDTO['geometry'] | null = null
let editHandleId: string | null = null
let moveAnchorWorld: PointDTO | null = null // pointer world pos at move start

// ----------------------------------------------------------------------------
// color / geometry helpers (render half)
// ----------------------------------------------------------------------------

function shapeColorHex(shape: ShapeDTO, index: number): string {
  const raw = geo.shapeColor(shape)
  if (typeof raw === 'string' && /^#?[0-9a-fA-F]{3,8}$/.test(raw)) {
    return raw.startsWith('#') ? raw : `#${raw}`
  }
  return PALETTE[index % PALETTE.length] ?? '#8B5CF6'
}

function hexToNumber(hex: string): number {
  const cleaned = hex.replace('#', '')
  const full = cleaned.length === 3 ? cleaned.split('').map((c) => c + c).join('') : cleaned
  const n = parseInt(full.slice(0, 6), 16)
  return Number.isNaN(n) ? 0x8b5cf6 : n
}

function scalarValue(spec: ScalarSpecDTO | undefined): number {
  return geo.scalarValue(spec)
}

function isFixed(shape: ShapeDTO): boolean {
  return geo.isFixed(shape)
}
function isStrategyPlaced(shape: ShapeDTO): boolean {
  return geo.isStrategyPlaced(shape)
}

function ghostAnchor(shape: ShapeDTO): PointDTO {
  if (shape.geometry?.position) return shape.geometry.position
  if (shape.geometry?.points?.length) return shape.geometry.points[0]!
  return { x: props.modelValue.width / 2, y: props.modelValue.height / 2 }
}

// ----------------------------------------------------------------------------
// coordinate transforms
// ----------------------------------------------------------------------------

function worldToScreen(x: number, y: number): { x: number; y: number } {
  if (!worldContainer) return { x, y }
  return {
    x: x * worldContainer.scale.x + worldContainer.x,
    y: y * worldContainer.scale.y + worldContainer.y,
  }
}
function screenToWorld(x: number, y: number): { x: number; y: number } {
  if (!worldContainer || worldContainer.scale.x === 0) return { x, y }
  return {
    x: (x - worldContainer.x) / worldContainer.scale.x,
    y: (y - worldContainer.y) / worldContainer.scale.y,
  }
}
defineExpose({ worldToScreen, screenToWorld, fitView, startFigureDraw })

// World-units-per-screen-pixel (for hit tolerances/handle sizes).
function worldPerPx(): number {
  if (!worldContainer) return 1
  return 1 / Math.max(worldContainer.scale.x, MIN_SCALE)
}

// ----------------------------------------------------------------------------
// selection / side panel
// ----------------------------------------------------------------------------
const selectedShape = computed<ShapeDTO | null>(() => {
  if (!selectedUid.value) return null
  return (props.modelValue.shapes ?? []).find((s) => s.uid === selectedUid.value) ?? null
})

const canDeleteSelected = computed(
  () => !props.locked && !!selectedShape.value && !selectedShape.value.locked
)

function selectTool(t: Tool) {
  if (props.locked) return
  cancelPolygon()
  tool.value = t
  if (t !== 'select') setSelected(null)
  renderOverlay()
}

// ----------------------------------------------------------------------------
// emit / mutation helpers (immutable; emit the WHOLE environment)
// ----------------------------------------------------------------------------
function emitShapes(shapes: ShapeDTO[]) {
  emit('update:modelValue', { ...props.modelValue, shapes })
}

function appendShape(shape: ShapeDTO) {
  emitShapes([...(props.modelValue.shapes ?? []), shape])
}

function replaceShape(uid: string, updated: ShapeDTO) {
  emitShapes((props.modelValue.shapes ?? []).map((s) => (s.uid === uid ? updated : s)))
}

function deleteSelected() {
  const s = selectedShape.value
  if (!s || !canDeleteSelected.value) return
  emitShapes((props.modelValue.shapes ?? []).filter((x) => x.uid !== s.uid))
  setSelected(null)
}

// ----------------------------------------------------------------------------
// drawing — create FIXED shapes
// ----------------------------------------------------------------------------
function quantize(p: PointDTO): PointDTO {
  return geo.quantizePoint(p, props.modelValue, props.cellSize ?? 0, snap.value)
}

function newShapeMeta(type: ShapeDTO['type']) {
  const taken = geo.collectModelUids(props.model, props.modelValue)
  const uid = geo.nextShapeUid(taken)
  const name = geo.nextShapeName(type, props.modelValue.shapes ?? [])
  const index = (props.modelValue.shapes ?? []).length
  const colorHex = PALETTE[index % PALETTE.length] ?? '#8B5CF6'
  return { uid, name, colorHex }
}

function finishRect(a: PointDTO, b: PointDTO) {
  const x = Math.min(a.x, b.x)
  const y = Math.min(a.y, b.y)
  const w = Math.abs(b.x - a.x)
  const h = Math.abs(b.y - a.y)
  if (w < 1 || h < 1) return // ignore degenerate
  const meta = newShapeMeta('rectangle')
  const shape = geo.createShape({ type: 'rectangle', ...meta, position: { x, y }, width: w, height: h })
  appendShape(shape)
  afterCreate(shape)
}

function finishCircle(center: PointDTO, edge: PointDTO) {
  const r = Math.round(Math.hypot(edge.x - center.x, edge.y - center.y))
  if (r < 1) return
  // clamp radius so the circle stays in bounds
  const maxR = Math.min(center.x, center.y, props.modelValue.width - center.x, props.modelValue.height - center.y)
  const rr = Math.max(1, Math.min(r, maxR))
  const meta = newShapeMeta('circle')
  const shape = geo.createShape({ type: 'circle', ...meta, position: center, radius: rr })
  appendShape(shape)
  afterCreate(shape)
}

function finishLine(a: PointDTO, b: PointDTO) {
  if (a.x === b.x && a.y === b.y) return
  const meta = newShapeMeta('line')
  const shape = geo.createShape({ type: 'line', ...meta, points: [a, b] })
  appendShape(shape)
  afterCreate(shape)
}

function finishPolygon() {
  const pts = polyPoints.value
  if (pts.length < 3) {
    cancelPolygon()
    return
  }
  // Figure-draw mode: write the points (anchor-relative to the shape position)
  // into the target shape's display.figure instead of creating a new shape.
  if (figureTargetUid.value) {
    finishFigure(pts)
    return
  }
  const meta = newShapeMeta('polygon')
  const shape = geo.createShape({ type: 'polygon', ...meta, points: pts.map((p) => ({ ...p })) })
  cancelPolygon()
  appendShape(shape)
  afterCreate(shape)
}

function cancelPolygon() {
  polyPoints.value = []
  polyCursor.value = null
  figureTargetUid.value = null
  // I1: also clear the line anchor and any rect/circle draw-drag so that
  // switching tools (which calls cancelPolygon) never leaves stale state.
  pendingLineStart.value = null
  drawStart.value = null
  drawCurrent.value = null
  dragMode = 'none'
}

// ----------------------------------------------------------------------------
// display figure drawing (writes display.figure on the selected shape)
// ----------------------------------------------------------------------------

// The anchor a shape's figure points are relative to = its geometry.position,
// else its first geometry point, else the world center.
function figureAnchor(shape: ShapeDTO): PointDTO {
  if (shape.geometry?.position) return shape.geometry.position
  if (shape.geometry?.points?.length) return shape.geometry.points[0]!
  return { x: props.modelValue.width / 2, y: props.modelValue.height / 2 }
}

// Begin drawing a display figure for a shape. Called imperatively by the parent
// (EnvironmentForm) via the exposed ref when the ShapeForm's DisplayEditor emits
// draw-figure — the target uid is passed explicitly (the selection prop may not
// have propagated yet). Falls back to the current selection when omitted.
function startFigureDraw(uid?: string) {
  const targetUid = uid ?? selectedUid.value
  const s = (props.modelValue.shapes ?? []).find((x) => x.uid === targetUid)
  if (!s || props.locked || s.locked) return
  figureTargetUid.value = s.uid
  tool.value = 'polygon'
  polyPoints.value = []
  polyCursor.value = null
  renderOverlay()
}

function finishFigure(absPts: PointDTO[]) {
  const uid = figureTargetUid.value
  const target = (props.modelValue.shapes ?? []).find((s) => s.uid === uid)
  if (!target) {
    cancelPolygon()
    return
  }
  const anchor = figureAnchor(target)
  // store anchor-relative points (offsets from the shape position)
  const rel = absPts.map((p) => ({ x: Math.round(p.x - anchor.x), y: Math.round(p.y - anchor.y) }))
  const display = { ...(target.display ?? {}), kind: 'figure' as const, figure: rel }
  cancelPolygon()
  replaceShape(target.uid, { ...target, display })
  // keep the shape selected so the form stays on it
  tool.value = 'select'
  setSelected(target.uid)
  renderOverlay()
}

// After creating a shape: switch back to select and select the new shape.
function afterCreate(shape: ShapeDTO) {
  tool.value = 'select'
  setSelected(shape.uid)
  drawStart.value = null
  drawCurrent.value = null
  dragMode = 'none'
  renderOverlay()
}

// ----------------------------------------------------------------------------
// hit testing (topmost by layer)
// ----------------------------------------------------------------------------
function hitTestShape(world: PointDTO): ShapeDTO | null {
  const shapes = props.modelValue.shapes ?? []
  const tol = HANDLE_PX * worldPerPx()
  // topmost = highest layer, then later in array order → iterate reverse of the
  // same ordering the renderer uses
  const ordered = shapes
    .map((shape, index) => ({ shape, index }))
    .sort((a, b) => (a.shape.layer ?? 0) - (b.shape.layer ?? 0) || a.index - b.index)
  for (let i = ordered.length - 1; i >= 0; i--) {
    const s = ordered[i]!.shape
    if (s.visible === false) continue
    if (geo.pointInShape(s, world, tol)) return s
  }
  return null
}

// Can a shape be edited (moved/resized) on canvas?
function shapeEditable(s: ShapeDTO): boolean {
  return !props.locked && !s.locked && !isStrategyPlaced(s)
}
// Can the currently selected shape be edited?
function selectedEditable(): boolean {
  const s = selectedShape.value
  return !!s && shapeEditable(s)
}

// ----------------------------------------------------------------------------
// pointer routing
// ----------------------------------------------------------------------------
function isPanGesture(e: FederatedPointerEvent): boolean {
  // middle button, or space-drag, or pan tool
  const btn = (e.nativeEvent as PointerEvent | undefined)?.button
  return tool.value === 'pan' || spaceDown.value || btn === 1
}

function onStagePointerDown(e: FederatedPointerEvent) {
  if (!worldContainer) return
  containerRef.value?.focus()

  // pan takes precedence (pan tool / middle / space)
  if (isPanGesture(e)) {
    panning = true
    panStart.x = e.globalX
    panStart.y = e.globalY
    panStart.worldX = worldContainer.x
    panStart.worldY = worldContainer.y
    return
  }
  if (props.locked) return

  const w = screenToWorld(e.globalX, e.globalY)

  switch (tool.value) {
    case 'select':
      startSelectGesture(w)
      break
    case 'rectangle':
      dragMode = 'draw-rect'
      drawStart.value = quantize(w)
      drawCurrent.value = drawStart.value
      break
    case 'circle':
      dragMode = 'draw-circle'
      drawStart.value = quantize(w) // center
      drawCurrent.value = drawStart.value
      break
    case 'line':
      handleLineClick(quantize(w))
      break
    case 'polygon': {
      const pt = quantize(w)
      const last = polyPoints.value[polyPoints.value.length - 1]
      // skip a duplicate of the previous vertex (e.g. the two clicks of a
      // double-click-to-finish landing on the same cell)
      if (!last || last.x !== pt.x || last.y !== pt.y) {
        polyPoints.value = [...polyPoints.value, pt]
      }
      polyCursor.value = pt
      renderOverlay()
      break
    }
  }
}

function onStagePointerMove(e: FederatedPointerEvent) {
  if (panning && worldContainer) {
    worldContainer.x = panStart.worldX + (e.globalX - panStart.x)
    worldContainer.y = panStart.worldY + (e.globalY - panStart.y)
    rerenderForScale()
    return
  }
  if (props.locked) return

  const w = screenToWorld(e.globalX, e.globalY)

  if (dragMode === 'draw-rect' || dragMode === 'draw-circle') {
    drawCurrent.value = quantize(w)
    renderOverlay()
    return
  }
  if (dragMode === 'move') {
    const s = selectedShape.value
    if (s && moveAnchorWorld) {
      editGeometry = geo.translateShape(
        s,
        w.x - moveAnchorWorld.x,
        w.y - moveAnchorWorld.y,
        props.modelValue,
        props.cellSize ?? 0,
        snap.value
      )
      renderShapesWithPreview()
      renderOverlay()
    }
    return
  }
  if (dragMode === 'resize') {
    const s = selectedShape.value
    if (s && editHandleId) {
      editGeometry = geo.resizeShape(s, editHandleId, w, props.modelValue, props.cellSize ?? 0, snap.value)
      renderShapesWithPreview()
      renderOverlay()
    }
    return
  }

  // live preview for line / polygon in-progress
  if (tool.value === 'line' && pendingLineStart.value) {
    polyCursor.value = quantize(w)
    renderOverlay()
  } else if (tool.value === 'polygon' && polyPoints.value.length) {
    polyCursor.value = quantize(w)
    renderOverlay()
  }
}

// I2: shared end-gesture logic called by both the stage pointerup and the
// window-level fallback. Returns true if it consumed an active gesture (so the
// window handler can guard against a double-commit when the stage already fired).
function endGesture(): boolean {
  if (panning) {
    panning = false
    return true
  }
  if (props.locked) return false

  if (dragMode === 'draw-rect' && drawStart.value && drawCurrent.value) {
    finishRect(drawStart.value, drawCurrent.value)
    dragMode = 'none'
    drawStart.value = null
    drawCurrent.value = null
    renderOverlay()
    return true
  }
  if (dragMode === 'draw-circle' && drawStart.value && drawCurrent.value) {
    finishCircle(drawStart.value, drawCurrent.value)
    dragMode = 'none'
    drawStart.value = null
    drawCurrent.value = null
    renderOverlay()
    return true
  }
  if ((dragMode === 'move' || dragMode === 'resize') && editGeometry) {
    // commit the edit ONCE, on release — but only if the geometry actually
    // changed (a plain click on a shape starts a move but should not emit).
    const s = selectedShape.value
    if (s && JSON.stringify(s.geometry ?? {}) !== JSON.stringify(editGeometry)) {
      replaceShape(s.uid, { ...s, geometry: editGeometry })
    }
    editGeometry = null
    editHandleId = null
    moveAnchorWorld = null
    dragMode = 'none'
    // re-render from props now that the store has the committed geometry
    renderShapes()
    renderOverlay()
    return true
  }
  return false
}

function onStagePointerUp(_e: FederatedPointerEvent) {
  endGesture()
}

// I2: window-level fallback — fires when the pointer is released outside the
// browser window (the stage pointerup/pointerupoutside may not fire in that
// case). Guard: if the stage handler already reset everything, endGesture is
// a no-op (panning===false, dragMode==='none').
function onWindowPointerUp() {
  endGesture()
}

// I2: also handle pointercancel (e.g. system interrupt, touch cancel) —
// abort any active gesture without committing.
function onWindowPointerCancel() {
  panning = false
  if (dragMode !== 'none') {
    // discard the in-progress draw/move/resize without committing
    dragMode = 'none'
    drawStart.value = null
    drawCurrent.value = null
    editGeometry = null
    editHandleId = null
    moveAnchorWorld = null
    renderShapes()
    renderOverlay()
  }
}

// --- select-tool gesture entry: pick handle → resize, body → move, else select ---
function startSelectGesture(world: PointDTO) {
  const s = selectedShape.value
  // 1) if a shape is already selected and editable, test its handles first
  if (s && selectedEditable()) {
    const handles = geo.getHandles(s)
    const hit = geo.hitHandle(handles, world, HANDLE_PX * worldPerPx())
    if (hit) {
      dragMode = 'resize'
      editHandleId = hit.id
      editGeometry = s.geometry ? { ...s.geometry } : {}
      return
    }
  }
  // 2) hit-test shapes
  const picked = hitTestShape(world)
  if (!picked) {
    setSelected(null)
    renderOverlay()
    return
  }
  setSelected(picked.uid)
  // start a body-move only if the picked shape is editable. Check `picked`
  // directly — the controlled selectedUid prop hasn't propagated back yet this
  // tick, so selectedShape/selectedEditable() would still read the OLD selection.
  if (shapeEditable(picked)) {
    dragMode = 'move'
    moveAnchorWorld = world
    editGeometry = picked.geometry ? { ...picked.geometry } : {}
  }
  renderOverlay()
}

// --- line tool: two-click ---
const pendingLineStart = ref<PointDTO | null>(null)
function handleLineClick(p: PointDTO) {
  if (!pendingLineStart.value) {
    pendingLineStart.value = p
    polyCursor.value = p
  } else {
    finishLine(pendingLineStart.value, p)
    pendingLineStart.value = null
    polyCursor.value = null
  }
  renderOverlay()
}

// ----------------------------------------------------------------------------
// keyboard
// ----------------------------------------------------------------------------
function onKeyDown(e: KeyboardEvent) {
  if (props.locked) return
  if (e.key === ' ') {
    spaceDown.value = true
    return
  }
  if (e.key === 'Escape') {
    cancelPolygon()
    pendingLineStart.value = null
    renderOverlay()
    return
  }
  if (e.key === 'Enter' && tool.value === 'polygon' && polyPoints.value.length >= 3) {
    finishPolygon()
    return
  }
  if ((e.key === 'Delete' || e.key === 'Backspace') && canDeleteSelected.value) {
    e.preventDefault()
    deleteSelected()
  }
}
function onKeyUp(e: KeyboardEvent) {
  if (e.key === ' ') spaceDown.value = false
}

// double-click on the container finishes a polygon
function onDblClick() {
  if (props.locked) return
  if (tool.value === 'polygon' && polyPoints.value.length >= 3) finishPolygon()
}

// ----------------------------------------------------------------------------
// legend
// ----------------------------------------------------------------------------
const legend = computed(() => {
  const shapes = props.modelValue.shapes ?? []
  return shapes.map((s, i) => ({
    uid: s.uid || `shape-${i}`,
    name: s.name || s.type || 'shape',
    color: shapeColorHex(s, i),
    ghost: isStrategyPlaced(s),
    locked: s.locked,
  }))
})

// ----------------------------------------------------------------------------
// pixi bootstrap
// ----------------------------------------------------------------------------
async function initPixi() {
  if (!canvasRef.value || !containerRef.value) return

  const a = new Application()
  await a.init({
    canvas: canvasRef.value,
    width: Math.max(1, containerRef.value.clientWidth),
    height: Math.max(1, containerRef.value.clientHeight),
    backgroundColor: 0xf4f4f5,
    antialias: true,
    resolution: window.devicePixelRatio || 1,
    autoDensity: true,
  })
  if (!mounted) {
    a.destroy(true)
    return
  }
  app = a

  worldContainer = new Container()
  gridLayer = new Container()
  shapesLayer = new Container()
  overlayLayer = new Container()
  worldContainer.addChild(gridLayer, shapesLayer, overlayLayer)
  app.stage.addChild(worldContainer)

  app.stage.eventMode = 'static'
  app.stage.hitArea = app.screen
  app.stage.on('pointerdown', onStagePointerDown)
  app.stage.on('pointermove', onStagePointerMove)
  app.stage.on('pointerup', onStagePointerUp)
  app.stage.on('pointerupoutside', onStagePointerUp)

  containerRef.value.addEventListener('wheel', handleWheel, { passive: false })
  containerRef.value.addEventListener('dblclick', onDblClick)
  window.addEventListener('keyup', onKeyUp)
  // I2: catch pointer-release that happens outside the browser window
  window.addEventListener('pointerup', onWindowPointerUp)
  window.addEventListener('pointercancel', onWindowPointerCancel)

  renderAll()
  fitView()
}

// ----------------------------------------------------------------------------
// pan / zoom / fit
// ----------------------------------------------------------------------------
function clampScale(s: number): number {
  return Math.min(MAX_SCALE, Math.max(MIN_SCALE, s))
}

function zoomAt(screenX: number, screenY: number, factor: number) {
  if (!worldContainer) return
  const before = clampScale(worldContainer.scale.x)
  const after = clampScale(before * factor)
  if (after === before) return
  const wx = (screenX - worldContainer.x) / before
  const wy = (screenY - worldContainer.y) / before
  worldContainer.scale.set(after)
  worldContainer.x = screenX - wx * after
  worldContainer.y = screenY - wy * after
  rerenderForScale()
}

function handleWheel(e: WheelEvent) {
  e.preventDefault()
  if (!worldContainer || !containerRef.value) return
  const rect = containerRef.value.getBoundingClientRect()
  const factor = e.deltaY > 0 ? 0.9 : 1.1
  zoomAt(e.clientX - rect.left, e.clientY - rect.top, factor)
}

function zoomIn() {
  if (!containerRef.value) return
  zoomAt(containerRef.value.clientWidth / 2, containerRef.value.clientHeight / 2, 1.2)
}
function zoomOut() {
  if (!containerRef.value) return
  zoomAt(containerRef.value.clientWidth / 2, containerRef.value.clientHeight / 2, 0.8)
}

function fitView() {
  if (!worldContainer || !containerRef.value) return
  const w = props.modelValue.width
  const h = props.modelValue.height
  const cw = containerRef.value.clientWidth
  const ch = containerRef.value.clientHeight
  if (w <= 0 || h <= 0 || cw <= 0 || ch <= 0) {
    worldContainer.scale.set(1)
    worldContainer.x = 0
    worldContainer.y = 0
    return
  }
  const scale = clampScale(Math.min(cw / w, ch / h) * 0.9)
  worldContainer.scale.set(scale)
  worldContainer.x = (cw - w * scale) / 2
  worldContainer.y = (ch - h * scale) / 2
  if (app) rerenderForScale()
}

// ----------------------------------------------------------------------------
// rendering
// ----------------------------------------------------------------------------
function renderAll() {
  renderGrid()
  renderShapes()
  renderOverlay()
}

function renderGrid() {
  if (!gridLayer) return
  gridLayer.removeChildren()

  const w = props.modelValue.width
  const h = props.modelValue.height
  if (w <= 0 || h <= 0) return

  const bg = new Graphics()
  bg.rect(0, 0, w, h)
  bg.fill({ color: 0xffffff })
  gridLayer.addChild(bg)

  const step = props.cellSize && props.cellSize > 0 ? props.cellSize : DEFAULT_GRID
  if (step > 0) {
    const cols = w / step
    const rows = h / step
    if (cols + rows <= 4000) {
      const grid = new Graphics()
      for (let x = 0; x <= w + 0.001; x += step) {
        grid.moveTo(x, 0)
        grid.lineTo(x, h)
      }
      for (let y = 0; y <= h + 0.001; y += step) {
        grid.moveTo(0, y)
        grid.lineTo(w, y)
      }
      const px = worldContainer ? 1 / Math.max(worldContainer.scale.x, MIN_SCALE) : 1
      grid.stroke({ color: 0xe4e4e7, width: px, alpha: 0.8 })
      gridLayer.addChild(grid)
    }
  }

  const boundary = new Graphics()
  boundary.rect(0, 0, w, h)
  const bpx = worldContainer ? 1.5 / Math.max(worldContainer.scale.x, MIN_SCALE) : 1.5
  boundary.stroke({ color: props.modelValue.torus ? 0x6366f1 : 0xa1a1aa, width: bpx })
  gridLayer.addChild(boundary)
}

// Render shapes from props.
function renderShapes() {
  drawShapes(null)
}

// Render shapes from props, but substitute `editGeometry` for the selected shape
// (live drag preview, NOT committed to the store yet).
function renderShapesWithPreview() {
  drawShapes(editGeometry)
}

function drawShapes(previewGeometry: ShapeDTO['geometry'] | null) {
  if (!shapesLayer) return
  shapesRenderToken++
  shapesLayer.removeChildren()

  const shapes = props.modelValue.shapes ?? []
  const ordered = shapes
    .map((shape, index) => ({ shape, index }))
    .sort((a, b) => (a.shape.layer ?? 0) - (b.shape.layer ?? 0) || a.index - b.index)

  for (const { shape, index } of ordered) {
    if (shape.visible === false) continue
    // substitute the live-edited geometry for the selected shape
    const eff =
      previewGeometry && shape.uid === selectedUid.value ? { ...shape, geometry: previewGeometry } : shape
    if (isStrategyPlaced(eff)) {
      drawGhost(eff, index)
    } else {
      drawFixed(eff, index)
    }
  }
}

function drawFixed(shape: ShapeDTO, index: number) {
  if (!shapesLayer) return
  const colorHex = shapeColorHex(shape, index)
  const color = hexToNumber(colorHex)
  const geom = shape.geometry

  // Display override (image sprite / drawn figure). Image is drawn over the
  // (still hit-testable) underlying geometry; figure REPLACES the geometry body.
  if (displayIsImage(shape.display)) {
    drawShapeImage(shape)
    if (shape.locked) addLockBadge(shape)
    return
  }
  if (displayIsFigure(shape.display)) {
    drawShapeFigure(shape, color)
    if (shape.locked) addLockBadge(shape)
    return
  }

  const g = new Graphics()
  const strokeWidth = geom?.stroke_width ?? geom?.thickness ?? 1
  const doFill = geom?.fill !== false
  const doStroke = geom?.stroke !== false

  switch (shape.type) {
    case 'rectangle': {
      const pos = geom?.position ?? { x: 0, y: 0 }
      const w = scalarValue(geom?.width)
      const h = scalarValue(geom?.height)
      g.rect(pos.x, pos.y, w, h)
      break
    }
    case 'circle': {
      const pos = geom?.position ?? { x: 0, y: 0 }
      const r = scalarValue(geom?.radius)
      g.circle(pos.x, pos.y, r)
      break
    }
    case 'line': {
      const pts = geom?.points ?? []
      if (pts.length >= 2) {
        g.moveTo(pts[0]!.x, pts[0]!.y)
        for (let i = 1; i < pts.length; i++) g.lineTo(pts[i]!.x, pts[i]!.y)
      }
      break
    }
    case 'polygon': {
      const pts = geom?.points ?? []
      if (pts.length >= 3) {
        g.poly(pts.map((p) => ({ x: p.x, y: p.y })))
      }
      break
    }
  }

  if (doFill && shape.type !== 'line') {
    g.fill({ color, alpha: shape.locked ? 0.18 : 0.3 })
  }
  if (doStroke || shape.type === 'line') {
    const sw = shape.type === 'line' ? geom?.thickness ?? strokeWidth : strokeWidth
    g.stroke({ color: shape.locked ? 0x9ca3af : color, width: worldStroke(sw), alpha: 0.9 })
  }

  shapesLayer.addChild(g)

  if (shape.locked) addLockBadge(shape)
}

// Render an image-display shape as a Sprite scaled into the shape's bounding box,
// centered on the box. Async (texture decode); discarded if the layer was redrawn.
function drawShapeImage(shape: ShapeDTO) {
  const img = shape.display?.image
  if (!img?.data) return
  const b = geo.shapeBounds(shape)
  if (!b) return
  const token = shapesRenderToken
  const scale = shape.display?.scale && shape.display.scale > 0 ? shape.display.scale : 1
  const bw = (b.maxX - b.minX) * scale
  const bh = (b.maxY - b.minY) * scale
  const cx = (b.minX + b.maxX) / 2
  const cy = (b.minY + b.maxY) / 2
  loadTextureFromBase64(img.data, img.mime_type).then((texture) => {
    if (!shapesLayer || token !== shapesRenderToken) return
    const sprite = new Sprite(texture)
    sprite.anchor.set(0.5)
    sprite.position.set(cx, cy)
    // Fit the sprite into the geometry bounding box (fallback to texture size).
    const tw = texture.width || bw || 1
    const th = texture.height || bh || 1
    sprite.width = bw > 0 ? bw : tw
    sprite.height = bh > 0 ? bh : th
    if (shape.locked) sprite.alpha = 0.6
    shapesLayer.addChild(sprite)
  })
}

// Render a figure-display shape: the user-drawn polygon (anchor-relative to the
// shape position), filled with the shape colour, REPLACING the plain geometry.
function drawShapeFigure(shape: ShapeDTO, color: number) {
  if (!shapesLayer) return
  const figure = shape.display?.figure ?? []
  if (figure.length < 3) return
  const anchor = figureAnchor(shape)
  const scale = shape.display?.scale && shape.display.scale > 0 ? shape.display.scale : 1
  const pts = figure.map((p) => ({ x: anchor.x + p.x * scale, y: anchor.y + p.y * scale }))
  const g = new Graphics()
  g.poly(pts)
  g.fill({ color, alpha: shape.locked ? 0.18 : 0.3 })
  g.stroke({ color: shape.locked ? 0x9ca3af : color, width: worldStroke(1), alpha: 0.9 })
  shapesLayer.addChild(g)
}

function drawGhost(shape: ShapeDTO, index: number) {
  if (!shapesLayer) return
  const anchor = ghostAnchor(shape)
  const colorHex = shapeColorHex(shape, index)
  const color = hexToNumber(colorHex)
  const r = worldRadius(8)

  const marker = new Graphics()
  marker.circle(anchor.x, anchor.y, r)
  marker.fill({ color, alpha: 0.18 })
  marker.stroke({ color, width: worldStroke(1.5), alpha: 0.6 })
  marker.circle(anchor.x, anchor.y, r * 0.3)
  marker.fill({ color, alpha: 0.5 })
  shapesLayer.addChild(marker)

  addLabel(`${shape.name || shape.type} (${shape.initialization?.placement?.type})`, anchor.x, anchor.y - r, color, true)

  if (shape.locked) addLockBadge(shape)
}

function addLockBadge(shape: ShapeDTO) {
  if (!shapesLayer) return
  const anchor = isFixed(shape) ? shape.geometry?.position ?? ghostAnchor(shape) : ghostAnchor(shape)
  const size = worldRadius(7)
  const badge = new Graphics()
  badge.roundRect(anchor.x - size / 2, anchor.y - size / 2, size, size, size * 0.25)
  badge.fill({ color: 0xfde68a, alpha: 0.95 })
  badge.stroke({ color: 0xf59e0b, width: worldStroke(1) })
  shapesLayer.addChild(badge)
}

function addLabel(text: string, x: number, y: number, color: number, ghost: boolean) {
  if (!shapesLayer) return
  const scale = worldContainer ? 1 / Math.max(worldContainer.scale.x, MIN_SCALE) : 1
  const label = new Text({
    text,
    style: { fontSize: 12, fill: ghost ? 0x52525b : color, fontFamily: 'system-ui, sans-serif' },
  })
  label.scale.set(scale)
  label.x = x
  label.y = y - 14 * scale
  label.alpha = ghost ? 0.85 : 1
  shapesLayer.addChild(label)
}

function worldStroke(px: number): number {
  if (!worldContainer) return px
  return Math.max(px, px / Math.max(worldContainer.scale.x, MIN_SCALE))
}
function worldRadius(px: number): number {
  if (!worldContainer) return px
  return px / Math.max(worldContainer.scale.x, MIN_SCALE)
}

// ----------------------------------------------------------------------------
// overlay: selection outline + handles + draw preview (re-rendered live; no emit)
// ----------------------------------------------------------------------------
function renderOverlay() {
  if (!overlayLayer) return
  overlayLayer.removeChildren()
  drawSelectionOverlay()
  drawInProgressPreview()
}

function drawSelectionOverlay() {
  if (!overlayLayer) return
  const s = selectedShape.value
  if (!s) return
  // use the live edit geometry while dragging, else the committed geometry
  const eff = (dragMode === 'move' || dragMode === 'resize') && editGeometry ? { ...s, geometry: editGeometry } : s

  const sw = worldStroke(1.5)
  const accent = props.locked || s.locked ? 0xf59e0b : 0x6366f1

  // selection outline = bounds box (for all kinds, as a guide)
  const b = geo.shapeBounds(eff)
  if (b) {
    const outline = new Graphics()
    outline.rect(b.minX, b.minY, b.maxX - b.minX, b.maxY - b.minY)
    outline.stroke({ color: accent, width: sw, alpha: 0.9 })
    overlayLayer.addChild(outline)
  }

  // handles — only when editable
  if (selectedEditable() || ((dragMode === 'move' || dragMode === 'resize') && editGeometry)) {
    const handles = geo.getHandles(eff)
    const hs = worldRadius(HANDLE_PX)
    for (const hnd of handles) {
      const hg = new Graphics()
      hg.rect(hnd.pos.x - hs, hnd.pos.y - hs, hs * 2, hs * 2)
      hg.fill({ color: 0xffffff })
      hg.stroke({ color: accent, width: worldStroke(1) })
      overlayLayer.addChild(hg)
    }
  }
}

function drawInProgressPreview() {
  if (!overlayLayer) return
  const accent = 0x6366f1
  const sw = worldStroke(1.5)

  // rectangle drag preview
  if (dragMode === 'draw-rect' && drawStart.value && drawCurrent.value) {
    const a = drawStart.value
    const c = drawCurrent.value
    const g = new Graphics()
    g.rect(Math.min(a.x, c.x), Math.min(a.y, c.y), Math.abs(c.x - a.x), Math.abs(c.y - a.y))
    g.fill({ color: accent, alpha: 0.12 })
    g.stroke({ color: accent, width: sw })
    overlayLayer.addChild(g)
  }

  // circle drag preview
  if (dragMode === 'draw-circle' && drawStart.value && drawCurrent.value) {
    const center = drawStart.value
    const r = Math.hypot(drawCurrent.value.x - center.x, drawCurrent.value.y - center.y)
    const g = new Graphics()
    g.circle(center.x, center.y, r)
    g.fill({ color: accent, alpha: 0.12 })
    g.stroke({ color: accent, width: sw })
    overlayLayer.addChild(g)
  }

  // line in-progress (after first click)
  if (tool.value === 'line' && pendingLineStart.value && polyCursor.value) {
    const g = new Graphics()
    g.moveTo(pendingLineStart.value.x, pendingLineStart.value.y)
    g.lineTo(polyCursor.value.x, polyCursor.value.y)
    g.stroke({ color: accent, width: sw })
    drawVertexDot(pendingLineStart.value, accent)
    overlayLayer.addChild(g)
  }

  // polygon in-progress
  if (tool.value === 'polygon' && polyPoints.value.length) {
    const g = new Graphics()
    const pts = polyPoints.value
    g.moveTo(pts[0]!.x, pts[0]!.y)
    for (let i = 1; i < pts.length; i++) g.lineTo(pts[i]!.x, pts[i]!.y)
    if (polyCursor.value) g.lineTo(polyCursor.value.x, polyCursor.value.y)
    g.stroke({ color: accent, width: sw })
    overlayLayer.addChild(g)
    for (const p of pts) drawVertexDot(p, accent)
  }
}

function drawVertexDot(p: PointDTO, color: number) {
  if (!overlayLayer) return
  const dot = new Graphics()
  dot.circle(p.x, p.y, worldRadius(3))
  dot.fill({ color })
  overlayLayer.addChild(dot)
}

// Re-render scale-compensated layers (grid/shapes/overlay) after pan/zoom.
function rerenderForScale() {
  renderGrid()
  if ((dragMode === 'move' || dragMode === 'resize') && editGeometry) renderShapesWithPreview()
  else renderShapes()
  renderOverlay()
}

// ----------------------------------------------------------------------------
// reactivity / lifecycle
// ----------------------------------------------------------------------------
watch(
  () => props.modelValue,
  () => {
    if (!app) return
    // a committed external change → redraw from props (drop any stale preview)
    if (dragMode === 'none') {
      renderShapes()
      renderOverlay()
    }
  },
  { deep: true }
)
watch(() => props.cellSize, () => { if (app) renderGrid() })
watch(() => props.selectedUid, () => { if (app) renderOverlay() })

onMounted(() => {
  mounted = true
  initPixi()

  useResizeObserver(containerRef, (entries) => {
    const entry = entries[0]
    if (!entry || !app) return
    const { width, height } = entry.contentRect
    if (width <= 0 || height <= 0) return
    app.renderer.resize(width, height)
    app.stage.hitArea = app.screen
    fitView()
  })
})

onUnmounted(() => {
  mounted = false
  containerRef.value?.removeEventListener('wheel', handleWheel)
  containerRef.value?.removeEventListener('dblclick', onDblClick)
  window.removeEventListener('keyup', onKeyUp)
  // I2: remove the off-window gesture-end listeners
  window.removeEventListener('pointerup', onWindowPointerUp)
  window.removeEventListener('pointercancel', onWindowPointerCancel)
  app?.destroy(true)
  app = null
  worldContainer = null
  gridLayer = null
  shapesLayer = null
  overlayLayer = null
})
</script>
