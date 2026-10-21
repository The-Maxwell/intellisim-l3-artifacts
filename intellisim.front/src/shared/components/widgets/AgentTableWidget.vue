<template>
  <WidgetShell :title="widget.title" :readonly="readonly" @edit="$emit('edit')" @delete="$emit('delete')">
    <DataTable
      v-if="rows.length"
      :value="rows"
      :paginator="rows.length > PAGE_SIZE"
      :rows="PAGE_SIZE"
      size="small"
      scrollable
      scrollHeight="100%"
      class="w-full h-full text-xs"
    >
      <Column
        v-for="col in columns"
        :key="col"
        :field="col"
        :header="col"
        sortable
        :style="{ minWidth: '80px', fontSize: '11px' }"
      />
    </DataTable>
    <div v-else class="flex items-center justify-center h-full text-zinc-400 text-xs">No data</div>
  </WidgetShell>
</template>

<script setup lang="ts">
// Typed agent-table preview: one row per instance of the single referenced
// agent × the selected columns (property uids + id/name/state pseudo-cols).
// Computes from initial-state data.
import { computed } from 'vue'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import WidgetShell from './WidgetShell.vue'
import { tableRows, type PreviewContext } from './widgetData'
import type { WidgetConfig } from '@/shared/types/widget'

const PAGE_SIZE = 8

const props = defineProps<{ widget: WidgetConfig; ctx: PreviewContext; readonly?: boolean }>()
defineEmits<{ edit: []; delete: [] }>()

const columns = computed<string[]>(() => props.widget.columns ?? [])
const rows = computed(() => tableRows(props.widget.source, columns.value, props.ctx))
</script>
