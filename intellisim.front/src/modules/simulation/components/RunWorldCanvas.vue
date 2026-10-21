<template>
  <div ref="containerRef" class="w-full h-full relative bg-zinc-100 overflow-hidden select-none">
    <!-- Spatial: PixiJS world -->
    <template v-if="isSpatial">
      <canvas ref="canvasRef" class="block" :class="spaceDown ? 'cursor-grab' : 'cursor-default'" />

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
        World has no bounds to render.
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

      <!-- Live agent count -->
      <div class="absolute bottom-3 right-3 bg-white/90 rounded-md shadow border border-zinc-200 px-2 py-1 text-[11px] text-zinc-500">
        {{ frameAgents.length }} agent(s)
      </div>

      <!-- Agent inspector: appears when a sprite is clicked -->
      <AgentInspector
        :simulationId="simulationId"
        :agent="selectedAgent"
        :memory="selectedAgentMemory"
        :agentUid="selectedAgentUid"
        :archetypeColor="selectedAgentColor"
        :runStatus="runStatus ?? null"
        :replay="replay ?? false"
        :maxStep="maxMemoryStep"
        @close="selectedAgentId = null"
      />
    </template>

    <!-- Non-spatial: compact metrics panel -->
    <div v-else class="w-full h-full overflow-auto p-4">
      <div class="grid grid-cols-2 sm:grid-cols-3 gap-3">
        <div
          v-for="metric in nonSpatialMetrics"
          :key="metric.label"
          class="rounded-lg border border-zinc-200 bg-white px-3 py-2"
        >
          <div class="text-[11px] uppercase tracking-wide text-zinc-400 truncate">{{ metric.label }}</div>
          <div class="text-lg font-semibold text-zinc-800 tabular-nums">{{ metric.value }}</div>
        </div>
      </div>
      <div v-if="!nonSpatialMetrics.length" class="text-center text-zinc-400 text-xs py-8">
        No metrics to display yet.
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// Live run world canvas (Task 24). Reuses the SimulationWorldCanvas /
// EnvironmentCanvas PixiJS architecture (pannable/zoomable world container with
// grid + agents layers, pan/zoom/fit) but is READ-ONLY and driven by the run
// store's live agentFrame instead of editable instances.
//
// Tweening: each live frame updates a persistent Map<id, AgentSprite>. Rather
// than snapping agents to their new positions, every sprite stores from/to
// world coordinates and a PixiJS ticker lerps them over the step interval so
// movement looks smooth. Sprites are created when an agent first appears and
// removed (with a brief fade) when it disappears from the frame.
//
// Non-spatial models have no positions to plot, so the canvas instead shows a
// compact metrics panel summarising population + global properties.
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useResizeObserver } from '@vueuse/core'
import { Application, Container, Graphics, Sprite, Ticker } from 'pixi.js'
import type { SimulationEnvironment, SimulationShape, ModelSnapshot, SimulationAgent, MemoryEntry, SimulationStatus } from '@/shared/types'
import type { AgentFrameState } from '@/modules/simulation/stores/runStore'
import * as wc from './worldCanvasInteractions'
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
import AgentInspector from './AgentInspector.vue'

const props = withDefaults(
  defineProps<{
    // The simulation id — forwarded to the inspector for its agent-detail fetch.
    simulationId: string
    // The live per-agent state, keyed by runtime instance id (runStore.agentFrame).
    agentFrame: Map<number, AgentFrameState>
    environment: SimulationEnvironment | null
    snapshot: ModelSnapshot | null
    // Global properties (used for the non-spatial metrics panel).
    globalProperties?: Record<string, unknown>
    // The inter-step delay the engine honours (ms) — drives the tween duration.
    stepDelayMs?: number
    // The persisted simulation agents (with memory), sourced from
    // store.simulation.agents. Memory here is a load-time snapshot; the inspector
    // fetches fresh memory live from the backend — this is only the fallback.
    simulationAgents?: SimulationAgent[]
    // The run lifecycle status — forwarded to the inspector to gate its poll.
    runStatus?: SimulationStatus | null
    // Replay mode: the run has finished and the canvas shows a scrubbed step.
    // Forwarded to the inspector so it filters memory to the displayed step and
    // skips its live poll.
    replay?: boolean
    // In replay, the currently-displayed (scrubbed) step; the inspector filters
    // the agent's memory to entries at or before this step. Undefined while live.
    maxMemoryStep?: number
  }>(),
  { stepDelayMs: 100 }
)

const containerRef = ref<HTMLElement | null>(null)
const canvasRef = ref<HTMLCanvasElement | null>(null)

// --- constants ---
const MIN_SCALE = 0.02
const MAX_SCALE = 200
// Tween duration is bounded so very fast / very slow runs still animate sanely.
const MIN_TWEEN_MS = 60
const MAX_TWEEN_MS = 1200

const spaceDown = ref(false)

// --- pixi handles ---
let app: Application | null = null
let worldContainer: Container | null = null
let gridLayer: Container | null = null
let shapesLayer: Container | null = null
let agentsLayer: Container | null = null
let mounted = false
// Bumped on every renderShapes() so async image loads from a stale pass are discarded.
let shapesRenderToken = 0

// --- pan state ---
let panning = false
const panStart = { x: 0, y: 0, worldX: 0, worldY: 0 }

// One persistent display object per live agent, tweened between frames. `g` is a
// Container so the body can be either a Graphics (dot / footprint / figure) or a
// Sprite (image display); tweening sets the container position either way.
interface AgentSprite {
  g: Container
  fromX: number
  fromY: number
  toX: number
  toY: number
  // Tween clock: 0..1 progress over `dur` ms; t>=1 means settled.
  t: number
  dur: number
  removing: boolean
  alpha: number
  // The archetype uid this sprite resolved to (for body rebuilds on repaint).
  agentUid?: string
}
const sprites = new Map<number, AgentSprite>()

// --- agent selection (inspector) ---
// selectedAgentId is set when the user clicks an agent sprite. Cleared by the
// inspector close button or a click on empty canvas space.
const selectedAgentId = ref<number | null>(null)

// The live AgentFrameState for the selected agent (drives the inspector header,
// state, position, and properties — all updated per step).
const selectedAgent = computed<AgentFrameState | null>(() =>
  selectedAgentId.value !== null ? (props.agentFrame.get(selectedAgentId.value) ?? null) : null
)

// The persisted SimulationAgent entry for the selected id (used for memory + uid).
const selectedSimAgent = computed(() => {
  if (selectedAgentId.value === null || !props.simulationAgents) return undefined
  return props.simulationAgents.find((a) => a.id === selectedAgentId.value)
})

// Memory sourced from the persisted simulation agents snapshot (best-effort).
// Not live-streamed per step; reflects state at simulation load time.
const selectedAgentMemory = computed<MemoryEntry[]>(() => selectedSimAgent.value?.memory ?? [])

// The archetype uid from the persisted snapshot (for display in the inspector).
const selectedAgentUid = computed<string | undefined>(() => selectedSimAgent.value?.agent_uid)

// Color of the selected agent's archetype (for the inspector badge).
const selectedAgentColor = computed<string | undefined>(() => {
  if (!selectedAgent.value) return undefined
  const frame = selectedAgent.value
  // Try the agent's own color property first, then archetype palette lookup by uid.
  const inst = frame.properties?.color
  if (typeof inst === 'string') return inst
  const uid = selectedAgentUid.value ?? props.snapshot?.agents?.[0]?.uid
  return uid ? archetypeColors.value.get(uid) : undefined
})

// How far (px, in canvas space) a pointerdown can travel before we treat it as
// a pan rather than a click. This prevents selecting an agent while panning.
const CLICK_THRESHOLD_PX = 5
// Screen coords where the pointer went down (for click-vs-pan discrimination).
let pointerDownX = 0
let pointerDownY = 0

// ----------------------------------------------------------------------------
// derived data
// ----------------------------------------------------------------------------
const isSpatial = computed(() => props.snapshot?.is_spatial ?? false)
const env = computed<SimulationEnvironment>(
  () => props.environment ?? { width: 0, height: 0, torus: false, shapes: [] }
)
const archetypes = computed(() => props.snapshot?.agents ?? [])
const archetypeColors = computed(() => wc.archetypeColorMap(props.snapshot))

// Map a runtime instance id → its archetype uid, sourced from the persisted
// simulation agents (the live frame carries no archetype uid). Used to resolve
// per-archetype footprint/display + colour for each sprite.
const agentUidById = computed(() => {
  const m = new Map<number, string>()
  for (const a of props.simulationAgents ?? []) m.set(a.id, a.agent_uid)
  return m
})

function archetypeUidFor(id: number): string | undefined {
  return agentUidById.value.get(id) ?? archetypes.value[0]?.uid
}

const frameAgents = computed<AgentFrameState[]>(() => Array.from(props.agentFrame.values()))

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

// Map a runtime instance id to its archetype color. We colour by the instance's
// `color` property if present, else by the resolved archetype's palette colour
// (resolved via the persisted simulation agents → snapshot), falling back to the
// first archetype.
function agentColor(a: AgentFrameState): number {
  const inst = a.properties?.color
  if (typeof inst === 'string' && /^#?[0-9a-fA-F]{3,8}$/.test(inst)) {
    return wc.hexToNumber(inst)
  }
  const uid = archetypeUidFor(a.id)
  const hex = uid ? archetypeColors.value.get(uid) : undefined
  return wc.hexToNumber(hex ?? wc.paletteColor(0))
}

// Build the body display object for an agent (footprint / figure Graphics or an
// image Sprite), positioned at the container origin (the container is tweened).
// Returns the child to add. Image sprites resolve asynchronously and swap in.
function buildAgentBody(a: AgentFrameState, container: Container) {
  const r = agentRadius.value
  const color = agentColor(a)
  const uid = archetypeUidFor(a.id)
  const { footprint, display } = resolveArchetypeVisual(props.snapshot, uid)

  if (displayIsImage(display) && display!.image?.data) {
    const rad = footprintRadiusHint(footprint, r)
    const s = display!.scale && display!.scale > 0 ? display!.scale : 1
    const token = container
    loadTextureFromBase64(display!.image.data, display!.image.mime_type).then((texture) => {
      // discard if the container was destroyed/replaced meanwhile
      if (token.destroyed) return
      const sprite = new Sprite(texture)
      sprite.anchor.set(0.5)
      sprite.width = rad * 2 * s
      sprite.height = rad * 2 * s
      container.addChild(sprite)
    })
    return
  }

  const g = new Graphics()
  let drew = false
  if (displayIsFigure(display)) {
    drew = drawFigureInto(g, { x: 0, y: 0 }, display!.figure, display!.scale)
  }
  if (!drew) drew = drawFootprintInto(g, { x: 0, y: 0 }, footprint, r)
  if (!drew) g.circle(0, 0, r)
  g.fill({ color, alpha: 0.9 })
  g.stroke({ color: 0xffffff, width: worldStroke(0.5), alpha: 0.6 })
  container.addChild(g)
}

// ----------------------------------------------------------------------------
// non-spatial metrics
// ----------------------------------------------------------------------------
const nonSpatialMetrics = computed<{ label: string; value: string }[]>(() => {
  const out: { label: string; value: string }[] = []
  out.push({ label: 'Agents', value: String(frameAgents.value.length) })
  // State distribution.
  const byState = new Map<string, number>()
  for (const a of frameAgents.value) {
    const s = a.state || '—'
    byState.set(s, (byState.get(s) ?? 0) + 1)
  }
  for (const [s, n] of byState) out.push({ label: `state: ${s}`, value: String(n) })
  // Global properties (scalars only).
  for (const [k, v] of Object.entries(props.globalProperties ?? {})) {
    if (typeof v === 'number') out.push({ label: k, value: formatNum(v) })
    else if (typeof v === 'string' || typeof v === 'boolean') out.push({ label: k, value: String(v) })
  }
  return out
})

function formatNum(n: number): string {
  return Number.isInteger(n) ? String(n) : n.toFixed(2)
}

// ----------------------------------------------------------------------------
// coordinate transforms
// ----------------------------------------------------------------------------
function worldStroke(px: number): number {
  if (!worldContainer) return px
  return Math.max(px, px / Math.max(worldContainer.scale.x, MIN_SCALE))
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
  renderGrid()
  renderShapes()
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
  renderGrid()
  renderShapes()
}

// ----------------------------------------------------------------------------
// pointer (pan + agent click)
// ----------------------------------------------------------------------------
function onPointerDown(e: PointerEvent) {
  if (!worldContainer || !containerRef.value) return
  panning = true
  const rect = containerRef.value.getBoundingClientRect()
  panStart.x = e.clientX - rect.left
  panStart.y = e.clientY - rect.top
  panStart.worldX = worldContainer.x
  panStart.worldY = worldContainer.y
  pointerDownX = e.clientX
  pointerDownY = e.clientY
}
function onPointerMove(e: PointerEvent) {
  if (!panning || !worldContainer || !containerRef.value) return
  const rect = containerRef.value.getBoundingClientRect()
  worldContainer.x = panStart.worldX + (e.clientX - rect.left - panStart.x)
  worldContainer.y = panStart.worldY + (e.clientY - rect.top - panStart.y)
  renderGrid()
}
function onPointerUp(e: PointerEvent) {
  if (!panning) return
  panning = false
  // Only treat as a click if the pointer hasn't moved more than the threshold.
  const dx = e.clientX - pointerDownX
  const dy = e.clientY - pointerDownY
  if (Math.sqrt(dx * dx + dy * dy) > CLICK_THRESHOLD_PX) return
  handleCanvasClick(e)
}

// Try to find the nearest agent sprite to the click position. If close enough,
// select it; otherwise deselect (click on empty space clears the inspector).
function handleCanvasClick(e: PointerEvent) {
  if (!worldContainer || !containerRef.value) return
  const rect = containerRef.value.getBoundingClientRect()
  const screenX = e.clientX - rect.left
  const screenY = e.clientY - rect.top
  const scale = worldContainer.scale.x
  // Convert screen coords to world coords.
  const worldX = (screenX - worldContainer.x) / scale
  const worldY = (screenY - worldContainer.y) / scale
  const hitRadius = agentRadius.value * 2.5 // generous hit area

  let closest: number | null = null
  let closestDist = hitRadius

  for (const [id, sp] of sprites) {
    if (sp.removing) continue
    const dist = Math.sqrt((sp.g.x - worldX) ** 2 + (sp.g.y - worldY) ** 2)
    if (dist < closestDist) {
      closestDist = dist
      closest = id
    }
  }

  selectedAgentId.value = closest
}
function onKeyDown(e: KeyboardEvent) {
  if (e.key === ' ') spaceDown.value = true
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
  // Shapes render above the grid but beneath agent sprites.
  worldContainer.addChild(gridLayer, shapesLayer, agentsLayer)
  app.stage.addChild(worldContainer)

  containerRef.value.addEventListener('wheel', handleWheel, { passive: false })
  containerRef.value.addEventListener('pointerdown', onPointerDown)
  window.addEventListener('pointermove', onPointerMove)
  window.addEventListener('pointerup', onPointerUp)
  window.addEventListener('keydown', onKeyDown)
  window.addEventListener('keyup', onKeyUp)

  renderGrid()
  renderShapes()
  syncSprites(true)
  fitView()

  app.ticker.add(tween)
}

// ----------------------------------------------------------------------------
// grid (static world boundary + light grid)
// ----------------------------------------------------------------------------
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

  const boundary = new Graphics()
  boundary.rect(0, 0, w, h)
  boundary.stroke({ color: env.value.torus ? 0x6366f1 : 0xa1a1aa, width: worldStroke(1.5) })
  gridLayer.addChild(boundary)
}

// ----------------------------------------------------------------------------
// environment shapes (static furniture: walls / zones / obstacles)
// ----------------------------------------------------------------------------
// Draw the environment's shapes into shapesLayer, honoring layer order, visible,
// fill/stroke/stroke_width/thickness and the display image/figure override — full
// parity with the simulation environment editor (SimulationWorldCanvas). Shapes
// are static for a run (their geometry is not streamed per step), so this is
// called only when the world is (re)built, on scale change, and when the
// environment changes — never per animation frame.
function renderShapes() {
  if (!shapesLayer) return
  shapesRenderToken++
  shapesLayer.removeChildren()
  for (const { shape, index } of orderShapesForDraw(env.value.shapes ?? [])) {
    if (shape.visible === false) continue
    drawShape(shape, index)
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

// ----------------------------------------------------------------------------
// sprite sync + tween
// ----------------------------------------------------------------------------
// Reconcile the persistent sprite map against the latest frame: create sprites
// for new agents, set the tween target for moved agents, mark vanished agents
// for fade-out removal. When `immediate` is true (initial frame), agents snap to
// their position instead of tweening in from origin.
function syncSprites(immediate = false) {
  if (!agentsLayer) return
  const seen = new Set<number>()
  const dur = clampTween(props.stepDelayMs)

  for (const a of props.agentFrame.values()) {
    if (!a.position) continue
    seen.add(a.id)
    let sp = sprites.get(a.id)
    if (!sp) {
      const g = new Container()
      buildAgentBody(a, g)
      g.x = a.position.x
      g.y = a.position.y
      agentsLayer.addChild(g)
      sp = {
        g,
        fromX: a.position.x,
        fromY: a.position.y,
        toX: a.position.x,
        toY: a.position.y,
        t: 1,
        dur,
        removing: false,
        alpha: 1,
        agentUid: archetypeUidFor(a.id),
      }
      sprites.set(a.id, sp)
    } else {
      sp.removing = false
      sp.fromX = sp.g.x
      sp.fromY = sp.g.y
      sp.toX = a.position.x
      sp.toY = a.position.y
      sp.dur = dur
      // A torus wrap can make a tween jump across the whole world; if the move
      // is more than half the world span, snap instead of sliding through it.
      if (immediate || isWrapJump(sp)) {
        sp.g.x = sp.toX
        sp.g.y = sp.toY
        sp.fromX = sp.toX
        sp.fromY = sp.toY
        sp.t = 1
      } else {
        sp.t = 0
      }
    }
  }

  // Mark vanished agents for fade-out.
  for (const [id, sp] of sprites) {
    if (!seen.has(id)) sp.removing = true
  }
}

function isWrapJump(sp: AgentSprite): boolean {
  const w = env.value.width
  const h = env.value.height
  if (!env.value.torus || w <= 0 || h <= 0) return false
  return Math.abs(sp.toX - sp.fromX) > w / 2 || Math.abs(sp.toY - sp.fromY) > h / 2
}

function clampTween(ms: number | undefined): number {
  const v = ms ?? 100
  return Math.min(MAX_TWEEN_MS, Math.max(MIN_TWEEN_MS, v))
}

// Per-frame tween step driven by the Pixi ticker. Lerps each sprite from→to and
// fades out + frees removed sprites.
function tween(ticker: Ticker) {
  const dtMs = ticker.deltaMS
  for (const [id, sp] of sprites) {
    if (sp.removing) {
      sp.alpha -= dtMs / 200
      if (sp.alpha <= 0) {
        agentsLayer?.removeChild(sp.g)
        sp.g.destroy()
        sprites.delete(id)
        continue
      }
      sp.g.alpha = sp.alpha
      continue
    }
    if (sp.t < 1) {
      sp.t = Math.min(1, sp.t + dtMs / Math.max(1, sp.dur))
      const e = easeOut(sp.t)
      sp.g.x = sp.fromX + (sp.toX - sp.fromX) * e
      sp.g.y = sp.fromY + (sp.toY - sp.fromY) * e
    }
  }
}

function easeOut(t: number): number {
  return 1 - (1 - t) * (1 - t)
}

// Rebuild all sprite bodies (after a snapshot/world change that may alter the
// resolved footprint/display/colour). Replaces each container's children.
function repaintSprites() {
  if (!agentsLayer) return
  for (const a of props.agentFrame.values()) {
    const sp = sprites.get(a.id)
    if (!sp) continue
    sp.g.removeChildren().forEach((c) => c.destroy())
    buildAgentBody(a, sp.g)
    sp.agentUid = archetypeUidFor(a.id)
  }
}

// ----------------------------------------------------------------------------
// reactivity / lifecycle
// ----------------------------------------------------------------------------
// React to every new frame: reconcile + tween. The store reassigns the Map on
// each frame, so a shallow watch on the ref is enough.
watch(
  () => props.agentFrame,
  () => {
    if (!app) return
    syncSprites()
  }
)

watch(
  () => props.snapshot,
  () => {
    if (!app) return
    repaintSprites()
  }
)

// The persisted simulation agents (id→archetype uid) may arrive after the first
// frames; rebuild bodies once they do so footprint/display resolve correctly.
watch(
  () => props.simulationAgents,
  () => {
    if (!app) return
    repaintSprites()
  }
)

watch(
  () => props.environment,
  () => {
    if (!app) return
    renderGrid()
    renderShapes()
  },
  { deep: true }
)

onMounted(() => {
  mounted = true
  if (isSpatial.value) {
    initPixi()
    useResizeObserver(containerRef, (entries) => {
      const entry = entries[0]
      if (!entry || !app) return
      const { width, height } = entry.contentRect
      if (width <= 0 || height <= 0) return
      app.renderer.resize(width, height)
      fitView()
    })
  }
})

onUnmounted(() => {
  mounted = false
  containerRef.value?.removeEventListener('wheel', handleWheel)
  containerRef.value?.removeEventListener('pointerdown', onPointerDown)
  window.removeEventListener('pointermove', onPointerMove)
  window.removeEventListener('pointerup', onPointerUp)
  window.removeEventListener('keydown', onKeyDown)
  window.removeEventListener('keyup', onKeyUp)
  if (app) {
    app.ticker.remove(tween)
    app.destroy(true)
  }
  app = null
  worldContainer = null
  gridLayer = null
  shapesLayer = null
  agentsLayer = null
  sprites.clear()
})

defineExpose({ fitView })
</script>
