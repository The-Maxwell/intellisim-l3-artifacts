<template>
  <div class="h-full flex flex-col min-h-0 bg-white">
    <!-- Header -->
    <div class="flex items-center gap-2 px-4 py-2.5 border-b border-zinc-200">
      <span class="w-2 h-2 rounded-full flex-none" :class="readyDotClass" />
      <span class="text-sm font-semibold text-zinc-800">Model summary</span>
      <span class="ml-auto text-[11px] text-zinc-400">live</span>
    </div>

    <div class="flex-1 min-h-0 overflow-y-auto p-3 space-y-3">
      <!-- Readiness verdict + its blockers, kept together so issues are visible
           without scrolling past the tiles. -->
      <div class="space-y-1.5">
        <div class="flex items-center gap-3 rounded-lg border px-3 py-2" :class="bannerClass">
          <span
            class="w-7 h-7 rounded-lg grid place-items-center text-white flex-none"
            :class="ready ? 'bg-emerald-500' : 'bg-rose-500'"
          >
            <i :class="ready ? 'pi pi-check' : 'pi pi-exclamation-triangle'" />
          </span>
          <div class="min-w-0">
            <div class="text-sm font-semibold" :class="ready ? 'text-emerald-700' : 'text-rose-700'">
              {{ ready ? 'Ready to run' : 'Not ready to run' }}
            </div>
            <div class="text-[11px] text-zinc-500">{{ readySubtitle }}</div>
          </div>
        </div>
        <!-- Blockers/warnings listed directly under the verdict; each deep-links. -->
        <ul v-if="issues.length" class="space-y-1">
          <li v-for="(iss, i) in issues" :key="i">
            <button
              type="button"
              class="w-full flex items-start gap-2 rounded-md px-2.5 py-1.5 text-left text-xs transition"
              :class="iss.severity === 'error' ? 'bg-rose-50 hover:bg-rose-100' : 'bg-amber-50 hover:bg-amber-100'"
              @click="goIssue(iss)"
            >
              <span
                class="flex-none mt-px font-mono text-[9px] font-bold px-1 py-0.5 rounded text-white"
                :class="iss.severity === 'error' ? 'bg-rose-500' : 'bg-amber-500'"
              >{{ iss.severity === 'error' ? 'ERR' : 'WARN' }}</span>
              <span class="min-w-0 text-zinc-700">
                <code v-if="iss.path" class="text-[10.5px] text-zinc-500">{{ iss.path }}</code>{{ iss.path ? ' — ' : '' }}{{ iss.message }}
                <span class="text-violet-600 whitespace-nowrap"> →</span>
              </span>
            </button>
          </li>
        </ul>
      </div>

      <!-- Count tiles -->
      <div class="grid grid-cols-3 gap-1.5">
        <button
          v-for="t in tiles"
          :key="t.key"
          type="button"
          class="text-left rounded-lg border border-zinc-200 bg-zinc-50 px-2 py-1.5 transition hover:border-violet-300 hover:bg-violet-50"
          @click="emit('navigate', t.tab)"
        >
          <div class="text-lg font-bold leading-none tabular-nums text-zinc-800">{{ t.n }}</div>
          <div class="mt-0.5 text-[9px] uppercase tracking-wide text-zinc-400">{{ t.label }}</div>
        </button>
      </div>

      <!-- Environment — shown for every model. Spatial models get geometry facts;
           non-spatial models get their global-properties + prompt-context summary. -->
      <section>
        <button
          type="button"
          class="w-full text-left text-[11px] font-semibold uppercase tracking-wide text-zinc-400 mb-1 hover:text-violet-600"
          @click="emit('navigate', 'environment')"
        >
          Environment
        </button>
        <div class="text-xs divide-y divide-zinc-100 rounded-lg border border-zinc-200">
          <template v-if="model.is_spatial">
            <div class="flex items-center gap-2 px-2.5 py-1">
              <span class="text-zinc-500 flex-1">Size</span>
              <span class="font-mono text-[11px] text-zinc-700 tabular-nums">{{ env.width }} × {{ env.height }}</span>
            </div>
            <div class="flex items-center gap-2 px-2.5 py-1">
              <span class="text-zinc-500 flex-1">Wrap (torus)</span>
              <span class="font-mono text-[11px] text-zinc-700">{{ env.torus ? 'on' : 'off' }}</span>
            </div>
            <div class="flex items-center gap-2 px-2.5 py-1">
              <span class="text-zinc-500 flex-1">Collision</span>
              <span class="font-mono text-[11px] text-zinc-700">{{ env.collision }}</span>
            </div>
            <div class="flex items-center gap-2 px-2.5 py-1">
              <span class="text-zinc-500 flex-1">Shapes · global props</span>
              <span class="font-mono text-[11px] text-zinc-700 tabular-nums">{{ env.shapes }} · {{ env.globals }}</span>
            </div>
          </template>
          <template v-else>
            <div class="flex items-center gap-2 px-2.5 py-1">
              <span class="text-zinc-500 flex-1">Global properties</span>
              <span class="font-mono text-[11px] text-zinc-700 tabular-nums">{{ env.globals }}</span>
            </div>
            <div class="flex items-center gap-2 px-2.5 py-1">
              <span class="text-zinc-500 flex-1">Prompt context</span>
              <span class="font-mono text-[11px] text-zinc-700">{{ env.promptContext ? 'set' : '—' }}</span>
            </div>
          </template>
        </div>
      </section>

      <!-- LLM wiring -->
      <section>
        <button
          type="button"
          class="w-full text-left text-[11px] font-semibold uppercase tracking-wide text-zinc-400 mb-1 hover:text-violet-600"
          @click="emit('navigate', 'llm')"
        >
          LLM wiring
        </button>
        <div class="text-xs divide-y divide-zinc-100 rounded-lg border border-zinc-200">
          <div v-for="w in wiring" :key="w.key" class="flex items-center gap-2 px-2.5 py-1">
            <span class="text-zinc-500 flex-1">{{ w.label }}</span>
            <span v-if="w.dot !== undefined" class="w-2 h-2 rounded-full flex-none" :class="w.dot ? 'bg-emerald-500' : 'bg-zinc-300'" />
            <span class="font-mono text-[11px] tabular-nums" :class="w.muted ? 'text-zinc-400' : 'text-zinc-700'">{{ w.value }}</span>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
// Read-only model summary — replaces the JSON pane. Everything is DERIVED from the
// model body + the store's ValidationResult (no extra fetch). Surfaces readiness
// with its blockers right under the verdict, plus composition counts, environment
// facts and LLM wiring; clicking a tile / issue emits `navigate(tab, index?)` so
// the editor can jump to (and select) the relevant section/item.
import { computed } from 'vue'
import type { Model, ValidationResult, ValidationIssue } from '@/shared/types'

const props = defineProps<{ model: Model; validation: ValidationResult | null }>()
const emit = defineEmits<{ navigate: [tab: string, index?: number] }>()

const ready = computed(() => !!props.validation?.simulation_ready)
const errorCount = computed(() => props.validation?.errors?.length ?? 0)
const warningCount = computed(() => props.validation?.warnings?.length ?? 0)

const readyDotClass = computed(() => (ready.value ? 'bg-emerald-500' : 'bg-rose-500'))
const bannerClass = computed(() =>
  ready.value ? 'bg-emerald-50 border-emerald-200' : 'bg-rose-50 border-rose-200'
)
const readySubtitle = computed(() => {
  const parts: string[] = []
  parts.push(ready.value ? 'No blockers' : `${errorCount.value} blocker${errorCount.value === 1 ? '' : 's'}`)
  if (warningCount.value) parts.push(`${warningCount.value} warning${warningCount.value === 1 ? '' : 's'}`)
  return parts.join(' · ')
})

const tiles = computed(() => {
  const m = props.model
  const propCount =
    m.agents.reduce((s, a) => s + (a.properties?.length ?? 0), 0) +
    (m.environment?.global_properties?.length ?? 0)
  return [
    { key: 'agents', label: 'Agents', n: m.agents.length, tab: 'agents' },
    { key: 'rules', label: 'Rules', n: m.rules.length, tab: 'rules' },
    { key: 'interactions', label: 'Interact.', n: m.interactions.length, tab: 'interactions' },
    { key: 'events', label: 'Events', n: m.events.length, tab: 'events' },
    { key: 'widgets', label: 'Widgets', n: (m.widgets ?? []).length, tab: 'widgets' },
    { key: 'props', label: 'Properties', n: propCount, tab: 'agents' },
  ]
})

const env = computed(() => {
  const e = props.model.environment
  return {
    width: e?.width ?? 0,
    height: e?.height ?? 0,
    torus: !!e?.torus,
    collision: e?.collision_mode === 'block' ? 'block' : 'none',
    shapes: e?.shapes?.length ?? 0,
    globals: e?.global_properties?.length ?? 0,
    promptContext: !!e?.llm_prompt_context,
  }
})

const wiring = computed(() => {
  const m = props.model
  const sc = m.simulation_config
  const exec = sc?.llm_simulation_execution_config
  const targetLabel = (uid?: string, model_uid?: string) =>
    uid ? model_uid || 'set' : '—'
  const chatWired = !!m.llm_config?.target?.provider_uid
  const genWired = !!sc?.llm_simulation_generation_config?.target?.provider_uid
  const execWired = !!exec?.target?.provider_uid
  const strategy = exec?.default_strategy?.type ?? 'none'
  const overrides = m.agents.filter((a) => a.llm_config?.target?.provider_uid).length
  return [
    {
      key: 'chat',
      label: 'Model & chat',
      dot: chatWired,
      muted: !chatWired,
      value: targetLabel(m.llm_config?.target?.provider_uid, m.llm_config?.target?.model_uid),
    },
    {
      key: 'gen',
      label: 'Sim. generation',
      dot: genWired,
      muted: !genWired,
      value: targetLabel(
        sc?.llm_simulation_generation_config?.target?.provider_uid,
        sc?.llm_simulation_generation_config?.target?.model_uid
      ),
    },
    {
      key: 'exec',
      label: 'Sim. execution',
      dot: execWired,
      muted: !execWired,
      value: execWired ? `${strategy} · ${exec?.target?.model_uid || 'set'}` : '—',
    },
    {
      key: 'overrides',
      label: 'Per-agent overrides',
      dot: undefined,
      muted: overrides === 0,
      value: String(overrides),
    },
  ]
})

// Errors first, then warnings — each issue already carries its severity.
const issues = computed<ValidationIssue[]>(() => [
  ...(props.validation?.errors ?? []),
  ...(props.validation?.warnings ?? []),
])

// Map a validation path (e.g. "agents[0].properties[1].name") to a tab + optional
// item index so the editor can jump there. Unknown heads fall back to general;
// the editor guards against tabs that don't currently exist (e.g. environment on
// a non-spatial model).
function goIssue(iss: ValidationIssue) {
  const m = /^(\w+)(?:\[(\d+)\])?/.exec(iss.path || '')
  const head = m?.[1] ?? ''
  const index = m?.[2] != null ? Number(m[2]) : undefined
  const map: Record<string, string> = {
    agents: 'agents',
    rules: 'rules',
    interactions: 'interactions',
    events: 'events',
    environment: 'environment',
    llm_config: 'llm',
    simulation_config: 'llm',
    widgets: 'widgets',
  }
  emit('navigate', map[head] ?? 'general', index)
}
</script>
