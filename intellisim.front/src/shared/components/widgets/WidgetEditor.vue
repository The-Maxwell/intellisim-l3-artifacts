<template>
  <Dialog
    :visible="visible"
    modal
    :header="existing ? 'Edit widget' : 'Add widget'"
    :style="{ width: '560px' }"
    @update:visible="$emit('update:visible', $event)"
  >
    <div v-if="draft" class="space-y-4">
      <!-- Type picker -->
      <div>
        <label class="text-xs font-medium text-zinc-600 mb-1 block">Widget type</label>
        <div class="grid grid-cols-2 gap-2">
          <button
            v-for="wt in WIDGET_TYPE_OPTIONS"
            :key="wt.value"
            type="button"
            :class="[
              'flex flex-col items-start gap-0.5 rounded-lg border p-2.5 text-left transition-colors',
              draft.type === wt.value
                ? 'border-violet-500 bg-violet-50'
                : 'border-zinc-200 hover:border-zinc-300 bg-white',
            ]"
            @click="onTypeChange(wt.value)"
          >
            <div class="flex items-center gap-1.5">
              <i :class="['text-sm', wt.icon, draft.type === wt.value ? 'text-violet-600' : 'text-zinc-500']" />
              <span class="text-xs font-medium text-zinc-700">{{ wt.label }}</span>
            </div>
            <span class="text-[10px] text-zinc-400 leading-tight">{{ wt.description }}</span>
          </button>
        </div>
      </div>

      <!-- Title + size -->
      <div class="flex gap-2">
        <div class="flex-1">
          <label class="text-xs font-medium text-zinc-600 mb-1 block">Title</label>
          <InputText v-model="draft.title" class="w-full" size="small" placeholder="My widget" />
        </div>
        <div class="w-20">
          <label class="text-xs font-medium text-zinc-600 mb-1 block">Width</label>
          <InputNumber
            :modelValue="draft.position.w"
            class="w-full"
            size="small"
            :min="1"
            :max="12"
            @update:modelValue="(v) => (draft!.position.w = v ?? 4)"
          />
        </div>
        <div class="w-20">
          <label class="text-xs font-medium text-zinc-600 mb-1 block">Height</label>
          <InputNumber
            :modelValue="draft.position.h"
            class="w-full"
            size="small"
            :min="1"
            :max="12"
            @update:modelValue="(v) => (draft!.position.h = v ?? 3)"
          />
        </div>
      </div>

      <!-- bar_chart mode toggle -->
      <div v-if="draft.type === 'bar_chart'">
        <label class="text-xs font-medium text-zinc-600 mb-1 block">Bar chart mode</label>
        <SelectButton
          :modelValue="barMode"
          :options="BAR_MODES"
          optionLabel="label"
          optionValue="value"
          :allowEmpty="false"
          size="small"
          @update:modelValue="onBarModeChange"
        />
      </div>

      <!-- SINGLE-SOURCE builders (monitor, pie_chart, histogram, agent_table, bar_chart single) -->
      <div v-if="usesSingleSource" class="rounded-md border border-zinc-200 bg-zinc-50 p-3 space-y-2">
        <label class="text-[11px] font-semibold uppercase tracking-wide text-zinc-400">Data source</label>
        <WidgetSourceBuilder
          :source="draft.source ?? { type: singleAllowed[0]! }"
          :ctx="ctx"
          :allowedTypes="singleAllowed"
          @update:source="(s) => (draft!.source = s)"
        />
      </div>

      <!-- histogram buckets -->
      <div v-if="draft.type === 'histogram'">
        <label class="text-xs font-medium text-zinc-600 mb-1 block">Buckets (optional, ≥ 2)</label>
        <InputNumber
          :modelValue="draft.buckets ?? null"
          class="w-full"
          size="small"
          :min="2"
          showClear
          placeholder="auto (10)"
          @update:modelValue="(v) => (draft!.buckets = v ?? undefined)"
        />
      </div>

      <!-- agent_table columns -->
      <div v-if="draft.type === 'agent_table'">
        <label class="text-xs font-medium text-zinc-600 mb-1 block">Columns</label>
        <MultiSelect
          :modelValue="draft.columns ?? []"
          :options="tableColumnOptions"
          optionLabel="label"
          optionValue="value"
          display="chip"
          class="w-full"
          size="small"
          :placeholder="tableColumnOptions.length ? 'Select columns' : 'Pick an agent first'"
          @update:modelValue="(v) => (draft!.columns = v && v.length ? v : undefined)"
        />
      </div>

      <!-- SERIES editor (line_chart always; bar_chart in series mode) -->
      <div v-if="usesSeries" class="space-y-2">
        <div class="flex items-center justify-between">
          <label class="text-[11px] font-semibold uppercase tracking-wide text-zinc-400">Series</label>
          <Button label="Add series" icon="pi pi-plus" size="small" text @click="addSeries" />
        </div>
        <div
          v-for="(s, i) in draft.series ?? []"
          :key="i"
          class="rounded-md border border-zinc-200 bg-zinc-50 p-3 space-y-2"
        >
          <div class="flex items-end gap-2">
            <div class="flex-1 min-w-0">
              <label class="text-[11px] text-zinc-500 mb-0.5 block">Label</label>
              <InputText
                :modelValue="s.label"
                class="w-full"
                size="small"
                placeholder="Series label"
                @update:modelValue="(v) => patchSeries(i, { label: v ?? '' })"
              />
            </div>
            <div>
              <label class="text-[11px] text-zinc-500 mb-0.5 block">Color</label>
              <ColorPicker
                :modelValue="(s.color ?? '#8B5CF6')"
                format="hex"
                @update:modelValue="(v) => patchSeries(i, { color: normalizeHex(v) })"
              />
            </div>
            <Button
              icon="pi pi-trash"
              severity="danger"
              text
              rounded
              size="small"
              aria-label="Remove series"
              @click="removeSeries(i)"
            />
          </div>
          <WidgetSourceBuilder
            :source="s.source"
            :ctx="ctx"
            :allowedTypes="seriesAllowed"
            @update:source="(src) => patchSeries(i, { source: src })"
          />
        </div>
        <p v-if="!(draft.series ?? []).length" class="text-xs text-zinc-400">
          Add at least one series.
        </p>
      </div>

      <!-- text_log kinds -->
      <div v-if="draft.type === 'text_log'">
        <label class="text-xs font-medium text-zinc-600 mb-1 block">Log kinds</label>
        <div class="flex flex-col gap-1.5">
          <label v-for="k in LOG_KINDS" :key="k.value" class="flex items-center gap-2 text-xs text-zinc-600">
            <Checkbox
              :modelValue="(draft.log_kinds ?? []).includes(k.value)"
              :binary="true"
              @update:modelValue="(on) => toggleLogKind(k.value, !!on)"
            />
            <span>{{ k.label }}</span>
          </label>
        </div>
      </div>

      <!-- Validation issues -->
      <div v-if="issues.length" class="rounded-md border border-red-200 bg-red-50 p-2.5">
        <p class="text-[11px] font-medium text-red-700 mb-1">Fix before saving:</p>
        <ul class="text-[11px] text-red-600 list-disc pl-4 space-y-0.5">
          <li v-for="(iss, i) in issues" :key="i">{{ iss.path }} — {{ iss.reason }}</li>
        </ul>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-2">
        <Button label="Cancel" severity="secondary" size="small" @click="$emit('update:visible', false)" />
        <Button label="Save widget" size="small" :disabled="!draft || issues.length > 0" @click="onSave" />
      </div>
    </template>
  </Dialog>
</template>

<script setup lang="ts">
// Typed widget config editor (Task 17). Creates or edits ONE typed WidgetConfig
// against a WidgetSchemaContext (agents / property uids / globals / states),
// so the SAME editor works for a model OR a simulation snapshot. The source
// builders are driven by the WIDGET_SOURCE_MATRIX; bar_chart enforces its two
// mutually-exclusive modes (single source vs series). Live validation mirrors the
// backend ValidateWidget matrix — Save is disabled while any issue remains.
import { computed, ref, watch } from 'vue'
import Dialog from 'primevue/dialog'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import InputNumber from 'primevue/inputnumber'
import MultiSelect from 'primevue/multiselect'
import SelectButton from 'primevue/selectbutton'
import Checkbox from 'primevue/checkbox'
import ColorPicker from 'primevue/colorpicker'
import WidgetSourceBuilder from './WidgetSourceBuilder.vue'
import { allowedSourceTypes, agentByUid, validateWidget, type WidgetSchemaContext } from './widgetSchema'
import { WIDGET_TYPE_OPTIONS } from './widgetTypes'
import { normalizeHex } from '@/modules/simulation/colorUtil'
import type {
  WidgetConfig,
  WidgetType,
  WidgetSourceType,
  WidgetSeries,
  WidgetLogKind,
} from '@/shared/types/widget'

const BAR_MODES = [
  { value: 'single' as const, label: 'Population / states' },
  { value: 'series' as const, label: 'Series' },
]

// Series-mode source types differ by widget: line_chart series may plot a
// sim_metric run counter over time; bar_chart series may not (mirrors backend).
const LINE_SERIES_ALLOWED: WidgetSourceType[] = ['agent_property', 'global_property', 'population', 'sim_metric']
const BAR_SERIES_ALLOWED: WidgetSourceType[] = ['agent_property', 'global_property', 'population']

const LOG_KINDS: { value: WidgetLogKind; label: string }[] = [
  { value: 'events', label: 'Events' },
  { value: 'state_transitions', label: 'State transitions' },
  { value: 'llm_decisions', label: 'LLM decisions' },
]

const props = defineProps<{
  visible: boolean
  existing?: WidgetConfig | null
  ctx: WidgetSchemaContext
}>()
const emit = defineEmits<{
  'update:visible': [v: boolean]
  save: [w: WidgetConfig]
}>()

const draft = ref<WidgetConfig | null>(null)
const barMode = ref<'single' | 'series'>('single')

// Initialise the draft each time the dialog opens.
watch(
  () => props.visible,
  (open) => {
    if (!open) return
    if (props.existing) {
      draft.value = JSON.parse(JSON.stringify(props.existing)) as WidgetConfig
      barMode.value = draft.value.series && draft.value.series.length ? 'series' : 'single'
    } else {
      draft.value = blankWidget('monitor')
      barMode.value = 'single'
    }
  },
  { immediate: true },
)

function blankWidget(type: WidgetType): WidgetConfig {
  const w: WidgetConfig = {
    id: crypto.randomUUID(),
    type,
    title: '',
    position: { x: 0, y: 0, w: 4, h: 3 },
  }
  applyTypeDefaults(w, type, 'single')
  return w
}

// Reset type-specific fields when the widget type changes so stale fields from
// the previous type can't leak (mirrors the backend matrix per-type shape).
function applyTypeDefaults(w: WidgetConfig, type: WidgetType, mode: 'single' | 'series') {
  w.source = undefined
  w.series = undefined
  w.columns = undefined
  w.buckets = undefined
  w.log_kinds = undefined
  const allowed = allowedSourceTypes(type)
  switch (type) {
    case 'monitor':
    case 'pie_chart':
      w.source = { type: allowed[0]!, ...(allowed[0] === 'agent_property' ? { aggregation: 'avg' } : {}) }
      break
    case 'histogram':
      w.source = { type: 'agent_property' }
      break
    case 'agent_table':
      w.source = { type: 'agent_property' }
      w.columns = []
      break
    case 'line_chart':
      w.series = []
      break
    case 'bar_chart':
      if (mode === 'series') w.series = []
      else w.source = { type: 'population' }
      break
    case 'text_log':
      w.log_kinds = []
      break
  }
}

function onTypeChange(type: WidgetType) {
  if (!draft.value || draft.value.type === type) return
  draft.value.type = type
  barMode.value = 'single'
  applyTypeDefaults(draft.value, type, 'single')
}

function onBarModeChange(mode: 'single' | 'series') {
  if (!draft.value) return
  barMode.value = mode
  applyTypeDefaults(draft.value, 'bar_chart', mode)
}

// --- single-source widgets ---
const usesSingleSource = computed(() => {
  const t = draft.value?.type
  if (!t) return false
  if (t === 'line_chart' || t === 'text_log') return false
  if (t === 'bar_chart') return barMode.value === 'single'
  return true // monitor / pie_chart / histogram / agent_table
})

// Allowed source types for the single-source builder of the current type/mode.
const singleAllowed = computed<WidgetSourceType[]>(() => {
  const t = draft.value?.type
  if (t === 'monitor') return ['agent_property', 'global_property', 'population', 'sim_metric']
  if (t === 'pie_chart') return ['population', 'state_distribution']
  if (t === 'bar_chart') return ['population', 'state_distribution']
  if (t === 'histogram' || t === 'agent_table') return ['agent_property']
  return allowedSourceTypes(t ?? 'monitor')
})

// --- series widgets ---
const usesSeries = computed(() => {
  const t = draft.value?.type
  return t === 'line_chart' || (t === 'bar_chart' && barMode.value === 'series')
})

// Allowed source types for the series source builder — line series allow
// sim_metric, bar series do not.
const seriesAllowed = computed<WidgetSourceType[]>(() =>
  draft.value?.type === 'line_chart' ? LINE_SERIES_ALLOWED : BAR_SERIES_ALLOWED,
)

function addSeries() {
  if (!draft.value) return
  const series: WidgetSeries = { label: '', color: '#8B5CF6', source: { type: 'population' } }
  draft.value.series = [...(draft.value.series ?? []), series]
}
function patchSeries(i: number, partial: Partial<WidgetSeries>) {
  if (!draft.value?.series) return
  draft.value.series = draft.value.series.map((s, idx) => (idx === i ? { ...s, ...partial } : s))
}
function removeSeries(i: number) {
  if (!draft.value?.series) return
  draft.value.series = draft.value.series.filter((_, idx) => idx !== i)
}

// --- agent_table columns ---
const tableColumnOptions = computed<{ value: string; label: string }[]>(() => {
  const uid = draft.value?.source?.agent_uids?.[0]
  if (!uid) return []
  const agent = agentByUid(props.ctx, uid)
  if (!agent) return []
  const base = [
    { value: 'id', label: 'id' },
    { value: 'name', label: 'name' },
    { value: 'state', label: 'state' },
  ]
  return [...base, ...agent.properties.map((p) => ({ value: p.uid, label: p.name }))]
})

// --- text_log kinds ---
function toggleLogKind(kind: WidgetLogKind, on: boolean) {
  if (!draft.value) return
  const set = new Set(draft.value.log_kinds ?? [])
  if (on) set.add(kind)
  else set.delete(kind)
  draft.value.log_kinds = [...set]
}

// --- validation ---
const issues = computed(() => (draft.value ? validateWidget(draft.value) : []))

function onSave() {
  if (!draft.value || issues.value.length) return
  emit('save', JSON.parse(JSON.stringify(draft.value)) as WidgetConfig)
  emit('update:visible', false)
}
</script>
