<template>
  <div class="h-full w-full min-h-0 flex">
    <!-- Fixed world/environment pane (LEFT, most of the screen). Spatial only.
         Not part of the grid and not draggable — its position is fixed. -->
    <div
      v-if="spatial"
      class="h-full min-h-0 shrink-0 overflow-hidden"
      :style="{ flexBasis: worldPercent + '%', width: worldPercent + '%' }"
    >
      <slot name="world" />
    </div>

    <!-- Widget grid (RIGHT when spatial, FULL width when non-spatial). Free
         placement with collision reflow (no overlap). -->
    <div class="h-full min-h-0 flex-1 min-w-0 overflow-auto" :class="spatial ? 'border-l border-zinc-200' : ''">
      <GridLayout
        v-if="layout.length"
        v-model:layout="layout"
        :col-num="columns"
        :row-height="rowHeight"
        :margin="margin"
        :is-draggable="editable"
        :is-resizable="editable"
        :vertical-compact="true"
        :prevent-collision="false"
        :use-css-transforms="true"
        :responsive="false"
        @layout-updated="onLayoutUpdated"
      >
        <GridItem
          v-for="item in layout"
          :key="item.i"
          :i="item.i"
          :x="item.x"
          :y="item.y"
          :w="item.w"
          :h="item.h"
          :min-w="item.minW"
          :min-h="item.minH"
          :static="!editable"
          :is-draggable="editable"
          :is-resizable="editable"
        >
          <slot name="widget" :widget="widgetById.get(String(item.i))" />
        </GridItem>
      </GridLayout>
      <div
        v-else
        class="h-full flex items-center justify-center text-center text-xs text-zinc-400 px-6"
      >
        <slot name="empty">No widgets to lay out.</slot>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// Shared dashboard layout surface, used two ways:
//   • Arrange view  — editable=true, widget slots render placeholders; the user
//     drags/resizes widgets and positions persist to WidgetConfig.position.
//   • Run screen    — editable=false, widget slots render live widgets; the world
//     slot renders the PixiJS canvas.
//
// The world/environment occupies a FIXED pane on the left (spatial only); widgets
// live in a grid-layout-plus grid on the right (or full-width when non-spatial).
// The world is NOT a grid item and NOT stored — only widget positions round-trip.
// grid-layout-plus with preventCollision:false + verticalCompact:true guarantees
// widgets never overlap.
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { GridLayout, GridItem } from 'grid-layout-plus'
import type { Layout, LayoutItem } from 'grid-layout-plus'
import {
  DASHBOARD_COLUMNS,
  DASHBOARD_ROW_HEIGHT,
  DASHBOARD_MARGIN,
  WORLD_PANE_PERCENT,
  WIDGET_MIN_SIZE,
  footprint,
  flowWidgets,
  needsAutoArrange,
} from './dashboardLayout'
import type { WidgetConfig } from '@/shared/types/widget'

const props = withDefaults(
  defineProps<{
    widgets: WidgetConfig[]
    spatial?: boolean
    editable?: boolean
    columns?: number
    rowHeight?: number
    margin?: [number, number]
    worldPercent?: number
  }>(),
  {
    spatial: false,
    editable: false,
    columns: DASHBOARD_COLUMNS,
    rowHeight: DASHBOARD_ROW_HEIGHT,
    margin: () => DASHBOARD_MARGIN,
    worldPercent: WORLD_PANE_PERCENT,
  },
)
const emit = defineEmits<{ 'update:widgets': [v: WidgetConfig[]] }>()

// Lookup so the widget slot can resolve the WidgetConfig from a grid item id.
const widgetById = computed(() => {
  const m = new Map<string, WidgetConfig>()
  for (const w of props.widgets) m.set(w.id, w)
  return m
})

// --- grid-layout state ------------------------------------------------------
// A local, mutable Layout that grid-layout-plus reflows in place. Rebuilt from
// the widgets prop whenever positions change externally; edits flow back out via
// onLayoutUpdated. An echo guard prevents the emit→prop→rebuild loop.
const layout = ref<Layout>(buildLayout(props.widgets))

// grid-layout-plus fires `layout-updated` after its initial mount/compaction, not
// just on user gestures. Ignore those early events so merely OPENING the arrange
// dialog (which may clamp a sub-min widget or compact gaps) doesn't silently dirty
// the editor. Intentional auto-arrange is emitted by ArrangeLayoutDialog itself.
const settled = ref(false)
onMounted(() => nextTick(() => { settled.value = true }))

function buildLayout(widgets: WidgetConfig[]): Layout {
  // Widgets that were never arranged all persist at (0,0); flow them for display
  // so they don't stack into one column (matches the old auto-flowing CSS grid).
  // Display-only — never emitted; the arrange dialog persists its own flow.
  const source = needsAutoArrange(widgets) ? flowWidgets(widgets, props.columns) : widgets
  return source.map((w) => {
    const min = WIDGET_MIN_SIZE[w.type] ?? { minW: 1, minH: 1 }
    const { w: width, h: height } = footprint(w, props.columns)
    return {
      i: w.id,
      x: Math.max(0, w.position?.x ?? 0),
      y: Math.max(0, w.position?.y ?? 0),
      w: width,
      h: height,
      minW: min.minW,
      minH: min.minH,
    } satisfies LayoutItem
  })
}

// Positions differ between the widgets prop and a candidate layout?
function positionsDiffer(widgets: WidgetConfig[], lay: Layout): boolean {
  if (widgets.length !== lay.length) return true
  const byId = new Map(lay.map((it) => [String(it.i), it]))
  for (const w of widgets) {
    const it = byId.get(w.id)
    if (!it) return true
    const p = w.position
    if (!p || p.x !== it.x || p.y !== it.y || p.w !== it.w || p.h !== it.h) return true
  }
  return false
}

// Rebuild the local layout only when the incoming widgets truly changed (added/
// removed/repositioned) — not on our own echoed emit. (editable toggling needs no
// rebuild: GridItem's :static/:is-draggable/:is-resizable bindings are reactive.)
watch(
  () => props.widgets,
  (widgets) => {
    if (positionsDiffer(widgets, layout.value)) layout.value = buildLayout(widgets)
  },
  { deep: true },
)

function onLayoutUpdated(next: Layout) {
  if (!props.editable || !settled.value) return
  if (!positionsDiffer(props.widgets, next)) return
  const byId = new Map(next.map((it) => [String(it.i), it]))
  const updated = props.widgets.map((w) => {
    const it = byId.get(w.id)
    if (!it) return w
    return { ...w, position: { x: it.x, y: it.y, w: it.w, h: it.h } }
  })
  emit('update:widgets', updated)
}
</script>
