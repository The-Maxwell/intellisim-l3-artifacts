<template>
  <!-- Live shape properties panel — rendered ONLY when the run stream has
       delivered shape property frames (live.size > 0), so models without
       LLM-driven shapes see zero UI change. One row per shape instance with a
       live entry: display name + merged (static ⊕ live) properties. -->
  <div v-if="rows.length" class="border-b border-zinc-200 bg-white shrink-0 text-xs">
    <button
      type="button"
      class="w-full flex items-center gap-2 px-4 py-2 text-left hover:bg-zinc-50"
      :title="collapsed ? 'Expand shapes' : 'Collapse shapes'"
      @click="collapsed = !collapsed"
    >
      <i :class="collapsed ? 'pi pi-chevron-right' : 'pi pi-chevron-down'" class="text-[10px] text-zinc-400" />
      <span class="text-zinc-400 uppercase tracking-wide text-[10px] font-medium">Shapes</span>
      <span class="text-zinc-400 text-[10px]">({{ rows.length }})</span>
    </button>
    <div v-show="!collapsed" class="max-h-40 overflow-y-auto px-4 pb-2 space-y-1.5">
      <div v-for="row in rows" :key="row.id" class="flex items-baseline gap-2 min-w-0">
        <span class="font-medium text-zinc-700 shrink-0 truncate max-w-[140px]" :title="row.name">
          {{ row.name }}
        </span>
        <span v-if="!row.entries.length" class="text-zinc-400 italic">no properties</span>
        <span v-else class="flex flex-wrap gap-x-1 gap-y-0.5 min-w-0">
          <span
            v-for="entry in row.entries"
            :key="entry.key"
            class="inline-flex items-baseline gap-1 rounded bg-zinc-50 border border-zinc-100 px-1.5 py-px"
          >
            <span class="text-zinc-500">{{ entry.key }}:</span>
            <span class="text-zinc-700 font-mono">{{ entry.value }}</span>
          </span>
        </span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// RunShapesPanel — compact live view of environment shape properties during a
// run / replay. Fed by runStore.shapeFrame (live per-shape properties folded
// from the WS stream or replay accumulation) joined against the simulation's
// static shape config for names + baseline properties.
import { computed, ref } from 'vue'
import type { SimulationShape } from '@/shared/types'
import type { ShapeFrameState } from '@/modules/simulation/stores/runStore'

const props = defineProps<{
  // Static shape config (simulation.environment.shapes) — names + base properties.
  shapes: SimulationShape[]
  // Live per-shape properties, keyed by shape instance id (runStore.shapeFrame).
  live: Map<number, ShapeFrameState>
}>()

const collapsed = ref(false)

interface ShapeRow {
  id: number
  name: string
  entries: { key: string; value: string }[]
}

// One row per live shape entry (id-ordered), joined with its static config:
// display name = shape name, falling back to `shape_uid#id`; properties are the
// static config overlaid by the live values.
const rows = computed<ShapeRow[]>(() => {
  const byId = new Map<number, SimulationShape>()
  for (const s of props.shapes) byId.set(s.id, s)
  return Array.from(props.live.values())
    .sort((a, b) => a.id - b.id)
    .map((f) => {
      const shape = byId.get(f.id)
      const name = shape?.name || `${shape?.shape_uid ?? 'shape'}#${f.id}`
      const merged = { ...(shape?.properties ?? {}), ...f.properties }
      return {
        id: f.id,
        name,
        entries: Object.entries(merged).map(([key, value]) => ({ key, value: formatValue(value) })),
      }
    })
})

function formatValue(v: unknown): string {
  if (typeof v === 'number') {
    if (Number.isInteger(v)) return String(v)
    return v.toFixed(2)
  }
  if (typeof v === 'string' || typeof v === 'boolean') return String(v)
  if (v === null || v === undefined) return '—'
  try {
    const s = JSON.stringify(v)
    return s.length > 40 ? `${s.slice(0, 40)}…` : s
  } catch {
    return String(v)
  }
}
</script>
