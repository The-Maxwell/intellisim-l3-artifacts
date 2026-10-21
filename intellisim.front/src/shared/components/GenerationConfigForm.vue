<template>
  <div class="space-y-3">
    <div v-for="p in params" :key="p.key">
      <label class="text-xs text-zinc-500 mb-1 block">
        {{ p.label || p.key }}<span v-if="p.required" class="text-red-500"> *</span>
      </label>

      <!-- numeric -->
      <InputNumber
        v-if="p.type === 'double' || p.type === 'int' || p.type === 'uint'"
        :model-value="numberOf(p.key)"
        class="w-full"
        :min="p.min"
        :max="p.max"
        :minFractionDigits="p.type === 'double' ? 0 : undefined"
        :maxFractionDigits="p.type === 'double' ? 4 : 0"
        :placeholder="placeholder(p)"
        @update:model-value="(v) => setNumber(p, v)"
      />

      <!-- enum -->
      <Select
        v-else-if="p.type === 'enum'"
        :model-value="stringOf(p.key)"
        :options="p.enum_values ?? []"
        class="w-full"
        showClear
        :placeholder="placeholder(p)"
        @update:model-value="(v) => setString(p.key, v)"
      />

      <!-- bool -->
      <ToggleSwitch
        v-else-if="p.type === 'bool'"
        :model-value="boolOf(p.key)"
        @update:model-value="(v) => setBool(p.key, v)"
      />

      <!-- string list -->
      <InputChips
        v-else-if="p.type === 'string_list'"
        :model-value="listOf(p.key)"
        class="w-full"
        @update:model-value="(v) => setList(p.key, v)"
      />

      <!-- plain string -->
      <InputText
        v-else
        :model-value="stringOf(p.key)"
        class="w-full"
        @update:model-value="(v) => setString(p.key, v)"
      />

      <p v-if="p.description" class="text-[11px] text-zinc-400 mt-0.5">{{ p.description }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import InputNumber from 'primevue/inputnumber'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import ToggleSwitch from 'primevue/toggleswitch'
import InputChips from 'primevue/inputchips'
import type { ConfigParam, ConfigValue, GenerationParams } from '@/shared/types'

const props = defineProps<{
  params: ConfigParam[]
  modelValue: GenerationParams
}>()

const emit = defineEmits<{ 'update:modelValue': [value: GenerationParams] }>()

function emitWith(key: string, value: ConfigValue | undefined) {
  const next: GenerationParams = { ...props.modelValue }
  if (value === undefined) delete next[key]
  else next[key] = value
  emit('update:modelValue', next)
}

const numberOf = (k: string) =>
  props.modelValue[k]?.double ?? props.modelValue[k]?.int ?? props.modelValue[k]?.uint ?? null
const stringOf = (k: string) => props.modelValue[k]?.string ?? ''
const boolOf = (k: string) => props.modelValue[k]?.bool ?? false
const listOf = (k: string) => props.modelValue[k]?.list ?? []

function placeholder(p: ConfigParam): string {
  const d = p.default
  if (!d) return ''
  return String(d.double ?? d.int ?? d.uint ?? d.string ?? '')
}

function setNumber(p: ConfigParam, v: number | null) {
  if (v === null || v === undefined) return emitWith(p.key, undefined)
  if (p.type === 'double') emitWith(p.key, { double: v })
  else if (p.type === 'int') emitWith(p.key, { int: Math.trunc(v) })
  else emitWith(p.key, { uint: Math.max(0, Math.trunc(v)) })
}
function setString(key: string, v: string | null | undefined) {
  emitWith(key, v ? { string: v } : undefined)
}
function setBool(key: string, v: boolean) {
  emitWith(key, { bool: v })
}
function setList(key: string, v: string[]) {
  emitWith(key, v && v.length ? { list: v } : undefined)
}
</script>
