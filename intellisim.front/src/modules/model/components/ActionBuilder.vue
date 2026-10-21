<template>
  <div class="rounded-lg border border-zinc-200 bg-zinc-50 p-3 space-y-2">
    <!-- Type + remove -->
    <div class="flex items-end gap-2">
      <div class="flex-1 min-w-0">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Action type</label>
        <Select
          :modelValue="modelValue.type"
          :options="ACTION_TYPES as unknown as string[]"
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
        aria-label="Remove action"
        @click="emit('remove')"
      />
    </div>

    <!-- set_property / modify_property: target (agent|environment) + property + modifier + value -->
    <template v-if="modelValue.type === 'set_property' || modelValue.type === 'modify_property'">
      <div>
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Target</label>
        <SelectButton
          :modelValue="propertyTarget"
          :options="PROPERTY_TARGETS"
          optionLabel="label"
          optionValue="value"
          :allowEmpty="false"
          size="small"
          @update:modelValue="onPropertyTargetChange"
        />
      </div>
      <div>
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Property UID</label>
        <Select
          :modelValue="modelValue.property_uid ?? null"
          :options="propertyTarget === 'environment' ? globalPropMutableOptions : agentPropOptions"
          optionLabel="label"
          optionValue="value"
          editable
          class="w-full"
          size="small"
          placeholder="Select or type a property"
          @update:modelValue="(v) => patch({ property_uid: v || undefined })"
        />
      </div>
      <div class="flex gap-2">
        <div class="w-36">
          <label class="text-[11px] text-zinc-500 mb-0.5 block">Modifier</label>
          <Select
            :modelValue="modelValue.modifier ?? null"
            :options="ACTION_MODIFIERS as unknown as string[]"
            class="w-full"
            size="small"
            showClear
            @update:modelValue="(v) => patch({ modifier: v ?? undefined })"
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
    </template>

    <!-- speed + direction: move -->
    <div v-if="modelValue.type === 'move'" class="flex gap-2">
      <div class="flex-1">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Speed</label>
        <InputNumber
          :modelValue="modelValue.speed ?? null"
          class="w-full"
          size="small"
          @update:modelValue="(v) => patch({ speed: v ?? undefined })"
        />
      </div>
      <div class="flex-1">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Direction (deg)</label>
        <InputNumber
          :modelValue="modelValue.direction ?? null"
          class="w-full"
          size="small"
          @update:modelValue="(v) => patch({ direction: v ?? undefined })"
        />
      </div>
    </div>

    <!-- move_toward / move_away: speed + target select(s) -->
    <template v-if="modelValue.type === 'move_toward' || modelValue.type === 'move_away'">
      <div>
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Speed</label>
        <InputNumber
          :modelValue="modelValue.speed ?? null"
          class="w-full"
          size="small"
          @update:modelValue="(v) => patch({ speed: v ?? undefined })"
        />
      </div>
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
    </template>

    <!-- teleport: explicit x,y (parameters) OR nearest of target(s) -->
    <template v-if="modelValue.type === 'teleport'">
      <div class="flex gap-2">
        <div class="flex-1">
          <label class="text-[11px] text-zinc-500 mb-0.5 block">X</label>
          <InputNumber
            :modelValue="teleportX"
            class="w-full"
            size="small"
            @update:modelValue="(v) => setTeleportCoord('x', v)"
          />
        </div>
        <div class="flex-1">
          <label class="text-[11px] text-zinc-500 mb-0.5 block">Y</label>
          <InputNumber
            :modelValue="teleportY"
            class="w-full"
            size="small"
            @update:modelValue="(v) => setTeleportCoord('y', v)"
          />
        </div>
      </div>
      <div>
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Or teleport to nearest of</label>
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
      <p class="text-[11px] text-zinc-400">Explicit X/Y take precedence; otherwise teleports to the nearest listed target.</p>
    </template>

    <!-- interact: interaction select -->
    <template v-if="modelValue.type === 'interact'">
      <div>
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Interaction</label>
        <Select
          :modelValue="modelValue.target_uid ?? null"
          :options="interactionOpts"
          optionLabel="label"
          optionValue="value"
          class="w-full"
          size="small"
          showClear
          filter
          placeholder="Select interaction"
          @update:modelValue="(v) => patch({ target_uid: v ?? undefined })"
        />
      </div>
      <p class="text-[11px] text-zinc-400">Force-fires this interaction against the nearest eligible target (skips its conditions).</p>
    </template>

    <!-- call_llm: informational only -->
    <p v-if="modelValue.type === 'call_llm'" class="text-[11px] text-zinc-500">
      Requests an on-demand LLM decision for this agent on the next step (requires a
      decision-capable LLM strategy on the archetype). No parameters.
    </p>

    <!-- emit_event: event select + event_data (JSON-or-text) -->
    <template v-if="modelValue.type === 'emit_event'">
      <div>
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Event</label>
        <Select
          :modelValue="modelValue.event_uid ?? null"
          :options="eventOpts"
          optionLabel="label"
          optionValue="value"
          class="w-full"
          size="small"
          showClear
          filter
          placeholder="Select event"
          @update:modelValue="(v) => patch({ event_uid: v ?? undefined })"
        />
      </div>
      <div>
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Event data</label>
        <InputText
          :modelValue="eventDataAsString"
          class="w-full"
          size="small"
          placeholder="Plain text or JSON"
          @update:modelValue="(v) => onEventDataChange(v)"
        />
        <p class="text-[11px] text-zinc-400">Parsed as JSON when valid, otherwise sent as plain text.</p>
      </div>
    </template>

    <!-- transition: new_state select -->
    <div v-if="modelValue.type === 'transition'">
      <label class="text-[11px] text-zinc-500 mb-0.5 block">New state</label>
      <Select
        :modelValue="modelValue.new_state ?? null"
        :options="stateOpts"
        optionLabel="label"
        optionValue="value"
        editable
        class="w-full"
        size="small"
        placeholder="Select or type a state"
        @update:modelValue="(v) => patch({ new_state: v || undefined })"
      />
    </div>

    <!-- transform: morph into archetype + inherit + spawn position -->
    <template v-if="modelValue.type === 'transform'">
      <div>
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Transform into agent</label>
        <Select
          :modelValue="modelValue.spawn_agent_uid ?? null"
          :options="agentOpts"
          optionLabel="label"
          optionValue="value"
          class="w-full"
          size="small"
          showClear
          filter
          placeholder="Select target archetype"
          @update:modelValue="(v) => patch({ spawn_agent_uid: v ?? undefined })"
        />
      </div>
      <div>
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Inherit properties from (UIDs)</label>
        <MultiSelect
          :modelValue="modelValue.spawn_inherit_from ?? []"
          :options="parentPropertyOpts"
          optionLabel="label"
          optionValue="value"
          class="w-full"
          size="small"
          display="chip"
          filter
          placeholder="Select properties"
          @update:modelValue="(v) => patch({ spawn_inherit_from: v && v.length ? v : undefined })"
        />
      </div>
      <SpawnPositionEditor
        :modelValue="modelValue.spawn_position"
        :model="model"
        @update:modelValue="(sp) => patch({ spawn_position: sp })"
      />
    </template>

    <!-- spawn_agent: spawn_* fields + spawn_position -->
    <template v-if="modelValue.type === 'spawn_agent'">
      <div class="flex gap-2">
        <div class="flex-1 min-w-0">
          <label class="text-[11px] text-zinc-500 mb-0.5 block">Spawn agent UID</label>
          <Select
            :modelValue="modelValue.spawn_agent_uid ?? null"
            :options="agentOpts"
            optionLabel="label"
            optionValue="value"
            class="w-full"
            size="small"
            showClear
            filter
            placeholder="Select archetype"
            @update:modelValue="(v) => patch({ spawn_agent_uid: v ?? undefined })"
          />
        </div>
        <div class="w-28">
          <label class="text-[11px] text-zinc-500 mb-0.5 block">Count</label>
          <InputNumber
            :modelValue="modelValue.spawn_count ?? null"
            class="w-full"
            size="small"
            :min="0"
            @update:modelValue="(v) => patch({ spawn_count: v ?? undefined })"
          />
        </div>
      </div>
      <div>
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Inherit properties from (UIDs)</label>
        <MultiSelect
          :modelValue="modelValue.spawn_inherit_from ?? []"
          :options="parentPropertyOpts"
          optionLabel="label"
          optionValue="value"
          class="w-full"
          size="small"
          display="chip"
          filter
          placeholder="Select properties"
          @update:modelValue="(v) => patch({ spawn_inherit_from: v && v.length ? v : undefined })"
        />
      </div>
      <SpawnPositionEditor
        :modelValue="modelValue.spawn_position"
        :model="model"
        @update:modelValue="(sp) => patch({ spawn_position: sp })"
      />
    </template>

    <!-- custom: parameters JSON -->
    <ParametersJsonEditor
      v-if="modelValue.type === 'custom'"
      :modelValue="modelValue.parameters"
      hint='Reads a "set_properties" object, e.g. { "set_properties": { "energy": 10 } }.'
      placeholder='{ "set_properties": { } }'
      @update:modelValue="(p) => patch({ parameters: p })"
    />

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

    <!-- llm prompt context (not for call_llm — the backend ignores it there) -->
    <div v-if="modelValue.type !== 'call_llm'">
      <label class="text-[11px] text-zinc-500 mb-0.5 block">LLM prompt context</label>
      <Textarea
        :modelValue="modelValue.llm_prompt_context ?? ''"
        class="w-full text-xs"
        rows="2"
        autoResize
        @update:modelValue="(v) => patch({ llm_prompt_context: v || undefined })"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
// Edits one ActionDTO with type-conditional fields. Always emits the full
// updated action; the parent owns the surrounding list (add/remove). Receives the
// full model (+ optional applies_to agent scope) so property/target/event/
// interaction/state fields can offer referential selects instead of free text.
import Select from 'primevue/select'
import SelectButton from 'primevue/selectbutton'
import InputText from 'primevue/inputtext'
import InputNumber from 'primevue/inputnumber'
import MultiSelect from 'primevue/multiselect'
import Textarea from 'primevue/textarea'
import Button from 'primevue/button'
import ParametersJsonEditor from './ParametersJsonEditor.vue'
import SpawnPositionEditor from './SpawnPositionEditor.vue'
import { computed } from 'vue'
import type { ActionDTO, Model } from '@/shared/types'
import { ACTION_TYPES, ACTION_MODIFIERS, EnvironmentUID } from '@/shared/validation/modelRules'
import {
  agentPropertyOptions,
  globalPropertyOptions,
  targetOptions,
  agentOptions,
  eventOptions,
  interactionOptions,
  stateOptions,
} from '../referentialOptions'

const props = withDefaults(
  defineProps<{ modelValue: ActionDTO; model: Model; appliesTo?: string[]; removable?: boolean }>(),
  { removable: true }
)
const emit = defineEmits<{ 'update:modelValue': [v: ActionDTO]; remove: [] }>()

const PROPERTY_TARGETS: { label: string; value: 'agent' | 'environment' }[] = [
  { label: 'Agent', value: 'agent' },
  { label: 'Environment', value: 'environment' },
]

const agentPropOptions = computed(() => agentPropertyOptions(props.model, props.appliesTo))
const globalPropMutableOptions = computed(() => globalPropertyOptions(props.model, true))
const targetOpts = computed(() => targetOptions(props.model))
const agentOpts = computed(() => agentOptions(props.model))
const eventOpts = computed(() => eventOptions(props.model))
const interactionOpts = computed(() => interactionOptions(props.model))
const stateOpts = computed(() => stateOptions(props.model, props.appliesTo))
// spawn_inherit_from lists the PARENT agents' properties (the union of the
// applies_to scope; falls back to all agents when unscoped).
const parentPropertyOpts = computed(() =>
  agentPropertyOptions(props.model, props.appliesTo).filter(
    (o) => o.value !== 'state' && o.value !== 'position'
  )
)

// property-target SelectButton state (agent vs environment).
const propertyTarget = computed<'agent' | 'environment'>(() =>
  props.modelValue.target_uid === EnvironmentUID ? 'environment' : 'agent'
)
function onPropertyTargetChange(v: 'agent' | 'environment') {
  // Switching target changes the applicable property list, so clear the stale
  // property selection along with the target_uid marker.
  if (v === 'environment') {
    patch({ target_uid: EnvironmentUID, property_uid: undefined })
  } else {
    patch({ target_uid: undefined, property_uid: undefined })
  }
}

function patch(partial: Partial<ActionDTO>) {
  emit('update:modelValue', { ...props.modelValue, ...partial })
}

// Reset all type-specific discriminant fields when the type changes so stale
// fields (e.g. speed/spawn_*/event_uid) don't linger in the DTO.
// Only type-independent fields (description, llm_prompt_context) are preserved.
function onTypeChange(newType: ActionDTO['type']) {
  const { description, llm_prompt_context } = props.modelValue
  const next: ActionDTO = { type: newType }
  if (description !== undefined) next.description = description
  if (llm_prompt_context !== undefined) next.llm_prompt_context = llm_prompt_context
  emit('update:modelValue', next)
}

const valueAsString = computed(() =>
  props.modelValue.value === undefined || props.modelValue.value === null
    ? ''
    : String(props.modelValue.value)
)
const eventDataAsString = computed(() =>
  props.modelValue.event_data === undefined || props.modelValue.event_data === null
    ? ''
    : typeof props.modelValue.event_data === 'string'
      ? props.modelValue.event_data
      : JSON.stringify(props.modelValue.event_data)
)

// event_data: parse as JSON when valid, otherwise keep as plain text.
function onEventDataChange(v: string | undefined) {
  const raw = v ?? ''
  if (raw === '') {
    patch({ event_data: undefined })
    return
  }
  try {
    patch({ event_data: JSON.parse(raw) })
  } catch {
    patch({ event_data: raw })
  }
}

// teleport explicit coordinates live in parameters.x / parameters.y (numbers).
const teleportX = computed(() => {
  const v = props.modelValue.parameters?.['x']
  return typeof v === 'number' ? v : null
})
const teleportY = computed(() => {
  const v = props.modelValue.parameters?.['y']
  return typeof v === 'number' ? v : null
})
function setTeleportCoord(axis: 'x' | 'y', value: number | null | undefined) {
  const params = { ...(props.modelValue.parameters ?? {}) }
  if (value === null || value === undefined) {
    delete params[axis]
  } else {
    params[axis] = value
  }
  patch({ parameters: Object.keys(params).length ? params : undefined })
}
</script>
