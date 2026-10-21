<template>
  <div class="space-y-3">
    <p class="text-xs text-zinc-500">
      The three LLM roles a model can use. Advanced knobs (strategy, runtime, fallback)
      stay collapsed until you need them.
    </p>

    <!-- Model generation & chat LLM (slim) -->
    <Fieldset legend="Model generation & chat LLM" :toggleable="true" :collapsed="false">
      <LlmSlimConfigForm
        :modelValue="modelValue.llm_config"
        capability="model_generation"
        @update:modelValue="(c) => patch({ llm_config: c })"
      />
    </Fieldset>

    <!-- Simulation generation LLM (slim) -->
    <Fieldset legend="Simulation generation LLM" :toggleable="true" :collapsed="false">
      <LlmSlimConfigForm
        :modelValue="simulationConfig.llm_simulation_generation_config"
        capability="simulation_generation"
        @update:modelValue="(c) => patchSimulationConfig({ llm_simulation_generation_config: c })"
      />
      <p v-if="generationTargetError" class="text-[11px] text-red-500 mt-1.5">
        {{ generationTargetError }}
      </p>
    </Fieldset>

    <!-- Simulation execution LLM (full editor: strategy + runtime knobs) -->
    <Fieldset legend="Simulation execution LLM" :toggleable="true" :collapsed="false">
      <LlmConfigForm
        :modelValue="simulationConfig.llm_simulation_execution_config"
        variant="execution"
        @update:modelValue="(c) => patchSimulationConfig({ llm_simulation_execution_config: c as LLMSimulationExecutionConfigDTO })"
      />
      <p v-if="executionTargetError" class="text-[11px] text-red-500 mt-1.5">
        {{ executionTargetError }}
      </p>
    </Fieldset>
  </div>
</template>

<script setup lang="ts">
// The model's THREE LLM configs, lifted out of GeneralForm into their own tab:
//   • Model generation & chat LLM — slim model.llm_config (model_generation)
//   • Simulation generation LLM — slim simulation_config.llm_simulation_generation_config
//   • Simulation execution LLM — full simulation_config.llm_simulation_execution_config
// Emits a full updated Model; the editor pushes it to the store. The conditional-
// required hints mirror the backend 422 enforcement (the backend stays authority).
import Fieldset from 'primevue/fieldset'
import LlmConfigForm from './LlmConfigForm.vue'
import LlmSlimConfigForm from './LlmSlimConfigForm.vue'
import { computed } from 'vue'
import type { Model, SimulationConfigDTO, LLMSimulationExecutionConfigDTO } from '@/shared/types'
import { modelUsesLLMForGeneration, modelUsesLLMForExecution } from '@/shared/validation/modelRules'

const props = defineProps<{ modelValue: Model }>()
const emit = defineEmits<{ 'update:modelValue': [v: Model] }>()

function patch(partial: Partial<Model>) {
  emit('update:modelValue', { ...props.modelValue, ...partial })
}

const simulationConfig = computed<SimulationConfigDTO>(() => props.modelValue.simulation_config)

function patchSimulationConfig(partial: Partial<SimulationConfigDTO>) {
  patch({ simulation_config: { ...simulationConfig.value, ...partial } })
}

const generationTargetError = computed(() =>
  modelUsesLLMForGeneration(props.modelValue) &&
  !simulationConfig.value.llm_simulation_generation_config?.target?.provider_uid
    ? 'Select a simulation-generation model — the model uses the LLM to seed simulations.'
    : ''
)
const executionTargetError = computed(() =>
  modelUsesLLMForExecution(props.modelValue) &&
  !simulationConfig.value.llm_simulation_execution_config?.target?.provider_uid
    ? 'Select a simulation-execution model — the model uses the LLM to run simulations.'
    : ''
)
</script>
