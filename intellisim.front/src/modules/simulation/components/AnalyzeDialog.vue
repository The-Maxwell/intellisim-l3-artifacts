<template>
  <Dialog
    :visible="visible"
    modal
    header="New analysis"
    :style="{ width: '560px' }"
    @update:visible="(v: boolean) => emit('update:visible', v)"
  >
    <div class="space-y-4">
      <div>
        <label class="text-xs text-zinc-500 mb-1 block">Name (optional)</label>
        <InputText v-model="name" class="w-full" :maxlength="ANALYSIS_NAME_MAX" placeholder="Analysis name" />
      </div>

      <div>
        <label class="text-xs text-zinc-500 mb-1 block">Analysis context / questions *</label>
        <Textarea v-model="contextMessage" class="w-full" rows="4" :maxlength="CONTEXT_MESSAGE_MAX"
          placeholder="What should the analysis focus on? e.g. explain the population collapse and its causes." />
        <div class="text-xs text-zinc-400 text-right">{{ contextMessage.length }}/{{ CONTEXT_MESSAGE_MAX }}</div>
      </div>

      <div class="flex items-center gap-2">
        <div class="flex-1">
          <label class="text-xs text-zinc-500 mb-1 block">Report language</label>
          <Select v-model="language" :options="languageOptions" optionLabel="label" optionValue="value" class="w-full" />
        </div>
      </div>

      <div>
        <label class="text-xs text-zinc-500 mb-1 block">Analysis model *</label>
        <LlmExecutionConfig v-model="target" capability="results_analysis" />
      </div>

      <details class="border border-zinc-200 rounded p-2">
        <summary class="text-sm cursor-pointer select-none">Scope (optional — defaults to the whole run)</summary>
        <div class="space-y-3 pt-3">
          <div class="flex gap-2">
            <div class="flex-1">
              <label class="text-xs text-zinc-500 mb-1 block">Step from</label>
              <InputNumber v-model="stepFrom" class="w-full" :min="summary?.step_range.min ?? 0" :max="summary?.step_range.max ?? undefined" />
            </div>
            <div class="flex-1">
              <label class="text-xs text-zinc-500 mb-1 block">Step to</label>
              <InputNumber v-model="stepTo" class="w-full" :min="summary?.step_range.min ?? 0" :max="summary?.step_range.max ?? undefined" />
            </div>
            <div class="w-28">
              <label class="text-xs text-zinc-500 mb-1 block">Sampling</label>
              <InputNumber v-model="sampling" class="w-full" :min="1" placeholder="1" />
            </div>
          </div>
          <div>
            <label class="text-xs text-zinc-500 mb-1 block">Agents</label>
            <MultiSelect v-model="agentUids" :options="summary?.agent_types ?? []" optionLabel="name" optionValue="uid"
              class="w-full" display="chip" placeholder="All agents" />
          </div>
          <div>
            <label class="text-xs text-zinc-500 mb-1 block">Properties</label>
            <MultiSelect v-model="propertyUids" :options="propertyOptions" optionLabel="label" optionValue="uid"
              class="w-full" display="chip" placeholder="All properties" />
          </div>
          <div>
            <label class="text-xs text-zinc-500 mb-1 block">Focus areas</label>
            <MultiSelect v-model="focusAreas" :options="focusAreaOptions" optionLabel="label" optionValue="value"
              class="w-full" display="chip" placeholder="All aspects" />
          </div>
        </div>
      </details>
    </div>

    <template #footer>
      <Button label="Cancel" text @click="emit('update:visible', false)" />
      <Button label="Run analysis" icon="pi pi-sparkles" :disabled="!canSubmit" :loading="submitting" @click="submit" />
    </template>
  </Dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Textarea from 'primevue/textarea'
import Select from 'primevue/select'
import MultiSelect from 'primevue/multiselect'
import InputNumber from 'primevue/inputnumber'
import Button from 'primevue/button'
import LlmExecutionConfig from '@/shared/components/LlmExecutionConfig.vue'
import { useAnalysisApi } from '../composables/useAnalysisApi'
import {
  ANALYSIS_LANGUAGES, ANALYSIS_NAME_MAX, CONTEXT_MESSAGE_MAX, FOCUS_AREAS,
} from '@/shared/validation/analysisRules'
import type {
  AnalysisFocusArea, AnalysisLanguage, AnalyzeRequest, ExecutionLlmTarget, ResultsSummary,
} from '@/shared/types'

const props = defineProps<{ visible: boolean; simulationId: string }>()
const emit = defineEmits<{ 'update:visible': [v: boolean]; submit: [req: AnalyzeRequest] }>()

const api = useAnalysisApi()

// PrimeVue's options prop expects a mutable array; the validation constants are
// `as const` (readonly), so copy them into plain mutable arrays here.
const languageOptions = [...ANALYSIS_LANGUAGES]
const focusAreaOptions = [...FOCUS_AREAS]

const name = ref('')
const contextMessage = ref('')
const language = ref<AnalysisLanguage>('en')
const target = ref<ExecutionLlmTarget | null>(null)
const stepFrom = ref<number | null>(null)
const stepTo = ref<number | null>(null)
const sampling = ref<number | null>(null)
const agentUids = ref<string[]>([])
const propertyUids = ref<string[]>([])
const focusAreas = ref<AnalysisFocusArea[]>([])
const summary = ref<ResultsSummary | null>(null)
const submitting = ref(false)

const canSubmit = computed(
  () => contextMessage.value.trim().length > 0 && !!target.value?.provider_uid && !!target.value?.model_uid
)

// Property options with shape entries labelled explicitly — the summary mixes
// agent and shape properties (entity_type), and the two can share a name.
const propertyOptions = computed(() =>
  (summary.value?.properties ?? []).map((p) => ({
    ...p,
    label: p.entity_type === 'shape' ? `${p.name} (shape)` : p.name,
  }))
)

watch(
  () => props.visible,
  async (v) => {
    if (!v) return
    // Reset + load selectable dimensions when the dialog opens.
    name.value = ''
    contextMessage.value = ''
    language.value = 'en'
    target.value = null
    stepFrom.value = null
    stepTo.value = null
    sampling.value = null
    agentUids.value = []
    propertyUids.value = []
    focusAreas.value = []
    try {
      summary.value = await api.resultsSummary(props.simulationId)
    } catch {
      summary.value = null
    }
  }
)

async function submit(): Promise<void> {
  if (!canSubmit.value || !target.value) return
  submitting.value = true
  try {
    const req: AnalyzeRequest = {
      name: name.value.trim() || undefined,
      context_message: contextMessage.value.trim(),
      language: language.value,
      llm_target: target.value,
      selection: {
        step_from: stepFrom.value ?? undefined,
        step_to: stepTo.value ?? undefined,
        sampling: sampling.value ?? undefined,
        agent_uids: agentUids.value.length ? agentUids.value : undefined,
        property_uids: propertyUids.value.length ? propertyUids.value : undefined,
        focus_areas: focusAreas.value.length ? focusAreas.value : undefined,
      },
    }
    emit('submit', req)
  } finally {
    submitting.value = false
  }
}
</script>
