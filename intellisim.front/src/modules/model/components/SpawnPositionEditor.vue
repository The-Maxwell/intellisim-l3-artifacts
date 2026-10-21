<template>
  <div class="rounded-md border border-zinc-200 bg-white p-2 space-y-2">
    <label class="text-[11px] font-semibold uppercase tracking-wide text-zinc-400">
      Spawn position
    </label>
    <Select
      :modelValue="modelValue?.type ?? null"
      :options="SPAWN_POSITION_TYPES as unknown as string[]"
      class="w-40"
      size="small"
      showClear
      placeholder="type"
      @update:modelValue="(v) => onTypeChange(v)"
    />
    <div v-if="modelValue?.type === 'offset'" class="flex gap-2">
      <div class="flex-1">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Offset X</label>
        <InputNumber
          :modelValue="modelValue?.offset_x ?? null"
          class="w-full"
          size="small"
          @update:modelValue="(v) => patchPos({ offset_x: v ?? undefined })"
        />
      </div>
      <div class="flex-1">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Offset Y</label>
        <InputNumber
          :modelValue="modelValue?.offset_y ?? null"
          class="w-full"
          size="small"
          @update:modelValue="(v) => patchPos({ offset_y: v ?? undefined })"
        />
      </div>
    </div>
    <div v-if="modelValue?.type === 'nearby' || modelValue?.type === 'random'">
      <label class="text-[11px] text-zinc-500 mb-0.5 block">Radius</label>
      <InputNumber
        :modelValue="modelValue?.radius ?? null"
        class="w-full"
        size="small"
        :min="0"
        @update:modelValue="(v) => patchPos({ radius: v ?? undefined })"
      />
    </div>
    <div v-if="modelValue?.type === 'at_shape'">
      <label class="text-[11px] text-zinc-500 mb-0.5 block">Shape UID</label>
      <Select
        :modelValue="modelValue?.shape_uid ?? null"
        :options="shapeOpts"
        optionLabel="label"
        optionValue="value"
        editable
        class="w-full"
        size="small"
        placeholder="Select or type a shape"
        @update:modelValue="(v) => patchPos({ shape_uid: v || undefined })"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
// Edits an optional SpawnPositionDTO (spawn_agent + transform actions). Extracted
// from ActionBuilder so both spawn-producing action types share one editor. The
// at_shape shape_uid is a referential select over the model's shapes.
import Select from 'primevue/select'
import InputNumber from 'primevue/inputnumber'
import { computed } from 'vue'
import type { SpawnPositionDTO, SpawnPositionType, Model } from '@/shared/types'
import { SPAWN_POSITION_TYPES } from '@/shared/validation/modelRules'

const props = defineProps<{ modelValue: SpawnPositionDTO | undefined; model: Model }>()
const emit = defineEmits<{ 'update:modelValue': [v: SpawnPositionDTO | undefined] }>()

// Bare shape UIDs (no "(shape)" suffix — a spawn position always means a shape).
const shapeOpts = computed(() =>
  (props.model.environment?.shapes ?? []).map((s) => ({ label: s.name || s.uid, value: s.uid }))
)

function onTypeChange(type: SpawnPositionType | null) {
  if (!type) {
    emit('update:modelValue', undefined)
    return
  }
  emit('update:modelValue', { ...(props.modelValue ?? {}), type } as SpawnPositionDTO)
}
function patchPos(partial: Partial<SpawnPositionDTO>) {
  if (!props.modelValue) return
  emit('update:modelValue', { ...props.modelValue, ...partial })
}
</script>
