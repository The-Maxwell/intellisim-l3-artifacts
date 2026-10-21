<template>
  <div class="rounded-lg border border-zinc-200 bg-zinc-50 p-3 space-y-2">
    <!-- Type + remove -->
    <div class="flex items-end gap-2">
      <div class="flex-1 min-w-0">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Condition type</label>
        <Select
          :modelValue="modelValue.type"
          :options="CONDITION_TYPES as unknown as string[]"
          class="w-full"
          size="small"
          @update:modelValue="(v) => onTypeChange(v)"
        />
      </div>
      <Button
        v-if="removable"
        icon="pi pi-trash"
        severity="danger"
        text
        rounded
        size="small"
        aria-label="Remove condition"
        @click="emit('remove')"
      />
    </div>

    <!-- property_uid: property (agent props + pseudo, editable) -->
    <div v-if="modelValue.type === 'property'">
      <label class="text-[11px] text-zinc-500 mb-0.5 block">Property UID</label>
      <Select
        :modelValue="modelValue.property_uid ?? null"
        :options="agentPropOptions"
        optionLabel="label"
        optionValue="value"
        editable
        class="w-full"
        size="small"
        placeholder="Select or type a property"
        @update:modelValue="(v) => patch({ property_uid: v || undefined })"
      />
    </div>

    <!-- property_uid: global_property (environment globals) -->
    <div v-if="modelValue.type === 'global_property'">
      <label class="text-[11px] text-zinc-500 mb-0.5 block">Global property UID</label>
      <Select
        :modelValue="modelValue.property_uid ?? null"
        :options="globalPropOptions"
        optionLabel="label"
        optionValue="value"
        editable
        class="w-full"
        size="small"
        placeholder="Select or type a global property"
        @update:modelValue="(v) => patch({ property_uid: v || undefined })"
      />
    </div>

    <!-- event: event select (target_uid) + optional payload match -->
    <template v-if="modelValue.type === 'event'">
      <div>
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Event</label>
        <Select
          :modelValue="modelValue.target_uid ?? null"
          :options="eventOpts"
          optionLabel="label"
          optionValue="value"
          class="w-full"
          size="small"
          showClear
          filter
          placeholder="Select event"
          @update:modelValue="(v) => patch({ target_uid: v ?? undefined })"
        />
      </div>
      <div class="flex gap-2">
        <div class="w-32">
          <label class="text-[11px] text-zinc-500 mb-0.5 block">Payload operator</label>
          <Select
            :modelValue="modelValue.operator ?? null"
            :options="CONDITION_OPERATORS as unknown as string[]"
            class="w-full"
            size="small"
            showClear
            @update:modelValue="(v) => patch({ operator: v ?? undefined })"
          />
        </div>
        <div class="flex-1 min-w-0">
          <label class="text-[11px] text-zinc-500 mb-0.5 block">Payload value</label>
          <InputText
            :modelValue="valueAsString"
            class="w-full"
            size="small"
            placeholder="Any payload"
            @update:modelValue="(v) => patch({ value: v || undefined })"
          />
        </div>
      </div>
      <p class="text-[11px] text-zinc-400">Leave payload fields empty to match any emission of this event.</p>
    </template>

    <!-- step: operator + value (+ range when in_range) -->
    <div v-if="modelValue.type === 'step'" class="flex gap-2">
      <div class="w-32">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Operator</label>
        <Select
          :modelValue="modelValue.operator ?? null"
          :options="CONDITION_OPERATORS as unknown as string[]"
          class="w-full"
          size="small"
          showClear
          @update:modelValue="(v) => patch({ operator: v ?? undefined })"
        />
      </div>
      <div v-if="modelValue.operator !== 'in_range'" class="flex-1 min-w-0">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Step</label>
        <InputNumber
          :modelValue="valueAsNumber"
          class="w-full"
          size="small"
          :min="0"
          @update:modelValue="(v) => patch({ value: v ?? undefined })"
        />
      </div>
    </div>

    <!-- random: probability -->
    <div v-if="modelValue.type === 'random'">
      <label class="text-[11px] text-zinc-500 mb-0.5 block">Probability (0–1)</label>
      <InputNumber
        :modelValue="valueAsNumber"
        class="w-full"
        size="small"
        :min="RANDOM_PROBABILITY_MIN"
        :max="RANDOM_PROBABILITY_MAX"
        :step="RANDOM_PROBABILITY_STEP"
        :minFractionDigits="0"
        :maxFractionDigits="3"
        @update:modelValue="(v) => patch({ value: v ?? undefined })"
      />
      <p class="text-[11px] text-zinc-400">Fires when a random draw is below this value (≥1 always, ≤0 never).</p>
    </div>

    <!-- decision: label + optional operator -->
    <template v-if="modelValue.type === 'decision'">
      <div class="flex gap-2">
        <div class="w-32">
          <label class="text-[11px] text-zinc-500 mb-0.5 block">Operator</label>
          <Select
            :modelValue="modelValue.operator ?? null"
            :options="CONDITION_OPERATORS as unknown as string[]"
            class="w-full"
            size="small"
            showClear
            placeholder="equals"
            @update:modelValue="(v) => patch({ operator: v ?? undefined })"
          />
        </div>
        <div class="flex-1 min-w-0">
          <label class="text-[11px] text-zinc-500 mb-0.5 block">Decision label</label>
          <InputText
            :modelValue="valueAsString"
            class="w-full"
            size="small"
            @update:modelValue="(v) => patch({ value: v || undefined })"
          />
        </div>
      </div>
      <p class="text-[11px] text-zinc-400">Matches an LLM decision label from the current step.</p>
    </template>

    <!-- memory: type + within + substring -->
    <template v-if="modelValue.type === 'memory'">
      <div class="flex gap-2">
        <div class="flex-1 min-w-0">
          <label class="text-[11px] text-zinc-500 mb-0.5 block">Memory type</label>
          <Select
            :modelValue="memoryType"
            :options="MEMORY_TYPES as unknown as string[]"
            class="w-full"
            size="small"
            showClear
            placeholder="Any type"
            @update:modelValue="(v) => setParam('memory_type', v ?? undefined)"
          />
        </div>
        <div class="w-32">
          <label class="text-[11px] text-zinc-500 mb-0.5 block">Within steps</label>
          <InputNumber
            :modelValue="memoryWithin"
            class="w-full"
            size="small"
            :min="0"
            @update:modelValue="(v) => setParam('within', v ?? undefined)"
          />
        </div>
      </div>
      <div>
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Substring</label>
        <InputText
          :modelValue="valueAsString"
          class="w-full"
          size="small"
          placeholder="Any event of this type"
          @update:modelValue="(v) => patch({ value: v || undefined })"
        />
      </div>
    </template>

    <!-- operator + value: state -->
    <div v-if="modelValue.type === 'state'" class="flex gap-2">
      <div class="w-32">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Operator</label>
        <Select
          :modelValue="modelValue.operator ?? null"
          :options="CONDITION_OPERATORS as unknown as string[]"
          class="w-full"
          size="small"
          showClear
          @update:modelValue="(v) => patch({ operator: v ?? undefined })"
        />
      </div>
      <div class="flex-1 min-w-0">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">State</label>
        <Select
          :modelValue="modelValue.value != null ? String(modelValue.value) : null"
          :options="stateOpts"
          optionLabel="label"
          optionValue="value"
          editable
          class="w-full"
          size="small"
          placeholder="Select or type a state"
          @update:modelValue="(v) => patch({ value: v || undefined })"
        />
      </div>
    </div>

    <!-- operator + value: property / global_property -->
    <div v-if="modelValue.type === 'property' || modelValue.type === 'global_property'" class="flex gap-2">
      <div class="w-32">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Operator</label>
        <Select
          :modelValue="modelValue.operator ?? null"
          :options="CONDITION_OPERATORS as unknown as string[]"
          class="w-full"
          size="small"
          showClear
          @update:modelValue="(v) => patch({ operator: v ?? undefined })"
        />
      </div>
      <div class="flex-1 min-w-0">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Value</label>
        <InputText
          :modelValue="valueAsString"
          class="w-full"
          size="small"
          @update:modelValue="(v) => patch({ value: v || undefined })"
        />
      </div>
    </div>

    <!-- min/max value: step in_range / position -->
    <div v-if="show('range')" class="flex gap-2">
      <div class="flex-1">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Min value</label>
        <InputNumber
          :modelValue="modelValue.min_value ?? null"
          class="w-full"
          size="small"
          @update:modelValue="(v) => patch({ min_value: v ?? undefined })"
        />
      </div>
      <div class="flex-1">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Max value</label>
        <InputNumber
          :modelValue="modelValue.max_value ?? null"
          class="w-full"
          size="small"
          @update:modelValue="(v) => patch({ max_value: v ?? undefined })"
        />
      </div>
    </div>

    <!-- radius: proximity / count / barrier / distance -->
    <div v-if="show('radius')">
      <label class="text-[11px] text-zinc-500 mb-0.5 block">Radius</label>
      <InputNumber
        :modelValue="modelValue.radius ?? null"
        class="w-full"
        size="small"
        :min="0"
        @update:modelValue="(v) => patch({ radius: v ?? undefined })"
      />
    </div>

    <!-- target_uid + target_uids: proximity / count / distance / barrier -->
    <div v-if="show('targets')" class="space-y-2">
      <div>
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Target UID</label>
        <Select
          :modelValue="modelValue.target_uid ?? null"
          :options="targetOpts"
          optionLabel="label"
          optionValue="value"
          class="w-full"
          size="small"
          showClear
          filter
          placeholder="Select agent or shape"
          @update:modelValue="(v) => patch({ target_uid: v ?? undefined })"
        />
      </div>
      <div>
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Target UIDs</label>
        <MultiSelect
          :modelValue="modelValue.target_uids ?? []"
          :options="targetOpts"
          optionLabel="label"
          optionValue="value"
          class="w-full"
          size="small"
          display="chip"
          filter
          placeholder="Select agents or shapes"
          @update:modelValue="(v) => patch({ target_uids: v && v.length ? v : undefined })"
        />
      </div>
    </div>

    <!-- custom: parameters JSON -->
    <ParametersJsonEditor
      v-if="modelValue.type === 'custom'"
      :modelValue="modelValue.parameters"
      hint="Free-form parameters passed to the custom condition evaluator."
      @update:modelValue="(p) => patch({ parameters: p })"
    />

    <!-- composite: logic operator + recursive sub-conditions -->
    <template v-if="modelValue.type === 'composite'">
      <div>
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Logic operator</label>
        <Select
          :modelValue="modelValue.logic_operator ?? 'AND'"
          :options="LOGIC_OPERATORS as unknown as string[]"
          class="w-40"
          size="small"
          @update:modelValue="(v) => patch({ logic_operator: v })"
        />
      </div>
      <div class="pl-3 border-l-2 border-zinc-200 space-y-2">
        <p class="text-[11px] font-semibold uppercase tracking-wide text-zinc-400">
          Sub-conditions
        </p>
        <!-- NOTE: index key — items lack a stable client id under immutable updates; removing a non-last item mid-edit can shift focus. Acceptable for now; revisit with a stable-key composable if it becomes a problem. -->
        <ConditionBuilder
          v-for="(sub, si) in modelValue.sub_conditions ?? []"
          :key="si"
          :modelValue="sub"
          :model="model"
          :appliesTo="appliesTo"
          :depth="depth + 1"
          @update:modelValue="(v) => updateSub(si, v)"
          @remove="removeSub(si)"
        />
        <Button
          label="Add sub-condition"
          icon="pi pi-plus"
          size="small"
          severity="secondary"
          outlined
          @click="addSub"
        />
      </div>
    </template>

    <!-- description -->
    <div>
      <label class="text-[11px] text-zinc-500 mb-0.5 block">Description</label>
      <InputText
        :modelValue="modelValue.description ?? ''"
        class="w-full"
        size="small"
        @update:modelValue="(v) => patch({ description: v || undefined })"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
// Edits one ConditionDTO; recursive for the `composite` type (nested
// sub_conditions). Fields are type-conditional. Always emits the full updated
// condition; the parent owns the surrounding list (add/remove). Receives the full
// model (+ optional applies_to agent scope) so property/target/event fields can
// offer referential selects instead of free text.
import Select from 'primevue/select'
import InputText from 'primevue/inputtext'
import InputNumber from 'primevue/inputnumber'
import MultiSelect from 'primevue/multiselect'
import Button from 'primevue/button'
import ParametersJsonEditor from './ParametersJsonEditor.vue'
import { computed } from 'vue'
import type { ConditionDTO, Model } from '@/shared/types'
import {
  CONDITION_TYPES,
  CONDITION_OPERATORS,
  LOGIC_OPERATORS,
  MEMORY_TYPES,
  RANDOM_PROBABILITY_MIN,
  RANDOM_PROBABILITY_MAX,
  RANDOM_PROBABILITY_STEP,
} from '@/shared/validation/modelRules'
import {
  agentPropertyOptions,
  globalPropertyOptions,
  targetOptions,
  eventOptions,
  stateOptions,
} from '../referentialOptions'

const props = withDefaults(
  defineProps<{
    modelValue: ConditionDTO
    model: Model
    appliesTo?: string[]
    removable?: boolean
    depth?: number
  }>(),
  { removable: true, depth: 0 }
)
const emit = defineEmits<{ 'update:modelValue': [v: ConditionDTO]; remove: [] }>()

const agentPropOptions = computed(() => agentPropertyOptions(props.model, props.appliesTo))
const globalPropOptions = computed(() => globalPropertyOptions(props.model))
const targetOpts = computed(() => targetOptions(props.model))
const eventOpts = computed(() => eventOptions(props.model))
const stateOpts = computed(() => stateOptions(props.model, props.appliesTo))

function patch(partial: Partial<ConditionDTO>) {
  emit('update:modelValue', { ...props.modelValue, ...partial })
}

// Reset all type-specific discriminant fields when the type changes so stale
// fields (e.g. radius from a proximity condition) don't linger in the DTO.
// Only type-independent fields (description) are preserved.
function onTypeChange(newType: ConditionDTO['type']) {
  const { description } = props.modelValue
  const next: ConditionDTO = { type: newType }
  if (description !== undefined) next.description = description
  emit('update:modelValue', next)
}

const valueAsString = computed(() =>
  props.modelValue.value === undefined || props.modelValue.value === null
    ? ''
    : String(props.modelValue.value)
)
const valueAsNumber = computed(() => {
  const v = props.modelValue.value
  if (v === undefined || v === null || v === '') return null
  const n = typeof v === 'number' ? v : Number(v)
  return Number.isFinite(n) ? n : null
})

// memory parameters helpers (memory_type + within live in the parameters map).
const memoryType = computed(() => {
  const p = props.modelValue.parameters
  const v = p?.['memory_type']
  return typeof v === 'string' && v ? v : null
})
const memoryWithin = computed(() => {
  const p = props.modelValue.parameters
  const v = p?.['within']
  return typeof v === 'number' ? v : null
})
function setParam(key: string, value: unknown) {
  const params = { ...(props.modelValue.parameters ?? {}) }
  if (value === undefined || value === null || value === '') {
    delete params[key]
  } else {
    params[key] = value
  }
  patch({ parameters: Object.keys(params).length ? params : undefined })
}

// Which conditional field groups to show for the current condition type.
function show(group: 'range' | 'radius' | 'targets'): boolean {
  const t = props.modelValue.type
  switch (group) {
    case 'range':
      return t === 'position' || (t === 'step' && props.modelValue.operator === 'in_range')
    case 'radius':
      return t === 'proximity' || t === 'count' || t === 'barrier' || t === 'distance'
    case 'targets':
      return t === 'proximity' || t === 'count' || t === 'barrier' || t === 'distance'
    default:
      return false
  }
}

function updateSub(index: number, sub: ConditionDTO) {
  const subs = (props.modelValue.sub_conditions ?? []).map((s, i) => (i === index ? sub : s))
  patch({ sub_conditions: subs })
}
function removeSub(index: number) {
  const subs = (props.modelValue.sub_conditions ?? []).filter((_, i) => i !== index)
  patch({ sub_conditions: subs })
}
function addSub() {
  const subs = [...(props.modelValue.sub_conditions ?? []), { type: 'property' } as ConditionDTO]
  patch({ sub_conditions: subs })
}
</script>
