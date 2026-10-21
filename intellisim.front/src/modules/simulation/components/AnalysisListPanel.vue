<template>
  <DataTable :value="list" dataKey="id" size="small" :loading="loading"
    selectionMode="single" @rowSelect="(e: { data: AnalysisSummary }) => emit('view', e.data.id)">
    <template #empty><div class="text-sm text-zinc-400 py-4 text-center">No analyses yet.</div></template>
    <Column field="name" header="Name" />
    <Column header="Status">
      <template #body="{ data }">
        <div class="flex items-center gap-2">
          <Tag :value="data.status" :severity="statusSeverity(data.status)" />
          <span v-if="progress[data.id]" class="text-xs text-zinc-400">
            {{ progress[data.id]?.stage }} ({{ progress[data.id]?.percent }}%)
          </span>
        </div>
      </template>
    </Column>
    <Column field="language" header="Lang" />
    <Column field="model_label" header="Model" />
    <Column header="Created">
      <template #body="{ data }">{{ new Date(data.created_at).toLocaleString() }}</template>
    </Column>
    <Column header="Actions">
      <template #body="{ data }">
        <div class="flex gap-1">
          <Button icon="pi pi-eye" text size="small" @click="emit('view', data.id)" />
          <Button icon="pi pi-download" text size="small" :disabled="!data.has_file" @click="emit('download', data.id)" />
          <Button icon="pi pi-trash" text severity="danger" size="small" @click="emit('delete', data.id)" />
        </div>
      </template>
    </Column>
  </DataTable>
</template>

<script setup lang="ts">
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Tag from 'primevue/tag'
import Button from 'primevue/button'
import type { AnalysisSummary } from '@/shared/types'

defineProps<{
  list: AnalysisSummary[]
  loading: boolean
  progress: Record<string, { stage: string; percent: number }>
}>()
const emit = defineEmits<{ view: [id: string]; download: [id: string]; delete: [id: string] }>()

function statusSeverity(s: string): string {
  switch (s) {
    case 'completed': return 'success'
    case 'failed': return 'danger'
    case 'running': return 'info'
    default: return 'warn'
  }
}
</script>
