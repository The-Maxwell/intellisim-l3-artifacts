<template>
  <div class="space-y-1" :data-testid="testid">
    <div class="flex items-end gap-2">
      <div class="flex-none">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">{{ label }}</label>
        <SelectButton
          :modelValue="mode"
          :options="MODES"
          optionLabel="label"
          optionValue="value"
          :allowEmpty="false"
          size="small"
          :aria-label="`${label} mode`"
          @update:modelValue="(v) => setMode(v as SpecMode)"
        />
      </div>
      <!-- Fixed: a single value. -->
      <div v-if="mode === 'fixed'" class="w-28">
        <label class="text-[11px] text-zinc-400 mb-0.5 block">Value</label>
        <InputNumber
          :modelValue="spec.value ?? null"
          class="w-full"
          size="small"
          :min="min"
          :data-testid="`${testid}-value`"
          @update:modelValue="(v) => emitFixed(v)"
        />
      </div>
      <!-- Range: each instance draws a value from [min, max]. -->
      <template v-else>
        <div class="w-24">
          <label class="text-[11px] text-zinc-400 mb-0.5 block">Min</label>
          <InputNumber
            :modelValue="spec.min ?? null"
            class="w-full"
            size="small"
            :min="min"
            :data-testid="`${testid}-min`"
            @update:modelValue="(v) => emitRange({ min: v ?? undefined })"
          />
        </div>
        <div class="w-24">
          <label class="text-[11px] text-zinc-400 mb-0.5 block">Max</label>
          <InputNumber
            :modelValue="spec.max ?? null"
            class="w-full"
            size="small"
            :min="min"
            :data-testid="`${testid}-max`"
            @update:modelValue="(v) => emitRange({ max: v ?? undefined })"
          />
        </div>
      </template>
    </div>
    <p v-if="rangeInvalid" class="text-[11px] text-red-600" :data-testid="`${testid}-error`">
      Min must be ≤ max.
    </p>
    <p v-else-if="mode === 'range'" class="text-[11px] text-zinc-400">
      Each instance draws a random {{ label.toLowerCase() }} in [min, max].
    </p>
  </div>
</template>

<script setup lang="ts">
// Edits one ScalarSpecDTO (a shape dimension that is either a fixed value OR a
// random [min,max] range resolved per instance). Enforces the backend
// "exactly-one-of" invariant by construction: Fixed emits { value } and clears
// min/max; Range emits { min, max } and clears value. Always emits a fresh spec.
import SelectButton from 'primevue/selectbutton'
import InputNumber from 'primevue/inputnumber'
import { computed } from 'vue'
import type { ScalarSpecDTO } from '@/shared/types'

type SpecMode = 'fixed' | 'range'

const props = withDefaults(
  defineProps<{
    modelValue: ScalarSpecDTO | undefined
    label: string
    /** Lower bound for the numeric inputs (world dimensions are ≥ 0). */
    min?: number
    /** Slug for data-testid hooks (defaults from the label). */
    testid?: string
  }>(),
  { min: 0, testid: 'scalar' }
)
const emit = defineEmits<{ 'update:modelValue': [v: ScalarSpecDTO] }>()

const spec = computed<ScalarSpecDTO>(() => props.modelValue ?? {})

const MODES: { value: SpecMode; label: string }[] = [
  { value: 'fixed', label: 'Fixed' },
  { value: 'range', label: 'Range' },
]

// Mode is derived from the data: a range is active when either bound is set and
// no fixed value is present; otherwise fixed.
const mode = computed<SpecMode>(() => {
  const s = spec.value
  if (s.value === undefined && (s.min !== undefined || s.max !== undefined)) return 'range'
  return 'fixed'
})

const rangeInvalid = computed(() => {
  const s = spec.value
  return (
    mode.value === 'range' &&
    typeof s.min === 'number' &&
    typeof s.max === 'number' &&
    s.min > s.max
  )
})

function setMode(next: SpecMode) {
  if (next === mode.value) return
  if (next === 'fixed') {
    // carry the current midpoint/known value across so the field isn't blank
    const carried = spec.value.value ?? spec.value.min ?? spec.value.max
    emit('update:modelValue', { value: carried ?? props.min })
  } else {
    const base = spec.value.value ?? props.min
    emit('update:modelValue', { min: base, max: base })
  }
}

function emitFixed(v: number | null) {
  emit('update:modelValue', { value: v ?? undefined })
}
function emitRange(partial: { min?: number; max?: number }) {
  const s = spec.value
  emit('update:modelValue', { min: s.min, max: s.max, ...partial })
}
</script>
