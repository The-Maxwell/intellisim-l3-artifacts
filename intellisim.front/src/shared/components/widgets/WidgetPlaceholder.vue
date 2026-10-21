<template>
  <div
    class="h-full w-full rounded-lg border border-zinc-200 bg-white flex flex-col overflow-hidden select-none"
  >
    <div class="px-3 py-2 border-b border-zinc-100 flex items-center gap-2">
      <i :class="[meta.icon, 'text-violet-500 text-xs shrink-0']" />
      <span class="text-xs font-medium text-zinc-700 truncate flex-1">
        {{ widget.title || meta.label }}
      </span>
      <span class="text-[10px] text-zinc-400 tabular-nums shrink-0">
        {{ widget.position?.w ?? DEFAULT_W }}×{{ widget.position?.h ?? DEFAULT_H }}
      </span>
    </div>
    <div class="flex-1 min-h-0 flex flex-col items-center justify-center gap-1 text-zinc-300">
      <i :class="[meta.icon, 'text-2xl']" />
      <span class="text-[10px] text-zinc-400">{{ meta.label }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
// Layout placeholder for a widget in the arrange view — echoes the WidgetShell
// chrome (title bar + type icon) and shows the widget's grid footprint (w×h), but
// renders NO data. The arrange surface is a layout tool: real charts/tables draw
// only at actual run time. Editing widgets stays in the Widgets tab.
import { computed } from 'vue'
import { widgetTypeMeta } from './widgetTypes'
import { DEFAULT_W, DEFAULT_H } from './dashboardLayout'
import type { WidgetConfig } from '@/shared/types/widget'

const props = defineProps<{ widget: WidgetConfig }>()
const meta = computed(() => widgetTypeMeta(props.widget.type))
</script>
