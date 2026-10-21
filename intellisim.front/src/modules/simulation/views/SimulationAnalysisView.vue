<template>
  <div class="h-full flex flex-col min-h-0">
    <!-- Header (always visible). A SINGLE context-aware back arrow (same style as
         the simulation views): on the list it goes back to the simulation; with an
         analysis open it goes back one step to the list. "New analysis" shows only
         on the list. -->
    <div class="border-b border-zinc-200 bg-white px-6 h-14 shrink-0 flex items-center justify-between gap-4">
      <div class="min-w-0 flex items-center gap-2">
        <RouterLink
          v-if="!store.current"
          :to="`/simulations/${id}`"
          class="text-zinc-400 hover:text-zinc-700 shrink-0"
          title="Back to simulation"
        >
          <i class="pi pi-arrow-left text-sm" />
        </RouterLink>
        <button
          v-else
          type="button"
          class="text-zinc-400 hover:text-zinc-700 shrink-0"
          title="Back to analyses"
          @click="store.clearCurrent()"
        >
          <i class="pi pi-arrow-left text-sm" />
        </button>
        <h1 class="text-lg font-semibold text-zinc-800 truncate">Results Analysis</h1>
      </div>
      <!-- v-tooltip on a wrapper span: a disabled button emits no hover events,
           so the hint must live on an enabled ancestor. Empty when analyzable. -->
      <span v-if="!store.current" v-tooltip.bottom="analyzable ? '' : notReadyHint">
        <Button label="New analysis" icon="pi pi-sparkles" :disabled="!analyzable" @click="dialogVisible = true" />
      </span>
    </div>

    <!-- Body: ONE of list / detail at a time, both full width. -->
    <div class="flex-1 min-h-0 flex flex-col">
      <!-- LIST (default) -->
      <div v-if="!store.current" class="flex-1 min-h-0 overflow-auto p-6 space-y-4">
        <Message v-if="!analyzable" severity="info" :closable="false">
          Analysis is available only for a finished or paused simulation (current status: {{ simStatus || 'unknown' }}).
        </Message>
        <AnalysisListPanel
          :list="store.list"
          :loading="store.loading"
          :progress="store.progress"
          @view="onView"
          @download="onDownload"
          @delete="onDelete"
        />
      </div>

      <!-- DETAIL (full width + height) -->
      <div v-else class="flex-1 min-h-0 p-4">
        <AnalysisDetailView :analysis="store.current" :simulationId="id" />
      </div>
    </div>

    <AnalyzeDialog v-model:visible="dialogVisible" :simulationId="id" @submit="onSubmit" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref } from 'vue'
import { RouterLink } from 'vue-router'
import Button from 'primevue/button'
import Message from 'primevue/message'
import Tooltip from 'primevue/tooltip'
import { useConfirm } from 'primevue/useconfirm'
import { useToast } from 'primevue/usetoast'
import AnalysisListPanel from '../components/AnalysisListPanel.vue'
import AnalysisDetailView from '../components/AnalysisDetailView.vue'
import AnalyzeDialog from '../components/AnalyzeDialog.vue'
import { useAnalysisStore } from '../stores/analysisStore'
import { useSimulationApi } from '../composables/useSimulationApi'
import { useErrorHandler } from '@/shared/composables/useErrorHandler'
import type { AnalyzeRequest } from '@/shared/types'

const vTooltip = Tooltip

const props = defineProps<{ id: string }>()
const id = props.id

const store = useAnalysisStore()
const simApi = useSimulationApi()
const confirm = useConfirm()
const toast = useToast()
const { handleError } = useErrorHandler()

const dialogVisible = ref(false)
const simStatus = ref('')

const analyzable = computed(() => simStatus.value === 'finished' || simStatus.value === 'paused')

// Hint shown on hover over the disabled "New analysis" button.
const notReadyHint = computed(
  () =>
    `Analysis is available only when the simulation has finished or is paused (current status: ${simStatus.value || 'unknown'}).`
)

onMounted(async () => {
  try {
    const sim = await simApi.get(id)
    simStatus.value = sim.status
  } catch (e) {
    handleError(e, 'Open simulation')
  }
  await store.open(id)
})
onBeforeUnmount(() => store.close())

async function onSubmit(req: AnalyzeRequest): Promise<void> {
  try {
    await store.create(req)
    dialogVisible.value = false
    toast.add({ severity: 'success', summary: 'Analysis started', detail: 'You will see progress as it runs.', life: 4000 })
  } catch (e) {
    handleError(e, 'Start analysis')
  }
}

async function onView(analysisId: string): Promise<void> {
  try {
    await store.openAnalysis(analysisId)
  } catch (e) {
    handleError(e, 'Open analysis')
  }
}

async function onDownload(analysisId: string): Promise<void> {
  try {
    await store.openAnalysis(analysisId)
    // AnalysisDetailView exposes its own download button; opening is enough to surface it.
  } catch (e) {
    handleError(e, 'Open analysis')
  }
}

function onDelete(analysisId: string): void {
  confirm.require({
    message: 'Delete this analysis and its report file?',
    header: 'Delete analysis',
    icon: 'pi pi-exclamation-triangle',
    accept: async () => {
      try {
        await store.remove(analysisId)
        toast.add({ severity: 'success', summary: 'Deleted', life: 3000 })
      } catch (e) {
        handleError(e, 'Delete analysis')
      }
    },
  })
}
</script>
