// Pure interaction / geometry / hit-testing / mutation helpers for the PixiJS
// SimulationWorldCanvas (Task 16). Side-effect-free (no Pixi, no Vue, no DOM) so
// the SFC stays readable. The canvas edits CONCRETE instances:
//   - SimulationShape (absolute geometry: position + width/height | radius |
//     points; points are ANCHOR-RELATIVE — absolute = position + point)
//   - SimulationAgent (a point with an optional position)
//
// We REUSE the model-editor geometry primitives in
// `../../model/components/environmentCanvasGeometry` for the heavy lifting
// (rectangle/circle/polygon containment, segment distance, rect-corner +
// circle-radius + vertex handle math, quantize) by ADAPTING a SimulationShape
// into the ShapeDTO shape those helpers expect. We never copy that math here.
import type {
  SimulationShape,
  SimulationAgent,
  ModelSnapshot,
  SnapshotShape,
  SnapshotAgent,
  PointXY,
} from '@/shared/types'
import type { ShapeDTO } from '@/shared/types/model'
import * as geo from '@/modules/model/components/environmentCanvasGeometry'

// ---------------------------------------------------------------------------
// selection
// ---------------------------------------------------------------------------

export type SelKind = 'shape' | 'agent'
export interface SelRef {
  kind: SelKind
  id: number
}

/** Stable string key for a selection ref (set membership / comparison). */
export function refKey(r: SelRef): string {
  return `${r.kind}:${r.id}`
}

export function sameRef(a: SelRef, b: SelRef): boolean {
  return a.kind === b.kind && a.id === b.id
}

export function selectionHas(sel: SelRef[], r: SelRef): boolean {
  return sel.some((s) => sameRef(s, r))
}

// ---------------------------------------------------------------------------
// SimulationShape <-> ShapeDTO adapter (so we can reuse environmentCanvasGeometry)
// ---------------------------------------------------------------------------

/** Absolute world points of a shape (position + anchor-relative `points`). */
export function absolutePoints(shape: SimulationShape): PointXY[] {
  const pts = shape.points ?? []
  return pts.map((p) => ({ x: shape.position.x + p.x, y: shape.position.y + p.y }))
}

/**
 * Build the minimal ShapeDTO that the environmentCanvasGeometry helpers read:
 * they only ever touch `type`, `layer`, `visible` and `geometry`
 * (position / width / height / radius / points). For line/polygon we pass the
 * ABSOLUTE points (geometry helpers operate in absolute world space).
 */
export function toShapeDTO(shape: SimulationShape): ShapeDTO {
  const geometry: ShapeDTO['geometry'] = {}
  switch (shape.type) {
    case 'rectangle':
      geometry.position = { ...shape.position }
      geometry.width = { value: shape.width ?? 0 }
      geometry.height = { value: shape.height ?? 0 }
      break
    case 'circle':
      geometry.position = { ...shape.position }
      geometry.radius = { value: shape.radius ?? 0 }
      break
    case 'line':
    case 'polygon':
      geometry.points = absolutePoints(shape)
      geometry.position = { ...shape.position }
      break
    default:
      geometry.position = { ...shape.position }
  }
  return {
    uid: shape.shape_uid,
    name: shape.name,
    description: '',
    type: shape.type as ShapeDTO['type'],
    layer: shape.layer ?? 0,
    visible: shape.visible !== false,
    properties: [],
    initialization: { population_count: 1 },
    geometry,
    locked: false,
  }
}

/**
 * Apply a mutated ShapeDTO geometry (absolute) back onto a SimulationShape,
 * re-deriving the anchor-relative `points` from the new absolute points + the
 * new anchor position. Returns a NEW SimulationShape.
 */
function fromShapeGeometry(shape: SimulationShape, g: ShapeDTO['geometry']): SimulationShape {
  if (!g) return shape
  switch (shape.type) {
    case 'rectangle': {
      const pos = g.position ?? shape.position
      return {
        ...shape,
        position: { x: Math.round(pos.x), y: Math.round(pos.y) },
        width: Math.round(geo.scalarValue(g.width)),
        height: Math.round(geo.scalarValue(g.height)),
      }
    }
    case 'circle': {
      const pos = g.position ?? shape.position
      return {
        ...shape,
        position: { x: Math.round(pos.x), y: Math.round(pos.y) },
        radius: Math.round(geo.scalarValue(g.radius)),
      }
    }
    case 'line':
    case 'polygon': {
      const abs = g.points ?? absolutePoints(shape)
      const anchor = abs[0] ?? shape.position
      const rel = abs.map((p) => ({ x: Math.round(p.x - anchor.x), y: Math.round(p.y - anchor.y) }))
      return {
        ...shape,
        position: { x: Math.round(anchor.x), y: Math.round(anchor.y) },
        points: rel,
      }
    }
    default:
      return shape
  }
}

// ---------------------------------------------------------------------------
// quantize / snap (delegates to geo.quantizePoint)
// ---------------------------------------------------------------------------

export function quantizePoint(
  p: PointXY,
  env: { width: number; height: number },
  cell: number,
  snap: boolean
): PointXY {
  return geo.quantizePoint(p, env, cell, snap)
}

// ---------------------------------------------------------------------------
// hit-testing
// ---------------------------------------------------------------------------

/** Topmost shape at a world point (highest layer, then last in array order). */
export function hitTestShape(
  shapes: SimulationShape[],
  world: PointXY,
  tolerance: number
): SimulationShape | null {
  const ordered = shapes
    .map((shape, index) => ({ shape, index }))
    .sort((a, b) => (a.shape.layer ?? 0) - (b.shape.layer ?? 0) || a.index - b.index)
  for (let i = ordered.length - 1; i >= 0; i--) {
    const s = ordered[i]!.shape
    if (s.visible === false) continue
    if (geo.pointInShape(toShapeDTO(s), world, tolerance)) return s
  }
  return null
}

/** Topmost agent at a world point (within `radius` world units of its center). */
export function hitTestAgent(
  agents: SimulationAgent[],
  world: PointXY,
  radius: number
): SimulationAgent | null {
  // Later in the array = drawn on top, so iterate in reverse and return the
  // first (topmost) agent whose center is within `radius` of the point.
  for (let i = agents.length - 1; i >= 0; i--) {
    const a = agents[i]!
    if (!a.position) continue
    if (Math.hypot(a.position.x - world.x, a.position.y - world.y) <= radius) return a
  }
  return null
}

/**
 * Topmost item (agent preferred when it is under the pointer, since agents are
 * drawn above shapes) at a world point. Returns null when nothing is hit.
 */
export function hitTest(
  shapes: SimulationShape[],
  agents: SimulationAgent[],
  world: PointXY,
  shapeTol: number,
  agentRadius: number
): SelRef | null {
  const a = hitTestAgent(agents, world, agentRadius)
  if (a) return { kind: 'agent', id: a.id }
  const s = hitTestShape(shapes, world, shapeTol)
  if (s) return { kind: 'shape', id: s.id }
  return null
}

// ---------------------------------------------------------------------------
// marquee → hits
// ---------------------------------------------------------------------------

export interface Rect {
  minX: number
  minY: number
  maxX: number
  maxY: number
}

export function normalizeRect(a: PointXY, b: PointXY): Rect {
  return {
    minX: Math.min(a.x, b.x),
    minY: Math.min(a.y, b.y),
    maxX: Math.max(a.x, b.x),
    maxY: Math.max(a.y, b.y),
  }
}

function rectsOverlap(a: Rect, b: Rect): boolean {
  return a.minX <= b.maxX && a.maxX >= b.minX && a.minY <= b.maxY && a.maxY >= b.minY
}

/** Selection refs whose geometry intersects the marquee rect. */
export function marqueeHits(
  shapes: SimulationShape[],
  agents: SimulationAgent[],
  rect: Rect
): SelRef[] {
  const out: SelRef[] = []
  for (const s of shapes) {
    if (s.visible === false) continue
    const b = geo.shapeBounds(toShapeDTO(s))
    if (b && rectsOverlap(b, rect)) out.push({ kind: 'shape', id: s.id })
  }
  for (const a of agents) {
    if (!a.position) continue
    if (
      a.position.x >= rect.minX &&
      a.position.x <= rect.maxX &&
      a.position.y >= rect.minY &&
      a.position.y <= rect.maxY
    ) {
      out.push({ kind: 'agent', id: a.id })
    }
  }
  return out
}

// ---------------------------------------------------------------------------
// bounds + handles (delegates to geo)
// ---------------------------------------------------------------------------

export function shapeBounds(shape: SimulationShape): geo.Bounds | null {
  return geo.shapeBounds(toShapeDTO(shape))
}

export function getHandles(shape: SimulationShape): geo.Handle[] {
  return geo.getHandles(toShapeDTO(shape))
}

export function hitHandle(handles: geo.Handle[], pt: PointXY, radius: number): geo.Handle | null {
  return geo.hitHandle(handles, pt, radius)
}

// ---------------------------------------------------------------------------
// shape mutation: translate / resize (immutable, via geo + adapter)
// ---------------------------------------------------------------------------

/** Translate one shape by (dx,dy) world units, clamped to bounds. NEW shape. */
export function translateShape(
  shape: SimulationShape,
  dx: number,
  dy: number,
  env: { width: number; height: number },
  cell: number,
  snap: boolean
): SimulationShape {
  const g = geo.translateShape(toShapeDTO(shape), dx, dy, env, cell, snap)
  return fromShapeGeometry(shape, g)
}

/** Resize one shape by dragging handle `handleId` to world point `to`. NEW shape. */
export function resizeShape(
  shape: SimulationShape,
  handleId: string,
  to: PointXY,
  env: { width: number; height: number },
  cell: number,
  snap: boolean
): SimulationShape {
  const g = geo.resizeShape(toShapeDTO(shape), handleId, to, env, cell, snap)
  return fromShapeGeometry(shape, g)
}

/** Clamp an agent position into [0,w] x [0,h], rounded to int. NEW point. */
export function clampAgentPos(
  p: PointXY,
  env: { width: number; height: number }
): PointXY {
  return {
    x: Math.round(geo.clamp(p.x, env.width)),
    y: Math.round(geo.clamp(p.y, env.height)),
  }
}

/** Translate one agent by (dx,dy), snap-aware, clamped to bounds. NEW agent. */
export function translateAgent(
  agent: SimulationAgent,
  dx: number,
  dy: number,
  env: { width: number; height: number },
  cell: number,
  snap: boolean
): SimulationAgent {
  const base = agent.position ?? { x: 0, y: 0 }
  const snapDelta = (d: number) =>
    snap && Number.isFinite(cell) && cell > 0 ? Math.round(d / cell) * cell : Math.round(d)
  const next = clampAgentPos(
    { x: base.x + snapDelta(dx), y: base.y + snapDelta(dy) },
    env
  )
  return { ...agent, position: next }
}

// ---------------------------------------------------------------------------
// id generation (fresh = max existing id + 1 within that collection)
// ---------------------------------------------------------------------------

export function nextShapeId(shapes: SimulationShape[]): number {
  return shapes.reduce((m, s) => Math.max(m, s.id), 0) + 1
}

export function nextAgentId(agents: SimulationAgent[]): number {
  return agents.reduce((m, a) => Math.max(m, a.id), 0) + 1
}

// ---------------------------------------------------------------------------
// add operations — produce a NEW SimulationShape / SimulationAgent from a def
// ---------------------------------------------------------------------------

/** A reasonable default size for sized shapes: ~1/8 of the smaller world dim. */
function defaultSize(env: { width: number; height: number }): number {
  const base = Math.max(8, Math.round(Math.min(env.width, env.height) / 8))
  return base
}

/**
 * Build a new SimulationShape instance from a snapshot shape definition, placed
 * at world point `at`. Fixed-position defs (geometry.position present) use the
 * def position; otherwise the click point. Sized defs sample def width/height/
 * radius; polygon/line drop a small default polygon the user can then edit.
 */
export function makeShapeFromDef(
  def: SnapshotShape,
  at: PointXY,
  env: { width: number; height: number },
  shapes: SimulationShape[]
): SimulationShape {
  const id = nextShapeId(shapes)
  const g = def.geometry
  const defPos = g?.position
  // fixed-position defs keep their authored anchor; else use the click point
  const pos = defPos ? { ...defPos } : { ...at }
  // Seed the instance colour from the definition's `color`-typed property default so
  // a manually placed shape carries an explicit, editable hex (matches the live-run
  // render fallback and lets the side panel edit it per-instance).
  const defColor = colorPropDefault(def.properties)
  const base: SimulationShape = {
    id,
    shape_uid: def.uid,
    name: def.name || def.uid,
    type: def.type,
    layer: def.layer ?? 0,
    visible: def.visible !== false,
    properties: defColor ? { color: defColor } : {},
    position: { x: Math.round(pos.x), y: Math.round(pos.y) },
    fill: g?.fill,
    stroke: g?.stroke,
    stroke_width: g?.stroke_width,
    thickness: g?.thickness,
  }
  const size = defaultSize(env)
  switch (def.type) {
    case 'rectangle':
      base.width = Math.round(geo.scalarValue(g?.width) || size)
      base.height = Math.round(geo.scalarValue(g?.height) || size)
      break
    case 'circle':
      base.radius = Math.round(geo.scalarValue(g?.radius) || Math.round(size / 2))
      break
    case 'line':
      // anchor-relative two-point segment
      base.points = [
        { x: 0, y: 0 },
        { x: size, y: 0 },
      ]
      break
    case 'polygon':
      // anchor-relative small triangle
      base.points = [
        { x: 0, y: 0 },
        { x: size, y: 0 },
        { x: Math.round(size / 2), y: size },
      ]
      break
  }
  // Re-clamp the freshly-placed shape into world bounds: a zero-delta translate
  // reuses the exact same bounds-clamping math as drag-move (rect/circle clamp
  // position, line/polygon clamp the point set), so a shape dropped near an edge
  // can never spill outside [0,width] x [0,height]. snap=false → cell is unused.
  return translateShape(base, 0, 0, env, 0, false)
}

/**
 * Build a new SimulationAgent instance for an archetype, placed at world point
 * `at`. Default state = archetype.initialization.initial_state.
 */
export function makeAgentFromArchetype(
  arch: SnapshotAgent,
  at: PointXY,
  agents: SimulationAgent[]
): SimulationAgent {
  // Seed the instance colour from the archetype's `color`-typed property default so a
  // manually placed agent carries an explicit, editable hex (matches the live-run
  // render fallback and lets the side panel edit it per-instance).
  const archColor = colorPropDefault(arch.properties)
  return {
    id: nextAgentId(agents),
    agent_uid: arch.uid,
    name: '',
    state: arch.initialization?.initial_state ?? '',
    properties: archColor ? { color: archColor } : {},
    position: { x: Math.round(at.x), y: Math.round(at.y) },
  }
}

// ---------------------------------------------------------------------------
// color resolution
// ---------------------------------------------------------------------------

const PALETTE = [
  '#8B5CF6', '#3B82F6', '#22C55E', '#EF4444', '#F59E0B',
  '#EC4899', '#14B8A6', '#F97316', '#6366F1', '#84CC16',
]

export function paletteColor(index: number): string {
  return PALETTE[((index % PALETTE.length) + PALETTE.length) % PALETTE.length] ?? '#8B5CF6'
}

function isHex(v: unknown): v is string {
  return typeof v === 'string' && /^#?[0-9a-fA-F]{3,8}$/.test(v)
}

function normHex(v: string): string {
  return v.startsWith('#') ? v : `#${v}`
}

/** First `color`-typed property's default on a snapshot definition, if any. */
function colorPropDefault(props: { type: string; default_value?: unknown }[] | undefined): string | undefined {
  const p = (props ?? []).find((x) => x.type === 'color')
  return isHex(p?.default_value) ? normHex(p!.default_value as string) : undefined
}

/**
 * Resolve an archetype's display color: a `color`-typed property default on the
 * archetype, else a palette index (its position among the snapshot archetypes).
 */
export function archetypeColor(arch: SnapshotAgent | undefined, index: number): string {
  const fromProp = colorPropDefault(arch?.properties)
  if (fromProp) return fromProp
  return paletteColor(index)
}

/** Build an archetype-uid → color map for legend + agent rendering. */
export function archetypeColorMap(snapshot: ModelSnapshot | null): Map<string, string> {
  const m = new Map<string, string>()
  const archs = snapshot?.agents ?? []
  archs.forEach((a, i) => m.set(a.uid, archetypeColor(a, i)))
  return m
}

/**
 * Resolve a shape's display color: an instance `color` property, else the
 * snapshot shape def's `color`-typed property default, else a palette index.
 */
export function shapeColor(
  shape: SimulationShape,
  snapshot: ModelSnapshot | null,
  index: number
): string {
  const inst = shape.properties?.color
  if (isHex(inst)) return normHex(inst)
  const def = (snapshot?.environment?.shapes ?? []).find((d) => d.uid === shape.shape_uid)
  const fromDef = colorPropDefault(def?.properties)
  if (fromDef) return fromDef
  return paletteColor(index)
}

/** Convert a hex string to a 0xRRGGBB number (for Pixi tinting). */
export function hexToNumber(hex: string): number {
  const cleaned = hex.replace('#', '')
  const full = cleaned.length === 3 ? cleaned.split('').map((c) => c + c).join('') : cleaned
  const n = parseInt(full.slice(0, 6), 16)
  return Number.isNaN(n) ? 0x8b5cf6 : n
}
