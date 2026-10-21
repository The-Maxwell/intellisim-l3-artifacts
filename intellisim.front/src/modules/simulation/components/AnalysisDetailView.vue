<template>
  <div class="h-full flex flex-col min-h-0 space-y-3">
    <div class="flex items-center justify-between shrink-0">
      <div>
        <div class="font-medium">{{ analysis.name }}</div>
        <div class="text-xs text-zinc-400">
          {{ analysis.status }} · {{ analysis.language }} · {{ analysis.llm_target.provider_uid }} · {{ analysis.llm_target.model_uid }}
        </div>
      </div>
      <Button v-if="analysis.file" label="Download PDF" icon="pi pi-download" size="small" @click="download" />
    </div>

    <Message v-if="analysis.status === 'failed'" severity="error" :closable="false" class="shrink-0">
      {{ analysis.error || 'Analysis failed.' }}
    </Message>
    <Message v-else-if="analysis.status !== 'completed'" severity="info" :closable="false" class="shrink-0">
      Analysis in progress: {{ analysis.progress.stage }} ({{ analysis.progress.percent }}%)
    </Message>

    <Message v-if="analysis.warnings?.length" severity="warn" :closable="false" class="shrink-0" data-testid="analysis-warnings">
      <ul class="list-disc list-inside space-y-0.5 text-sm">
        <li v-for="(w, i) in analysis.warnings" :key="i">
          {{ w.message }} <span class="text-xs opacity-70">({{ w.stage }} · {{ w.code }})</span>
        </li>
      </ul>
    </Message>

    <!-- PDF fills the full available width + height of the pane -->
    <div v-if="pdfUrl" class="flex-1 min-h-0 border border-zinc-200 rounded overflow-hidden">
      <iframe :src="pdfUrl" class="w-full h-full" title="Analysis report"></iframe>
    </div>

    <div v-else-if="analysis.report" class="flex-1 min-h-0 overflow-auto space-y-3 text-sm">
      <section v-if="analysis.context_message">
        <p class="text-xs text-zinc-500 italic whitespace-pre-line">{{ analysis.context_message }}</p>
      </section>
      <section v-if="analysis.report.executive_summary">
        <h3 class="font-semibold">Executive summary</h3>
        <p class="whitespace-pre-line">{{ analysis.report.executive_summary }}</p>
      </section>
      <section v-for="s in analysis.report.sections ?? []" :key="s.key">
        <h3 class="font-semibold">{{ s.title }}</h3>
        <p class="whitespace-pre-line">{{ s.narrative }}</p>
        <div v-if="s.key_findings?.length" class="mt-1">
          <p class="text-xs font-semibold text-zinc-500 uppercase tracking-wide mb-0.5">Key findings</p>
          <ul class="list-disc list-inside space-y-0.5 text-zinc-700">
            <li v-for="(f, i) in s.key_findings" :key="i">{{ f }}</li>
          </ul>
        </div>
        <div v-if="s.causal_links?.length" class="mt-1">
          <p class="text-xs font-semibold text-zinc-500 uppercase tracking-wide mb-0.5">Causal links</p>
          <ul class="list-disc list-inside space-y-0.5 text-zinc-700">
            <li v-for="(c, i) in s.causal_links" :key="i">{{ c }}</li>
          </ul>
        </div>
        <div v-if="s.emergent_phenomena?.length" class="mt-1">
          <p class="text-xs font-semibold text-zinc-500 uppercase tracking-wide mb-0.5">Emergent phenomena</p>
          <ul class="list-disc list-inside space-y-0.5 text-zinc-700">
            <li v-for="(e, i) in s.emergent_phenomena" :key="i">{{ e }}</li>
          </ul>
        </div>
      </section>
      <section v-if="analysis.report.conclusions?.length">
        <h3 class="font-semibold">Conclusions</h3>
        <ul class="list-disc list-inside space-y-0.5 text-zinc-700">
          <li v-for="(c, i) in analysis.report.conclusions" :key="i">{{ c }}</li>
        </ul>
      </section>
      <section v-if="analysis.report.recommendations?.length">
        <h3 class="font-semibold">Recommendations</h3>
        <ul class="list-disc list-inside space-y-0.5 text-zinc-700">
          <li v-for="(r, i) in analysis.report.recommendations" :key="i">{{ r }}</li>
        </ul>
      </section>
      <p v-if="analysis.updated_at" class="text-[11px] text-zinc-400 pt-1">
        Updated {{ new Date(analysis.updated_at).toLocaleString() }}
      </p>
    </div>

    <div class="shrink-0 text-[11px] text-zinc-400 space-x-3" data-testid="analysis-meta">
      <span v-if="analysis.steps_covered?.count">
        Steps {{ analysis.steps_covered.from }}–{{ analysis.steps_covered.to }}
        (sampling {{ analysis.steps_covered.sampling }}, {{ analysis.steps_covered.count }} points)
      </span>
      <span v-if="analysis.token_usage?.total?.total_tokens">
        Tokens: {{ analysis.token_usage.total.prompt_tokens }} prompt ·
        {{ analysis.token_usage.total.completion_tokens }} completion ·
        {{ analysis.token_usage.total.total_tokens }} total · {{ analysis.requests }} requests
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import Button from 'primevue/button'
import Message from 'primevue/message'
import { useAnalysisApi } from '../composables/useAnalysisApi'
import type { Analysis } from '@/shared/types'

const props = defineProps<{ analysis: Analysis; simulationId: string }>()
const api = useAnalysisApi()

const pdfUrl = ref<string | null>(null)

function revoke(): void {
  if (pdfUrl.value) {
    URL.revokeObjectURL(pdfUrl.value)
    pdfUrl.value = null
  }
}

async function loadPdf(): Promise<void> {
  revoke()
  if (props.analysis.status !== 'completed' || !props.analysis.file) return
  try {
    const blob = await api.fetchFile(props.simulationId, props.analysis.id)
    pdfUrl.value = URL.createObjectURL(blob)
  } catch {
    pdfUrl.value = null
  }
}

async function download(): Promise<void> {
  const blob = await api.fetchFile(props.simulationId, props.analysis.id, true)
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = props.analysis.file?.name ?? 'analysis.pdf'
  a.click()
  URL.revokeObjectURL(url)
}

watch(() => [props.analysis.id, props.analysis.status], loadPdf, { immediate: true })
onBeforeUnmount(revoke)
</script>
