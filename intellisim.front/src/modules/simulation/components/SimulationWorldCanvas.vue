<template>
  <div
    ref="containerRef"
    class="w-full h-full relative bg-zinc-100 overflow-hidden select-none"
    tabindex="0"
    data-testid="world-canvas"
    @keydown="onKeyDown"
    @dragover="onCanvasDragOver"
    @drop="onCanvasDrop"
  >
    <canvas ref="canvasRef" class="block" :class="cursorClass" />

    <!-- Toolbar -->
    <div class="absolute top-3 left-3 flex flex-col gap-1">
      <div class="flex items-center gap-1 bg-white/90 rounded-lg shadow border border-zinc-200 p-1">
        <SelectButton
          :modelValue="tool"
          :options="toolOptions"
          optionLabel="label"
          optionValue="value"
          :allowEmpty="false"
          size="small"
          :disabled="readonly"
          @update:modelValue="(v: Tool) => v && selectTool(v)"
        >
          <template #option="{ option }">
            <i :class="option.icon" :title="option.label" />
          </template>
        </SelectButton>

        <span class="w-px h-5 bg-zinc-200 mx-0.5" />

        <Button
          :icon="'pi pi-th-large'"
          size="small"
          text
          :severity="snap ? 'primary' : 'secondary'"
          title="Snap to grid"
          :disabled="readonly"
          @click="snap = !snap"
        />

        <!-- Add shape -->
        <Button
          icon="pi pi-plus-circle"
          size="small"
          text
          severity="secondary"
          label="Shape"
          :disabled="readonly || !shapeDefs.length"
          title="Add shape"
          @click="(e: MouseEvent) => addShapeMenu?.toggle(e)"
        />
        <Menu ref="addShapeMenu" :model="shapeMenuItems" :popup="true" />

        <!-- Add agent -->
        <Button
          icon="pi pi-user-plus"
          size="small"
          text
          severity="secondary"
          label="Agent"
          :disabled="readonly || !archetypes.length"
          title="Add agent"
          @click="(e: MouseEvent) => addAgentMenu?.toggle(e)"
        />
        <Menu ref="addAgentMenu" :model="agentMenuItems" :popup="true" />

        <span class="w-px h-5 bg-zinc-200 mx-0.5" />

        <Button
          icon="pi pi-trash"
          size="small"
          text
          severity="danger"
          title="Delete selected"
          :disabled="readonly || !selection.length"
          @click="deleteSelection"
        />
      </div>

      <!-- hint -->
      <div class="bg-white/90 rounded-md shadow border border-zinc-200 px-2 py-1 text-[10px] text-zinc-500 max-w-[280px]">
        <template v-if="readonly">Read-only while generating.</template>
        <template v-else>{{ hint }}</template>
      </div>
    </div>

    <!-- View controls -->
    <div class="absolute top-3 right-3 flex flex-col gap-1 bg-white/90 rounded-lg shadow border border-zinc-200 p-1">
      <button type="button" class="w-7 h-7 flex items-center justify-center text-zinc-600 hover:bg-zinc-100 rounded" title="Zoom in" @click="zoomIn">
        <i class="pi pi-plus text-xs" />
      </button>
      <button type="button" class="w-7 h-7 flex items-center justify-center text-zinc-600 hover:bg-zinc-100 rounded" title="Zoom out" @click="zoomOut">
        <i class="pi pi-minus text-xs" />
      </button>
      <button type="button" class="w-7 h-7 flex items-center justify-center text-zinc-600 hover:bg-zinc-100 rounded" title="Fit to view" @click="fitView">
        <i class="pi pi-expand text-xs" />
      </button>
    </div>

    <!-- Empty / no-bounds hint -->
    <div
      v-if="!environment || environment.width <= 0 || environment.height <= 0"
      class="absolute inset-0 flex items-center justify-center text-xs text-zinc-400 pointer-events-none"
    >
      Set a width and height to render the world.
    </div>

    <!-- "To place" tray: manual-placement instances awaiting a position. Drag a
         chip onto the world to place it. -->
    <div
      v-if="!readonly && unplacedAgents.length"
      class="absolute right-3 top-24 w-56 max-h-[55%] flex flex-col bg-white/95 rounded-lg shadow border border-amber-300"
      data-testid="unplaced-tray"
    >
      <div class="px-2.5 py-1.5 border-b border-zinc-200">
        <p class="text-[11px] font-semibold text-amber-700">
          To place ({{ unplacedAgents.length }})
        </p>
        <p class="text-[10px] text-zinc-500 leading-snug">
          Drag each into the world. The run stays blocked until all are placed.
        </p>
      </div>
      <div class="overflow-y-auto p-1.5 space-y-1">
        <div
          v-for="a in unplacedAgents"
          :key="a.id"
          draggable="true"
          class="flex items-center gap-2 px-2 py-1 rounded-md border border-zinc-200 bg-white cursor-grab hover:border-amber-400 active:cursor-grabbing"
          :class="{ 'opacity-40': draggingAgentId === a.id }"
          data-testid="unplaced-chip"
          @dragstart="onChipDragStart(a.id, $event)"
          @dragend="draggingAgentId = null"
        >
          <span
            :style="{ backgroundColor: archetypeColors.get(a.agent_uid) ?? '#71717a' }"
            class="w-2.5 h-2.5 rounded-full flex-shrink-0"
          />
          <span class="text-[11px] text-zinc-700 truncate">{{ a.name || a.agent_uid }}</span>
        </div>
      </div>
    </div>

    <!-- Legend: archetype → color -->
    <div
      v-if="legend.length"
      class="absolute bottom-3 left-3 bg-white/90 rounded-lg shadow border border-zinc-200 p-2 text-xs max-w-[60%]"
    >
      <div v-for="item in legend" :key="item.uid" class="flex items-center gap-2 mb-1 last:mb-0">
        <span :style="{ backgroundColor: item.color }" class="w-3 h-3 rounded-full flex-shrink-0" />
        <span class="text-zinc-600 truncate">{{ item.name }}</span>
      </div>
    </div>

    <!-- Hover tooltip for agents -->
    <div
      v-if="hoverTip"
      class="absolute pointer-events-none bg-zinc-900/90 text-white text-[10px] rounded px-1.5 py-0.5 shadow"
      :style="{ left: hoverTip.x + 12 + 'px', top: hoverTip.y + 12 + 'px' }"
    >
      {{ hoverTip.text }}
    </div>
  </div>
</template>

<script setup lang="ts">
// PixiJS world canvas for the simulation editor (Task 16). Built on the model
// EnvironmentCanvas architecture (pannable/zoomable world container; grid /
// shapes / agents / overlay layers; pan/zoom/fit; snap-to-grid; selection +
// handles; immutable mutation pushed UP via emit).
//
// Unlike EnvironmentCanvas (which authors model DEFINITIONS) this edits the
// simulation's CONCRETE instances:
//   - SimulationShape (absolute geometry) — move / resize / vertex-edit / add /
//     delete
//   - SimulationAgent (a positioned point) — drag / add (click-to-place) / delete
// Multi-select (shift-click + marquee) with group drag. Every committed mutation
// emits a NEW agents and/or environment slice upward (ONE source of truth — the
// editor's store; this component never keeps a divergent copy).
//
// Pure geometry / hit-testing / mutation / color lives in
// ./worldCanvasInteractions (which itself reuses the model
// environmentCanvasGeometry primitives via a SimulationShape→ShapeDTO adapter).
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useResizeObserver } from '@vueuse/core'
import { Application, Container, Graphics, Sprite } from 'pixi.js'
import type { FederatedPointerEvent } from 'pixi.js'
import Button from 'primevue/button'
import SelectButton from 'primevue/selectbutton'
import Menu from 'primevue/menu'
import type { MenuItem } from 'primevue/menuitem'
import type {
  SimulationEnvironment,
  SimulationShape,
  SimulationAgent,
  ModelSnapshot,
  PointXY,
} from '@/shared/types'
import * as wc from './worldCanvasInteractions'
import type { SelRef } from './worldCanvasInteractions'
import {
  resolveArchetypeVisual,
  displayIsImage,
  displayIsFigure,
  drawFootprintInto,
  drawFigureInto,
  loadTextureFromBase64,
  footprintRadiusHint,
  orderShapesForDraw,
  drawShapeGeometryInto,
} from './displayRender'

const props = withDefaults(
  defineProps<{
    environment: SimulationEnvironment | null
    agents: SimulationAgent[]
    snapshot: ModelSnapshot | null
    readonly?: boolean
  }>(),
  { readonly: false }
)

const emit = defineEmits<{
  'update:environment': [v: SimulationEnvironment]
  'update:agents': [v: SimulationAgent[]]
  'selection-change': [v: SelRef[]]
}>()

const containerRef = ref<HTMLElement | null>(null)
const canvasRef = ref<HTMLCanvasElement | null>(null)
const addShapeMenu = ref<InstanceType<typeof Menu> | null>(null)
const addAgentMenu = ref<InstanceType<typeof Menu> | null>(null)

// --- constants ---
const DEFAULT_GRID = 20
const MIN_SCALE = 0.02
const MAX_SCALE = 200
const HANDLE_PX = 7

// --- tools ---
type Tool = 'select' | 'pan'
// Icon scheme matches EnvironmentCanvas: pan uses the canonical four-way move
// glyph (`pi-arrows-alt`); select uses a cursor-style arrow so the two read
// distinctly. (Pan was `pi-arrows-h`, a horizontal-only glyph ill-suited to 2D pan.)
const toolOptions: { value: Tool; label: string; icon: string }[] = [
  { value: 'select', label: 'Select / move', icon: 'pi pi-arrow-up-left' },
  { value: 'pan', label: 'Pan (drag to move the view)', icon: 'pi pi-arrows-alt' },
]
const tool = ref<Tool>('select')
const snap = ref(false)

// Place mode: after picking "Add shape <def>" / "Add agent <archetype>", the
// next canvas click drops the instance. We store the pending kind+uid.
type PlaceMode = { kind: 'shape' | 'agent'; uid: string } | null
const placeMode = ref<PlaceMode>(null)

const hint = computed(() => {
  if (placeMode.value) return 'Click on the world to place the new instance. Esc to cancel.'
  if (tool.value === 'pan') return 'Drag to pan the view.'
  return 'Click to select. Shift-click to multi-select. Drag a body to move; drag empty space to marquee. Handles resize / edit points. Delete removes selection.'
})

const cursorClass = computed(() => {
  if (props.readonly) return 'cursor-default'
  if (placeMode.value) return 'cursor-crosshair'
  if (tool.value === 'pan' || spaceDown.value) return 'cursor-grab'
  return 'cursor-default'
})

// --- pixi handles ---
let app: Application | null = null
let worldContainer: Container | null = null
let gridLayer: Container | null = null
let shapesLayer: Container | null = null
let agentsLayer: Container | null = null
let overlayLayer: Container | null = null
let mounted = false
// Bumped on every agents re-render so an async sprite texture resolving after the
// layer was cleared is discarded instead of orphan-added.
let agentsRenderToken = 0
let shapesRenderToken = 0

// --- pan state ---
let panning = false
const panStart = { x: 0, y: 0, worldX: 0, worldY: 0 }
const spaceDown = ref(false)

// --- selection state (refs into the prop arrays) ---
const selection = ref<SelRef[]>([])

// --- interaction state ---
type DragMode = 'none' | 'move' | 'resize' | 'marquee'
let dragMode: DragMode = 'none'
let moveAnchorWorld: PointXY | null = null
let editHandleId: string | null = null
// Live previews (NOT committed to the store until pointerup). Keyed by id.
let shapePreview = new Map<number, SimulationShape>()
let agentPreview = new Map<number, SimulationAgent>()
const marqueeStart = ref<PointXY | null>(null)
const marqueeCurrent = ref<PointXY | null>(null)
const placeGhost = ref<PointXY | null>(null)

// Hover tooltip (screen coords + text)
const hoverTip = ref<{ x: number; y: number; text: string } | null>(null)

// ----------------------------------------------------------------------------
// derived data
// ----------------------------------------------------------------------------
const env = computed<SimulationEnvironment>(
  () => props.environment ?? { width: 0, height: 0, torus: false, shapes: [] }
)
const shapes = computed<SimulationShape[]>(() => env.value.shapes ?? [])
const agents = computed<SimulationAgent[]>(() => props.agents ?? [])
const archetypes = computed(() => props.snapshot?.agents ?? [])
const shapeDefs = computed(() => props.snapshot?.environment?.shapes ?? [])

const archetypeColors = computed(() => wc.archetypeColorMap(props.snapshot))

const legend = computed(() =>
  archetypes.value.map((a, i) => ({
    uid: a.uid,
    name: a.name || a.uid,
    color: archetypeColors.value.get(a.uid) ?? wc.paletteColor(i),
  }))
)

// Agent display radius in world units (~ a fraction of the smaller world dim).
const agentRadius = computed(() => {
  const w = env.value.width
  const h = env.value.height
  if (w <= 0 || h <= 0) return 4
  return Math.max(2, Math.round(Math.min(w, h) / 60))
})

// ----------------------------------------------------------------------------
// add menus
// ----------------------------------------------------------------------------
const shapeMenuItems = computed<MenuItem[]>(() =>
  shapeDefs.value.map((d) => ({
    label: `${d.name || d.uid} (${d.type})`,
    command: () => enterPlace('shape', d.uid),
  }))
)
const agentMenuItems = computed<MenuItem[]>(() =>
  archetypes.value.map((a) => ({
    label: a.name || a.uid,
    command: () => enterPlace('agent', a.uid),
  }))
)

function enterPlace(kind: 'shape' | 'agent', uid: string) {
  if (props.readonly) return
  tool.value = 'select'
  placeMode.value = { kind, uid }
  placeGhost.value = null
}

// ----------------------------------------------------------------------------
// emit helpers (immutable; ONE source of truth — props)
// ----------------------------------------------------------------------------
function emitShapes(next: SimulationShape[]) {
  emit('update:environment', { ...env.value, shapes: next })
}
function emitAgents(next: SimulationAgent[]) {
  emit('update:agents', next)
}

function replaceShape(id: number, updated: SimulationShape) {
  emitShapes(shapes.value.map((s) => (s.id === id ? updated : s)))
}

function setSelection(next: SelRef[]) {
  selection.value = next
  emit('selection-change', next)
  renderOverlay()
}

// ----------------------------------------------------------------------------
// place / delete
// ----------------------------------------------------------------------------
function placeAt(world: PointXY) {
  const pm = placeMode.value
  if (!pm) return
  const at = wc.quantizePoint(world, env.value, gridStep(), snap.value)
  if (pm.kind === 'shape') {
    const def = shapeDefs.value.find((d) => d.uid === pm.uid)
    if (!def) return
    const shape = wc.makeShapeFromDef(def, at, env.value, shapes.value)
    emitShapes([...shapes.value, shape])
    setSelection([{ kind: 'shape', id: shape.id }])
  } else {
    const arch = archetypes.value.find((a) => a.uid === pm.uid)
    if (!arch) return
    const agent = wc.makeAgentFromArchetype(arch, at, agents.value)
    emitAgents([...agents.value, agent])
    setSelection([{ kind: 'agent', id: agent.id }])
  }
  placeMode.value = null
  placeGhost.value = null
}

function deleteSelection() {
  if (props.readonly || !selection.value.length) return
  const shapeIds = new Set(selection.value.filter((r) => r.kind === 'shape').map((r) => r.id))
  const agentIds = new Set(selection.value.filter((r) => r.kind === 'agent').map((r) => r.id))
  if (shapeIds.size) emitShapes(shapes.value.filter((s) => !shapeIds.has(s.id)))
  if (agentIds.size) emitAgents(agents.value.filter((a) => !agentIds.has(a.id)))
  setSelection([])
}

// ----------------------------------------------------------------------------
// manual placement: unplaced instances (nil position) are placed by dragging a
// tray chip onto the world. A null-position agent uniquely means "manual
// placement, awaiting a position" (every other placement type is resolved by
// the generator).
// ----------------------------------------------------------------------------
const unplacedAgents = computed<SimulationAgent[]>(() => agents.value.filter((a) => !a.position))
const draggingAgentId = ref<number | null>(null)

function onChipDragStart(id: number, e: DragEvent) {
  if (props.readonly) return
  draggingAgentId.value = id
  // Firefox requires data to be set for the drag to start.
  e.dataTransfer?.setData('text/plain', String(id))
  if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
}

function onCanvasDragOver(e: DragEvent) {
  if (props.readonly || draggingAgentId.value == null) return
  e.preventDefault() // allow drop
  if (e.dataTransfer) e.dataTransfer.dropEffect = 'move'
}

function onCanvasDrop(e: DragEvent) {
  const id = draggingAgentId.value
  draggingAgentId.value = null
  if (props.readonly || id == null || !containerRef.value) return
  e.preventDefault()
  const rect = containerRef.value.getBoundingClientRect()
  const world = screenToWorld(e.clientX - rect.left, e.clientY - rect.top)
  const snapped = wc.quantizePoint(world, env.value, gridStep(), snap.value)
  const pos = wc.clampAgentPos(snapped, env.value)
  emitAgents(agents.value.map((a) => (a.id === id ? { ...a, position: pos } : a)))
  setSelection([{ kind: 'agent', id }])
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
function worldPerPx(): number {
  if (!worldContainer) return 1
  return 1 / Math.max(worldContainer.scale.x, MIN_SCALE)
}
function gridStep(): number {
  // The new snapshot/environment carries no cell size; use the default grid.
  return DEFAULT_GRID
}

// ----------------------------------------------------------------------------
// selection helpers
// ----------------------------------------------------------------------------
function selectedShape(): SimulationShape | null {
  if (selection.value.length !== 1 || selection.value[0]!.kind !== 'shape') return null
  return shapes.value.find((s) => s.id === selection.value[0]!.id) ?? null
}

function effShape(s: SimulationShape): SimulationShape {
  return shapePreview.get(s.id) ?? s
}
function effAgent(a: SimulationAgent): SimulationAgent {
  return agentPreview.get(a.id) ?? a
}

function selectTool(t: Tool) {
  if (props.readonly) return
  placeMode.value = null
  placeGhost.value = null
  tool.value = t
  renderOverlay()
}

// ----------------------------------------------------------------------------
// pointer routing
// ----------------------------------------------------------------------------
function isPanGesture(e: FederatedPointerEvent): boolean {
  const btn = (e.nativeEvent as PointerEvent | undefined)?.button
  return tool.value === 'pan' || spaceDown.value || btn === 1
}

function onStagePointerDown(e: FederatedPointerEvent) {
  if (!worldContainer) return
  containerRef.value?.focus()

  if (isPanGesture(e)) {
    panning = true
    panStart.x = e.globalX
    panStart.y = e.globalY
    panStart.worldX = worldContainer.x
    panStart.worldY = worldContainer.y
    return
  }
  if (props.readonly) return

  const w = screenToWorld(e.globalX, e.globalY)

  // place-mode: drop the pending instance
  if (placeMode.value) {
    placeAt(w)
    return
  }

  const shiftKey = (e.nativeEvent as PointerEvent | undefined)?.shiftKey ?? false
  startSelectGesture(w, shiftKey)
}

function onStagePointerMove(e: FederatedPointerEvent) {
  if (panning && worldContainer) {
    worldContainer.x = panStart.worldX + (e.globalX - panStart.x)
    worldContainer.y = panStart.worldY + (e.globalY - panStart.y)
    rerenderForScale()
    return
  }

  const w = screenToWorld(e.globalX, e.globalY)

  // place ghost
  if (placeMode.value) {
    placeGhost.value = wc.quantizePoint(w, env.value, gridStep(), snap.value)
    renderOverlay()
    return
  }
  if (props.readonly) {
    updateHover(e, w)
    return
  }

  if (dragMode === 'marquee') {
    marqueeCurrent.value = w
    renderOverlay()
    return
  }
  if (dragMode === 'move' && moveAnchorWorld) {
    const dx = w.x - moveAnchorWorld.x
    const dy = w.y - moveAnchorWorld.y
    buildMovePreview(dx, dy)
    renderShapes()
    renderAgents()
    renderOverlay()
    return
  }
  if (dragMode === 'resize' && editHandleId) {
    const s = selectedShape()
    if (s) {
      shapePreview.set(s.id, wc.resizeShape(s, editHandleId, w, env.value, gridStep(), snap.value))
      renderShapes()
      renderOverlay()
    }
    return
  }

  updateHover(e, w)
}

function endGesture(): boolean {
  if (panning) {
    panning = false
    return true
  }
  if (props.readonly) return false

  if (dragMode === 'marquee') {
    commitMarquee()
    dragMode = 'none'
    marqueeStart.value = null
    marqueeCurrent.value = null
    renderOverlay()
    return true
  }
  if (dragMode === 'move') {
    commitMovePreview()
    dragMode = 'none'
    moveAnchorWorld = null
    clearPreviews()
    renderShapes()
    renderAgents()
    renderOverlay()
    return true
  }
  if (dragMode === 'resize') {
    const s = selectedShape()
    const preview = s ? shapePreview.get(s.id) : null
    if (s && preview && JSON.stringify(s) !== JSON.stringify(preview)) {
      replaceShape(s.id, preview)
    }
    dragMode = 'none'
    editHandleId = null
    clearPreviews()
    renderShapes()
    renderOverlay()
    return true
  }
  return false
}

function onStagePointerUp(_e: FederatedPointerEvent) {
  endGesture()
}
function onWindowPointerUp() {
  endGesture()
}
function onWindowPointerCancel() {
  panning = false
  if (dragMode !== 'none') {
    dragMode = 'none'
    moveAnchorWorld = null
    editHandleId = null
    marqueeStart.value = null
    marqueeCurrent.value = null
    clearPreviews()
    renderShapes()
    renderAgents()
    renderOverlay()
  }
}

// --- select gesture: handle → resize, body → move (group), empty → marquee ---
function startSelectGesture(world: PointXY, shiftKey: boolean) {
  // 1) single-shape selected & editable → test its handles first
  const sel = selectedShape()
  if (sel && !shiftKey) {
    const handles = wc.getHandles(sel)
    const hit = wc.hitHandle(handles, world, HANDLE_PX * worldPerPx())
    if (hit) {
      dragMode = 'resize'
      editHandleId = hit.id
      shapePreview.set(sel.id, sel)
      return
    }
  }

  // 2) hit-test topmost item
  const hit = wc.hitTest(
    shapes.value,
    agents.value,
    world,
    HANDLE_PX * worldPerPx(),
    agentRadius.value + HANDLE_PX * worldPerPx()
  )

  if (!hit) {
    if (!shiftKey) setSelection([])
    // begin a marquee on empty space
    dragMode = 'marquee'
    marqueeStart.value = world
    marqueeCurrent.value = world
    return
  }

  // 3) (de)select
  if (shiftKey) {
    if (wc.selectionHas(selection.value, hit)) {
      setSelection(selection.value.filter((r) => !wc.sameRef(r, hit)))
      return
    }
    setSelection([...selection.value, hit])
  } else if (!wc.selectionHas(selection.value, hit)) {
    setSelection([hit])
  }

  // 4) start a group move on the (now-)selected set
  dragMode = 'move'
  moveAnchorWorld = world
}

// Build per-id previews translating the WHOLE selected set by (dx,dy).
function buildMovePreview(dx: number, dy: number) {
  shapePreview = new Map()
  agentPreview = new Map()
  const step = gridStep()
  for (const r of selection.value) {
    if (r.kind === 'shape') {
      const s = shapes.value.find((x) => x.id === r.id)
      if (s) shapePreview.set(s.id, wc.translateShape(s, dx, dy, env.value, step, snap.value))
    } else {
      const a = agents.value.find((x) => x.id === r.id)
      if (a) agentPreview.set(a.id, wc.translateAgent(a, dx, dy, env.value, step, snap.value))
    }
  }
}

function commitMovePreview() {
  if (!shapePreview.size && !agentPreview.size) return
  let changed = false
  if (shapePreview.size) {
    const next = shapes.value.map((s) => {
      const p = shapePreview.get(s.id)
      if (p && JSON.stringify(p) !== JSON.stringify(s)) {
        changed = true
        return p
      }
      return s
    })
    if (changed) emitShapes(next)
  }
  let aChanged = false
  if (agentPreview.size) {
    const next = agents.value.map((a) => {
      const p = agentPreview.get(a.id)
      if (p && JSON.stringify(p) !== JSON.stringify(a)) {
        aChanged = true
        return p
      }
      return a
    })
    if (aChanged) emitAgents(next)
  }
}

function commitMarquee() {
  if (!marqueeStart.value || !marqueeCurrent.value) return
  const rect = wc.normalizeRect(marqueeStart.value, marqueeCurrent.value)
  // ignore a near-zero marquee (treated as a plain empty-click already handled)
  if (Math.abs(rect.maxX - rect.minX) < 1 && Math.abs(rect.maxY - rect.minY) < 1) return
  setSelection(wc.marqueeHits(shapes.value, agents.value, rect))
}

function clearPreviews() {
  shapePreview = new Map()
  agentPreview = new Map()
}

// ----------------------------------------------------------------------------
// hover tooltip (agents)
// ----------------------------------------------------------------------------
function updateHover(e: FederatedPointerEvent, w: PointXY) {
  const a = wc.hitTestAgent(agents.value, w, agentRadius.value)
  if (a) {
    hoverTip.value = {
      x: e.globalX,
      y: e.globalY,
      text: `${a.name || a.agent_uid}${a.state ? ` · ${a.state}` : ''}`,
    }
  } else {
    hoverTip.value = null
  }
}

// ----------------------------------------------------------------------------
// keyboard
// ----------------------------------------------------------------------------
function onKeyDown(e: KeyboardEvent) {
  if (e.key === ' ') {
    spaceDown.value = true
    return
  }
  if (props.readonly) return
  if (e.key === 'Escape') {
    placeMode.value = null
    placeGhost.value = null
    renderOverlay()
    return
  }
  if ((e.key === 'Delete' || e.key === 'Backspace') && selection.value.length) {
    e.preventDefault()
    deleteSelection()
  }
}
function onKeyUp(e: KeyboardEvent) {
  if (e.key === ' ') spaceDown.value = false
}

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
  agentsLayer = new Container()
  overlayLayer = new Container()
  worldContainer.addChild(gridLayer, shapesLayer, agentsLayer, overlayLayer)
  app.stage.addChild(worldContainer)

  app.stage.eventMode = 'static'
  app.stage.hitArea = app.screen
  app.stage.on('pointerdown', onStagePointerDown)
  app.stage.on('pointermove', onStagePointerMove)
  app.stage.on('pointerup', onStagePointerUp)
  app.stage.on('pointerupoutside', onStagePointerUp)

  containerRef.value.addEventListener('wheel', handleWheel, { passive: false })
  window.addEventListener('keyup', onKeyUp)
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
  const w = env.value.width
  const h = env.value.height
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
  renderAgents()
  renderOverlay()
}

function worldStroke(px: number): number {
  if (!worldContainer) return px
  return Math.max(px, px / Math.max(worldContainer.scale.x, MIN_SCALE))
}
function worldUnits(px: number): number {
  if (!worldContainer) return px
  return px / Math.max(worldContainer.scale.x, MIN_SCALE)
}

function renderGrid() {
  if (!gridLayer) return
  gridLayer.removeChildren()
  const w = env.value.width
  const h = env.value.height
  if (w <= 0 || h <= 0) return

  const bg = new Graphics()
  bg.rect(0, 0, w, h)
  bg.fill({ color: 0xffffff })
  gridLayer.addChild(bg)

  const step = gridStep()
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
      grid.stroke({ color: 0xe4e4e7, width: worldUnits(1), alpha: 0.8 })
      gridLayer.addChild(grid)
    }
  }

  const boundary = new Graphics()
  boundary.rect(0, 0, w, h)
  boundary.stroke({ color: env.value.torus ? 0x6366f1 : 0xa1a1aa, width: worldStroke(1.5) })
  gridLayer.addChild(boundary)
}

function renderShapes() {
  if (!shapesLayer) return
  shapesRenderToken++
  shapesLayer.removeChildren()
  const ordered = orderShapesForDraw(shapes.value)
  for (const { shape, index } of ordered) {
    const s = effShape(shape)
    if (s.visible === false) continue
    drawShape(s, index)
  }
}

function drawShape(shape: SimulationShape, index: number) {
  if (!shapesLayer) return
  const color = wc.hexToNumber(wc.shapeColor(shape, props.snapshot, index))

  // Display override: image sprite over geometry, or figure replacing it.
  if (displayIsImage(shape.display) && shape.display!.image?.data) {
    drawShapeImage(shape, shape.display!.image.data, shape.display!.image.mime_type, shape.display!.scale)
    return
  }
  if (displayIsFigure(shape.display)) {
    const g = new Graphics()
    if (drawFigureInto(g, shape.position, shape.display!.figure, shape.display!.scale)) {
      g.fill({ color, alpha: 0.3 })
      g.stroke({ color, width: worldStroke(1), alpha: 0.9 })
      shapesLayer.addChild(g)
      return
    }
  }

  const g = new Graphics()
  drawShapeGeometryInto(g, shape, color, worldStroke)
  shapesLayer.addChild(g)
}

// Draw a shape image sprite fit into its bounding box. Async; discarded on redraw.
function drawShapeImage(
  shape: SimulationShape,
  data: string,
  mime: string | undefined,
  scale: number | undefined
) {
  const b = wc.shapeBounds(shape)
  if (!b) return
  const token = shapesRenderToken
  const s = scale && scale > 0 ? scale : 1
  const bw = (b.maxX - b.minX) * s
  const bh = (b.maxY - b.minY) * s
  const cx = (b.minX + b.maxX) / 2
  const cy = (b.minY + b.maxY) / 2
  loadTextureFromBase64(data, mime).then((texture) => {
    if (!shapesLayer || token !== shapesRenderToken) return
    const sprite = new Sprite(texture)
    sprite.anchor.set(0.5)
    sprite.position.set(cx, cy)
    sprite.width = bw > 0 ? bw : texture.width || 1
    sprite.height = bh > 0 ? bh : texture.height || 1
    shapesLayer.addChild(sprite)
  })
}

function renderAgents() {
  if (!agentsLayer) return
  agentsRenderToken++
  agentsLayer.removeChildren()
  const r = agentRadius.value
  const colorIndex = new Map<string, number>()
  archetypes.value.forEach((a, i) => colorIndex.set(a.uid, i))
  for (const raw of agents.value) {
    const a = effAgent(raw)
    if (!a.position) continue
    const idx = colorIndex.get(a.agent_uid) ?? 0
    const hex = archetypeColors.value.get(a.agent_uid) ?? wc.paletteColor(idx)
    const color = wc.hexToNumber(hex)
    const { footprint, display } = resolveArchetypeVisual(props.snapshot, a.agent_uid)

    // image sprite (centered on the agent position, sized from footprint bounds)
    if (displayIsImage(display) && display!.image?.data) {
      drawAgentImage(a.position, display!.image.data, display!.image.mime_type, footprint, display!.scale)
      continue
    }

    const g = new Graphics()
    let drewBody = false
    if (displayIsFigure(display)) {
      drewBody = drawFigureInto(g, a.position, display!.figure, display!.scale)
    }
    if (!drewBody) {
      drewBody = drawFootprintInto(g, a.position, footprint, r)
    }
    if (!drewBody) {
      // point / auto → legacy small dot
      g.circle(a.position.x, a.position.y, r)
    }
    g.fill({ color, alpha: 0.9 })
    g.stroke({ color: 0xffffff, width: worldStroke(0.5), alpha: 0.6 })
    agentsLayer.addChild(g)
  }
}

// Draw an agent image sprite centered on `pos`, sized from the footprint bounds
// (fallback to a default square). Async; discarded if the layer was redrawn.
function drawAgentImage(
  pos: PointXY,
  data: string,
  mime: string | undefined,
  footprint: ReturnType<typeof resolveArchetypeVisual>['footprint'],
  scale: number | undefined
) {
  const token = agentsRenderToken
  const r = footprintRadiusHint(footprint, agentRadius.value)
  const s = scale && scale > 0 ? scale : 1
  loadTextureFromBase64(data, mime).then((texture) => {
    if (!agentsLayer || token !== agentsRenderToken) return
    const sprite = new Sprite(texture)
    sprite.anchor.set(0.5)
    sprite.position.set(pos.x, pos.y)
    sprite.width = r * 2 * s
    sprite.height = r * 2 * s
    agentsLayer.addChild(sprite)
  })
}

// ----------------------------------------------------------------------------
// overlay: selection outlines + handles + marquee + place ghost
// ----------------------------------------------------------------------------
function renderOverlay() {
  if (!overlayLayer) return
  overlayLayer.removeChildren()
  drawSelectionOverlay()
  drawMarquee()
  drawPlaceGhost()
}

function drawSelectionOverlay() {
  if (!overlayLayer) return
  const accent = 0x6366f1
  const sw = worldStroke(1.5)
  const single = selection.value.length === 1

  for (const r of selection.value) {
    if (r.kind === 'shape') {
      const raw = shapes.value.find((s) => s.id === r.id)
      if (!raw) continue
      const s = effShape(raw)
      const b = wc.shapeBounds(s)
      if (b) {
        const outline = new Graphics()
        outline.rect(b.minX, b.minY, b.maxX - b.minX, b.maxY - b.minY)
        outline.stroke({ color: accent, width: sw, alpha: 0.9 })
        overlayLayer.addChild(outline)
      }
      // handles only for a single selected shape (and not while readonly)
      if (single && !props.readonly) {
        const hs = worldUnits(HANDLE_PX)
        for (const h of wc.getHandles(s)) {
          const hg = new Graphics()
          hg.rect(h.pos.x - hs, h.pos.y - hs, hs * 2, hs * 2)
          hg.fill({ color: 0xffffff })
          hg.stroke({ color: accent, width: worldStroke(1) })
          overlayLayer.addChild(hg)
        }
      }
    } else {
      const raw = agents.value.find((a) => a.id === r.id)
      if (!raw) continue
      const a = effAgent(raw)
      if (!a.position) continue
      const ring = new Graphics()
      ring.circle(a.position.x, a.position.y, agentRadius.value + worldUnits(2))
      ring.stroke({ color: accent, width: sw, alpha: 0.9 })
      overlayLayer.addChild(ring)
    }
  }
}

function drawMarquee() {
  if (!overlayLayer || dragMode !== 'marquee' || !marqueeStart.value || !marqueeCurrent.value) return
  const rect = wc.normalizeRect(marqueeStart.value, marqueeCurrent.value)
  const g = new Graphics()
  g.rect(rect.minX, rect.minY, rect.maxX - rect.minX, rect.maxY - rect.minY)
  g.fill({ color: 0x6366f1, alpha: 0.1 })
  g.stroke({ color: 0x6366f1, width: worldStroke(1) })
  overlayLayer.addChild(g)
}

function drawPlaceGhost() {
  if (!overlayLayer || !placeMode.value || !placeGhost.value) return
  const p = placeGhost.value
  const g = new Graphics()
  g.circle(p.x, p.y, worldUnits(HANDLE_PX))
  g.fill({ color: 0x6366f1, alpha: 0.25 })
  g.stroke({ color: 0x6366f1, width: worldStroke(1) })
  overlayLayer.addChild(g)
}

function rerenderForScale() {
  renderGrid()
  renderShapes()
  renderAgents()
  renderOverlay()
}

// ----------------------------------------------------------------------------
// reactivity / lifecycle
// ----------------------------------------------------------------------------
watch(
  () => props.environment,
  () => {
    if (!app) return
    if (dragMode === 'none') {
      // prune selection refs that no longer exist
      pruneSelection()
      renderShapes()
      renderOverlay()
    }
  },
  { deep: true }
)
watch(
  () => props.agents,
  () => {
    if (!app) return
    if (dragMode === 'none') {
      pruneSelection()
      renderAgents()
      renderOverlay()
    }
  },
  { deep: true }
)
watch(
  () => props.readonly,
  () => {
    if (props.readonly) {
      placeMode.value = null
      placeGhost.value = null
    }
    renderOverlay()
  }
)

function pruneSelection() {
  const shapeIds = new Set(shapes.value.map((s) => s.id))
  const agentIds = new Set(agents.value.map((a) => a.id))
  const next = selection.value.filter((r) =>
    r.kind === 'shape' ? shapeIds.has(r.id) : agentIds.has(r.id)
  )
  if (next.length !== selection.value.length) {
    selection.value = next
    emit('selection-change', next)
  }
}

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
  window.removeEventListener('keyup', onKeyUp)
  window.removeEventListener('pointerup', onWindowPointerUp)
  window.removeEventListener('pointercancel', onWindowPointerCancel)
  app?.destroy(true)
  app = null
  worldContainer = null
  gridLayer = null
  shapesLayer = null
  agentsLayer = null
  overlayLayer = null
})

defineExpose({ fitView, worldToScreen, screenToWorld })
</script>
