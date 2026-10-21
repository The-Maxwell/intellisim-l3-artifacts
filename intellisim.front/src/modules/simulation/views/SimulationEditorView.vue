<template>
  <div class="h-full flex flex-col min-h-0">
    <div v-if="store.loading && !store.current" class="p-6 text-sm text-zinc-500">Loading…</div>

    <div
      v-else-if="!store.current"
      class="h-full flex flex-col items-center justify-center text-zinc-500 px-6 text-center"
    >
      <div class="w-16 h-16 bg-zinc-100 rounded-2xl flex items-center justify-center mb-4">
        <i class="pi pi-sliders-h text-3xl text-zinc-400" />
      </div>
      <p class="text-lg font-medium text-zinc-700 mb-1">Simulation not found</p>
      <p class="text-sm text-zinc-500 mb-4">This simulation may have been deleted.</p>
      <Button label="Back to simulations" icon="pi pi-arrow-left" size="small" @click="router.replace('/simulations')" />
    </div>

    <template v-else>
      <!-- Top bar: name + status + dirty + Save + Generate + Duplicate -->
      <div class="border-b border-zinc-200 bg-white px-6 h-14 shrink-0 flex items-center justify-between gap-4">
        <div class="min-w-0 flex items-center gap-2">
          <router-link
            to="/simulations"
            class="text-zinc-400 hover:text-zinc-700 shrink-0"
            title="Back to simulations"
          >
            <i class="pi pi-arrow-left text-sm" />
          </router-link>
          <InputText
            :modelValue="editable.name"
            class="font-semibold text-sm"
            size="small"
            placeholder="Untitled simulation"
            :disabled="readOnly"
            @update:modelValue="(v) => onEdited({ name: v ?? '' })"
          />
          <Tag :value="store.current.status" :severity="statusSeverity(store.current.status)" />
          <span v-if="store.dirty" class="inline-flex items-center gap-1 text-[11px] text-amber-600">
            <span class="w-1.5 h-1.5 rounded-full bg-amber-500" />
            unsaved
          </span>
        </div>
        <div class="flex items-center gap-2">
          <Button
            label="Run"
            icon="pi pi-play"
            size="small"
            severity="success"
            :disabled="!store.startReady || store.generating || store.dirty || store.terminal"
            :title="runButtonTitle"
            @click="goToRun"
          />
          <!-- View results: once a run has ended (finished/stopped/failed) the Run
               button is disabled, so this opens the run screen in replay mode to
               view the final world + widget dashboard with the collected data. -->
          <Button
            v-if="store.terminal"
            label="View results"
            icon="pi pi-eye"
            size="small"
            severity="secondary"
            title="View the final world and widgets (replay)"
            @click="router.push(`/simulations/${id}/run`)"
          />
          <!-- Wrapper span carries the tooltip: a disabled button emits no hover
               events, so the hint must live on an enabled ancestor. -->
          <span class="inline-flex" v-tooltip.bottom="analysisHint">
            <Button
              label="Analysis"
              icon="pi pi-chart-bar"
              size="small"
              severity="secondary"
              :disabled="!['finished', 'paused'].includes(store.current.status)"
              @click="router.push(`/simulations/${id}/analysis`)"
            />
          </span>
          <!-- Regenerate is only offered when the simulation is actually
               (re)generatable (store.editable — mirrors backend Editable():
               ready / generation_failed) or a generation is in flight. Terminal
               and running states hide it entirely, so the UI never presents an
               action the backend would reject with 409 not_editable. -->
          <GenerateMenu
            v-if="store.editable || store.generating"
            v-model:generationTarget="generationTarget"
            :archetypes="archetypes"
            :disabled="store.generating"
            @generate="onGenerate"
          />
          <Button
            label="Duplicate"
            icon="pi pi-copy"
            size="small"
            severity="secondary"
            outlined
            :disabled="store.generating"
            @click="openDuplicate"
          />
          <!-- Collapse / expand the fixed-width summary pane entirely -->
          <Button
            :icon="rightPaneCollapsed ? 'pi pi-angle-double-left' : 'pi pi-angle-double-right'"
            severity="secondary"
            text
            size="small"
            :title="rightPaneCollapsed ? 'Show summary panel' : 'Hide summary panel'"
            aria-label="Toggle side panel"
            @click="rightPaneCollapsed = !rightPaneCollapsed"
          />
          <Button
            label="Save"
            icon="pi pi-save"
            size="small"
            :loading="saving"
            :disabled="!store.dirty || saving || store.generating"
            @click="save"
          />
        </div>
      </div>

      <!-- Save validation errors -->
      <Message
        v-if="saveErrors.length"
        severity="error"
        :closable="true"
        class="mx-6 mt-3"
        @close="saveErrors = []"
      >
        <p class="text-xs font-medium mb-1">Save failed — validation:</p>
        <ul class="text-xs list-disc pl-4">
          <li v-for="(err, i) in saveErrors" :key="i">{{ err }}</li>
        </ul>
      </Message>

      <!-- Generation progress banner -->
      <Message v-if="store.generating" severity="info" :closable="false" class="mx-6 mt-3">
        <span class="text-sm">
          Generating…
          <template v-if="store.progress">
            run {{ store.progress.run }} · {{ store.progress.stage }}
            <template v-if="store.progress.detail"> — {{ store.progress.detail }}</template>
          </template>
        </span>
      </Message>

      <!-- Conflict: an external (SSE) update arrived while local edits are unsaved. -->
      <Message v-if="store.pendingExternal" severity="warn" :closable="false" class="mx-6 mt-3">
        <p class="text-xs font-medium mb-2">
          This simulation was updated elsewhere while you have unsaved changes.
        </p>
        <div class="flex items-center gap-2">
          <Button
            label="Apply update (discard my edits)"
            size="small"
            severity="secondary"
            @click="store.acceptExternalUpdate()"
          />
          <Button
            label="Keep my edits"
            size="small"
            severity="secondary"
            outlined
            @click="store.dismissExternalUpdate()"
          />
        </div>
      </Message>

      <!-- Manual placement pending: instances await positioning before the run. -->
      <Message
        v-if="unplacedCount > 0"
        severity="warn"
        :closable="false"
        class="mx-6 mt-3"
        data-testid="unplaced-banner"
      >
        <div class="flex items-center justify-between gap-3">
          <span class="text-xs">
            {{ unplacedCount }} agent(s) need placing before this simulation can run — drag them
            into the world in the Environment tab.
          </span>
          <Button
            v-if="hasEnvironmentSection"
            label="Place agents"
            size="small"
            severity="warn"
            @click="activeTab = 'environment'"
          />
        </div>
      </Message>

      <!-- Body: LEFT (tab-navigated form) grows; RIGHT (read-only summary) is a
           FIXED-width column that fully collapses. The whole editor is overlaid
           read-only while a generation run is in progress. -->
      <div class="flex-1 min-h-0 relative">
        <div v-if="readOnly" class="absolute inset-0 z-10 bg-white/40 cursor-not-allowed" />
        <div class="h-full flex">
          <!-- Sections as tabs (one at a time; lazy so the PixiJS environment
               canvas only mounts when opened). Status dot + count per tab. -->
          <div
            class="flex-1 min-w-0 h-full"
            :class="{ 'pointer-events-none': readOnly }"
            :inert="readOnly"
          >
            <Tabs v-model:value="activeTab" @update:value="autoSave" scrollable lazy class="h-full flex flex-col min-h-0">
              <TabList>
                <Tab value="general"><span class="tab-label"><span class="tab-dot" :class="dotClass('general')" />General</span></Tab>
                <Tab value="agents"><span class="tab-label"><span class="tab-dot" :class="dotClass('agents')" />Agents<span class="tab-count">{{ editable.agents.length }}</span></span></Tab>
                <Tab v-if="hasEnvironmentSection" value="environment"><span class="tab-label"><span class="tab-dot" :class="dotClass('environment')" />Environment</span></Tab>
                <Tab value="widgets"><span class="tab-label"><span class="tab-dot" :class="dotClass('widgets')" />Widgets<span class="tab-count">{{ editable.widgets.length }}</span></span></Tab>
                <Tab value="snapshot"><span class="tab-label"><i class="pi pi-camera text-[11px]" />Model snapshot</span></Tab>
              </TabList>
              <TabPanels class="flex-1 min-h-0 overflow-y-auto !px-4 !py-4">
                <TabPanel value="general">
                  <GeneralSection :modelValue="editable" @update:modelValue="onEdited" />
                </TabPanel>

                <TabPanel value="agents">
                  <AgentsSection
                    :agents="editable.agents"
                    :archetypes="archetypes"
                    :spatial="isSpatial"
                    @update:agents="(a) => onEdited({ agents: a })"
                  />
                </TabPanel>

                <TabPanel v-if="hasEnvironmentSection" value="environment">
                  <EnvironmentSection
                    :snapshotEnvironment="store.current.model_snapshot?.environment ?? null"
                    :globals="environmentGlobals"
                    :spatial="isSpatial"
                    :environment="store.current.environment ?? null"
                    :agents="editable.agents"
                    :snapshot="store.current.model_snapshot ?? null"
                    :readonly="readOnly"
                    @update:globals="onGlobalsEdited"
                    @update:environment="(e) => onEdited({ environment: e })"
                    @update:agents="(a) => onEdited({ agents: a })"
                  />
                </TabPanel>

                <TabPanel value="widgets">
                  <WidgetsSection
                    :widgets="editable.widgets"
                    :schema="widgetSchema"
                    :agents="editable.agents"
                    :environment="editable.environment"
                    :snapshot="store.current.model_snapshot ?? null"
                    :readonly="readOnly"
                    :spatial="isSpatial"
                    :envWidth="editable.environment?.width"
                    :envHeight="editable.environment?.height"
                    @update:widgets="(w) => onEdited({ widgets: w })"
                  />
                </TabPanel>

                <!-- Read-only view of the frozen source-model definition this
                     simulation was generated from (independent of later live-model
                     edits). Lazily fetched via loadSnapshot when opened. -->
                <TabPanel value="snapshot">
                  <ModelSnapshotPanel
                    :snapshot="modelSnapshot"
                    :loading="snapshotLoading"
                    :error="snapshotError"
                    @retry="retrySnapshot"
                  />
                </TabPanel>
              </TabPanels>
            </Tabs>
          </div>

          <!-- Read-only summary (replaces the JSON editor): fixed width, collapsible.
               Validation blockers + Token usage + Status history all fold in here. -->
          <div
            v-show="!rightPaneCollapsed"
            class="basis-1/4 shrink-0 grow-0 h-full min-h-0 border-l border-zinc-200"
            :class="{ 'pointer-events-none': readOnly }"
            :inert="readOnly"
          >
            <SimulationSummaryPanel
              :key="id"
              :simulation="store.current"
              :validation="store.validation"
              @navigate="goToSection"
            />
          </div>
        </div>
      </div>
    </template>

    <!-- Duplicate dialog -->
    <Dialog v-model:visible="duplicateDialog" modal header="Duplicate simulation" :style="{ width: '420px' }">
      <div>
        <label class="text-xs font-medium text-zinc-600 mb-1 block">Name</label>
        <InputText
          v-model="duplicateName"
          class="w-full"
          size="small"
          placeholder="Copy name"
          @keydown.enter="duplicate"
        />
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <Button label="Cancel" severity="secondary" size="small" @click="duplicateDialog = false" />
          <Button label="Duplicate" size="small" :loading="duplicating" @click="duplicate" />
        </div>
      </template>
    </Dialog>
  </div>
</template>

<script setup lang="ts">
// Full simulation editor. Mirrors the redesigned ModelEditorView: a top bar
// (name/status/dirty + Run/Analysis/Generate/Duplicate + summary collapse +
// Save), a tab-navigated LEFT form (General / Agents / Environment* / Widgets)
// and a fixed-width, collapsible RIGHT read-only SimulationSummaryPanel (which
// folds in validation blockers + token usage + status history). No JSON editor;
// simulations have no chat.
//
// EDIT PROPAGATION (matching modelStore.setCurrentModel): every form section
// emits a partial editable slice → onEdited merges it onto the current editable
// view, applies it back onto a CLONE of store.current via applyEditable,
// reassigns store.current, and calls store.markDirty(). The summary re-derives
// live from store.current.
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import Message from 'primevue/message'
import Tag from 'primevue/tag'
import InputText from 'primevue/inputtext'
import Dialog from 'primevue/dialog'
import Tooltip from 'primevue/tooltip'
import Tabs from 'primevue/tabs'
import TabList from 'primevue/tablist'
import Tab from 'primevue/tab'
import TabPanels from 'primevue/tabpanels'
import TabPanel from 'primevue/tabpanel'
import GeneralSection from '@/modules/simulation/components/GeneralSection.vue'
import AgentsSection from '@/modules/simulation/components/AgentsSection.vue'
import EnvironmentSection from '@/modules/simulation/components/EnvironmentSection.vue'
import SimulationSummaryPanel from '@/modules/simulation/components/SimulationSummaryPanel.vue'
import GenerateMenu from '@/modules/simulation/components/GenerateMenu.vue'
import WidgetsSection from '@/modules/simulation/components/WidgetsSection.vue'
import ModelSnapshotPanel from '@/modules/simulation/components/snapshot/ModelSnapshotPanel.vue'
import { schemaFromSnapshot } from '@/shared/components/widgets/widgetSchema'
import { useSimulationStore } from '@/modules/simulation/stores/simulationStore'
import { useSimulationApi } from '@/modules/simulation/composables/useSimulationApi'
import { useEditorAutoSave } from '@/shared/composables/useEditorAutoSave'
import { useErrorHandler } from '@/shared/composables/useErrorHandler'
import { ApiError } from '@/shared/api/client'
import { statusSeverity } from '@/modules/simulation/views/simulationStatus'
import {
  applyEditable,
  toEditable,
  type EditableSimulation,
} from '@/modules/simulation/editableSimulation'
import { normalizeModelResponse } from '@/shared/types'
import type { ExecutionLlmTarget, GenerationScope, ModelResponse, SnapshotAgent } from '@/shared/types'

const props = defineProps<{ id: string }>()

const vTooltip = Tooltip

const router = useRouter()
const store = useSimulationStore()
const api = useSimulationApi()
const { handleError, handleSuccess } = useErrorHandler()

const saving = ref(false)
const saveErrors = ref<string[]>([])

// --- Model snapshot tab (lazy) ---
// The frozen full source-model copy the simulation was generated from. Fetched
// once, only when the Model snapshot tab is first opened (it's heavier than the
// trimmed inline model_snapshot and irrelevant to the other tabs). Kept in local
// refs — it's immutable, so no store/SSE/conflict machinery is needed.
const modelSnapshot = ref<ModelResponse | null>(null)
const snapshotLoading = ref(false)
const snapshotError = ref<string | null>(null)

async function loadSnapshot() {
  if (snapshotLoading.value || modelSnapshot.value) return // fetch once
  snapshotLoading.value = true
  snapshotError.value = null
  try {
    modelSnapshot.value = normalizeModelResponse(await api.getModelSnapshot(props.id))
  } catch (err) {
    snapshotError.value =
      err instanceof ApiError ? err.message : 'Failed to load the model snapshot.'
  } finally {
    snapshotLoading.value = false
  }
}

function retrySnapshot() {
  modelSnapshot.value = null
  snapshotError.value = null
  loadSnapshot()
}

// The generation-model override used by the next Generate run (GenerateMenu
// v-model). Defaults to the simulation's persisted llm_generation_target; the
// user can change it before generating.
const generationTarget = ref<ExecutionLlmTarget | null>(null)
watch(
  () => store.current?.llm_generation_target,
  (t) => {
    generationTarget.value = t ?? null
  },
  { immediate: true }
)

// Active section tab (replaces the multi-open accordion). Default = General.
const activeTab = ref<string>('general')

// Lazy trigger: fetch the model snapshot when its tab is opened (idempotent; the
// outer Tabs is also `lazy`, so the panel mounts on first open anyway).
watch(activeTab, (t) => {
  if (t === 'snapshot') loadSnapshot()
})

// Reset the cached snapshot if this view instance is reused for a different sim.
watch(
  () => props.id,
  () => {
    modelSnapshot.value = null
    snapshotError.value = null
    snapshotLoading.value = false
    if (activeTab.value === 'snapshot') activeTab.value = 'general'
  }
)

// Fixed-width summary column collapse toggle (mirrors the model editor).
const rightPaneCollapsed = ref(false)

// Read-only while a generation run is in progress (the SSE simulation.updated
// flips status back to ready/generation_failed via the store).
const readOnly = computed(() => store.generating)

// The editable slice of the open simulation (the subset the form + JSON edit).
const editable = computed<EditableSimulation>(() =>
  store.current ? toEditable(store.current) : blankEditable()
)

function blankEditable(): EditableSimulation {
  return { name: '', description: '', steps: 1, seed: 0, agents: [], environment: null, widgets: [] }
}

const isSpatial = computed(() => store.current?.model_snapshot?.is_spatial ?? false)
const archetypes = computed<SnapshotAgent[]>(() => store.current?.model_snapshot?.agents ?? [])

// Manual-placement instances awaiting a position: on a spatial simulation a
// null-position agent uniquely means "placed by hand, not yet positioned". The
// run is blocked (backend validation) until they are placed in the Environment tab.
const unplacedCount = computed(() =>
  isSpatial.value ? (store.current?.agents ?? []).filter((a) => !a.position).length : 0
)

// Schema context for the typed widget editor/preview, derived from the snapshot
// (archetypes + property uids + globals + states).
const widgetSchema = computed(() => schemaFromSnapshot(store.current?.model_snapshot ?? null))

// Environment globals map (bound to the EnvironmentSection global-properties form).
const environmentGlobals = computed<Record<string, unknown>>(
  () => store.current?.environment?.global_properties ?? {}
)

// Show the environment section when the model is spatial OR defines global props.
const hasEnvironmentSection = computed(
  () =>
    isSpatial.value ||
    (store.current?.model_snapshot?.environment?.global_properties?.length ?? 0) > 0
)

// --- section tabs (mirrors the model editor) ---
// Tabs that currently exist (Environment only when hasEnvironmentSection). Used
// to clamp summary deep-links to a rendered tab.
const tabKeys = computed<string[]>(() => {
  const keys = ['general', 'agents']
  if (hasEnvironmentSection.value) keys.push('environment')
  keys.push('widgets')
  keys.push('snapshot')
  return keys
})

// Summary emitted navigate(tab) — switch to that section (clamped to an existing
// tab; e.g. an environment link is dropped to General on a non-spatial sim).
function goToSection(tab: string) {
  activeTab.value = tabKeys.value.includes(tab) ? tab : 'general'
}

// Map a validation-issue path head to the tab it belongs to (then clamp).
function tabForPath(path: string): string {
  const head = /^(\w+)/.exec(path || '')?.[1] ?? ''
  const map: Record<string, string> = {
    agents: 'agents',
    environment: 'environment',
    globals: 'environment',
    widgets: 'widgets',
  }
  const tab = map[head] ?? 'general'
  return tabKeys.value.includes(tab) ? tab : 'general'
}

const errorTabs = computed(
  () => new Set((store.validation?.errors ?? []).map((e) => tabForPath(e.path)))
)
const warnTabs = computed(
  () => new Set((store.validation?.warnings ?? []).map((w) => tabForPath(w.path)))
)

function sectionHasContent(tab: string): boolean {
  switch (tab) {
    case 'general':
      return true
    case 'agents':
      return editable.value.agents.length > 0
    case 'environment':
      return hasEnvironmentSection.value
    case 'widgets':
      return editable.value.widgets.length > 0
    default:
      return false
  }
}

// Per-tab status dot: red if a validation error targets the section, amber if a
// warning does, emerald if the section has content, otherwise gray (empty).
function dotClass(tab: string): string {
  if (errorTabs.value.has(tab)) return 'bg-rose-500'
  if (warnTabs.value.has(tab)) return 'bg-amber-500'
  return sectionHasContent(tab) ? 'bg-emerald-500' : 'bg-zinc-300'
}

// Apply an editable-slice patch back onto store.current (clone + reassign +
// markDirty), the simulation-store analogue of modelStore.setCurrentModel.
function onEdited(patch: Partial<EditableSimulation>) {
  if (!store.current) return
  const next = { ...editable.value, ...patch }
  store.current = applyEditable(store.current, next)
  store.markDirty()
}

// Environment section emits the updated global_properties map; merge it into the
// current environment (creating a blank environment if none exists yet).
function onGlobalsEdited(globals: Record<string, unknown>) {
  if (!store.current) return
  const env = store.current.environment ?? {
    width: 0,
    height: 0,
    torus: false,
    shapes: [],
  }
  onEdited({ environment: { ...env, global_properties: globals } })
}

// Tooltip explaining why the Run button is disabled (start_ready gate + unsaved
// edits — a run executes the persisted simulation, so local edits must be saved
// first).
const runButtonTitle = computed(() => {
  if (store.terminal) return 'Simulation has finished — use View results'
  if (store.dirty) return 'Save your changes before running'
  if (!store.startReady) return 'Simulation is not ready to run'
  return 'Run this simulation'
})

// Hint shown on hover over the disabled "Analysis" nav button (empty when ready).
const analysisHint = computed(() =>
  ['finished', 'paused'].includes(store.current?.status ?? '')
    ? ''
    : 'Analysis is available only when the simulation has finished or is paused.'
)

function goToRun() {
  if (!store.startReady || store.dirty || store.terminal) return
  router.push(`/simulations/${props.id}/run`)
}

async function onGenerate(scope: GenerationScope) {
  try {
    await store.generate(scope, generationTarget.value)
  } catch (err) {
    handleError(err, 'Generate Simulation')
  }
}

// Persist the working copy WITHOUT announcing success — shared by the manual
// Save button and by auto-save. On a 400/422 it fills the inline saveErrors
// banner; on any error it rethrows so callers (the route-leave guard) can react.
async function persist(): Promise<void> {
  if (saving.value) return
  saving.value = true
  saveErrors.value = []
  try {
    await store.saveCurrent()
  } catch (err) {
    if (err instanceof ApiError && (err.status === 400 || err.status === 422)) {
      saveErrors.value =
        err.fields && Object.keys(err.fields).length
          ? Object.entries(err.fields).map(([f, r]) => `${f}: ${r}`)
          : [err.message]
    } else {
      handleError(err, 'Save Simulation')
    }
    throw err
  } finally {
    saving.value = false
  }
}

// Manual Save button: persist, then toast on success.
async function save() {
  if (saving.value || !store.dirty) return
  try {
    await persist()
    handleSuccess('Simulation saved')
  } catch {
    // persist() already surfaced the failure (banner or global handler).
  }
}

// Auto-save on tab switch + leaving the editor (route/refresh). Silent success;
// stands down while a save/generation is in flight or an external-update
// conflict is pending. Clearing dirty also re-enables Run promptly after a tab
// switch (Run is disabled while dirty).
const { autoSave } = useEditorAutoSave({
  isDirty: () => store.dirty,
  isBlocked: () => saving.value || store.generating || !!store.pendingExternal,
  save: persist,
})

// --- Duplicate ---
const duplicateDialog = ref(false)
const duplicating = ref(false)
const duplicateName = ref('')

function openDuplicate() {
  duplicateName.value = store.current?.name ? `${store.current.name} (copy)` : ''
  duplicateDialog.value = true
}

async function duplicate() {
  if (duplicating.value || !store.current) return
  duplicating.value = true
  try {
    const sim = await store.duplicate(store.current.id, duplicateName.value.trim() || undefined)
    duplicateDialog.value = false
    handleSuccess('Simulation duplicated')
    router.push(`/simulations/${sim.id}`)
  } catch (err) {
    handleError(err, 'Duplicate Simulation')
  } finally {
    duplicating.value = false
  }
}

onMounted(() => {
  store
    .open(props.id, {
      onCompleted: () => handleSuccess('Generation completed'),
      onFailed: (message) => handleError(new Error(message), 'Generation failed'),
    })
    .catch((err) => handleError(err, 'Open Simulation'))
})

onUnmounted(() => {
  store.close()
})
</script>

<style scoped>
/* Tab header decorations: status dot (colour from the bound Tailwind class) + a
   count chip. Plain CSS so it doesn't depend on Tailwind utilities surviving
   PrimeVue's tab markup. Mirrors ModelEditorView. */
.tab-label {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
}
.tab-dot {
  width: 0.5rem;
  height: 0.5rem;
  border-radius: 9999px;
  flex: none;
}
.tab-count {
  margin-left: 0.1rem;
  font-size: 0.6875rem;
  line-height: 1;
  padding: 0.12rem 0.4rem;
  border-radius: 9999px;
  background: var(--p-surface-200, #e4e4e7);
  color: var(--p-surface-600, #52525b);
  font-variant-numeric: tabular-nums;
}
</style>
