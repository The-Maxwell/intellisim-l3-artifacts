<template>
  <div class="space-y-3">
    <!-- Execution target (capability-filtered) -->
    <div>
      <label class="text-[11px] font-semibold uppercase tracking-wide text-zinc-400 mb-1 block">
        Execution target
      </label>
      <LlmExecutionConfig
        :modelValue="modelValue.target ?? null"
        :capability="capability"
        @update:modelValue="(t) => patch({ target: t })"
      />
    </div>

    <!-- Prompt context -->
    <div>
      <label class="text-[11px] text-zinc-500 mb-0.5 block">Prompt context</label>
      <Textarea
        :modelValue="modelValue.prompt_context ?? ''"
        class="w-full text-xs"
        rows="2"
        autoResize
        @update:modelValue="(v) => patch({ prompt_context: v || undefined })"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
// SLIM LLM config editor: an execution target (filtered by the `capability`
// prop) + a permanent prompt context. Used for the model-generation+chat config
// (model.llm_config) and the simulation-generation config
// (simulation_config.llm_simulation_generation_config). The execution config
// (full editor with strategy + runtime knobs) uses LlmConfigForm instead.
import LlmExecutionConfig from '@/shared/components/LlmExecutionConfig.vue'
import Textarea from 'primevue/textarea'
import type { ExecutionLlmTarget, LLMConfigDTO } from '@/shared/types'

const props = defineProps<{
  modelValue: LLMConfigDTO
  capability: string
}>()
const emit = defineEmits<{ 'update:modelValue': [v: LLMConfigDTO] }>()

function patch(partial: { target?: ExecutionLlmTarget; prompt_context?: string }) {
  emit('update:modelValue', { ...props.modelValue, ...partial })
}
</script>
