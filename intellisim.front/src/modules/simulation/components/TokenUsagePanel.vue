<template>
  <div class="space-y-4">
    <p v-if="!usage || isEmpty" class="text-xs text-zinc-400">No usage yet.</p>

    <template v-else>
      <!-- Total summary. Reasoning is a SUBSET of completion (already included in
           completion/total) — shown as an informational breakdown, never summed. -->
      <div class="grid grid-cols-5 gap-2">
        <div class="rounded-md border border-zinc-200 bg-white px-3 py-2">
          <p class="text-[11px] text-zinc-400">Prompt</p>
          <p class="text-sm font-semibold text-zinc-800">{{ fmt(usage.total.prompt_tokens) }}</p>
        </div>
        <div class="rounded-md border border-zinc-200 bg-white px-3 py-2">
          <p class="text-[11px] text-zinc-400">Completion</p>
          <p class="text-sm font-semibold text-zinc-800">{{ fmt(usage.total.completion_tokens) }}</p>
        </div>
        <div
          class="rounded-md border border-zinc-200 bg-white px-3 py-2"
          data-testid="token-reasoning-total"
          title="Reasoning tokens — the reasoning portion of completion tokens (already included in completion/total)"
        >
          <p class="text-[11px] text-zinc-400">Reasoning<span class="text-zinc-300"> ⊂ compl.</span></p>
          <p class="text-sm font-semibold text-zinc-800">{{ fmt(usage.total.reasoning_tokens) }}</p>
        </div>
        <div class="rounded-md border border-zinc-200 bg-white px-3 py-2">
          <p class="text-[11px] text-zinc-400">Total</p>
          <p class="text-sm font-semibold text-zinc-800">{{ fmt(usage.total.total_tokens) }}</p>
        </div>
        <div class="rounded-md border border-zinc-200 bg-white px-3 py-2">
          <p class="text-[11px] text-zinc-400">Calls</p>
          <p class="text-sm font-semibold text-zinc-800">{{ fmt(usage.total.calls) }}</p>
        </div>
      </div>

      <!-- By generation (run #) -->
      <div v-if="usage.by_generation?.length">
        <p class="text-[11px] font-semibold uppercase tracking-wide text-zinc-400 mb-1">
          By generation
        </p>
        <DataTable :value="byGeneration" size="small" class="text-xs" dataKey="run">
          <Column field="run" header="Run #" :style="{ width: '80px' }" />
          <Column field="prompt_tokens" header="Prompt"><template #body="{ data }">{{ fmt(data.prompt_tokens) }}</template></Column>
          <Column field="completion_tokens" header="Completion"><template #body="{ data }">{{ fmt(data.completion_tokens) }}</template></Column>
          <Column field="reasoning_tokens" header="Reasoning"><template #body="{ data }"><span class="text-zinc-500">{{ fmt(data.reasoning_tokens) }}</span></template></Column>
          <Column field="total_tokens" header="Total"><template #body="{ data }">{{ fmt(data.total_tokens) }}</template></Column>
          <Column field="calls" header="Calls"><template #body="{ data }">{{ fmt(data.calls) }}</template></Column>
        </DataTable>
      </div>

      <!-- By run (execution run seq, already seq-indexed) -->
      <div v-if="usage.by_run?.length">
        <p class="text-[11px] font-semibold uppercase tracking-wide text-zinc-400 mb-1">
          By run
        </p>
        <DataTable :value="byRun" size="small" class="text-xs" dataKey="run">
          <Column field="run" header="Run #" :style="{ width: '80px' }" />
          <Column field="prompt_tokens" header="Prompt"><template #body="{ data }">{{ fmt(data.prompt_tokens) }}</template></Column>
          <Column field="completion_tokens" header="Completion"><template #body="{ data }">{{ fmt(data.completion_tokens) }}</template></Column>
          <Column field="reasoning_tokens" header="Reasoning"><template #body="{ data }"><span class="text-zinc-500">{{ fmt(data.reasoning_tokens) }}</span></template></Column>
          <Column field="total_tokens" header="Total"><template #body="{ data }">{{ fmt(data.total_tokens) }}</template></Column>
          <Column field="calls" header="Calls"><template #body="{ data }">{{ fmt(data.calls) }}</template></Column>
        </DataTable>
      </div>

      <!-- By model -->
      <div v-if="usage.by_model?.length">
        <p class="text-[11px] font-semibold uppercase tracking-wide text-zinc-400 mb-1">By model</p>
        <DataTable :value="usage.by_model" size="small" class="text-xs">
          <Column header="Provider · Model">
            <template #body="{ data }">
              <span class="text-zinc-700">{{ data.provider_uid }} · {{ data.model_uid }}</span>
              <span v-if="data.effort" class="text-zinc-400"> ({{ data.effort }})</span>
            </template>
          </Column>
          <Column field="prompt_tokens" header="Prompt"><template #body="{ data }">{{ fmt(data.prompt_tokens) }}</template></Column>
          <Column field="completion_tokens" header="Completion"><template #body="{ data }">{{ fmt(data.completion_tokens) }}</template></Column>
          <Column field="reasoning_tokens" header="Reasoning"><template #body="{ data }"><span class="text-zinc-500">{{ fmt(data.reasoning_tokens) }}</span></template></Column>
          <Column field="total_tokens" header="Total"><template #body="{ data }">{{ fmt(data.total_tokens) }}</template></Column>
          <Column field="calls" header="Calls"><template #body="{ data }">{{ fmt(data.calls) }}</template></Column>
        </DataTable>
      </div>

      <!-- By agent -->
      <div v-if="usage.by_agent?.length">
        <p class="text-[11px] font-semibold uppercase tracking-wide text-zinc-400 mb-1">By agent</p>
        <DataTable :value="usage.by_agent" size="small" class="text-xs" dataKey="agent_uid">
          <Column field="agent_uid" header="Agent" />
          <Column field="prompt_tokens" header="Prompt"><template #body="{ data }">{{ fmt(data.prompt_tokens) }}</template></Column>
          <Column field="completion_tokens" header="Completion"><template #body="{ data }">{{ fmt(data.completion_tokens) }}</template></Column>
          <Column field="reasoning_tokens" header="Reasoning"><template #body="{ data }"><span class="text-zinc-500">{{ fmt(data.reasoning_tokens) }}</span></template></Column>
          <Column field="total_tokens" header="Total"><template #body="{ data }">{{ fmt(data.total_tokens) }}</template></Column>
          <Column field="calls" header="Calls"><template #body="{ data }">{{ fmt(data.calls) }}</template></Column>
        </DataTable>
      </div>

      <!-- By purpose -->
      <div v-if="usage.by_purpose?.length">
        <p class="text-[11px] font-semibold uppercase tracking-wide text-zinc-400 mb-1">By purpose</p>
        <DataTable :value="usage.by_purpose" size="small" class="text-xs" dataKey="purpose">
          <Column field="purpose" header="Purpose" />
          <Column field="prompt_tokens" header="Prompt"><template #body="{ data }">{{ fmt(data.prompt_tokens) }}</template></Column>
          <Column field="completion_tokens" header="Completion"><template #body="{ data }">{{ fmt(data.completion_tokens) }}</template></Column>
          <Column field="reasoning_tokens" header="Reasoning"><template #body="{ data }"><span class="text-zinc-500">{{ fmt(data.reasoning_tokens) }}</span></template></Column>
          <Column field="total_tokens" header="Total"><template #body="{ data }">{{ fmt(data.total_tokens) }}</template></Column>
          <Column field="calls" header="Calls"><template #body="{ data }">{{ fmt(data.calls) }}</template></Column>
        </DataTable>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
// Renders the simulation's SimulationTokenUsage: a Total summary card grid plus
// per-dimension DataTables (by_generation with the run index as the run #,
// by_model with provider/model/effort, by_agent, by_purpose). Shows "No usage
// yet" when the usage is null or every dimension is empty/zero.
import { computed } from 'vue'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import type { SimulationTokenUsage, TokenUsageDim } from '@/shared/types'

const props = defineProps<{ usage: SimulationTokenUsage | null | undefined }>()

const usage = computed(() => props.usage ?? null)

// Empty when no calls were ever made and no breakdown rows exist.
const isEmpty = computed(() => {
  const u = usage.value
  if (!u) return true
  const total = u.total
  const hasTotal = total && (total.total_tokens > 0 || total.calls > 0)
  const hasRows =
    (u.by_generation?.length ?? 0) +
      (u.by_run?.length ?? 0) +
      (u.by_model?.length ?? 0) +
      (u.by_agent?.length ?? 0) +
      (u.by_purpose?.length ?? 0) >
    0
  return !hasTotal && !hasRows
})

// by_generation entries carry no explicit run id, so use the array index as the
// run # for display.
const byGeneration = computed(() =>
  (usage.value?.by_generation ?? []).map((g: TokenUsageDim, i) => ({ ...g, run: i + 1 }))
)

// by_run entries are already seq-indexed (seq 1-based from the backend);
// use the array index + 1 as the run # for display (mirrors byGeneration).
const byRun = computed(() =>
  (usage.value?.by_run ?? []).map((r: TokenUsageDim, i) => ({ ...r, run: i + 1 }))
)

function fmt(n: number | undefined): string {
  return (n ?? 0).toLocaleString()
}
</script>
