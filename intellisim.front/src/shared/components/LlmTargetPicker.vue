<template>
  <div class="flex items-center gap-2">
    <Select
      :modelValue="selectedKey"
      :options="options"
      optionLabel="label"
      optionValue="key"
      :placeholder="providerStore.loading ? 'Loading models…' : 'Select model'"
      class="min-w-48"
      size="small"
      :disabled="providerStore.loading"
      @update:modelValue="onModelChange"
    />
    <Select
      v-if="efforts.length > 0"
      :modelValue="modelValue?.effort || ''"
      :options="effortOptions"
      optionLabel="label"
      optionValue="value"
      class="w-28"
      size="small"
      @update:modelValue="onEffortChange"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import Select from 'primevue/select'
import type { LLMTargetDTO } from '@/shared/types'
import { useLlmProviderStore } from '@/shared/stores/llmProviderStore'

interface TargetOption {
  key: string
  label: string
  provider_uid: string
  model_uid: string
  efforts: string[]
}

const props = defineProps<{ modelValue: LLMTargetDTO | null }>()
const emit = defineEmits<{ 'update:modelValue': [target: LLMTargetDTO] }>()

const providerStore = useLlmProviderStore()

onMounted(() => {
  providerStore.loadProviders()
})

const keyOf = (p: string, m: string) => `${p}::${m}`

// Only models that can actually chat are offered — an unavailable target
// cannot even be submitted from the UI (model availability principle).
const options = computed<TargetOption[]>(() =>
  providerStore.providers.flatMap((p) =>
    p.models
      .filter((m) => m.capabilities.includes('chat'))
      .map((m) => ({
        key: keyOf(p.uid, m.uid),
        label: `${p.name} · ${m.name}`,
        provider_uid: p.uid,
        model_uid: m.uid,
        efforts: m.efforts ?? [],
      }))
  )
)

const selectedKey = computed(() =>
  props.modelValue ? keyOf(props.modelValue.provider_uid, props.modelValue.model_uid) : null
)

const selectedOption = computed(() => options.value.find((o) => o.key === selectedKey.value))
const efforts = computed(() => selectedOption.value?.efforts ?? [])
const effortOptions = computed(() => [
  { label: 'default', value: '' },
  ...efforts.value.map((e) => ({ label: e, value: e })),
])

function onModelChange(key: string) {
  const opt = options.value.find((o) => o.key === key)
  if (!opt) return
  // Keep effort only if the new model allows it.
  const effort = props.modelValue?.effort && opt.efforts.includes(props.modelValue.effort)
    ? props.modelValue.effort
    : undefined
  emit('update:modelValue', { provider_uid: opt.provider_uid, model_uid: opt.model_uid, effort })
}

function onEffortChange(value: string) {
  if (!props.modelValue) return
  emit('update:modelValue', { ...props.modelValue, effort: value || undefined })
}
</script>
