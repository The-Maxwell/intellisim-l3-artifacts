<template>
  <div class="space-y-2 text-sm">
    <!-- uid / name / remove -->
    <div class="flex items-end gap-2">
      <div class="flex-1 min-w-0">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">UID</label>
        <InputText :modelValue="modelValue.uid" class="w-full" size="small" :maxlength="UID_MAX" @update:modelValue="(v) => patch({ uid: v ?? '' })" />
      </div>
      <div class="flex-1 min-w-0">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Name</label>
        <InputText :modelValue="modelValue.name" class="w-full" size="small" :maxlength="ENTITY_NAME_MAX" @update:modelValue="(v) => patch({ name: v ?? '' })" />
      </div>
      <Button icon="pi pi-trash" severity="danger" text rounded size="small" aria-label="Remove event" @click="emit('remove')" />
    </div>

    <!-- description -->
    <div>
      <label class="text-[11px] text-zinc-500 mb-0.5 block">Description</label>
      <Textarea :modelValue="modelValue.description" class="w-full text-xs" rows="2" autoResize @update:modelValue="(v) => patch({ description: v ?? '' })" />
    </div>

    <!-- scope / radius / ttl -->
    <div class="flex flex-wrap gap-2">
      <div class="w-40">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Scope</label>
        <Select :modelValue="modelValue.scope" :options="EVENT_SCOPES as unknown as string[]" class="w-full" size="small" @update:modelValue="(v) => patch({ scope: v })" />
      </div>
      <div v-if="modelValue.scope === 'local'" class="w-32">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Radius</label>
        <InputNumber :modelValue="modelValue.radius ?? null" class="w-full" size="small" :min="0" @update:modelValue="(v) => patch({ radius: v ?? undefined })" />
      </div>
      <div class="w-32">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">TTL steps</label>
        <InputNumber :modelValue="modelValue.ttl_steps ?? null" class="w-full" size="small" :min="0" @update:modelValue="(v) => patch({ ttl_steps: v ?? 0 })" />
      </div>
    </div>

    <!-- producers / producer_uids / consumer_uids -->
    <div class="flex flex-wrap gap-2">
      <div class="flex-1 min-w-44">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Producers</label>
        <MultiSelect :modelValue="modelValue.producers ?? []" :options="EVENT_PRODUCERS as unknown as string[]" class="w-full" size="small" display="chip" placeholder="Select producers" @update:modelValue="(v) => patch({ producers: v ?? [] })" />
      </div>
      <div class="flex-1 min-w-44">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Producer UIDs</label>
        <MultiSelect :modelValue="modelValue.producer_uids ?? []" :options="agentOptions" optionLabel="label" optionValue="value" class="w-full" size="small" display="chip" filter placeholder="Select agents" @update:modelValue="(v) => patch({ producer_uids: v && v.length ? v : undefined })" />
      </div>
      <div class="flex-1 min-w-44">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Consumer UIDs</label>
        <MultiSelect :modelValue="modelValue.consumer_uids ?? []" :options="agentOptions" optionLabel="label" optionValue="value" class="w-full" size="small" display="chip" filter placeholder="Select agents" @update:modelValue="(v) => patch({ consumer_uids: v && v.length ? v : undefined })" />
      </div>
    </div>

    <!-- llm prompt context + lock -->
    <div>
      <label class="text-[11px] text-zinc-500 mb-0.5 block">LLM prompt context</label>
      <Textarea :modelValue="modelValue.llm_prompt_context ?? ''" class="w-full text-xs" rows="2" autoResize @update:modelValue="(v) => patch({ llm_prompt_context: v || undefined })" />
    </div>

    <!-- llm target: routes LLM-produced emissions of this event -->
    <div v-if="producesViaLLM">
      <label class="text-[11px] text-zinc-500 mb-0.5 block">LLM target</label>
      <LlmExecutionConfig
        :modelValue="modelValue.llm_target ?? null"
        capability="simulation_execution"
        @update:modelValue="(t) => patch({ llm_target: t })"
      />
      <p class="text-[11px] text-zinc-400 mt-0.5">Routes LLM-produced emissions of this event; leave unset to inherit the agent/model target.</p>
    </div>
    <label v-tooltip.top="lockTooltip" class="flex items-center gap-2 text-xs text-zinc-600 cursor-pointer">
      <ToggleSwitch :modelValue="modelValue.locked" @update:modelValue="(v) => patch({ locked: !!v })" />
      <span class="flex items-center gap-1">
        <i :class="['pi text-xs', modelValue.locked ? 'pi-lock text-amber-500' : 'pi-lock-open text-zinc-400']" />
        Locked
      </span>
    </label>

    <!-- Payload -->
    <Fieldset :toggleable="true" :collapsed="false" class="text-sm">
      <template #legend>
        <span class="flex items-center gap-2">
          Payload
          <span class="text-[10px] rounded-full bg-zinc-200 text-zinc-600 px-1.5 py-0.5">{{ payloadCount }}</span>
        </span>
      </template>
      <PropertyEditor :modelValue="modelValue.payload ?? []" @update:modelValue="(p) => patch({ payload: p })" />
    </Fieldset>
  </div>
</template>

<script setup lang="ts">
// Edits a SINGLE EventDefDTO (payload via PropertyEditor, producer/consumer
// MultiSelects, lock). Extracted from the old EventForm per-item body for
// master–detail hosting. Emits the updated event and a `remove` request.
import InputText from 'primevue/inputtext'
import Textarea from 'primevue/textarea'
import InputNumber from 'primevue/inputnumber'
import Select from 'primevue/select'
import MultiSelect from 'primevue/multiselect'
import ToggleSwitch from 'primevue/toggleswitch'
import Fieldset from 'primevue/fieldset'
import Button from 'primevue/button'
import Tooltip from 'primevue/tooltip'
import PropertyEditor from './PropertyEditor.vue'
import LlmExecutionConfig from '@/shared/components/LlmExecutionConfig.vue'
import { computed } from 'vue'
import type { EventDefDTO, Agent } from '@/shared/types'
import { EVENT_SCOPES, EVENT_PRODUCERS, UID_MAX, ENTITY_NAME_MAX } from '@/shared/validation/modelRules'

const vTooltip = Tooltip

const props = defineProps<{ modelValue: EventDefDTO; agents: Agent[] }>()
const emit = defineEmits<{ 'update:modelValue': [v: EventDefDTO]; remove: [] }>()

const lockTooltip = 'Locked: the LLM will not modify this event'

const agentOptions = computed(() => props.agents.map((a) => ({ label: a.name || a.uid, value: a.uid })))

// The llm_target only matters when the LLM can produce this event.
const producesViaLLM = computed(() => (props.modelValue.producers ?? []).includes('llm'))

const payloadCount = computed(() => (props.modelValue.payload ?? []).length)

function patch(partial: Partial<EventDefDTO>) {
  emit('update:modelValue', { ...props.modelValue, ...partial })
}
</script>
