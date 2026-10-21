<template>
  <Dialog
    :visible="visible"
    modal
    header="Create simulation"
    :style="{ width: '560px' }"
    @update:visible="emit('update:visible', $event)"
  >
    <div class="space-y-4">
      <div>
        <label class="text-xs font-medium text-zinc-600 mb-1 block">Model</label>
        <!-- Preset model (opened from a specific model): locked, no picker. -->
        <InputText
          v-if="presetModelId"
          :modelValue="presetModelName || 'Selected model'"
          disabled
          class="w-full"
          size="small"
        />
        <template v-else>
          <Select
            v-model="modelId"
            :options="modelOptions"
            optionLabel="label"
            optionValue="value"
            optionDisabled="disabled"
            :loading="modelsLoading"
            :placeholder="modelsLoading ? 'Loading models…' : 'Select a model'"
            class="w-full"
            size="small"
          />
          <p class="text-xs text-zinc-400 mt-1">
            Only simulation-ready models can be used. Not-ready models are disabled.
          </p>
        </template>
      </div>

      <div>
        <label class="text-xs font-medium text-zinc-600 mb-1 block">Name</label>
        <InputText v-model="name" class="w-full" size="small" placeholder="Untitled simulation" />
      </div>

      <div class="flex gap-3">
        <div class="flex-1">
          <label class="text-xs font-medium text-zinc-600 mb-1 block">Steps</label>
          <InputNumber v-model="steps" class="w-full" :min="1" placeholder="100" />
        </div>
        <div class="flex-1">
          <label class="text-xs font-medium text-zinc-600 mb-1 block">Seed (optional)</label>
          <InputNumber v-model="seed" class="w-full" placeholder="random" :useGrouping="false" />
        </div>
      </div>

      <div>
        <label class="text-xs font-medium text-zinc-600 mb-1 block">
          Generation model override (optional)
        </label>
        <LlmExecutionConfig v-model="generationTarget" capability="simulation_generation" />
      </div>

      <div>
        <label class="text-xs font-medium text-zinc-600 mb-1 block">
          Execution model override (optional)
        </label>
        <LlmExecutionConfig v-model="executionTarget" capability="simulation_execution" />
      </div>
    </div>
    <template #footer>
      <div class="flex justify-end gap-2">
        <Button label="Cancel" severity="secondary" size="small" @click="emit('update:visible', false)" />
        <Button
          label="Create"
          size="small"
          :loading="creating"
          :disabled="!modelId"
          @click="createSimulation"
        />
      </div>
    </template>
  </Dialog>
</template>

<script setup lang="ts">
// Reusable "create simulation" dialog. Opened from the simulations list (no
// preset — user picks a model) and from a specific model (models list row /
// model editor) where `presetModelId` locks the model. Self-contained: it
// creates via the store and navigates to the new simulation on success.
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import Dialog from 'primevue/dialog'
import Select from 'primevue/select'
import InputText from 'primevue/inputtext'
import InputNumber from 'primevue/inputnumber'
import Button from 'primevue/button'
import LlmExecutionConfig from '@/shared/components/LlmExecutionConfig.vue'
import { useSimulationStore } from '@/modules/simulation/stores/simulationStore'
import { useModelApi } from '@/modules/model/composables/useModelApi'
import { useErrorHandler } from '@/shared/composables/useErrorHandler'
import type { ExecutionLlmTarget, ModelSummary, Simulation } from '@/shared/types'

const props = defineProps<{
  visible: boolean
  presetModelId?: string | null
  presetModelName?: string
}>()
const emit = defineEmits<{
  'update:visible': [value: boolean]
  created: [sim: Simulation]
}>()

const router = useRouter()
const store = useSimulationStore()
const modelApi = useModelApi()
const { handleError } = useErrorHandler()

const creating = ref(false)
const modelId = ref<string | null>(null)
const name = ref('')
const steps = ref<number | null>(100)
const seed = ref<number | null>(null)
const generationTarget = ref<ExecutionLlmTarget | null>(null)
const executionTarget = ref<ExecutionLlmTarget | null>(null)

// Model picker — only used when no preset model is supplied.
const models = ref<ModelSummary[]>([])
const modelsLoading = ref(false)
const modelOptions = computed(() =>
  models.value.map((m) => ({
    value: m.id,
    label: m.simulation_ready ? m.name || 'Untitled model' : `${m.name || 'Untitled model'} (not ready)`,
    disabled: !m.simulation_ready,
  }))
)

// Reset the form on each open; preselect/lock the preset model, otherwise load
// the picker. Failures surface via toast; the list is left empty.
watch(
  () => props.visible,
  (open) => {
    if (!open) return
    name.value = ''
    steps.value = 100
    seed.value = null
    generationTarget.value = null
    executionTarget.value = null
    if (props.presetModelId) {
      modelId.value = props.presetModelId
    } else {
      modelId.value = null
      void loadModels()
    }
  }
)

async function loadModels() {
  modelsLoading.value = true
  try {
    const resp = await modelApi.listModels(1, 100)
    models.value = resp.models ?? []
  } catch (err) {
    handleError(err, 'Load Models')
  } finally {
    modelsLoading.value = false
  }
}

async function createSimulation() {
  if (creating.value || !modelId.value) return
  creating.value = true
  try {
    const sim = await store.create({
      model_id: modelId.value,
      name: name.value.trim() || undefined,
      steps: steps.value ?? undefined,
      seed: seed.value ?? undefined,
      llm_generation_target: generationTarget.value,
      llm_execution_target: executionTarget.value,
    })
    emit('created', sim)
    emit('update:visible', false)
    router.push(`/simulations/${sim.id}`)
  } catch (err) {
    // 422 model-not-ready and other backend errors surface here.
    handleError(err, 'Create Simulation')
  } finally {
    creating.value = false
  }
}
</script>
