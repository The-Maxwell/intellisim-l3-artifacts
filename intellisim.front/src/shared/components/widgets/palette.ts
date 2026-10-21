// Shared chart palette for the typed preview widgets (Task 17). Same hues as the
// legacy widgets so the dashboards look consistent.
export const WIDGET_PALETTE = [
  '#8B5CF6',
  '#3B82F6',
  '#22C55E',
  '#EF4444',
  '#F59E0B',
  '#EC4899',
  '#14B8A6',
  '#6366F1',
  '#84CC16',
  '#F97316',
]

export function paletteColor(i: number): string {
  return WIDGET_PALETTE[i % WIDGET_PALETTE.length]!
}
