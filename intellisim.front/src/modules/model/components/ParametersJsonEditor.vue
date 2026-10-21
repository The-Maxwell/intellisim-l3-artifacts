<template>
  <div>
    <label class="text-[11px] text-zinc-500 mb-0.5 block">{{ label }}</label>
    <Textarea
      :modelValue="text"
      class="w-full text-xs font-mono"
      :class="{ 'p-invalid': error }"
      rows="4"
      autoResize
      spellcheck="false"
      :placeholder="placeholder"
      @update:modelValue="onInput"
    />
    <p v-if="error" class="text-[11px] text-red-500 mt-0.5">{{ error }}</p>
    <p v-else-if="hint" class="text-[11px] text-zinc-400 mt-0.5">{{ hint }}</p>
  </div>
</template>

<script setup lang="ts">
// A JSON-object editor for a free-form `parameters` map (custom condition /
// custom action). Parses the textarea on every input; only emits an updated map
// when the JSON is a valid object. Invalid JSON shows an inline error and does
// NOT propagate, so the parent DTO never holds a half-typed value. Keeps its own
// draft text so the user can type freely (including transiently-invalid states).
import Textarea from 'primevue/textarea'
import { ref, watch } from 'vue'

const props = withDefaults(
  defineProps<{
    modelValue: Record<string, unknown> | undefined
    label?: string
    hint?: string
    placeholder?: string
  }>(),
  { label: 'Parameters (JSON)', hint: '', placeholder: '{ }' }
)
const emit = defineEmits<{ 'update:modelValue': [v: Record<string, unknown> | undefined] }>()

const error = ref('')

function serialize(v: Record<string, unknown> | undefined): string {
  if (v === undefined || v === null) return ''
  try {
    return JSON.stringify(v, null, 2)
  } catch {
    return ''
  }
}

const text = ref(serialize(props.modelValue))

// Re-sync the draft from the model when the external value changes but not while
// the current draft already serializes to the same object (avoids clobbering the
// cursor mid-edit on the round-trip of our own emit).
watch(
  () => props.modelValue,
  (v) => {
    if (serialize(v) !== normalized(text.value)) {
      text.value = serialize(v)
      error.value = ''
    }
  }
)

// Normalize a draft to the canonical serialization for comparison (empty when
// unparseable), so an equivalent-but-differently-formatted external value does
// not reset the user's formatting.
function normalized(draft: string): string {
  const t = draft.trim()
  if (t === '') return ''
  try {
    const parsed = JSON.parse(t)
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return JSON.stringify(parsed, null, 2)
    }
    return t
  } catch {
    return t
  }
}

function onInput(v: string | undefined) {
  const raw = v ?? ''
  text.value = raw
  const t = raw.trim()
  if (t === '') {
    error.value = ''
    emit('update:modelValue', undefined)
    return
  }
  let parsed: unknown
  try {
    parsed = JSON.parse(t)
  } catch {
    error.value = 'Invalid JSON'
    return
  }
  if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) {
    error.value = 'Expected a JSON object'
    return
  }
  error.value = ''
  emit('update:modelValue', parsed as Record<string, unknown>)
}
</script>
