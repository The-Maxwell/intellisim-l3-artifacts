<template>
  <div class="space-y-2">
    <!-- Source type -->
    <div>
      <label class="text-[11px] text-zinc-500 mb-0.5 block">Source type</label>
      <Select
        :modelValue="source.type"
        :options="allowed"
        optionLabel="label"
        optionValue="value"
        class="w-full"
        size="small"
        @update:modelValue="onTypeChange"
      />
    </div>

    <!-- Agent multiselect (agent_property / population / state_distribution) -->
    <div v-if="usesAgents">
      <label class="text-[11px] text-zinc-500 mb-0.5 block">
        Agents
        <span v-if="source.type === 'state_distribution'" class="text-zinc-400">(exactly one)</span>
        <span v-else-if="source.type === 'population'" class="text-zinc-400">(optional filter)</span>
      </label>
      <MultiSelect
        :modelValue="source.agent_uids ?? []"
        :options="ctx.agents"
        optionLabel="name"
        optionValue="uid"
        display="chip"
        class="w-full"
        size="small"
        :placeholder="ctx.agents.length ? 'Select agents' : 'No agents'"
        @update:modelValue="(v) => patch({ agent_uids: (v && v.length ? v : undefined) })"
      />
    </div>

    <!-- Property picker (agent_property / global_property) -->
    <div v-if="usesProperty">
      <label class="text-[11px] text-zinc-500 mb-0.5 block">Property</label>
      <Select
        :modelValue="source.property_uid ?? null"
        :options="propertyOpts"
        optionLabel="name"
        optionValue="uid"
        class="w-full"
        size="small"
        showClear
        :placeholder="propertyOpts.length ? 'Select property' : 'No properties'"
        @update:modelValue="(v) => patch({ property_uid: v ?? undefined })"
      />
    </div>

    <!-- Aggregation (agent_property only) -->
    <div v-if="source.type === 'agent_property'">
      <label class="text-[11px] text-zinc-500 mb-0.5 block">Aggregation</label>
      <Select
        :modelValue="source.aggregation ?? null"
        :options="AGGREGATIONS"
        class="w-full"
        size="small"
        placeholder="Select aggregation"
        @update:modelValue="(v) => patch({ aggregation: v ?? undefined })"
      />
    </div>

    <!-- Metric (sim_metric only) -->
    <div v-if="source.type === 'sim_metric'">
      <label class="text-[11px] text-zinc-500 mb-0.5 block">Metric</label>
      <Select
        :modelValue="source.metric ?? null"
        :options="METRICS"
        optionLabel="label"
        optionValue="value"
        class="w-full"
        size="small"
        placeholder="Select metric"
        @update:modelValue="(v) => patch({ metric: v ?? undefined })"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
// Reusable source builder used by WidgetEditor for the top-level source AND each
// series source. Restricts the source-type picker to the `allowedTypes` passed in
// (which differ between widget modes) and shows the relevant per-type fields:
// agent multiselect, property picker, aggregation. Emits the full WidgetSource.
import { computed } from 'vue'
import Select from 'primevue/select'
import MultiSelect from 'primevue/multiselect'
import { propertyOptions, type WidgetSchemaContext, type SchemaProperty } from './widgetSchema'
import type { WidgetSource, WidgetSourceType, WidgetAggregation, WidgetMetric } from '@/shared/types/widget'

const AGGREGATIONS: WidgetAggregation[] = ['avg', 'sum', 'min', 'max', 'count', 'median']

const METRICS: { value: WidgetMetric; label: string }[] = [
  { value: 'total_agents', label: 'Total agents' },
  { value: 'agents_spawned', label: 'Agents spawned' },
  { value: 'agents_removed', label: 'Agents removed' },
  { value: 'interactions', label: 'Interactions' },
  { value: 'events', label: 'Events' },
  { value: 'rule_executions', label: 'Rule executions' },
  { value: 'movement_distance', label: 'Movement distance' },
]

const SOURCE_TYPE_LABELS: Record<WidgetSourceType, string> = {
  agent_property: 'Agent property',
  global_property: 'Global property',
  population: 'Population',
  state_distribution: 'State distribution',
  sim_metric: 'Simulation metric',
}

const props = defineProps<{
  source: WidgetSource
  ctx: WidgetSchemaContext
  allowedTypes: WidgetSourceType[]
}>()
const emit = defineEmits<{ 'update:source': [v: WidgetSource] }>()

const allowed = computed(() =>
  props.allowedTypes.map((t) => ({ value: t, label: SOURCE_TYPE_LABELS[t] })),
)

const usesProperty = computed(
  () => props.source.type === 'agent_property' || props.source.type === 'global_property',
)
const usesAgents = computed(
  () =>
    props.source.type === 'agent_property' ||
    props.source.type === 'population' ||
    props.source.type === 'state_distribution',
)

const propertyOpts = computed<SchemaProperty[]>(() =>
  propertyOptions(props.ctx, props.source.type, props.source.agent_uids),
)

function patch(partial: Partial<WidgetSource>) {
  emit('update:source', { ...props.source, ...partial })
}

// Switching source type resets type-specific fields so a stale property/agg/uid
// can't leak across types (mirrors PropertyEditor.onTypeChange).
function onTypeChange(type: WidgetSourceType) {
  const next: WidgetSource = { type }
  if (type === 'agent_property') next.aggregation = 'avg'
  else if (type === 'sim_metric') next.metric = 'total_agents'
  emit('update:source', next)
}
</script>
