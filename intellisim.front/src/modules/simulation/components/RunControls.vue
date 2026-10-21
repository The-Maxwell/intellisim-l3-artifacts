<template>
  <div class="flex items-center gap-3 flex-wrap">
    <!-- Transport buttons -->
    <div class="flex items-center gap-1">
      <Button
        v-if="!running"
        icon="pi pi-play"
        size="small"
        :label="paused ? 'Resume' : 'Play'"
        :disabled="!canResume || busy"
        :loading="busy"
        @click="onResume"
      />
      <Button
        v-else
        icon="pi pi-pause"
        size="small"
        label="Pause"
        severity="secondary"
        :disabled="busy"
        :loading="busy"
        @click="onPause"
      />
      <Button
        icon="pi pi-stop"
        size="small"
        label="Stop"
        severity="danger"
        outlined
        :disabled="!canStop || busy"
        @click="onStop"
      />
    </div>

    <span class="w-px h-6 bg-zinc-200" />

    <!-- Speed slider: maps a slider position to an inter-step delay (ms). Higher
         slider = faster = smaller delay. -->
    <div class="flex items-center gap-2 min-w-[180px]">
      <i class="pi pi-forward text-xs text-zinc-400" title="Speed" />
      <Slider
        :modelValue="speedSlider"
        :min="0"
        :max="SPEED_STEPS.length - 1"
        :step="1"
        class="flex-1"
        :disabled="!active || busy"
        @update:modelValue="onSpeedSlider"
      />
      <span class="text-[11px] text-zinc-500 tabular-nums w-14 text-right">{{ speedLabel }}</span>
    </div>

    <span class="w-px h-6 bg-zinc-200" />

    <!-- Step counter + status chip -->
    <div class="flex items-center gap-2 text-sm">
      <span class="text-zinc-500">Step</span>
      <span class="font-semibold text-zinc-800 tabular-nums" data-testid="run-step">{{ currentStep }}<span v-if="totalSteps"> / {{ totalSteps }}</span></span>
      <Tag :value="statusLabel" :severity="statusSeverity(runStatus ?? 'queued')" />
    </div>

    <template v-if="elapsedLabel">
      <span class="w-px h-6 bg-zinc-200" />
      <!-- Active run time (ticks while running, frozen while paused, final on
           finish). Excludes paused intervals. -->
      <div class="flex items-center gap-1.5 text-sm" title="Active run time (paused time excluded)">
        <i class="pi pi-clock text-zinc-400 text-xs" />
        <span class="font-medium text-zinc-700 tabular-nums" data-testid="run-elapsed">{{ elapsedLabel }}</span>
      </div>
    </template>

    <span class="w-px h-6 bg-zinc-200" />

    <!-- Live token readout (cumulative). Tooltip breaks out the components; the
         reasoning portion is a subset of completion, not a summand. -->
    <div
      class="flex items-center gap-1 text-[11px] text-zinc-500"
      data-testid="run-token-total"
      :title="tokenTooltip"
    >
      <i class="pi pi-bolt text-amber-500" />
      <span class="tabular-nums">{{ tokenLabel }}</span>
    </div>

    <!-- Per-step token delta (how many tokens THIS step just consumed). Shown
         only while a NON-ZERO delta is available — keyframes/no-LLM steps send
         an all-zero delta, which would otherwise render a meaningless "0 tok
         this step" chip on every such step. -->
    <template v-if="hasStepTokens">
      <span class="w-px h-6 bg-zinc-200" />
      <div
        class="flex items-center gap-1 text-[11px] text-emerald-600"
        data-testid="run-step-tokens"
        :title="stepTokenTooltip"
      >
        <i class="pi pi-plus text-[10px]" />
        <span class="tabular-nums">{{ stepTokenLabel }}</span>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
// Live run controls (Task 24): play/pause/resume/stop, a speed slider (mapped to
// the inter-step delay sent over POST /:id/control), the step counter + run
// status chip, and a live LLM-token readout. Every action is delegated to the
// run store (store.control / store.setSpeed); buttons disable per status.
import { computed, ref } from 'vue'
import Button from 'primevue/button'
import Slider from 'primevue/slider'
import Tag from 'primevue/tag'
import { statusSeverity } from '@/modules/simulation/views/simulationStatus'
import type {
  ControlAction,
  SimulationStatus,
  SimulationTokenUsage,
  StepTokenDelta,
} from '@/shared/types'

const props = defineProps<{
  runStatus: SimulationStatus | null
  currentStep: number
  totalSteps?: number
  speedMs: number
  tokenUsage?: SimulationTokenUsage | null
  // The most recent per-step token delta (live run only); omitted otherwise.
  stepTokens?: StepTokenDelta | null
  // Active run-time label (from useRunElapsed); omitted before a run starts.
  elapsedLabel?: string
}>()

const emit = defineEmits<{
  control: [action: ControlAction]
  // A new inter-step delay in ms (0 = fastest).
  speed: [ms: number]
}>()

// Discrete inter-step delays the slider exposes (ms). Index 0 is the slowest;
// the last index is the fastest (0ms delay).
const SPEED_STEPS = [1000, 500, 250, 100, 50, 0]

const busy = ref(false)

const running = computed(() => props.runStatus === 'running')
const paused = computed(() => props.runStatus === 'paused')
const queued = computed(() => props.runStatus === 'queued')
const active = computed(() => running.value || paused.value || queued.value)
const canResume = computed(() => paused.value)
const canStop = computed(() => active.value)

const statusLabel = computed(() => props.runStatus ?? '—')

const totalSteps = computed(() => props.totalSteps ?? 0)

// Slider position = nearest SPEED_STEPS index to the current delay. Defaults to
// the fastest end when no speed has been set yet.
const speedSlider = computed(() => {
  const ms = props.speedMs
  let best = SPEED_STEPS.length - 1
  let bestDiff = Infinity
  SPEED_STEPS.forEach((s, i) => {
    const d = Math.abs(s - ms)
    if (d < bestDiff) {
      bestDiff = d
      best = i
    }
  })
  return best
})

const speedLabel = computed(() => {
  const ms = SPEED_STEPS[speedSlider.value] ?? 0
  return ms === 0 ? 'max' : `${ms}ms`
})

const tokenLabel = computed(() => {
  const total = props.tokenUsage?.total?.total_tokens ?? 0
  return `${total.toLocaleString()} tok`
})

const tokenTooltip = computed(() => {
  const t = props.tokenUsage?.total
  if (!t) return 'LLM tokens used by this run'
  return `Prompt ${fmtNum(t.prompt_tokens)} · Completion ${fmtNum(t.completion_tokens)} (reasoning ${fmtNum(t.reasoning_tokens)}) · Total ${fmtNum(t.total_tokens)} · Calls ${fmtNum(t.calls)}`
})

// True only when the delta actually reflects LLM activity — an all-zero delta
// (keyframes/no-LLM steps) shouldn't render a "0 tok this step" chip.
const hasStepTokens = computed(() => {
  const s = props.stepTokens
  return !!s && (s.total_tokens > 0 || s.calls > 0)
})

const stepTokenLabel = computed(() => {
  const total = props.stepTokens?.total_tokens ?? 0
  return `${total.toLocaleString()} tok this step`
})

const stepTokenTooltip = computed(() => {
  const s = props.stepTokens
  if (!s) return ''
  return `This step — Prompt ${fmtNum(s.prompt_tokens)} · Completion ${fmtNum(s.completion_tokens)} (reasoning ${fmtNum(s.reasoning_tokens)}) · Calls ${fmtNum(s.calls)}`
})

function fmtNum(n: number | undefined): string {
  return (n ?? 0).toLocaleString()
}

async function guard(fn: () => void | Promise<void>): Promise<void> {
  if (busy.value) return
  busy.value = true
  try {
    await fn()
  } finally {
    busy.value = false
  }
}

function onPause() {
  void guard(() => emit('control', 'pause'))
}
function onResume() {
  void guard(() => emit('control', 'resume'))
}
function onStop() {
  void guard(() => emit('control', 'stop'))
}

function onSpeedSlider(value: number | number[]) {
  const idx = Array.isArray(value) ? (value[0] ?? 0) : value
  const ms = SPEED_STEPS[idx] ?? 0
  emit('speed', ms)
}
</script>
