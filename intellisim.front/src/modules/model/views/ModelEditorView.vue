<template>
  <div class="h-full flex flex-col min-h-0">
    <!-- Forbidden (403): the model exists but belongs to another user -->
    <div
      v-if="forbidden"
      class="h-full flex flex-col items-center justify-center text-zinc-500 px-6 text-center"
    >
      <div class="w-16 h-16 bg-zinc-100 rounded-2xl flex items-center justify-center mb-4">
        <i class="pi pi-lock text-3xl text-zinc-400" />
      </div>
      <p class="text-lg font-medium text-zinc-700 mb-1">Access denied</p>
      <p class="text-sm text-zinc-500 mb-4">You don't have access to this model.</p>
      <Button label="Back to models" icon="pi pi-arrow-left" size="small" @click="router.replace('/models')" />
    </div>

    <!-- Deleted / not found (404) -->
    <div
      v-else-if="notFound"
      class="h-full flex flex-col items-center justify-center text-zinc-500 px-6 text-center"
    >
      <div class="w-16 h-16 bg-zinc-100 rounded-2xl flex items-center justify-center mb-4">
        <i class="pi pi-trash text-3xl text-zinc-400" />
      </div>
      <p class="text-lg font-medium text-zinc-700 mb-1">Model not found</p>
      <p class="text-sm text-zinc-500 mb-4">This model may have been deleted.</p>
      <Button label="Back to models" icon="pi pi-arrow-left" size="small" @click="router.replace('/models')" />
    </div>

    <div v-else-if="store.loading && !store.current" class="p-6 text-sm text-zinc-500">Loading…</div>

    <template v-else-if="store.current">
      <!-- Top bar: name + dirty indicator + right-pane toggle + Save -->
      <div class="border-b border-zinc-200 bg-white px-6 h-14 shrink-0 flex items-center justify-between gap-4">
        <div class="min-w-0 flex items-center gap-2">
          <h2 class="text-sm font-semibold text-zinc-800 truncate">
            {{ store.current.model.name || 'Untitled model' }}
          </h2>
          <Tag v-if="store.dirty" value="unsaved" severity="warn" />
          <!-- Always-visible readiness chip (mirrors the summary banner; stays
               visible even when the right pane is showing Chat) -->
          <span
            class="flex items-center gap-1.5 text-xs whitespace-nowrap"
            :title="store.validation?.simulation_ready ? 'Simulation ready' : 'Not simulation ready'"
          >
            <span
              class="w-2 h-2 rounded-full flex-none"
              :class="store.validation?.simulation_ready ? 'bg-emerald-500' : 'bg-amber-500'"
            />
            <span class="text-zinc-500">{{ store.validation?.simulation_ready ? 'Ready' : 'Not ready' }}</span>
          </span>
        </div>
        <div class="flex items-center gap-2">
          <!-- Simulations for this model: view the filtered list / create a new one -->
          <Button
            label="Simulations"
            icon="pi pi-list"
            severity="secondary"
            outlined
            size="small"
            @click="viewSimulations"
          />
          <Button
            label="New simulation"
            icon="pi pi-plus"
            severity="secondary"
            outlined
            size="small"
            :disabled="!store.validation?.simulation_ready"
            :title="store.validation?.simulation_ready ? 'Create a simulation from this model' : 'Model must be simulation-ready'"
            @click="openCreateSim"
          />
          <Button
            label="Duplicate"
            icon="pi pi-copy"
            severity="secondary"
            outlined
            size="small"
            title="Duplicate this model (and its chat)"
            @click="dupDialog = true"
          />
          <!-- Right-pane: Summary ⟷ Chat selector (only while the pane is open) -->
          <SelectButton
            v-if="!rightPaneCollapsed"
            v-model="rightPane"
            :options="rightPaneOptions"
            optionLabel="label"
            optionValue="value"
            :allowEmpty="false"
            size="small"
          />
          <!-- Collapse / expand the fixed-width right pane entirely -->
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
            :disabled="!store.dirty || saving"
            @click="save"
          />
        </div>
      </div>

      <!-- Save validation errors (400/422 validation_failed → field messages) -->
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

      <!-- Conflict: an external (SSE) update arrived while local edits are unsaved.
           Non-dismissable — the user must choose which version wins. -->
      <Message
        v-if="store.pendingExternal"
        severity="warn"
        :closable="false"
        class="mx-6 mt-3"
      >
        <p class="text-xs font-medium mb-2">
          This model was updated elsewhere while you have unsaved changes.
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

      <!-- Body: LEFT (sectioned form, tab-navigated) grows; RIGHT (Summary ⟷ Chat)
           is a FIXED-width column (not resizable) that can be collapsed entirely. -->
      <div class="flex-1 min-h-0 flex">
        <div class="flex-1 min-w-0 h-full">
            <!-- Section tabs replace the old top-level accordion: one section
                 renders at a time (Tabs `lazy` so e.g. the PixiJS environment
                 canvas only mounts when its tab is opened). Each tab header shows
                 a status dot (validation-aware) + a count. Each section binds to a
                 slice of the model and pushes a full updated copy via onModelEdited
                 → store.setCurrentModel (marks dirty); the summary on the right
                 re-derives live. -->
            <Tabs v-model:value="activeTab" @update:value="autoSave" scrollable lazy class="h-full flex flex-col min-h-0">
              <TabList>
                <Tab value="general"><span class="tab-label"><span class="tab-dot" :class="dotClass('general')" />General</span></Tab>
                <Tab value="llm"><span class="tab-label"><span class="tab-dot" :class="dotClass('llm')" />LLM</span></Tab>
                <Tab value="agents"><span class="tab-label"><span class="tab-dot" :class="dotClass('agents')" />Agents<span class="tab-count">{{ model.agents.length }}</span></span></Tab>
                <Tab value="environment"><span class="tab-label"><span class="tab-dot" :class="dotClass('environment')" />Environment</span></Tab>
                <Tab value="rules"><span class="tab-label"><span class="tab-dot" :class="dotClass('rules')" />Rules<span class="tab-count">{{ model.rules.length }}</span></span></Tab>
                <Tab value="interactions"><span class="tab-label"><span class="tab-dot" :class="dotClass('interactions')" />Interactions<span class="tab-count">{{ model.interactions.length }}</span></span></Tab>
                <Tab value="events"><span class="tab-label"><span class="tab-dot" :class="dotClass('events')" />Events<span class="tab-count">{{ model.events.length }}</span></span></Tab>
                <Tab value="widgets"><span class="tab-label"><span class="tab-dot" :class="dotClass('widgets')" />Widgets<span class="tab-count">{{ (model.widgets ?? []).length }}</span></span></Tab>
              </TabList>
              <TabPanels class="flex-1 min-h-0 overflow-y-auto !px-4 !py-4">
                <TabPanel value="general">
                  <GeneralForm :modelValue="model" @update:modelValue="onModelEdited" />
                </TabPanel>

                <!-- LLM: the model's three LLM configs (advanced knobs stay collapsed) -->
                <TabPanel value="llm">
                  <ModelLlmForm :modelValue="model" @update:modelValue="onModelEdited" />
                </TabPanel>

                <TabPanel value="agents">
                  <MasterDetailList
                    :items="model.agents"
                    v-model:selectedIndex="agentSel"
                    addLabel="Add agent"
                    emptyText="No agents yet."
                    @add="addAgent"
                  >
                    <template #item="{ item, index }">
                      <span class="truncate flex-1">{{ item.name || item.uid || `Agent ${index + 1}` }}</span>
                      <span v-if="item.type === 'archetype'" class="text-[9px] uppercase tracking-wide text-zinc-400 flex-none">arch</span>
                    </template>
                    <template #detail="{ item, index }">
                      <AgentForm
                        :modelValue="item"
                        :collisionMode="model.environment?.collision_mode"
                        :isSpatial="model.is_spatial"
                        @update:modelValue="(a) => updateAgent(index, a)"
                        @remove="removeAgent(index)"
                      />
                    </template>
                  </MasterDetailList>
                </TabPanel>

                <TabPanel value="environment">
                  <EnvironmentForm
                    :modelValue="environment"
                    :cellSize="model.spatial_cell_size"
                    :model="model"
                    :modelId="id"
                    :isSpatial="model.is_spatial"
                    @update:modelValue="onEnvironmentEdited"
                  />
                </TabPanel>

                <TabPanel value="rules">
                  <RuleBuilder
                    :modelValue="model.rules"
                    :model="model"
                    v-model:selectedIndex="ruleSel"
                    @update:modelValue="(r) => onModelEdited({ ...model, rules: r })"
                  />
                </TabPanel>

                <TabPanel value="interactions">
                  <InteractionForm
                    :modelValue="model.interactions"
                    :model="model"
                    v-model:selectedIndex="interactionSel"
                    @update:modelValue="(it) => onModelEdited({ ...model, interactions: it })"
                  />
                </TabPanel>

                <TabPanel value="events">
                  <EventForm
                    :modelValue="model.events"
                    :agents="model.agents"
                    v-model:selectedIndex="eventSel"
                    @update:modelValue="(e) => onModelEdited({ ...model, events: e })"
                  />
                </TabPanel>

                <!-- Widgets (template layout; model has no instances → structure/empty previews) -->
                <TabPanel value="widgets">
                  <WidgetsSection
                    :widgets="model.widgets ?? []"
                    :schema="widgetSchema"
                    :spatial="model.is_spatial"
                    :envWidth="model.environment?.width"
                    :envHeight="model.environment?.height"
                    @update:widgets="onWidgetsEdited"
                  />
                </TabPanel>
              </TabPanels>
            </Tabs>
        </div>

        <!-- Right pane: FIXED width (no drag-resize); fully collapsible. v-show
             (not v-if) keeps the chat session's single SSE connection alive even
             while the pane is hidden. -->
        <div
          v-show="!rightPaneCollapsed"
          class="basis-1/4 shrink-0 grow-0 h-full min-h-0 border-l border-zinc-200"
        >
              <!-- Read-only summary replaces the JSON editor; clicking a tile/issue
                   jumps to the matching section via goToSection. -->
              <ModelSummaryPanel
                v-show="rightPane === 'summary'"
                :key="id"
                :model="model"
                :validation="store.validation"
                @navigate="goToSection"
              />
              <!-- Chat panel stays MOUNTED across pane toggles (v-show, not v-if)
                   so its session keeps the single SSE connection alive and
                   model.updated → store refreshes BOTH panes live.
                   :key="id" forces a remount on model-id changes so the panel
                   re-resolves useModelChatSession for the new id. -->
              <ModelChatPanel
                v-show="rightPane === 'chat'"
                :key="id"
                :modelId="id"
                :model="model"
              />
        </div>
      </div>

      <!-- Create a simulation from this model (locked to it) -->
      <CreateSimulationDialog
        v-model:visible="createSimDialog"
        :presetModelId="store.current.id"
        :presetModelName="store.current.model.name"
      />

      <!-- Duplicate this model (+ its companion chat) → open the copy -->
      <DuplicateModelDialog
        v-model:visible="dupDialog"
        :modelId="store.current.id"
        :modelName="store.current.model.name"
        @duplicated="onModelDuplicated"
      />
    </template>
  </div>
</template>

<script setup lang="ts">
// Phase 4A editor SHELL: top bar (name, dirty, save, JSON/Chat toggle) + a
// resizable split between a LEFT form placeholder (Phase 4B) and a RIGHT pane
// that toggles between the JSON editor and a chat placeholder (Phase 6). The
// PixiJS EnvironmentCanvas (Phase 5) lands inside the form's environment section.
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import Message from 'primevue/message'
import SelectButton from 'primevue/selectbutton'
import Tag from 'primevue/tag'
import Tabs from 'primevue/tabs'
import TabList from 'primevue/tablist'
import Tab from 'primevue/tab'
import TabPanels from 'primevue/tabpanels'
import TabPanel from 'primevue/tabpanel'
import ModelChatPanel from '@/modules/model/components/ModelChatPanel.vue'
import ModelSummaryPanel from '@/modules/model/components/ModelSummaryPanel.vue'
import GeneralForm from '@/modules/model/components/GeneralForm.vue'
import ModelLlmForm from '@/modules/model/components/ModelLlmForm.vue'
import AgentForm from '@/modules/model/components/AgentForm.vue'
import MasterDetailList from '@/shared/components/MasterDetailList.vue'
import EnvironmentForm from '@/modules/model/components/EnvironmentForm.vue'
import { colorProperty, SHAPE_PALETTE } from '@/modules/model/components/environmentCanvasGeometry'
import RuleBuilder from '@/modules/model/components/RuleBuilder.vue'
import InteractionForm from '@/modules/model/components/InteractionForm.vue'
import EventForm from '@/modules/model/components/EventForm.vue'
import WidgetsSection from '@/modules/simulation/components/WidgetsSection.vue'
import CreateSimulationDialog from '@/modules/simulation/components/CreateSimulationDialog.vue'
import DuplicateModelDialog from '@/modules/model/components/DuplicateModelDialog.vue'
import { schemaFromModel } from '@/shared/components/widgets/widgetSchema'
import { useModelStore } from '@/modules/model/stores/modelStore'
import { useModelChatSession } from '@/modules/model/composables/useModelChatSession'
import { useEditorAutoSave } from '@/shared/composables/useEditorAutoSave'
import { useErrorHandler } from '@/shared/composables/useErrorHandler'
import { ApiError } from '@/shared/api/client'
import type { Model, Agent, EnvironmentDTO, WidgetConfig, ModelResponse } from '@/shared/types'

const props = defineProps<{ id: string }>()

const router = useRouter()
const store = useModelStore()
const { handleError, handleSuccess } = useErrorHandler()

type RightPane = 'summary' | 'chat'
const rightPane = ref<RightPane>('summary')
// The right pane is a fixed-width column (no drag-resize); this collapses it
// entirely so the form spans the full width. Kept mounted via v-show when hidden.
const rightPaneCollapsed = ref(false)
const rightPaneOptions = [
  { label: 'Summary', value: 'summary' as RightPane },
  { label: 'Chat', value: 'chat' as RightPane },
]

// --- Simulations for this model ---
const createSimDialog = ref(false)

// Jump to the simulations list filtered to this model (deep-link ?model_id=).
function viewSimulations() {
  if (!store.current) return
  router.push({ path: '/simulations', query: { model_id: store.current.id } })
}

function openCreateSim() {
  createSimDialog.value = true
}

// --- Duplicate this model ---
const dupDialog = ref(false)

// A duplicate landed → open the copy's editor (the view re-opens on id change).
function onModelDuplicated(model: ModelResponse) {
  handleSuccess('Model duplicated')
  router.push(`/models/${model.id}`)
}

const saving = ref(false)
const saveErrors = ref<string[]>([])
const forbidden = ref(false)
const notFound = ref(false)

// The model-companion chat session is the SINGLE SSE owner for the open model:
// it routes message.created → its message list and model.updated →
// store.applyExternalModelUpdate (so the form/canvas/JSON refresh live, while a
// stale echo is ignored and an update over unsaved edits is parked for the
// conflict banner). The editor owns its open/close lifecycle; the
// ModelChatPanel consumes the same shared instance.
let session: ReturnType<typeof useModelChatSession> | null = null
let stopDeletedWatch: (() => void) | null = null

async function openModel(id: string) {
  forbidden.value = false
  notFound.value = false
  saveErrors.value = []
  try {
    await store.open(id)
  } catch (err) {
    if (err instanceof ApiError && err.status === 403) forbidden.value = true
    else if (err instanceof ApiError && err.status === 404) notFound.value = true
    else handleError(err, 'Open Model')
  }
}

function openSession(id: string) {
  closeSession()
  session = useModelChatSession(id)
  session.open(id)
  // model.deleted (via the session SSE) surfaces the not-found panel.
  stopDeletedWatch = watch(session.deleted, (d) => {
    if (d) notFound.value = true
  })
}

function closeSession() {
  stopDeletedWatch?.()
  stopDeletedWatch = null
  session?.close()
  session = null
}

onMounted(() => {
  resetSectionState()
  openModel(props.id)
  openSession(props.id)
})

watch(
  () => props.id,
  (id) => {
    if (!id) return
    resetSectionState()
    openModel(id)
    openSession(id)
  }
)

onUnmounted(() => {
  closeSession()
})

// Active section tab (replaces the old multi-open accordion). Default = General.
const activeTab = ref<string>('general')

// Master-detail selection per list section (also driven by summary deep-links).
const agentSel = ref(0)
const ruleSel = ref(0)
const interactionSel = ref(0)
const eventSel = ref(0)

// Tabs that currently exist. Environment is present for every model (non-spatial
// models edit env global properties + LLM prompt context; geometry/shapes are
// hidden inside the form). Used to clamp summary deep-links to a rendered tab.
const tabKeys = computed<string[]>(() => [
  'general',
  'llm',
  'agents',
  'environment',
  'rules',
  'interactions',
  'events',
  'widgets',
])

// Summary emitted navigate(tab, index?) — switch to that section and, for a list
// section, select the referenced item. Clamp to an existing tab (e.g. an unknown
// link falls back to General).
function goToSection(tab: string, index?: number) {
  const t = tabKeys.value.includes(tab) ? tab : 'general'
  activeTab.value = t
  if (index == null) return
  if (t === 'agents') agentSel.value = index
  else if (t === 'rules') ruleSel.value = index
  else if (t === 'interactions') interactionSel.value = index
  else if (t === 'events') eventSel.value = index
}

// Reset the active tab + per-list selections when the editor switches models.
function resetSectionState() {
  activeTab.value = 'general'
  agentSel.value = 0
  ruleSel.value = 0
  interactionSel.value = 0
  eventSel.value = 0
}

// Map a validation-issue path head to the tab it belongs to, then clamp to a tab
// that actually exists (so LLM issues light up General until the LLM tab exists).
function tabForPath(path: string): string {
  const head = /^(\w+)/.exec(path || '')?.[1] ?? ''
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
  const m = model.value
  switch (tab) {
    case 'general':
      return true
    case 'agents':
      return m.agents.length > 0
    case 'environment':
      return (
        !!m.environment &&
        ((m.environment.global_properties?.length ?? 0) > 0 ||
          !!m.environment.llm_prompt_context ||
          (m.is_spatial && (m.environment.width > 0 || m.environment.height > 0)))
      )
    case 'rules':
      return m.rules.length > 0
    case 'interactions':
      return m.interactions.length > 0
    case 'events':
      return m.events.length > 0
    case 'llm':
      return (
        !!m.llm_config?.target?.provider_uid ||
        !!m.simulation_config?.llm_simulation_generation_config?.target?.provider_uid ||
        !!m.simulation_config?.llm_simulation_execution_config?.target?.provider_uid
      )
    case 'widgets':
      return (m.widgets ?? []).length > 0
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

// The currently-open model body. Both the form sections and the JSON panel edit
// THIS object; every edit is pushed back through store.setCurrentModel.
const model = computed<Model>(() => store.current!.model)

// A non-optional environment for the form. Every model gets one (non-spatial
// models use it for global properties + LLM prompt context); fall back to a
// blank so the form never sees undefined.
const environment = computed<EnvironmentDTO>(
  () =>
    model.value.environment ?? {
      uid: '',
      width: 0,
      height: 0,
      torus: false,
      shapes: [],
      global_properties: [],
    }
)

// Schema context for the typed widget editor/preview, derived from the model's
// agents + environment global properties (the model has no instances, so widget
// previews show structure/empty rather than live numbers).
const widgetSchema = computed(() => schemaFromModel(model.value))

// JSON panel OR any form section edited the model → push the updated body into
// the store (which clones current + marks dirty). The JSON panel re-syncs from
// store.current.model, so both editors stay consistent.
function onModelEdited(updated: Model) {
  store.setCurrentModel(updated)
}

// Widgets section edited the model's template widget layout.
function onWidgetsEdited(widgets: WidgetConfig[]) {
  onModelEdited({ ...model.value, widgets })
}

function onEnvironmentEdited(env: EnvironmentDTO) {
  onModelEdited({ ...model.value, environment: env })
}

// --- agent list (add / update / remove) managed here; AgentForm edits one item ---
function updateAgent(index: number, agent: Agent) {
  onModelEdited({ ...model.value, agents: model.value.agents.map((a, i) => (i === index ? agent : a)) })
}
function removeAgent(index: number) {
  const next = model.value.agents.filter((_, i) => i !== index)
  onModelEdited({ ...model.value, agents: next })
  agentSel.value = Math.max(0, Math.min(agentSel.value, next.length - 1))
}
function addAgent() {
  // Spatial models auto-get the conventional `color` property (palette default by
  // index) so a new agent renders with a distinct colour out of the box, mirroring
  // the backend StampDefaultColorProperty and the "+ Add shape" flow. Non-spatial
  // models get no colour (nothing renders them).
  const properties: Agent['properties'] = model.value.is_spatial
    ? [colorProperty(SHAPE_PALETTE[model.value.agents.length % SHAPE_PALETTE.length] ?? SHAPE_PALETTE[0])]
    : []
  const blank: Agent = {
    uid: '',
    name: '',
    description: '',
    type: 'agent',
    move_type: 'static',
    properties,
    states: [],
    initialization: { population_count: 0 },
    locked: false,
  }
  const next = [...model.value.agents, blank]
  onModelEdited({ ...model.value, agents: next })
  agentSel.value = next.length - 1 // focus the newly added agent
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
      // validation_failed → surface field-level messages inline.
      saveErrors.value =
        err.fields && Object.keys(err.fields).length
          ? Object.entries(err.fields).map(([f, r]) => `${f}: ${r}`)
          : [err.message]
    } else {
      handleError(err, 'Save Model')
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
    handleSuccess('Model saved')
  } catch {
    // persist() already surfaced the failure (banner or global handler).
  }
}

// Auto-save on tab switch + leaving the editor (route/refresh). Silent success;
// never runs while a save is in flight or an external-update conflict is pending
// (auto-saving would discard the parked pendingExternal).
const { autoSave } = useEditorAutoSave({
  isDirty: () => store.dirty,
  isBlocked: () => saving.value || !!store.pendingExternal,
  save: persist,
})
</script>

<style scoped>
/* Tab header decorations: status dot (colour comes from the bound Tailwind class)
   + a count chip. Kept as plain CSS so it doesn't depend on Tailwind utilities
   surviving PrimeVue's tab markup. */
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
