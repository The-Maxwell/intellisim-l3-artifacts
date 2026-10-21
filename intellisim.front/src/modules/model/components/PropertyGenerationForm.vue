<template>
  <div class="space-y-2">
    <!-- Mode -->
    <div class="w-40">
      <label class="text-[11px] text-zinc-500 mb-0.5 block">Generation mode</label>
      <Select
        :modelValue="mode"
        :options="MODE_OPTIONS"
        optionLabel="label"
        optionValue="value"
        class="w-full"
        size="small"
        showClear
        placeholder="default"
        @update:modelValue="onModeChange"
      />
    </div>

    <!-- random → distribution + params -->
    <template v-if="value?.mode === 'random'">
      <div class="w-40">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Distribution</label>
        <Select
          :modelValue="value.distribution ?? null"
          :options="DISTRIBUTION_OPTIONS"
          optionLabel="label"
          optionValue="value"
          class="w-full"
          size="small"
          placeholder="select"
          @update:modelValue="onDistributionChange"
        />
      </div>

      <!-- normal: mean + std_dev -->
      <div v-if="value.distribution === 'normal'" class="flex gap-2">
        <div class="w-28">
          <label class="text-[11px] text-zinc-500 mb-0.5 block">Mean</label>
          <InputNumber
            :modelValue="value.mean ?? null"
            class="w-full"
            size="small"
            @update:modelValue="(v) => patch({ mean: v ?? undefined })"
          />
        </div>
        <div class="w-28">
          <label class="text-[11px] text-zinc-500 mb-0.5 block">Std dev</label>
          <InputNumber
            :modelValue="value.std_dev ?? null"
            class="w-full"
            size="small"
            :min="0"
            @update:modelValue="(v) => patch({ std_dev: v ?? undefined })"
          />
        </div>
      </div>

      <!-- uniform: min + max (optional) -->
      <div v-else-if="value.distribution === 'uniform'" class="flex gap-2">
        <div class="w-28">
          <label class="text-[11px] text-zinc-500 mb-0.5 block">Min</label>
          <InputNumber
            :modelValue="value.min ?? null"
            class="w-full"
            size="small"
            @update:modelValue="(v) => patch({ min: v ?? undefined })"
          />
        </div>
        <div class="w-28">
          <label class="text-[11px] text-zinc-500 mb-0.5 block">Max</label>
          <InputNumber
            :modelValue="value.max ?? null"
            class="w-full"
            size="small"
            @update:modelValue="(v) => patch({ max: v ?? undefined })"
          />
        </div>
      </div>

      <!-- choice: weighted choices editor -->
      <div v-else-if="value.distribution === 'choice'" class="space-y-1.5">
        <label class="text-[11px] text-zinc-500 block">Choices (value + weight)</label>
        <div
          v-for="(c, i) in value.choices ?? []"
          :key="i"
          class="flex items-end gap-2"
        >
          <div class="flex-1 min-w-0">
            <InputText
              :modelValue="choiceValueString(c)"
              class="w-full"
              size="small"
              placeholder="value"
              @update:modelValue="(v) => patchChoice(i, { value: v ?? '' })"
            />
          </div>
          <div class="w-24">
            <InputNumber
              :modelValue="c.weight ?? null"
              class="w-full"
              size="small"
              :min="0.001"
              :minFractionDigits="0"
              :maxFractionDigits="4"
              placeholder="weight"
              @update:modelValue="(v) => patchChoice(i, { weight: v ?? 1 })"
            />
          </div>
          <Button
            icon="pi pi-trash"
            severity="danger"
            text
            rounded
            size="small"
            aria-label="Remove choice"
            @click="removeChoice(i)"
          />
        </div>
        <Button label="Add choice" icon="pi pi-plus" size="small" text @click="addChoice" />
      </div>
    </template>

    <!-- llm → prompt context -->
    <div v-else-if="value?.mode === 'llm'">
      <label class="text-[11px] text-zinc-500 mb-0.5 block">Prompt context</label>
      <Textarea
        :modelValue="value.prompt_context ?? ''"
        class="w-full text-xs"
        rows="2"
        autoResize
        @update:modelValue="(v) => patch({ prompt_context: v || '' })"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
// Sub-form for an AgentPropertyDTO.generation (PropertyGenerationDTO): how the
// initial value of a property is produced. mode default → no extra fields (clears
// generation); random → distribution (uniform/normal/choice) + its params; llm →
// a prompt-context textarea. Emits the updated generation, or undefined to clear
// it (mode=default). The parent (PropertyEditor) owns the surrounding property.
import Select from 'primevue/select'
import InputText from 'primevue/inputtext'
import InputNumber from 'primevue/inputnumber'
import Textarea from 'primevue/textarea'
import Button from 'primevue/button'
import { computed } from 'vue'
import type {
  PropertyGenerationDTO,
  PropertyGenerationMode,
  PropertyGenerationDistribution,
  ChoiceWeightDTO,
} from '@/shared/types'

const MODE_OPTIONS: { value: PropertyGenerationMode; label: string }[] = [
  { value: 'default', label: 'Default' },
  { value: 'random', label: 'Random' },
  { value: 'llm', label: 'LLM' },
]
const DISTRIBUTION_OPTIONS: { value: PropertyGenerationDistribution; label: string }[] = [
  { value: 'uniform', label: 'Uniform' },
  { value: 'normal', label: 'Normal' },
  { value: 'choice', label: 'Choice' },
]

const props = defineProps<{ modelValue?: PropertyGenerationDTO }>()
const emit = defineEmits<{ 'update:modelValue': [v: PropertyGenerationDTO | undefined] }>()

const value = computed<PropertyGenerationDTO | undefined>(() => props.modelValue)
const mode = computed<PropertyGenerationMode>(() => props.modelValue?.mode ?? 'default')

function patch(partial: Partial<PropertyGenerationDTO>) {
  emit('update:modelValue', { ...(props.modelValue ?? {}), ...partial })
}

// Mode change: default clears the whole generation block; random/llm seed it.
function onModeChange(next: PropertyGenerationMode | null) {
  if (!next || next === 'default') {
    emit('update:modelValue', undefined)
    return
  }
  if (next === 'random') {
    emit('update:modelValue', { mode: 'random', distribution: 'uniform' })
  } else {
    emit('update:modelValue', { mode: 'llm', prompt_context: props.modelValue?.prompt_context })
  }
}

// Distribution change: reset distribution-specific params.
function onDistributionChange(dist: PropertyGenerationDistribution) {
  const base: PropertyGenerationDTO = { mode: 'random', distribution: dist }
  if (dist === 'choice') base.choices = props.modelValue?.choices ?? []
  emit('update:modelValue', base)
}

// --- choices ---
function choiceValueString(c: ChoiceWeightDTO): string {
  return c.value == null ? '' : String(c.value)
}
function patchChoice(i: number, partial: Partial<ChoiceWeightDTO>) {
  const choices = (props.modelValue?.choices ?? []).map((c, idx) => (idx === i ? { ...c, ...partial } : c))
  patch({ choices })
}
function addChoice() {
  const choices = [...(props.modelValue?.choices ?? []), { value: '', weight: 1 } as ChoiceWeightDTO]
  patch({ choices })
}
function removeChoice(i: number) {
  const choices = (props.modelValue?.choices ?? []).filter((_, idx) => idx !== i)
  patch({ choices })
}
</script>
