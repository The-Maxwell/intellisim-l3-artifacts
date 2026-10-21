<template>
  <div class="space-y-2">
    <div class="flex items-start justify-between gap-3">
      <p class="text-xs text-zinc-500 flex-1">
        Dashboard widgets visualise the simulation. Previews compute from the current
        initial-state values; time-series widgets (line chart, log) fill in during the run.
      </p>
      <Button
        v-if="!readonly"
        label="Arrange layout"
        icon="pi pi-th-large"
        size="small"
        severity="secondary"
        outlined
        class="shrink-0"
        :disabled="!widgets.length"
        @click="arrangeVisible = true"
      />
    </div>
    <WidgetPreviewGrid
      :widgets="widgets"
      :schema="schema"
      :agents="agents"
      :environment="environment"
      :snapshot="snapshot"
      :readonly="readonly"
      @update:widgets="(w) => emit('update:widgets', w)"
    />

    <ArrangeLayoutDialog
      v-model:visible="arrangeVisible"
      :widgets="widgets"
      :spatial="spatial"
      :envWidth="envWidth"
      :envHeight="envHeight"
      @update:widgets="(w) => emit('update:widgets', w)"
    />
  </div>
</template>

<script setup lang="ts">
// Shared Widgets section used by BOTH the simulation editor and the model editor.
// Wraps WidgetPreviewGrid (which embeds the WidgetEditor) bound to a widgets array
// (v-model) + a schema context. Preview data is optional: the simulation editor
// passes its initial-state agents/environment/snapshot (live counts); the model
// editor omits them (structure/empty previews — a model has no instances). Emits
// the updated widgets array, which the parent pushes to its store + marks dirty.
//
// The "Arrange layout" button opens the full-screen ArrangeLayoutDialog where the
// user positions widgets around the fixed world pane; position edits flow back
// through the same update:widgets path (persisted by the editor's save/auto-save).
import { ref } from 'vue'
import Button from 'primevue/button'
import WidgetPreviewGrid from '@/shared/components/widgets/WidgetPreviewGrid.vue'
import ArrangeLayoutDialog from '@/shared/components/widgets/ArrangeLayoutDialog.vue'
import type { WidgetConfig } from '@/shared/types/widget'
import type { WidgetSchemaContext } from '@/shared/components/widgets/widgetSchema'
import type { SimulationAgent, SimulationEnvironment, ModelSnapshot } from '@/shared/types/simulation'

defineProps<{
  widgets: WidgetConfig[]
  schema: WidgetSchemaContext
  agents?: SimulationAgent[]
  environment?: SimulationEnvironment | null
  snapshot?: ModelSnapshot | null
  readonly?: boolean
  // Layout context for the arrange dialog: whether the model is spatial (→ fixed
  // world pane) and the authored environment dimensions (shown on the placeholder).
  spatial?: boolean
  envWidth?: number | null
  envHeight?: number | null
}>()
const emit = defineEmits<{ 'update:widgets': [v: WidgetConfig[]] }>()

const arrangeVisible = ref(false)
</script>
