<template>
  <div class="space-y-3">
    <!-- Name -->
    <div>
      <label class="text-xs text-zinc-500 mb-1 block">Name</label>
      <InputText
        :modelValue="modelValue.name"
        class="w-full"
        :maxlength="MODEL_NAME_MAX"
        placeholder="Model name"
        @update:modelValue="(v) => patch({ name: v ?? '' })"
      />
      <p v-if="nameError" class="text-[11px] text-red-500 mt-0.5">{{ nameError }}</p>
    </div>

    <!-- Description -->
    <div>
      <label class="text-xs text-zinc-500 mb-1 block">Description</label>
      <Textarea
        :modelValue="modelValue.description"
        class="w-full text-sm"
        rows="3"
        autoResize
        :maxlength="MODEL_DESCRIPTION_MAX"
        @update:modelValue="(v) => patch({ description: v ?? '' })"
      />
      <p v-if="descriptionError" class="text-[11px] text-red-500 mt-0.5">{{ descriptionError }}</p>
    </div>

    <!-- Spatial toggle + cell size -->
    <div class="flex items-end gap-4">
      <label class="flex items-center gap-2 text-sm text-zinc-700 cursor-pointer">
        <ToggleSwitch :modelValue="modelValue.is_spatial" @update:modelValue="(v) => patch({ is_spatial: !!v })" />
        <span>Spatial model</span>
      </label>
      <div v-if="modelValue.is_spatial" class="w-40">
        <label class="text-xs text-zinc-500 mb-1 block">Spatial cell size</label>
        <InputNumber
          :modelValue="modelValue.spatial_cell_size ?? null"
          class="w-full"
          :min="0"
          @update:modelValue="(v) => patch({ spatial_cell_size: v ?? 0 })"
        />
      </div>
    </div>

    <!-- Simulation configuration (update mode) -->
    <Fieldset legend="Simulation configuration" :toggleable="true" :collapsed="false">
      <div class="flex gap-4">
        <div class="w-48">
          <label class="text-xs text-zinc-500 mb-1 block">Update mode</label>
          <Select
            :modelValue="simulationConfig.update_mode"
            :options="MODEL_UPDATE_MODES as unknown as string[]"
            class="w-full"
            @update:modelValue="(v) => patchSimulationConfig({ update_mode: v })"
          />
        </div>
      </div>
    </Fieldset>

    <!-- The three LLM configs moved to their own "LLM" tab (ModelLlmForm). -->
  </div>
</template>

<script setup lang="ts">
// Edits model-level basics: name, description, is_spatial, spatial_cell_size, and
// the simulation update_mode. The model's THREE LLM configs live in their own
// "LLM" tab now (ModelLlmForm). Emits a full updated Model; the parent
// (ModelEditorView) pushes it to the store.
import InputText from 'primevue/inputtext'
import Textarea from 'primevue/textarea'
import Select from 'primevue/select'
import InputNumber from 'primevue/inputnumber'
import ToggleSwitch from 'primevue/toggleswitch'
import Fieldset from 'primevue/fieldset'
import { computed } from 'vue'
import type { Model, SimulationConfigDTO } from '@/shared/types'
import {
  MODEL_UPDATE_MODES,
  MODEL_NAME_MAX,
  MODEL_DESCRIPTION_MAX,
  isValidName,
  isValidDescription,
} from '@/shared/validation/modelRules'

const props = defineProps<{ modelValue: Model }>()
const emit = defineEmits<{ 'update:modelValue': [v: Model] }>()

function patch(partial: Partial<Model>) {
  emit('update:modelValue', { ...props.modelValue, ...partial })
}

// simulation_config is guaranteed by normalizeModel, but guard anyway so the
// template never reads through undefined on a momentarily-stale model.
const simulationConfig = computed<SimulationConfigDTO>(() => props.modelValue.simulation_config)

function patchSimulationConfig(partial: Partial<SimulationConfigDTO>) {
  patch({ simulation_config: { ...simulationConfig.value, ...partial } })
}

const nameError = computed(() =>
  isValidName(props.modelValue.name) ? '' : `Name must be ≤ ${MODEL_NAME_MAX} characters.`
)
const descriptionError = computed(() =>
  isValidDescription(props.modelValue.description)
    ? ''
    : `Description must be ≤ ${MODEL_DESCRIPTION_MAX} characters.`
)
</script>
