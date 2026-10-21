<template>
  <div class="space-y-3">
    <!-- Description -->
    <div>
      <label class="text-xs text-zinc-500 mb-1 block">Description</label>
      <Textarea
        :modelValue="modelValue.description"
        class="w-full text-sm"
        rows="3"
        autoResize
        placeholder="What this simulation explores"
        @update:modelValue="(v) => patch({ description: v ?? '' })"
      />
    </div>

    <!-- Steps / Seed -->
    <div class="flex gap-4">
      <div class="w-40">
        <label class="text-xs text-zinc-500 mb-1 block">Steps</label>
        <InputNumber
          :modelValue="modelValue.steps"
          class="w-full"
          :min="1"
          @update:modelValue="(v) => patch({ steps: v ?? 1 })"
        />
      </div>
      <div class="w-48">
        <label class="text-xs text-zinc-500 mb-1 block">Seed</label>
        <InputNumber
          :modelValue="modelValue.seed"
          class="w-full"
          :useGrouping="false"
          :min="0"
          @update:modelValue="(v) => patch({ seed: v ?? 0 })"
        />
        <p class="text-[11px] text-zinc-400 mt-0.5">Seed controls reproducible generation.</p>
      </div>
    </div>

    <!-- Generation-model override -->
    <div>
      <label class="text-xs text-zinc-500 mb-1 block">Generation model override</label>
      <LlmExecutionConfig
        :modelValue="modelValue.llm_generation_target ?? null"
        capability="simulation_generation"
        @update:modelValue="(t) => patch({ llm_generation_target: t })"
      />
    </div>

    <!-- Execution-model override -->
    <div>
      <label class="text-xs text-zinc-500 mb-1 block">Execution model override</label>
      <LlmExecutionConfig
        :modelValue="modelValue.llm_execution_target ?? null"
        capability="simulation_execution"
        @update:modelValue="(t) => patch({ llm_execution_target: t })"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
// Edits the simulation's general fields: description, steps, seed, and the
// execution-model override (via the shared LlmExecutionConfig, capability
// "simulation_execution"). The name lives in the editor top bar (mirroring the
// model editor's GeneralForm/top-bar split), so it is intentionally omitted
// here. Emits a partial patch of the editable simulation; the parent merges it
// into store.current + markDirty.
import Textarea from 'primevue/textarea'
import InputNumber from 'primevue/inputnumber'
import LlmExecutionConfig from '@/shared/components/LlmExecutionConfig.vue'
import type { EditableSimulation } from '@/modules/simulation/editableSimulation'

defineProps<{ modelValue: EditableSimulation }>()
const emit = defineEmits<{ 'update:modelValue': [v: Partial<EditableSimulation>] }>()

function patch(partial: Partial<EditableSimulation>) {
  emit('update:modelValue', partial)
}
</script>
