<template>
  <div class="space-y-2 text-sm">
    <!-- id / name / remove -->
    <div class="flex items-end gap-2">
      <div class="flex-1 min-w-0">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">ID</label>
        <InputText :modelValue="modelValue.id" class="w-full" size="small" :maxlength="UID_MAX" @update:modelValue="(v) => patch({ id: v ?? '' })" />
      </div>
      <div class="flex-1 min-w-0">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Name</label>
        <InputText :modelValue="modelValue.name" class="w-full" size="small" :maxlength="ENTITY_NAME_MAX" @update:modelValue="(v) => patch({ name: v ?? '' })" />
      </div>
      <Button icon="pi pi-trash" severity="danger" text rounded size="small" aria-label="Remove interaction" @click="emit('remove')" />
    </div>

    <!-- description -->
    <div>
      <label class="text-[11px] text-zinc-500 mb-0.5 block">Description</label>
      <Textarea :modelValue="modelValue.description" class="w-full text-xs" rows="2" autoResize @update:modelValue="(v) => patch({ description: v ?? '' })" />
    </div>

    <!-- type / radius -->
    <div class="flex gap-2">
      <div class="w-44">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Type</label>
        <Select :modelValue="modelValue.type" :options="INTERACTION_TYPES as unknown as string[]" class="w-full" size="small" @update:modelValue="(v) => patch({ type: v })" />
      </div>
      <div class="w-32">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Radius</label>
        <InputNumber :modelValue="modelValue.radius ?? null" class="w-full" size="small" :min="0" @update:modelValue="(v) => patch({ radius: v ?? undefined })" />
      </div>
    </div>

    <!-- initiator / target uids -->
    <div class="flex flex-wrap gap-2">
      <div class="flex-1 min-w-48">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Initiator UIDs</label>
        <MultiSelect :modelValue="modelValue.initiator_uids ?? []" :options="agentOptions" optionLabel="label" optionValue="value" class="w-full" size="small" display="chip" filter placeholder="Select agents" @update:modelValue="(v) => patch({ initiator_uids: v && v.length ? v : undefined })" />
      </div>
      <div class="flex-1 min-w-48">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Target UIDs</label>
        <MultiSelect :modelValue="modelValue.target_uids ?? []" :options="agentOptions" optionLabel="label" optionValue="value" class="w-full" size="small" display="chip" filter placeholder="Select agents" @update:modelValue="(v) => patch({ target_uids: v && v.length ? v : undefined })" />
      </div>
    </div>

    <!-- toggles -->
    <div class="flex flex-wrap gap-4">
      <label class="flex items-center gap-2 text-xs text-zinc-600 cursor-pointer">
        <ToggleSwitch :modelValue="modelValue.enabled" @update:modelValue="(v) => patch({ enabled: !!v })" />
        <span>Enabled</span>
      </label>
      <label class="flex items-center gap-2 text-xs text-zinc-600 cursor-pointer">
        <ToggleSwitch :modelValue="modelValue.bidirectional" @update:modelValue="(v) => patch({ bidirectional: !!v })" />
        <span>Bidirectional</span>
      </label>
      <label class="flex items-center gap-2 text-xs text-zinc-600 cursor-pointer">
        <ToggleSwitch :modelValue="modelValue.use_llm" @update:modelValue="(v) => patch({ use_llm: !!v })" />
        <span>Use LLM</span>
      </label>
      <label v-tooltip.top="lockTooltip" class="flex items-center gap-2 text-xs text-zinc-600 cursor-pointer">
        <ToggleSwitch :modelValue="modelValue.locked" @update:modelValue="(v) => patch({ locked: !!v })" />
        <span class="flex items-center gap-1">
          <i :class="['pi text-xs', modelValue.locked ? 'pi-lock text-amber-500' : 'pi-lock-open text-zinc-400']" />
          Locked
        </span>
      </label>
    </div>

    <!-- llm prompt context -->
    <div v-if="modelValue.use_llm">
      <label class="text-[11px] text-zinc-500 mb-0.5 block">LLM prompt context</label>
      <Textarea :modelValue="modelValue.llm_prompt_context ?? ''" class="w-full text-xs" rows="2" autoResize @update:modelValue="(v) => patch({ llm_prompt_context: v || undefined })" />
    </div>

    <!-- Conditions -->
    <div class="space-y-2">
      <label class="text-[11px] font-semibold uppercase tracking-wide text-zinc-400">Conditions</label>
      <!-- NOTE: index key — items lack a stable client id under immutable updates; removing a non-last item mid-edit can shift focus. Acceptable for now; revisit with a stable-key composable if it becomes a problem. -->
      <ConditionBuilder v-for="(cond, ci) in modelValue.conditions ?? []" :key="ci" :modelValue="cond" :model="model" :appliesTo="scopeUids" @update:modelValue="(v) => updateCondition(ci, v)" @remove="removeCondition(ci)" />
      <Button label="Add condition" icon="pi pi-plus" size="small" severity="secondary" outlined @click="addCondition" />
    </div>

    <!-- Initiator actions -->
    <div class="space-y-2">
      <label class="text-[11px] font-semibold uppercase tracking-wide text-zinc-400">Initiator actions</label>
      <!-- NOTE: index key — items lack a stable client id under immutable updates; removing a non-last item mid-edit can shift focus. Acceptable for now; revisit with a stable-key composable if it becomes a problem. -->
      <ActionBuilder v-for="(act, ai) in modelValue.initiator_actions ?? []" :key="ai" :modelValue="act" :model="model" :appliesTo="modelValue.initiator_uids" @update:modelValue="(v) => updateAction('initiator_actions', ai, v)" @remove="removeAction('initiator_actions', ai)" />
      <Button label="Add initiator action" icon="pi pi-plus" size="small" severity="secondary" outlined @click="addAction('initiator_actions')" />
    </div>

    <!-- Target actions -->
    <div class="space-y-2">
      <label class="text-[11px] font-semibold uppercase tracking-wide text-zinc-400">Target actions</label>
      <!-- NOTE: index key — items lack a stable client id under immutable updates; removing a non-last item mid-edit can shift focus. Acceptable for now; revisit with a stable-key composable if it becomes a problem. -->
      <ActionBuilder v-for="(act, ai) in modelValue.target_actions ?? []" :key="ai" :modelValue="act" :model="model" :appliesTo="modelValue.target_uids" @update:modelValue="(v) => updateAction('target_actions', ai, v)" @remove="removeAction('target_actions', ai)" />
      <Button label="Add target action" icon="pi pi-plus" size="small" severity="secondary" outlined @click="addAction('target_actions')" />
    </div>
  </div>
</template>

<script setup lang="ts">
// Edits a SINGLE InteractionDTO (conditions + initiator/target action lists).
// Extracted from the old InteractionForm per-item body for master–detail hosting.
// Emits the updated interaction and a `remove` request; the host owns the array.
import InputText from 'primevue/inputtext'
import Textarea from 'primevue/textarea'
import InputNumber from 'primevue/inputnumber'
import Select from 'primevue/select'
import MultiSelect from 'primevue/multiselect'
import ToggleSwitch from 'primevue/toggleswitch'
import Button from 'primevue/button'
import Tooltip from 'primevue/tooltip'
import ConditionBuilder from './ConditionBuilder.vue'
import ActionBuilder from './ActionBuilder.vue'
import { computed } from 'vue'
import type { InteractionDTO, ConditionDTO, ActionDTO, Model } from '@/shared/types'
import { INTERACTION_TYPES, UID_MAX, ENTITY_NAME_MAX } from '@/shared/validation/modelRules'
import { agentOptions as buildAgentOptions } from '../referentialOptions'

const vTooltip = Tooltip

type ActionListKey = 'initiator_actions' | 'target_actions'

const props = defineProps<{ modelValue: InteractionDTO; model: Model }>()
const emit = defineEmits<{ 'update:modelValue': [v: InteractionDTO]; remove: [] }>()

const lockTooltip = 'Locked: the LLM will not modify this interaction'

const agentOptions = computed(() => buildAgentOptions(props.model))
// Condition scope is the union of the interaction's participants (initiator +
// target archetypes), so property/state selects offer both sides' fields.
const scopeUids = computed(() => [
  ...(props.modelValue.initiator_uids ?? []),
  ...(props.modelValue.target_uids ?? []),
])

function patch(partial: Partial<InteractionDTO>) {
  emit('update:modelValue', { ...props.modelValue, ...partial })
}

// --- conditions ---
function updateCondition(ci: number, cond: ConditionDTO) {
  patch({ conditions: (props.modelValue.conditions ?? []).map((c, i) => (i === ci ? cond : c)) })
}
function removeCondition(ci: number) {
  patch({ conditions: (props.modelValue.conditions ?? []).filter((_, i) => i !== ci) })
}
function addCondition() {
  patch({ conditions: [...(props.modelValue.conditions ?? []), { type: 'property' } as ConditionDTO] })
}

// --- actions (initiator + target lists) ---
function updateAction(key: ActionListKey, ai: number, act: ActionDTO) {
  patch({ [key]: (props.modelValue[key] ?? []).map((a, i) => (i === ai ? act : a)) })
}
function removeAction(key: ActionListKey, ai: number) {
  patch({ [key]: (props.modelValue[key] ?? []).filter((_, i) => i !== ai) })
}
function addAction(key: ActionListKey) {
  patch({ [key]: [...(props.modelValue[key] ?? []), { type: 'set_property' } as ActionDTO] })
}
</script>
