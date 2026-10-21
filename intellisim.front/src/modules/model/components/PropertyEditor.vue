<template>
  <div class="space-y-2">
    <!-- NOTE: index key — items lack a stable client id under immutable updates; removing a non-last item mid-edit can shift focus. Acceptable for now; revisit with a stable-key composable if it becomes a problem. -->
    <div
      v-for="(prop, i) in modelValue"
      :key="i"
      class="rounded-lg border border-zinc-200 bg-white p-3 space-y-2"
    >
      <!-- Row 1: uid / name / type / remove -->
      <div class="flex items-end gap-2">
        <div class="flex-1 min-w-0">
          <label class="text-[11px] text-zinc-500 mb-0.5 block">UID</label>
          <InputText
            :modelValue="prop.uid"
            class="w-full"
            size="small"
            placeholder="energy"
            @update:modelValue="(v) => patch(i, { uid: v ?? '' })"
          />
        </div>
        <div class="flex-1 min-w-0">
          <label class="text-[11px] text-zinc-500 mb-0.5 block">Name</label>
          <InputText
            :modelValue="prop.name"
            class="w-full"
            size="small"
            placeholder="Energy"
            @update:modelValue="(v) => patch(i, { name: v ?? '' })"
          />
        </div>
        <div class="w-32">
          <label class="text-[11px] text-zinc-500 mb-0.5 block">Type</label>
          <Select
            :modelValue="prop.type"
            :options="PROPERTY_TYPES as unknown as string[]"
            class="w-full"
            size="small"
            @update:modelValue="(v) => onTypeChange(i, v)"
          />
        </div>
        <Button
          icon="pi pi-trash"
          severity="danger"
          text
          rounded
          size="small"
          aria-label="Remove property"
          @click="removeAt(i)"
        />
      </div>

      <!-- Description -->
      <div>
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Description</label>
        <InputText
          :modelValue="prop.description"
          class="w-full"
          size="small"
          @update:modelValue="(v) => patch(i, { description: v ?? '' })"
        />
      </div>

      <!-- Number-only: min / max -->
      <div v-if="prop.type === 'number'" class="flex gap-2">
        <div class="flex-1">
          <label class="text-[11px] text-zinc-500 mb-0.5 block">Min value</label>
          <InputNumber
            :modelValue="prop.min_value ?? null"
            class="w-full"
            size="small"
            @update:modelValue="(v) => patch(i, { min_value: v ?? undefined })"
          />
        </div>
        <div class="flex-1">
          <label class="text-[11px] text-zinc-500 mb-0.5 block">Max value</label>
          <InputNumber
            :modelValue="prop.max_value ?? null"
            class="w-full"
            size="small"
            @update:modelValue="(v) => patch(i, { max_value: v ?? undefined })"
          />
        </div>
      </div>

      <!-- Enum-only: allowed values -->
      <div v-if="prop.type === 'enum'">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Allowed values</label>
        <InputChips
          :modelValue="allowedAsStrings(prop)"
          class="w-full"
          size="small"
          @update:modelValue="(v) => patch(i, { allowed_values: v && v.length ? v : undefined })"
        />
      </div>

      <!-- Default value (type-conditional input) -->
      <div>
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Default value</label>
        <ColorPicker
          v-if="prop.type === 'color'"
          :modelValue="(prop.default_value as string) ?? '#888888'"
          format="hex"
          @update:modelValue="(v) => patch(i, { default_value: normalizeHex(v) })"
        />
        <ToggleSwitch
          v-else-if="prop.type === 'boolean'"
          :modelValue="(prop.default_value as boolean) ?? false"
          @update:modelValue="(v) => patch(i, { default_value: v })"
        />
        <Select
          v-else-if="prop.type === 'enum'"
          :modelValue="(prop.default_value as string) ?? null"
          :options="allowedAsStrings(prop)"
          class="w-full"
          size="small"
          showClear
          @update:modelValue="(v) => patch(i, { default_value: v ?? undefined })"
        />
        <InputNumber
          v-else-if="prop.type === 'number'"
          :modelValue="(prop.default_value as number) ?? null"
          class="w-full"
          size="small"
          @update:modelValue="(v) => patch(i, { default_value: v ?? undefined })"
        />
        <InputText
          v-else
          :modelValue="(prop.default_value as string) ?? ''"
          class="w-full"
          size="small"
          @update:modelValue="(v) => patch(i, { default_value: v ?? undefined })"
        />
      </div>

      <!-- LLM prompt context -->
      <div>
        <label class="text-[11px] text-zinc-500 mb-0.5 block">LLM prompt context</label>
        <Textarea
          :modelValue="prop.llm_prompt_context ?? ''"
          class="w-full text-xs"
          rows="2"
          autoResize
          @update:modelValue="(v) => patch(i, { llm_prompt_context: v || undefined })"
        />
      </div>

      <!-- Immutable -->
      <label class="flex items-center gap-2 text-xs text-zinc-600 cursor-pointer">
        <ToggleSwitch
          :modelValue="prop.immutable"
          @update:modelValue="(v) => patch(i, { immutable: !!v })"
        />
        <span>Immutable</span>
      </label>

      <!-- Value generation strategy (default / random / llm) -->
      <Fieldset
        :legend="generationLegend(prop)"
        :toggleable="true"
        :collapsed="!prop.generation"
        class="text-sm"
      >
        <PropertyGenerationForm
          :modelValue="prop.generation"
          @update:modelValue="(g) => patch(i, { generation: g })"
        />
      </Fieldset>
    </div>

    <Button
      label="Add property"
      icon="pi pi-plus"
      size="small"
      severity="secondary"
      outlined
      @click="addRow"
    />
  </div>
</template>

<script setup lang="ts">
// Edits an AgentPropertyDTO[] list (agent properties, environment global
// properties, event payloads). Type-conditional fields: min/max for number,
// allowed_values + enum default for enum, color picker for color. The parent
// owns the surrounding object; this component emits the full updated array.
import InputText from 'primevue/inputtext'
import InputNumber from 'primevue/inputnumber'
import Select from 'primevue/select'
import ToggleSwitch from 'primevue/toggleswitch'
import InputChips from 'primevue/inputchips'
import ColorPicker from 'primevue/colorpicker'
import Textarea from 'primevue/textarea'
import Button from 'primevue/button'
import Fieldset from 'primevue/fieldset'
import PropertyGenerationForm from './PropertyGenerationForm.vue'
import type { AgentPropertyDTO, PropertyType } from '@/shared/types'
import { PROPERTY_TYPES } from '@/shared/validation/modelRules'

const props = defineProps<{ modelValue: AgentPropertyDTO[] }>()
const emit = defineEmits<{ 'update:modelValue': [v: AgentPropertyDTO[]] }>()

function emitList(list: AgentPropertyDTO[]) {
  emit('update:modelValue', list)
}

function patch(index: number, partial: Partial<AgentPropertyDTO>) {
  const next = props.modelValue.map((p, i) => (i === index ? { ...p, ...partial } : p))
  emitList(next)
}

function onTypeChange(index: number, type: PropertyType) {
  // Reset type-specific fields so a stale default/min/max can't leak across types.
  const current = props.modelValue[index]
  if (!current) return
  const cleaned: AgentPropertyDTO = {
    ...current,
    type,
    default_value: undefined,
    min_value: type === 'number' ? current.min_value : undefined,
    max_value: type === 'number' ? current.max_value : undefined,
    allowed_values: type === 'enum' ? current.allowed_values : undefined,
  }
  emitList(props.modelValue.map((p, i) => (i === index ? cleaned : p)))
}

function removeAt(index: number) {
  emitList(props.modelValue.filter((_, i) => i !== index))
}

function addRow() {
  const blank: AgentPropertyDTO = {
    uid: '',
    name: '',
    type: 'number',
    description: '',
    immutable: false,
  }
  emitList([...props.modelValue, blank])
}

function allowedAsStrings(prop: AgentPropertyDTO): string[] {
  return (prop.allowed_values ?? []).map((v) => String(v))
}

function generationLegend(prop: AgentPropertyDTO): string {
  const m = prop.generation?.mode
  return m && m !== 'default' ? `Generation — ${m}` : 'Generation'
}

function normalizeHex(v: unknown): string {
  const s = String(v ?? '')
  return s.startsWith('#') ? s : `#${s}`
}
</script>
