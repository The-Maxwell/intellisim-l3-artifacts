// Dashboard layout constants + pure helpers for the arrange/run screen.
//
// Layout model:
//   • Spatial models: the world/environment occupies a FIXED pane on the LEFT
//     (most of the screen); widgets live in a free-placement grid on the RIGHT.
//   • Non-spatial models: there is no world pane — widgets fill the full area.
//
// The world pane is a fixed, client-side region — it is NOT stored in the DB and
// is NOT user-movable. Only widget positions persist, reusing the existing
// WidgetConfig.position { x, y, w, h } (grid units) that the backend already
// round-trips. grid-layout-plus drives the widget grid; its collision reflow
// (preventCollision:false + verticalCompact:true) guarantees widgets never
// overlap each other.
import type { WidgetConfig, WidgetType, WidgetPosition } from '@/shared/types/widget'

// Fixed 12-column grid (matches the legacy static CSS grid it replaces).
export const DASHBOARD_COLUMNS = 12
// Row height in px — the vertical unit of the widget grid (≈ the old auto-rows-[110px]).
export const DASHBOARD_ROW_HEIGHT = 100
// [horizontal, vertical] gap between grid cells, in px.
export const DASHBOARD_MARGIN: [number, number] = [8, 8]

// Share of the screen width the fixed world pane takes on spatial layouts. The
// user's intent: the world takes MOST of the screen on the left, widgets on the
// right. Tunable — kept high so the live world canvas has room for moving agents.
export const WORLD_PANE_PERCENT = 66

// Per-type minimum grid footprint, so a widget can't be shrunk below usability.
export const WIDGET_MIN_SIZE: Record<WidgetType, { minW: number; minH: number }> = {
  monitor: { minW: 2, minH: 2 },
  line_chart: { minW: 3, minH: 3 },
  bar_chart: { minW: 3, minH: 3 },
  pie_chart: { minW: 3, minH: 3 },
  histogram: { minW: 3, minH: 3 },
  agent_table: { minW: 4, minH: 4 },
  text_log: { minW: 3, minH: 4 },
}

// Default footprint when a widget has no usable size yet (shared by the grid,
// placeholders, and the widget editor's blankWidget).
export const DEFAULT_W = 4
export const DEFAULT_H = 3

function clampInt(v: number, lo: number, hi: number): number {
  if (!Number.isFinite(v)) return lo
  return Math.min(hi, Math.max(lo, Math.round(v)))
}

// The effective (clamped) footprint of a widget on a `columns`-wide grid,
// honoring its per-type minimum. w is clamped to [minW, columns]; h to >= minH.
export function footprint(w: WidgetConfig, columns: number): { w: number; h: number } {
  const min = WIDGET_MIN_SIZE[w.type] ?? { minW: 1, minH: 1 }
  const width = clampInt(w.position?.w || DEFAULT_W, min.minW, columns)
  const height = Math.max(min.minH, clampInt(w.position?.h || DEFAULT_H, 1, 9999))
  return { w: width, h: height }
}

// Do two rectangles overlap? (grid units; half-open — touching edges are fine.)
function rectsOverlap(a: WidgetPosition, b: WidgetPosition): boolean {
  return a.x < b.x + b.w && b.x < a.x + a.w && a.y < b.y + b.h && b.y < a.y + a.h
}

/**
 * Deterministically pack widgets into a `columns`-wide grid, left→right then
 * top→bottom, in slice order. Returns a NEW array with each widget's position
 * rewritten to a collision-free (x, y) (w/h are clamped to the per-type minimum).
 * Used to auto-arrange legacy widgets (which all persist x=0,y=0) and for the
 * "Reset layout" action. Mirrors the packing the run/arrange grid would settle on.
 */
export function flowWidgets(widgets: WidgetConfig[], columns = DASHBOARD_COLUMNS): WidgetConfig[] {
  const placed: WidgetPosition[] = []
  return widgets.map((widget) => {
    const { w, h } = footprint(widget, columns)
    // Scan rows top→bottom; within each row scan x left→right for the first slot
    // whose w×h rectangle collides with nothing already placed.
    let chosen: WidgetPosition | null = null
    for (let y = 0; chosen === null; y++) {
      for (let x = 0; x <= columns - w; x++) {
        const rect: WidgetPosition = { x, y, w, h }
        if (!placed.some((p) => rectsOverlap(rect, p))) {
          chosen = rect
          break
        }
      }
    }
    placed.push(chosen)
    return { ...widget, position: chosen }
  })
}

/**
 * True when the widgets need an initial auto-arrange: either every widget sits at
 * the origin (the legacy state — x/y were ignored so all persisted as 0,0) or any
 * two widgets overlap. Lets the arrange dialog seed a sane flowed layout on first
 * open instead of stacking everything in the top-left.
 */
export function needsAutoArrange(widgets: WidgetConfig[]): boolean {
  if (widgets.length <= 1) return false
  const allAtOrigin = widgets.every((w) => (w.position?.x || 0) === 0 && (w.position?.y || 0) === 0)
  if (allAtOrigin) return true
  for (let i = 0; i < widgets.length; i++) {
    for (let j = i + 1; j < widgets.length; j++) {
      const a = widgets[i]!.position
      const b = widgets[j]!.position
      if (a && b && rectsOverlap(a, b)) return true
    }
  }
  return false
}
