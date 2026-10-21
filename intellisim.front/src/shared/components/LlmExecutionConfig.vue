<template>
  <div class="space-y-2">
    <!-- Always visible: model + effort (+ reasoning indicator). These are the
         only controls most users touch; everything else is advanced. -->
    <div class="flex items-center gap-2 flex-wrap">
      <Select
        :modelValue="selectedKey"
        :options="options"
        optionLabel="label"
        optionValue="key"
        :placeholder="store.loading ? 'Loading models…' : 'Select execution model'"
        class="min-w-56"
        size="small"
        :disabled="store.loading"
        @update:modelValue="onModelChange"
      />
      <Select
        v-if="efforts.length"
        :modelValue="modelValue?.effort || ''"
        :options="effortOptions"
        optionLabel="label"
        optionValue="value"
        class="w-28"
        size="small"
        @update:modelValue="onEffortChange"
      />
      <Tag v-if="selected?.reasoning" value="reasoning" severity="info" />
    </div>

    <!-- Advanced: generation params + timeout. Collapsed by default so the picker
         stays a single row; auto-opens when the target already carries advanced
         values (editing an existing config). Only shown once a model is chosen. -->
    <Fieldset
      v-if="modelValue && modelValue.model_uid"
      v-model:collapsed="advancedCollapsed"
      legend="Advanced"
      :toggleable="true"
      class="text-sm"
    >
      <div class="space-y-3">
        <GenerationConfigForm
          v-if="selected && selected.config_params?.length"
          :params="selected.config_params"
          :modelValue="modelValue?.generation ?? {}"
          @update:modelValue="onGenerationChange"
        />

        <div>
          <label class="text-xs text-zinc-500 mb-1 block">LLM Timeout (ms)</label>
          <InputNumber
            :model-value="modelValue?.timeout_ms ?? null"
            class="w-full"
            :min="1"
            placeholder="30000"
            @update:model-value="onTimeoutChange"
          />
        </div>
      </div>
    </Fieldset>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import Select from 'primevue/select'
import InputNumber from 'primevue/inputnumber'
import Tag from 'primevue/tag'
import Fieldset from 'primevue/fieldset'
import GenerationConfigForm from './GenerationConfigForm.vue'
import type { ExecutionLlmTarget, GenerationParams, ModelInfo } from '@/shared/types'
import { useLlmProviderStore } from '@/shared/stores/llmProviderStore'

const props = defineProps<{
  modelValue: ExecutionLlmTarget | null
  capability?: string // e.g. 'simulation_execution'; omit to show all
}>()
const emit = defineEmits<{ 'update:modelValue': [t: ExecutionLlmTarget] }>()

const store = useLlmProviderStore()
onMounted(() => store.loadProviders())

const keyOf = (p: string, m: string) => `${p}::${m}`

interface Opt { key: string; label: string; provider_uid: string; model_uid: string; model: ModelInfo }

const options = computed<Opt[]>(() =>
  store.providers.flatMap((p) =>
    p.models
      .filter((m) => !props.capability || m.capabilities.includes(props.capability!))
      .map((m) => ({ key: keyOf(p.uid, m.uid), label: `${p.name} · ${m.name}`, provider_uid: p.uid, model_uid: m.uid, model: m }))
  )
)
const selectedKey = computed(() =>
  props.modelValue ? keyOf(props.modelValue.provider_uid, props.modelValue.model_uid) : null
)
const selected = computed(() => options.value.find((o) => o.key === selectedKey.value)?.model ?? null)
const efforts = computed(() => selected.value?.efforts ?? [])
const effortOptions = computed(() => [{ label: 'default', value: '' }, ...efforts.value.map((e) => ({ label: e, value: e }))])

// Advanced = generation params + timeout. True once the target holds any of them.
const hasAdvancedValues = computed(() => {
  const t = props.modelValue
  if (!t) return false
  const genCount = t.generation ? Object.keys(t.generation).length : 0
  return genCount > 0 || t.timeout_ms != null
})
// Collapsed by default; open when advanced values are already present. Only ever
// auto-opens (never auto-collapses) so a user's manual close is respected.
const advancedCollapsed = ref(!hasAdvancedValues.value)
watch(hasAdvancedValues, (has) => {
  if (has) advancedCollapsed.value = false
})

function onModelChange(key: string) {
  const opt = options.value.find((o) => o.key === key)
  if (!opt) return
  emit('update:modelValue', { provider_uid: opt.provider_uid, model_uid: opt.model_uid })
}
function onEffortChange(value: string) {
  if (!props.modelValue) return
  emit('update:modelValue', { ...props.modelValue, effort: value || undefined })
}
function onGenerationChange(generation: GenerationParams) {
  if (!props.modelValue) return
  emit('update:modelValue', { ...props.modelValue, generation: Object.keys(generation).length ? generation : undefined })
}
function onTimeoutChange(v: number | null) {
  if (!props.modelValue) return
  emit('update:modelValue', { ...props.modelValue, timeout_ms: v ?? undefined })
}
</script>
