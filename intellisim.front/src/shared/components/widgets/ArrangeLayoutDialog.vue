<template>
  <Dialog
    :visible="visible"
    modal
    :draggable="false"
    :dismissableMask="false"
    class="arrange-layout-dialog"
    :style="{ width: '96vw', height: '92vh' }"
    :pt="{ content: { style: 'flex:1 1 auto;min-height:0;display:flex;flex-direction:column;padding:0;' } }"
    @update:visible="emit('update:visible', $event)"
  >
    <template #header>
      <div class="flex items-center gap-3 flex-1 min-w-0">
        <i class="pi pi-th-large text-violet-500" />
        <div class="min-w-0">
          <div class="text-sm font-semibold text-zinc-800">Arrange layout</div>
          <div class="text-[11px] text-zinc-400 truncate">
            {{
              spatial
                ? 'The world sits on the left; drag & resize widgets on the right. Widgets never overlap.'
                : 'Non-spatial model — drag & resize widgets across the full area. Widgets never overlap.'
            }}
          </div>
        </div>
        <Button
          label="Reset layout"
          icon="pi pi-refresh"
          size="small"
          severity="secondary"
          outlined
          class="ml-auto shrink-0"
          :disabled="!widgets.length"
          @click="resetLayout"
        />
      </div>
    </template>

    <div class="flex-1 min-h-0 bg-zinc-50 p-3">
      <DashboardLayoutGrid
        :widgets="widgets"
        :spatial="spatial"
        :editable="true"
        @update:widgets="(w) => emit('update:widgets', w)"
      >
        <template #world>
          <WorldRegionPlaceholder :envWidth="envWidth" :envHeight="envHeight" />
        </template>
        <template #widget="{ widget }">
          <WidgetPlaceholder v-if="widget" :widget="widget" />
        </template>
        <template #empty>
          No widgets yet — add widgets in the Widgets tab, then arrange them here.
        </template>
      </DashboardLayoutGrid>
    </div>
  </Dialog>
</template>

<script setup lang="ts">
// Full-screen "Arrange layout" surface, opened from the Widgets section of both
// the model and simulation editors. Shows the fixed world pane (spatial) + the
// widget grid with draggable/resizable PLACEHOLDER tiles (no live data — real
// widgets render at run time). Reuses the in-memory working copy: position edits
// emit update:widgets, which the editor pushes to its store + marks dirty (the
// existing manual-Save / auto-save path persists them). On open, if the widgets
// still carry no real layout (all at 0,0 or overlapping) it seeds a flowed layout.
import { watch } from 'vue'
import Dialog from 'primevue/dialog'
import Button from 'primevue/button'
import DashboardLayoutGrid from './DashboardLayoutGrid.vue'
import WidgetPlaceholder from './WidgetPlaceholder.vue'
import WorldRegionPlaceholder from './WorldRegionPlaceholder.vue'
import { flowWidgets, needsAutoArrange } from './dashboardLayout'
import type { WidgetConfig } from '@/shared/types/widget'

const props = defineProps<{
  visible: boolean
  widgets: WidgetConfig[]
  spatial?: boolean
  envWidth?: number | null
  envHeight?: number | null
}>()
const emit = defineEmits<{
  'update:visible': [v: boolean]
  'update:widgets': [v: WidgetConfig[]]
}>()

function resetLayout() {
  emit('update:widgets', flowWidgets(props.widgets))
}

// Seed a sane flowed layout the first time the dialog opens on widgets that were
// never arranged (legacy widgets all persist x=0,y=0 → they'd otherwise stack).
watch(
  () => props.visible,
  (open) => {
    if (open && needsAutoArrange(props.widgets)) {
      emit('update:widgets', flowWidgets(props.widgets))
    }
  },
)
</script>
