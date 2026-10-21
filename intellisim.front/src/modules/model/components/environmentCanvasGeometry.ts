// Pure geometry / hit-testing / shape-construction helpers for the PixiJS
// EnvironmentCanvas editor (Phase 5B). Kept side-effect-free (no Pixi, no Vue,
// no DOM) so the SFC stays readable and these can be unit-tested in isolation.
//
// World coordinates are integer cell units in [0,width] x [0,height]. Shapes are
// authored as FIXED instances (geometry.position set, population 1, no
// placement) — placement-strategy authoring is NOT done here.
import type {
  ShapeDTO,
  ShapeType,
  PointDTO,
  EnvironmentDTO,
  Model,
  ScalarSpecDTO,
  ShapeGeometryDTO,
  AgentInitializationDTO,
  AgentPropertyDTO,
  PlacementType,
} from '@/shared/types'

// Shared shape colour palette + default (single source of truth for the canvas
// renderer, the shape list swatches, and the add-shape factory).
export const SHAPE_PALETTE = [
  '#8B5CF6', '#3B82F6', '#22C55E', '#EF4444', '#F59E0B',
  '#EC4899', '#14B8A6', '#F97316', '#6366F1', '#84CC16',
] as const
export const DEFAULT_SHAPE_COLOR = SHAPE_PALETTE[0]

/** The conventional `color` property carrying a shape's colour hex. */
export function colorProperty(hex: string): AgentPropertyDTO {
  return { uid: 'color', name: 'color', type: 'color', description: '', immutable: false, default_value: hex }
}

// ---------------------------------------------------------------------------
// coordinate clamp / snap / round
// ---------------------------------------------------------------------------

/** Clamp a scalar into [0, max]. */
export function clamp(v: number, max: number): number {
  if (v < 0) return 0
  if (v > max) return max
  return v
}

/**
 * Convert a raw world coordinate to an authored integer coordinate: optionally
 * snapped to a multiple of `cell`, clamped to [0, max], and rounded to int.
 *
 * I4: when snapping, use the largest in-bounds multiple so a near-edge point
 * never snaps beyond `max` and then lands off-grid after clamping.
 * The cell guard is hardened to `Number.isFinite(cell) && cell > 0` so that
 * NaN/Infinity cellSize degrades gracefully to plain round+clamp.
 */
export function quantize(v: number, max: number, cell: number, snap: boolean): number {
  let out = v
  if (snap && Number.isFinite(cell) && cell > 0) {
    const snapped = Math.round(v / cell) * cell
    // cap to the largest grid multiple that is still ≤ max
    out = Math.min(snapped, Math.floor(max / cell) * cell)
  }
  out = Math.round(out)
  return clamp(out, max)
}

/** Quantize a world point against the environment bounds. */
export function quantizePoint(
  p: PointDTO,
  env: { width: number; height: number },
  cell: number,
  snap: boolean
): PointDTO {
  return {
    x: quantize(p.x, env.width, cell, snap),
    y: quantize(p.y, env.height, cell, snap),
  }
}

// ---------------------------------------------------------------------------
// scalar spec
// ---------------------------------------------------------------------------

/** A ScalarSpec resolves to a single display value (mirrors the canvas render). */
export function scalarValue(spec: ScalarSpecDTO | undefined): number {
  if (!spec) return 0
  if (typeof spec.value === 'number') return spec.value
  if (typeof spec.min === 'number' && typeof spec.max === 'number') {
    return (spec.min + spec.max) / 2
  }
  return spec.min ?? spec.max ?? 0
}

// ---------------------------------------------------------------------------
// fixed / strategy classification (mirrors the render half)
// ---------------------------------------------------------------------------

// A shape is STRATEGY-PLACED when it declares a placement strategy AND has no
// fixed geometry.position. A fixed position always wins (the generator places the
// single instance there and skips placement), so position ⇒ fixed. This mirrors
// generator.go generateShapes: `if geometry.position != nil { count = 1 } else {
// PlaceAgents(...) }`.
export function isStrategyPlaced(shape: ShapeDTO): boolean {
  return !shape.geometry?.position && !!shape.initialization?.placement?.type
}

export function isFixed(shape: ShapeDTO): boolean {
  return !isStrategyPlaced(shape)
}

// ---------------------------------------------------------------------------
// uid generation (model-unique)
// ---------------------------------------------------------------------------

/**
 * Collect every uid in the model so a freshly drawn shape gets a model-unique
 * uid (the backend validates uniqueness across agents/shapes/events/etc.).
 */
export function collectModelUids(model: Model | null | undefined, env: EnvironmentDTO): Set<string> {
  const uids = new Set<string>()
  const add = (u: string | undefined) => {
    if (u) uids.add(u)
  }
  for (const s of env.shapes ?? []) add(s.uid)
  if (model) {
    for (const a of model.agents ?? []) add(a.uid)
    for (const e of model.events ?? []) add(e.uid)
    if (model.environment && model.environment !== env) {
      for (const s of model.environment.shapes ?? []) add(s.uid)
    }
  }
  return uids
}

/** Next `shape-<n>` uid not present in `taken`. */
export function nextShapeUid(taken: Set<string>): string {
  let n = 1
  while (taken.has(`shape-${n}`)) n++
  return `shape-${n}`
}

/** A human display name `<Type> <n>` that doesn't collide with existing names. */
export function nextShapeName(type: ShapeType, shapes: ShapeDTO[]): string {
  const base = type.charAt(0).toUpperCase() + type.slice(1)
  const taken = new Set((shapes ?? []).map((s) => s.name))
  let n = 1
  while (taken.has(`${base} ${n}`)) n++
  return `${base} ${n}`
}

// ---------------------------------------------------------------------------
// shape construction (FIXED instances only)
// ---------------------------------------------------------------------------

export interface CreateShapeArgs {
  type: ShapeType
  uid: string
  name: string
  colorHex: string
  /** rectangle: top-left position; circle: center; line/polygon: ignored. */
  position?: PointDTO
  /** rectangle */
  width?: number
  height?: number
  /** circle */
  radius?: number
  /** line / polygon */
  points?: PointDTO[]
  layer?: number
}

/** Build a fixed-position ShapeDTO ready to append to environment.shapes. */
export function createShape(args: CreateShapeArgs): ShapeDTO {
  const geometry: ShapeDTO['geometry'] = {}
  switch (args.type) {
    case 'rectangle':
      geometry.position = args.position ?? { x: 0, y: 0 }
      geometry.width = { value: args.width ?? 0 }
      geometry.height = { value: args.height ?? 0 }
      break
    case 'circle':
      geometry.position = args.position ?? { x: 0, y: 0 }
      geometry.radius = { value: args.radius ?? 0 }
      break
    case 'line':
    case 'polygon':
      geometry.points = (args.points ?? []).map((p) => ({ x: p.x, y: p.y }))
      // anchor the shape with a position (its first point) so it counts as FIXED
      geometry.position = geometry.points[0] ? { ...geometry.points[0] } : { x: 0, y: 0 }
      break
  }
  return {
    uid: args.uid,
    name: args.name,
    description: '',
    type: args.type,
    layer: args.layer ?? 0,
    visible: true,
    properties: [colorProperty(args.colorHex)],
    initialization: { population_count: 1 },
    geometry,
    locked: false,
  }
}

// ---------------------------------------------------------------------------
// default geometry + shape factory (fixed OR strategy-placed)
// ---------------------------------------------------------------------------

/** Sensible default SIZE geometry for a type (no position), scaled to the env. */
export function defaultGeometryForType(
  type: ShapeType,
  env: { width: number; height: number }
): ShapeGeometryDTO {
  const w = env.width > 0 ? env.width : 100
  const h = env.height > 0 ? env.height : 100
  switch (type) {
    case 'rectangle':
      return { width: { value: Math.max(1, Math.round(w / 5)) }, height: { value: Math.max(1, Math.round(h / 5)) } }
    case 'circle':
      return { radius: { value: Math.max(1, Math.round(Math.min(w, h) / 10)) } }
    case 'line':
      return {
        points: [
          { x: Math.round(w * 0.3), y: Math.round(h * 0.5) },
          { x: Math.round(w * 0.7), y: Math.round(h * 0.5) },
        ],
      }
    case 'polygon':
      return {
        points: [
          { x: Math.round(w * 0.4), y: Math.round(h * 0.4) },
          { x: Math.round(w * 0.6), y: Math.round(h * 0.4) },
          { x: Math.round(w * 0.5), y: Math.round(h * 0.6) },
        ],
      }
  }
}

/** The fixed anchor position that centres a shape of `type` in the env. */
export function centerPosition(
  type: ShapeType,
  geometry: ShapeGeometryDTO,
  env: { width: number; height: number }
): PointDTO {
  const cx = Math.round((env.width || 0) / 2)
  const cy = Math.round((env.height || 0) / 2)
  switch (type) {
    case 'rectangle': {
      const wv = scalarValue(geometry.width)
      const hv = scalarValue(geometry.height)
      return { x: clamp(Math.round(cx - wv / 2), env.width), y: clamp(Math.round(cy - hv / 2), env.height) }
    }
    case 'circle':
      return { x: cx, y: cy }
    case 'line':
    case 'polygon':
      return geometry.points?.[0] ? { ...geometry.points[0] } : { x: cx, y: cy }
  }
}

export interface MakeShapeArgs {
  type: ShapeType
  uid: string
  name: string
  colorHex: string
  env: { width: number; height: number }
  /** Placement strategy; omit for a fixed single instance you position on canvas. */
  placement?: PlacementType
  /** Default population for a strategy-placed shape. */
  population?: number
}

/**
 * Build a ShapeDTO for the form-first "+ Add shape" flow. Without `placement` the
 * shape is a fixed single instance centred in the env (position set, population 1,
 * ready to fine-tune on the canvas). With `placement` it is strategy-placed:
 * default size geometry, NO position, and an initialization carrying the strategy.
 */
export function makeShape(args: MakeShapeArgs): ShapeDTO {
  const geometry = defaultGeometryForType(args.type, args.env)
  let initialization: AgentInitializationDTO
  if (args.placement) {
    initialization = {
      population_count: args.population ?? 10,
      count_mode: 'fixed',
      placement: { type: args.placement },
    }
  } else {
    geometry.position = centerPosition(args.type, geometry, args.env)
    initialization = { population_count: 1 }
  }
  return {
    uid: args.uid,
    name: args.name,
    description: '',
    type: args.type,
    layer: 0,
    visible: true,
    properties: [colorProperty(args.colorHex)],
    initialization,
    geometry,
    locked: false,
  }
}

// ---------------------------------------------------------------------------
// bounds + hit-testing (world units)
// ---------------------------------------------------------------------------

export interface Bounds {
  minX: number
  minY: number
  maxX: number
  maxY: number
}

/** Axis-aligned bounds of a shape in world units (null if undrawable). */
export function shapeBounds(shape: ShapeDTO): Bounds | null {
  const geo = shape.geometry
  if (!geo) return null
  switch (shape.type) {
    case 'rectangle': {
      const p = geo.position
      if (!p) return null
      const w = scalarValue(geo.width)
      const h = scalarValue(geo.height)
      return { minX: Math.min(p.x, p.x + w), minY: Math.min(p.y, p.y + h), maxX: Math.max(p.x, p.x + w), maxY: Math.max(p.y, p.y + h) }
    }
    case 'circle': {
      const p = geo.position
      if (!p) return null
      const r = scalarValue(geo.radius)
      return { minX: p.x - r, minY: p.y - r, maxX: p.x + r, maxY: p.y + r }
    }
    case 'line':
    case 'polygon': {
      const pts = geo.points ?? []
      if (!pts.length) return null
      let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity
      for (const p of pts) {
        if (p.x < minX) minX = p.x
        if (p.y < minY) minY = p.y
        if (p.x > maxX) maxX = p.x
        if (p.y > maxY) maxY = p.y
      }
      return { minX, minY, maxX, maxY }
    }
  }
}

function distToSegment(p: PointDTO, a: PointDTO, b: PointDTO): number {
  const dx = b.x - a.x
  const dy = b.y - a.y
  const len2 = dx * dx + dy * dy
  if (len2 === 0) return Math.hypot(p.x - a.x, p.y - a.y)
  let t = ((p.x - a.x) * dx + (p.y - a.y) * dy) / len2
  t = Math.max(0, Math.min(1, t))
  return Math.hypot(p.x - (a.x + t * dx), p.y - (a.y + t * dy))
}

/**
 * Point-in-shape hit test. `tolerance` is in world units (for line/vertex
 * picking); pass the world-space equivalent of a few screen px.
 */
export function pointInShape(shape: ShapeDTO, pt: PointDTO, tolerance: number): boolean {
  const geo = shape.geometry
  if (!geo) return false
  switch (shape.type) {
    case 'rectangle': {
      const b = shapeBounds(shape)
      if (!b) return false
      return pt.x >= b.minX - tolerance && pt.x <= b.maxX + tolerance && pt.y >= b.minY - tolerance && pt.y <= b.maxY + tolerance
    }
    case 'circle': {
      const p = geo.position
      if (!p) return false
      const r = scalarValue(geo.radius)
      return Math.hypot(pt.x - p.x, pt.y - p.y) <= r + tolerance
    }
    case 'line': {
      const pts = geo.points ?? []
      for (let i = 0; i + 1 < pts.length; i++) {
        if (distToSegment(pt, pts[i]!, pts[i + 1]!) <= tolerance) return true
      }
      return false
    }
    case 'polygon': {
      const pts = geo.points ?? []
      if (pts.length < 3) return false
      // ray-cast even-odd test, plus edge tolerance
      let inside = false
      for (let i = 0, j = pts.length - 1; i < pts.length; j = i++) {
        const a = pts[i]!
        const b = pts[j]!
        if (distToSegment(pt, a, b) <= tolerance) return true
        const intersect = a.y > pt.y !== b.y > pt.y && pt.x < ((b.x - a.x) * (pt.y - a.y)) / (b.y - a.y) + a.x
        if (intersect) inside = !inside
      }
      return inside
    }
  }
}

// ---------------------------------------------------------------------------
// resize handles (world units)
// ---------------------------------------------------------------------------

export type HandleKind = 'rect-corner' | 'circle-radius' | 'vertex'

export interface Handle {
  /** Stable id within a shape: rect corner index 0..3, vertex index, or 'r'. */
  id: string
  kind: HandleKind
  /** Handle anchor position in world units. */
  pos: PointDTO
}

/** Resize/vertex handles for the selected shape, in world units. */
export function getHandles(shape: ShapeDTO): Handle[] {
  const geo = shape.geometry
  if (!geo) return []
  switch (shape.type) {
    case 'rectangle': {
      const p = geo.position
      if (!p) return []
      const w = scalarValue(geo.width)
      const h = scalarValue(geo.height)
      // corners: 0=TL 1=TR 2=BR 3=BL
      return [
        { id: '0', kind: 'rect-corner', pos: { x: p.x, y: p.y } },
        { id: '1', kind: 'rect-corner', pos: { x: p.x + w, y: p.y } },
        { id: '2', kind: 'rect-corner', pos: { x: p.x + w, y: p.y + h } },
        { id: '3', kind: 'rect-corner', pos: { x: p.x, y: p.y + h } },
      ]
    }
    case 'circle': {
      const p = geo.position
      if (!p) return []
      const r = scalarValue(geo.radius)
      // single radius handle at the east point
      return [{ id: 'r', kind: 'circle-radius', pos: { x: p.x + r, y: p.y } }]
    }
    case 'line':
    case 'polygon': {
      const pts = geo.points ?? []
      return pts.map((p, i) => ({ id: String(i), kind: 'vertex' as const, pos: { x: p.x, y: p.y } }))
    }
  }
}

/** The handle whose anchor is within `radius` world units of `pt`, if any. */
export function hitHandle(handles: Handle[], pt: PointDTO, radius: number): Handle | null {
  let best: Handle | null = null
  let bestD = radius
  for (const h of handles) {
    const d = Math.hypot(h.pos.x - pt.x, h.pos.y - pt.y)
    if (d <= bestD) {
      bestD = d
      best = h
    }
  }
  return best
}

// ---------------------------------------------------------------------------
// mutation: translate body / resize handle (return NEW geometry)
// ---------------------------------------------------------------------------

/**
 * Translate a shape's whole body by (dx,dy) world units, then clamp so the shape
 * stays within bounds. Returns new geometry (immutable). For rect/circle moves
 * the position; for line/polygon moves every point (and the anchor position).
 */
export function translateShape(
  shape: ShapeDTO,
  dx: number,
  dy: number,
  env: { width: number; height: number },
  cell: number,
  snap: boolean
): ShapeDTO['geometry'] {
  const geo = shape.geometry ?? {}
  // I4: harden guard to match quantize — NaN/Infinity cellSize degrades gracefully
  const snapDelta = (d: number) => (snap && Number.isFinite(cell) && cell > 0 ? Math.round(d / cell) * cell : Math.round(d))
  const sx = snapDelta(dx)
  const sy = snapDelta(dy)

  switch (shape.type) {
    case 'rectangle': {
      const p = geo.position ?? { x: 0, y: 0 }
      const w = scalarValue(geo.width)
      const h = scalarValue(geo.height)
      const nx = Math.max(0, Math.min(env.width - w, p.x + sx))
      const ny = Math.max(0, Math.min(env.height - h, p.y + sy))
      return { ...geo, position: { x: Math.round(nx), y: Math.round(ny) } }
    }
    case 'circle': {
      const p = geo.position ?? { x: 0, y: 0 }
      const r = scalarValue(geo.radius)
      const nx = Math.max(r, Math.min(env.width - r, p.x + sx))
      const ny = Math.max(r, Math.min(env.height - r, p.y + sy))
      return { ...geo, position: { x: Math.round(nx), y: Math.round(ny) } }
    }
    case 'line':
    case 'polygon': {
      const pts = geo.points ?? []
      // clamp the delta so no point leaves bounds
      const b = shapeBounds(shape)
      let cdx = sx
      let cdy = sy
      if (b) {
        cdx = Math.max(-b.minX, Math.min(env.width - b.maxX, sx))
        cdy = Math.max(-b.minY, Math.min(env.height - b.maxY, sy))
      }
      const moved = pts.map((p) => ({ x: Math.round(p.x + cdx), y: Math.round(p.y + cdy) }))
      return { ...geo, points: moved, position: moved[0] ? { ...moved[0] } : geo.position }
    }
  }
}

/**
 * Resize a shape by dragging a handle to world point `to` (already raw; this
 * function quantizes). Returns new geometry (immutable).
 */
export function resizeShape(
  shape: ShapeDTO,
  handleId: string,
  to: PointDTO,
  env: { width: number; height: number },
  cell: number,
  snap: boolean
): ShapeDTO['geometry'] {
  const geo = shape.geometry ?? {}
  const q = (p: PointDTO) => quantizePoint(p, env, cell, snap)

  switch (shape.type) {
    case 'rectangle': {
      const p = geo.position ?? { x: 0, y: 0 }
      const w = scalarValue(geo.width)
      const h = scalarValue(geo.height)
      // the opposite corner stays fixed; the dragged corner moves to `to`
      const corners = [
        { x: p.x, y: p.y }, // 0 TL
        { x: p.x + w, y: p.y }, // 1 TR
        { x: p.x + w, y: p.y + h }, // 2 BR
        { x: p.x, y: p.y + h }, // 3 BL
      ]
      const idx = Number(handleId)
      const oppIdx = (idx + 2) % 4
      const opp = corners[oppIdx]!
      const moved = q(to)
      const nx = Math.min(opp.x, moved.x)
      const ny = Math.min(opp.y, moved.y)
      // I3: floor to 1 so a fully-collapsed corner never creates a 0×0 dead rect
      const nw = Math.max(1, Math.abs(moved.x - opp.x))
      const nh = Math.max(1, Math.abs(moved.y - opp.y))
      return { ...geo, position: { x: nx, y: ny }, width: { value: nw }, height: { value: nh } }
    }
    case 'circle': {
      const p = geo.position ?? { x: 0, y: 0 }
      const moved = q(to)
      let r = Math.round(Math.hypot(moved.x - p.x, moved.y - p.y))
      // keep the circle inside bounds; I3: floor to 1 to avoid a 0-radius dead shape
      const maxR = Math.min(p.x, p.y, env.width - p.x, env.height - p.y)
      r = Math.max(1, Math.min(r, maxR))
      return { ...geo, radius: { value: r } }
    }
    case 'line':
    case 'polygon': {
      const pts = geo.points ?? []
      const idx = Number(handleId)
      const moved = q(to)
      const next = pts.map((p, i) => (i === idx ? moved : { x: p.x, y: p.y }))
      return { ...geo, points: next, position: next[0] ? { ...next[0] } : geo.position }
    }
  }
}

// ---------------------------------------------------------------------------
// color property helper
// ---------------------------------------------------------------------------

/** Upsert the `color` property on a shape, returning a NEW shape. */
export function setShapeColor(shape: ShapeDTO, hex: string): ShapeDTO {
  const props = shape.properties ?? []
  const existing = props.find((p) => p.uid === 'color')
  const nextProps = existing
    ? props.map((p) => (p.uid === 'color' ? { ...p, default_value: hex } : p))
    : [...props, colorProperty(hex)]
  return { ...shape, properties: nextProps }
}

/** Read a shape's color hex (or undefined). */
export function shapeColor(shape: ShapeDTO): string | undefined {
  const prop = shape.properties?.find((p) => p.uid === 'color')
  return typeof prop?.default_value === 'string' ? prop.default_value : undefined
}

// ---------------------------------------------------------------------------
// generic entity color helpers (agents + shapes) — key on the color-TYPED
// property (the renderer's "first color-typed property" convention), so they
// work regardless of the property's uid.
// ---------------------------------------------------------------------------

interface ColorableEntity {
  properties?: AgentPropertyDTO[]
}

/** Whether an entity already declares a color-typed property. */
export function hasColorProperty(entity: ColorableEntity): boolean {
  return (entity.properties ?? []).some((p) => p.type === 'color')
}

/** Read an entity's color hex from its first color-typed property (or undefined). */
export function entityColor(entity: ColorableEntity): string | undefined {
  const prop = entity.properties?.find((p) => p.type === 'color')
  return typeof prop?.default_value === 'string' ? prop.default_value : undefined
}

/**
 * Upsert the conventional `color` property on any entity (agent or shape),
 * returning a NEW entity. Updates the first color-typed property's default if
 * present, else appends `colorProperty(hex)`.
 */
export function setEntityColor<T extends ColorableEntity>(entity: T, hex: string): T {
  const props = entity.properties ?? []
  const idx = props.findIndex((p) => p.type === 'color')
  const nextProps =
    idx >= 0
      ? props.map((p, i) => (i === idx ? { ...p, default_value: hex } : p))
      : [...props, colorProperty(hex)]
  return { ...entity, properties: nextProps }
}
