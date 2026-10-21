// Shared render helpers for the simulation footprint/display feature, used by
// both PixiJS world canvases (SimulationWorldCanvas editor + RunWorldCanvas live
// runner) and the model EnvironmentCanvas. Two concerns:
//
//   1. DISPLAY — the purely-visual representation (auto / image / figure). For
//      `image` we lazily build a PixiJS Texture from a base64 data URL, cached by
//      the data string so identical sprites share one GPU texture.
//   2. FOOTPRINT — the simulation body (point / circle / rectangle / polygon)
//      drawn as a filled outline when no image/figure display overrides it.
//
// Side-effect-free except the texture cache; no Vue, no DOM mutation beyond an
// in-memory <img> used to decode the base64 data.
import { Texture } from 'pixi.js'
import type { Graphics } from 'pixi.js'
import type {
  AgentFootprintDTO,
  DisplayDTO,
  ModelSnapshot,
  PointXY,
  SimulationShape,
  SnapshotAgent,
} from '@/shared/types'
import { absolutePoints } from './worldCanvasInteractions'

// ---------------------------------------------------------------------------
// texture cache (keyed by base64 data string)
// ---------------------------------------------------------------------------

const textureCache = new Map<string, Promise<Texture>>()

/** Build the `data:<mime>;base64,<data>` URL a browser <img> can decode. */
export function dataUrl(data: string, mime?: string): string {
  return `data:${mime || 'image/png'};base64,${data}`
}

/**
 * Load (and memoise) a PixiJS Texture from a base64 image. The promise is cached
 * by the raw data string so repeated agents of the same archetype reuse one
 * texture. Resolves to Texture.EMPTY if the data fails to decode.
 */
export function loadTextureFromBase64(data: string, mime?: string): Promise<Texture> {
  if (!data) return Promise.resolve(Texture.EMPTY)
  const cached = textureCache.get(data)
  if (cached) return cached
  const p = new Promise<Texture>((resolve) => {
    const img = new Image()
    img.onload = () => {
      try {
        resolve(Texture.from(img))
      } catch {
        resolve(Texture.EMPTY)
      }
    }
    img.onerror = () => resolve(Texture.EMPTY)
    img.src = dataUrl(data, mime)
  })
  textureCache.set(data, p)
  return p
}

// ---------------------------------------------------------------------------
// footprint geometry
// ---------------------------------------------------------------------------

/**
 * A representative world-unit radius for a footprint (used for hit-testing and as
 * a default sprite half-size). Falls back to `fallback` for point/auto.
 */
export function footprintRadiusHint(
  footprint: AgentFootprintDTO | undefined,
  fallback: number
): number {
  if (!footprint) return fallback
  switch (footprint.kind) {
    case 'circle':
      return footprint.radius && footprint.radius > 0 ? footprint.radius : fallback
    case 'rectangle': {
      const w = footprint.width ?? 0
      const h = footprint.height ?? 0
      const half = Math.max(w, h) / 2
      return half > 0 ? half : fallback
    }
    case 'polygon': {
      const pts = footprint.points ?? []
      if (!pts.length) return fallback
      let max = 0
      for (const p of pts) max = Math.max(max, Math.hypot(p.x, p.y))
      return max > 0 ? max : fallback
    }
    default:
      return fallback
  }
}

/** Axis-aligned bounding half-extents (world units) of a footprint, centered. */
export function footprintHalfExtents(
  footprint: AgentFootprintDTO | undefined,
  fallback: number
): { hw: number; hh: number } {
  if (!footprint) return { hw: fallback, hh: fallback }
  switch (footprint.kind) {
    case 'circle': {
      const r = footprint.radius && footprint.radius > 0 ? footprint.radius : fallback
      return { hw: r, hh: r }
    }
    case 'rectangle': {
      const w = footprint.width ?? 0
      const h = footprint.height ?? 0
      return { hw: (w > 0 ? w : fallback * 2) / 2, hh: (h > 0 ? h : fallback * 2) / 2 }
    }
    case 'polygon': {
      const pts = footprint.points ?? []
      if (!pts.length) return { hw: fallback, hh: fallback }
      let mx = 0
      let my = 0
      for (const p of pts) {
        mx = Math.max(mx, Math.abs(p.x))
        my = Math.max(my, Math.abs(p.y))
      }
      return { hw: mx > 0 ? mx : fallback, hh: my > 0 ? my : fallback }
    }
    default:
      return { hw: fallback, hh: fallback }
  }
}

// A minimal Graphics-like surface — the subset of the PixiJS Graphics API the
// drawers below touch. Lets these helpers stay Pixi-version-tolerant and unit
// testable while still accepting a real Graphics instance.
export interface FootprintGraphics {
  circle(x: number, y: number, r: number): unknown
  rect(x: number, y: number, w: number, h: number): unknown
  poly(points: { x: number; y: number }[]): unknown
}

/**
 * Trace a footprint into a Graphics centered at world point `center`. Polygon /
 * figure points are anchor-relative (offset from center). Returns true if a body
 * outline was drawn (false for point — caller draws the legacy dot instead).
 */
export function drawFootprintInto(
  g: FootprintGraphics,
  center: PointXY,
  footprint: AgentFootprintDTO | undefined,
  fallbackRadius: number
): boolean {
  if (!footprint || footprint.kind === 'point') return false
  switch (footprint.kind) {
    case 'circle': {
      const r = footprint.radius && footprint.radius > 0 ? footprint.radius : fallbackRadius
      g.circle(center.x, center.y, r)
      return true
    }
    case 'rectangle': {
      const w = footprint.width && footprint.width > 0 ? footprint.width : fallbackRadius * 2
      const h = footprint.height && footprint.height > 0 ? footprint.height : fallbackRadius * 2
      g.rect(center.x - w / 2, center.y - h / 2, w, h)
      return true
    }
    case 'polygon': {
      const pts = footprint.points ?? []
      if (pts.length < 3) return false
      g.poly(pts.map((p) => ({ x: center.x + p.x, y: center.y + p.y })))
      return true
    }
    default:
      return false
  }
}

/**
 * Trace a display figure polygon (anchor-relative) into a Graphics centered at
 * `center`. Returns true if drawn (≥3 points).
 */
export function drawFigureInto(
  g: FootprintGraphics,
  center: PointXY,
  figure: PointXY[] | undefined,
  scale = 1
): boolean {
  const pts = figure ?? []
  if (pts.length < 3) return false
  const s = scale > 0 ? scale : 1
  g.poly(pts.map((p) => ({ x: center.x + p.x * s, y: center.y + p.y * s })))
  return true
}

// ---------------------------------------------------------------------------
// snapshot resolution
// ---------------------------------------------------------------------------

export interface ArchetypeVisual {
  footprint?: AgentFootprintDTO
  display?: DisplayDTO
}

/** Resolve an archetype's footprint + display from the snapshot by agent_uid. */
export function resolveArchetypeVisual(
  snapshot: ModelSnapshot | null | undefined,
  agentUid: string | undefined
): ArchetypeVisual {
  if (!snapshot || !agentUid) return {}
  const arch = (snapshot.agents ?? []).find((a: SnapshotAgent) => a.uid === agentUid)
  if (!arch) return {}
  return { footprint: arch.footprint, display: arch.display }
}

/** True when a display should render an image sprite (kind image + non-empty data). */
export function displayIsImage(display: DisplayDTO | undefined): boolean {
  return display?.kind === 'image' && !!display.image?.data
}

/** True when a display should render a drawn figure (kind figure + ≥3 points). */
export function displayIsFigure(display: DisplayDTO | undefined): boolean {
  return display?.kind === 'figure' && (display.figure?.length ?? 0) >= 3
}

// ---------------------------------------------------------------------------
// shape rendering (shared by SimulationWorldCanvas editor + RunWorldCanvas)
// ---------------------------------------------------------------------------

/**
 * Order shapes for painting: ascending `layer` (higher layer paints last, on
 * top), ties broken by original index for a stable order. Returns the shape with
 * its original index (callers use the index for palette colour fallback).
 */
export function orderShapesForDraw(
  shapes: SimulationShape[]
): { shape: SimulationShape; index: number }[] {
  return shapes
    .map((shape, index) => ({ shape, index }))
    .sort((a, b) => (a.shape.layer ?? 0) - (b.shape.layer ?? 0) || a.index - b.index)
}

/**
 * Draw a shape's geometry + style into a Graphics: the type switch (rectangle /
 * circle / line / polygon) plus the `fill` / `stroke` / `stroke_width` /
 * `thickness` styling. `worldStroke` maps a pixel stroke width to world units
 * (zoom-dependent, so it is injected per canvas). Does NOT handle the `display`
 * image/figure override or `visible` — callers decide those (image/figure need
 * async texture loading + a per-canvas dedup token).
 */
export function drawShapeGeometryInto(
  g: Graphics,
  shape: SimulationShape,
  color: number,
  worldStroke: (px: number) => number
): void {
  const strokeWidth = shape.stroke_width ?? shape.thickness ?? 1
  const doFill = shape.fill !== false
  const doStroke = shape.stroke !== false

  switch (shape.type) {
    case 'rectangle':
      g.rect(shape.position.x, shape.position.y, shape.width ?? 0, shape.height ?? 0)
      break
    case 'circle':
      g.circle(shape.position.x, shape.position.y, shape.radius ?? 0)
      break
    case 'line': {
      const pts = absolutePoints(shape)
      if (pts.length >= 2) {
        g.moveTo(pts[0]!.x, pts[0]!.y)
        for (let i = 1; i < pts.length; i++) g.lineTo(pts[i]!.x, pts[i]!.y)
      }
      break
    }
    case 'polygon': {
      const pts = absolutePoints(shape)
      if (pts.length >= 3) g.poly(pts.map((p) => ({ x: p.x, y: p.y })))
      break
    }
  }

  if (doFill && shape.type !== 'line') g.fill({ color, alpha: 0.3 })
  if (doStroke || shape.type === 'line') {
    const sw = shape.type === 'line' ? shape.thickness ?? strokeWidth : strokeWidth
    g.stroke({ color, width: worldStroke(sw), alpha: 0.9 })
  }
}
