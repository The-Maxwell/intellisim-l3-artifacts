<template>
  <div
    class="h-full w-full rounded-lg border-2 border-dashed border-violet-300 bg-violet-50/40
           flex flex-col items-center justify-center gap-2 text-center px-4 select-none"
  >
    <i class="pi pi-globe text-3xl text-violet-400" />
    <span class="text-sm font-medium text-violet-700">Environment / World</span>
    <span v-if="dims" class="text-[11px] text-violet-500 tabular-nums">{{ dims }}</span>
    <span class="text-[11px] text-zinc-400 max-w-[16rem]">
      The simulation world is shown here during a run. Its position is fixed;
      arrange the widgets around it.
    </span>
  </div>
</template>

<script setup lang="ts">
// Fixed world/environment region placeholder for the arrange view. Purely
// presentational — labels the reserved area the live PixiJS canvas will occupy at
// run time and shows the authored environment dimensions/aspect for context.
import { computed } from 'vue'

const props = defineProps<{ envWidth?: number | null; envHeight?: number | null }>()

const dims = computed(() => {
  const w = props.envWidth
  const h = props.envHeight
  if (!w || !h || w <= 0 || h <= 0) return ''
  return `${w} × ${h}`
})
</script>
