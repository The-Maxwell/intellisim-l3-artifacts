<template>
  <div class="space-y-2">
    <label class="text-[11px] font-semibold uppercase tracking-wide text-zinc-400">
      Fallback policy
    </label>

    <!-- Short explanation of the chain semantics (mirrors the engine's
         resolveFallback logic). -->
    <p class="text-[11px] text-zinc-500 leading-snug">
      What to do when an LLM call fails, tried in order:
      <span class="text-zinc-600">retry</span> re-calls the LLM,
      <span class="text-zinc-600">rules_only</span> runs the agent's rules,
      <span class="text-zinc-600">last_decision</span> reuses its previous decision,
      <span class="text-zinc-600">skip</span> does nothing this step.
      With no steps the engine defaults to <span class="text-zinc-600">rules_only</span>.
    </p>

    <!-- Empty list = inherit. The wire value stays unset (fallback_policy null/
         undefined) so the backend's pointer-unset "inherit model default" is
         preserved. -->
    <p v-if="!steps.length" class="text-[11px] text-zinc-400 leading-snug">
      No steps — inherits the model default fallback policy. Add a step to override it.
    </p>

    <!-- NOTE: index key — steps lack a stable client id under immutable updates;
         removing/reordering a non-last item mid-edit can shift focus. Acceptable
         for now; revisit with a stable-key composable if it becomes a problem. -->
    <div v-for="(step, si) in steps" :key="si" class="flex items-end gap-2">
      <div class="w-40">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Step {{ si + 1 }}</label>
        <Select
          :modelValue="step.type"
          :options="FALLBACK_STEP_TYPES as unknown as string[]"
          class="w-full"
          size="small"
          @update:modelValue="(v) => patchStep(si, { type: v as FallbackStepType })"
        />
      </div>

      <!-- retry-only knobs -->
      <template v-if="step.type === 'retry'">
        <div class="w-28">
          <label class="text-[11px] text-zinc-500 mb-0.5 block">Max retries</label>
          <InputNumber
            :modelValue="step.max_retries ?? null"
            class="w-full"
            size="small"
            :min="0"
            @update:modelValue="(v) => patchStep(si, { max_retries: v ?? 0 })"
          />
        </div>
        <div class="w-32">
          <label class="text-[11px] text-zinc-500 mb-0.5 block">Retry delay (ms)</label>
          <InputNumber
            :modelValue="step.retry_delay_ms ?? null"
            class="w-full"
            size="small"
            :min="0"
            @update:modelValue="(v) => patchStep(si, { retry_delay_ms: v ?? 0 })"
          />
        </div>
      </template>

      <div class="flex items-center gap-0.5 pb-0.5">
        <Button
          icon="pi pi-chevron-up"
          text
          rounded
          size="small"
          :disabled="si === 0"
          aria-label="Move step up"
          @click="moveStep(si, -1)"
        />
        <Button
          icon="pi pi-chevron-down"
          text
          rounded
          size="small"
          :disabled="si === steps.length - 1"
          aria-label="Move step down"
          @click="moveStep(si, 1)"
        />
        <Button
          icon="pi pi-trash"
          severity="danger"
          text
          rounded
          size="small"
          aria-label="Remove fallback step"
          @click="removeStep(si)"
        />
      </div>
    </div>

    <Button
      label="Add fallback step"
      icon="pi pi-plus"
      size="small"
      severity="secondary"
      outlined
      @click="addStep"
    />
  </div>
</template>

<script setup lang="ts">
// Ordered fallback-policy step editor. Each step is a type (retry / rules_only /
// last_decision / skip); the retry type additionally exposes max_retries +
// retry_delay_ms. Steps can be added, removed and reordered (up/down).
//
// Inheritance semantics: an EMPTY list emits `null`, never `{ steps: [] }`, so
// the backend's pointer-unset "inherit the model default" is preserved. The
// parent binds this to a pointer field (fallback_policy?), so null clears it.
import Select from 'primevue/select'
import InputNumber from 'primevue/inputnumber'
import Button from 'primevue/button'
import { computed } from 'vue'
import type { FallbackPolicyDTO, FallbackStepDTO, FallbackStepType } from '@/shared/types'
import { FALLBACK_STEP_TYPES } from '@/shared/validation/modelRules'

const props = defineProps<{
  // Pointer-unset (null/undefined) = inherit the model default.
  modelValue?: FallbackPolicyDTO | null
}>()
const emit = defineEmits<{ 'update:modelValue': [v: FallbackPolicyDTO | undefined] }>()

const steps = computed<FallbackStepDTO[]>(() => props.modelValue?.steps ?? [])

// Emit `undefined` when the list is empty so "unset = inherit" is preserved;
// otherwise emit a real policy object with the steps.
function setSteps(next: FallbackStepDTO[]) {
  emit('update:modelValue', next.length ? { steps: next } : undefined)
}
function patchStep(index: number, partial: Partial<FallbackStepDTO>) {
  setSteps(steps.value.map((s, i) => (i === index ? { ...s, ...partial } : s)))
}
function removeStep(index: number) {
  setSteps(steps.value.filter((_, i) => i !== index))
}
function addStep() {
  setSteps([...steps.value, { type: 'retry', max_retries: 1, retry_delay_ms: 0 }])
}
function moveStep(index: number, delta: number) {
  const target = index + delta
  if (target < 0 || target >= steps.value.length) return
  const next = [...steps.value]
  const [moved] = next.splice(index, 1)
  next.splice(target, 0, moved!)
  setSteps(next)
}
</script>
