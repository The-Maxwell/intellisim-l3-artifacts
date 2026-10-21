<template>
  <div class="space-y-3">
    <div class="flex items-center justify-between">
      <span class="text-[11px] text-zinc-400">{{ widgets.length }} widget(s)</span>
      <Button
        v-if="!readonly"
        label="Add widget"
        icon="pi pi-plus"
        size="small"
        severity="secondary"
        outlined
        @click="openNew"
      />
    </div>

    <!-- CSS grid honoring each widget's position w/h as column/row spans. A simple
         12-col grid; w clamps to 12, h drives row span (each row ≈ 110px). Drag
         reposition is intentionally NOT implemented here; this preview uses a
         static span layout. -->
    <div v-if="widgets.length" class="grid grid-cols-12 gap-3 auto-rows-[110px]">
      <div
        v-for="(w, i) in widgets"
        :key="w.id || i"
        class="min-h-0"
        :style="cellStyle(w)"
      >
        <WidgetPreview
          :widget="w"
          :ctx="previewCtx"
          :readonly="readonly"
          @edit="openEdit(i)"
          @delete="removeAt(i)"
        />
      </div>
    </div>
    <div v-else class="text-center py-8 text-zinc-400 text-xs rounded-lg border border-dashed border-zinc-200">
      No widgets yet.
      <template v-if="!readonly"> Click "Add widget" to create one.</template>
    </div>

    <WidgetEditor
      v-model:visible="editorVisible"
      :existing="editing"
      :ctx="schema"
      @save="onSave"
    />
  </div>
</template>

<script setup lang="ts">
// Lays out a typed WidgetConfig[] as a preview dashboard. Each cell renders the
// matching widgets component (via WidgetPreview) computing from the initial-state
// preview data, and exposes Edit + Delete; an "Add widget" button + the embedded
// WidgetEditor handle create/edit. Honors each widget's position w/h as a CSS-grid
// span (no drag). Emits the updated widgets array on every add / edit / delete.
import { computed, ref } from 'vue'
import Button from 'primevue/button'
import WidgetPreview from './WidgetPreview.vue'
import WidgetEditor from './WidgetEditor.vue'
import type { PreviewContext } from './widgetData'
import type { WidgetSchemaContext } from './widgetSchema'
import type { WidgetConfig } from '@/shared/types/widget'
import type { SimulationAgent, SimulationEnvironment, ModelSnapshot } from '@/shared/types/simulation'

const props = defineProps<{
  widgets: WidgetConfig[]
  schema: WidgetSchemaContext
  // Preview data — initial-state instances/environment + snapshot (for labels).
  // Optional: the model editor has no instances, so previews show structure/empty.
  agents?: SimulationAgent[]
  environment?: SimulationEnvironment | null
  snapshot?: ModelSnapshot | null
  readonly?: boolean
}>()
const emit = defineEmits<{ 'update:widgets': [v: WidgetConfig[]] }>()

const previewCtx = computed<PreviewContext>(() => ({
  agents: props.agents ?? [],
  environment: props.environment ?? null,
  snapshot: props.snapshot ?? null,
}))

function cellStyle(w: WidgetConfig): Record<string, string> {
  const span = Math.min(12, Math.max(1, w.position?.w || 4))
  const rows = Math.max(1, w.position?.h || 3)
  return { gridColumn: `span ${span} / span ${span}`, gridRow: `span ${rows} / span ${rows}` }
}

// --- editor lifecycle ---
const editorVisible = ref(false)
const editing = ref<WidgetConfig | null>(null)
const editingIndex = ref<number | null>(null)

function openNew() {
  editing.value = null
  editingIndex.value = null
  editorVisible.value = true
}
function openEdit(i: number) {
  editing.value = props.widgets[i] ?? null
  editingIndex.value = i
  editorVisible.value = true
}
function removeAt(i: number) {
  emit('update:widgets', props.widgets.filter((_, idx) => idx !== i))
}
function onSave(w: WidgetConfig) {
  if (editingIndex.value != null) {
    emit('update:widgets', props.widgets.map((x, idx) => (idx === editingIndex.value ? w : x)))
  } else {
    emit('update:widgets', [...props.widgets, w])
  }
  editing.value = null
  editingIndex.value = null
}
</script>
