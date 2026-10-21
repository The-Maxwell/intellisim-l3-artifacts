<template>
  <div class="h-full flex flex-col min-h-0">
    <!-- Loading -->
    <div v-if="loading" class="flex-1 grid place-items-center text-sm text-zinc-500">
      <div class="flex items-center gap-2">
        <i class="pi pi-spin pi-spinner" />
        Loading model snapshot…
      </div>
    </div>

    <!-- Error -->
    <div v-else-if="error" class="flex-1 grid place-items-center px-6">
      <div class="max-w-md w-full text-center">
        <Message severity="error" :closable="false" class="mb-3 text-left">{{ error }}</Message>
        <Button label="Retry" icon="pi pi-refresh" size="small" @click="emit('retry')" />
      </div>
    </div>

    <!-- Empty (defensive: sim with no snapshot) -->
    <div
      v-else-if="!snapshot"
      class="flex-1 grid place-items-center text-center text-sm text-zinc-400 px-6"
    >
      No model snapshot available for this simulation.
    </div>

    <!-- Loaded -->
    <template v-else>
      <!-- Read-only banner: this is the frozen model, not the (possibly changed) live model. -->
      <Message severity="info" :closable="false" class="mb-3 shrink-0">
        <span class="text-xs">
          Read-only snapshot of the source model
          <strong>«{{ m.name || 'Untitled model' }}»</strong>
          as it was frozen when this simulation was created. It is independent of later edits to the
          live model. Fields are disabled.
        </span>
      </Message>

      <!-- Forms region. Action buttons (add/remove/tools) are hidden here (scoped CSS
           below) so only DATA controls show; every detail form is inert (non-editable).
           Master lists stay selectable so all agents/rules/interactions/events are browsable. -->
      <div class="snapshot-forms flex-1 min-h-0">
        <Tabs v-model:value="innerTab" scrollable lazy class="h-full flex flex-col min-h-0">
          <TabList>
            <Tab value="general"><span class="tab-label">General</span></Tab>
            <Tab value="llm"><span class="tab-label">LLM</span></Tab>
            <Tab value="agents"><span class="tab-label">Agents<span class="tab-count">{{ m.agents.length }}</span></span></Tab>
            <Tab value="environment"><span class="tab-label">Environment</span></Tab>
            <Tab value="rules"><span class="tab-label">Rules<span class="tab-count">{{ m.rules.length }}</span></span></Tab>
            <Tab value="interactions"><span class="tab-label">Interactions<span class="tab-count">{{ m.interactions.length }}</span></span></Tab>
            <Tab value="events"><span class="tab-label">Events<span class="tab-count">{{ m.events.length }}</span></span></Tab>
            <Tab value="widgets"><span class="tab-label">Widgets<span class="tab-count">{{ (m.widgets ?? []).length }}</span></span></Tab>
          </TabList>

          <TabPanels class="flex-1 min-h-0 overflow-y-auto !px-4 !py-4">
            <TabPanel value="general">
              <div class="ro" :inert="true"><GeneralForm :modelValue="m" /></div>
            </TabPanel>

            <TabPanel value="llm">
              <div class="ro" :inert="true"><ModelLlmForm :modelValue="m" /></div>
            </TabPanel>

            <TabPanel value="agents">
              <MasterDetailList
                :items="m.agents"
                v-model:selectedIndex="agentSel"
                emptyText="This model has no agents."
              >
                <template #item="{ item, index }">
                  <span class="truncate flex-1">{{ item.name || item.uid || `Agent ${index + 1}` }}</span>
                  <span v-if="item.type === 'archetype'" class="text-[9px] uppercase tracking-wide text-zinc-400 flex-none">arch</span>
                </template>
                <template #detail="{ item }">
                  <div class="ro" :inert="true">
                    <AgentForm
                      :modelValue="item"
                      :collisionMode="m.environment?.collision_mode"
                      :isSpatial="m.is_spatial"
                    />
                  </div>
                </template>
              </MasterDetailList>
            </TabPanel>

            <TabPanel value="environment">
              <!-- NO modelId: keeps ImageToEnvironment (which mutates the LIVE model) unmounted. -->
              <div class="ro" :inert="true">
                <EnvironmentForm
                  :modelValue="environment"
                  :cellSize="m.spatial_cell_size"
                  :model="m"
                  :isSpatial="m.is_spatial"
                />
              </div>
            </TabPanel>

            <TabPanel value="rules">
              <MasterDetailList
                :items="m.rules"
                v-model:selectedIndex="ruleSel"
                emptyText="This model has no rules."
              >
                <template #item="{ item, index }">
                  <span class="truncate flex-1">{{ item.name || item.id || `Rule ${index + 1}` }}</span>
                  <i v-if="item.locked" class="pi pi-lock text-amber-500 text-[10px] flex-none" />
                  <span v-if="!item.enabled" class="text-[9px] uppercase tracking-wide text-zinc-400 flex-none">off</span>
                </template>
                <template #detail="{ item }">
                  <div class="ro" :inert="true"><RuleForm :modelValue="item" :model="m" /></div>
                </template>
              </MasterDetailList>
            </TabPanel>

            <TabPanel value="interactions">
              <MasterDetailList
                :items="m.interactions"
                v-model:selectedIndex="interactionSel"
                emptyText="This model has no interactions."
              >
                <template #item="{ item, index }">
                  <span class="truncate flex-1">{{ item.name || item.id || `Interaction ${index + 1}` }}</span>
                  <i v-if="item.locked" class="pi pi-lock text-amber-500 text-[10px] flex-none" />
                  <span v-if="!item.enabled" class="text-[9px] uppercase tracking-wide text-zinc-400 flex-none">off</span>
                </template>
                <template #detail="{ item }">
                  <div class="ro" :inert="true"><InteractionItemForm :modelValue="item" :model="m" /></div>
                </template>
              </MasterDetailList>
            </TabPanel>

            <TabPanel value="events">
              <MasterDetailList
                :items="m.events"
                v-model:selectedIndex="eventSel"
                emptyText="This model has no events."
              >
                <template #item="{ item, index }">
                  <span class="truncate flex-1">{{ item.name || item.uid || `Event ${index + 1}` }}</span>
                  <i v-if="item.locked" class="pi pi-lock text-amber-500 text-[10px] flex-none" />
                </template>
                <template #detail="{ item }">
                  <div class="ro" :inert="true"><EventItemForm :modelValue="item" :agents="m.agents" /></div>
                </template>
              </MasterDetailList>
            </TabPanel>

            <TabPanel value="widgets">
              <div class="ro" :inert="true">
                <WidgetsSection
                  :widgets="m.widgets ?? []"
                  :schema="widgetSchema"
                  :readonly="true"
                  :spatial="m.is_spatial"
                  :envWidth="m.environment?.width"
                  :envHeight="m.environment?.height"
                />
              </div>
            </TabPanel>
          </TabPanels>
        </Tabs>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
// Read-only viewer for a simulation's frozen ModelSnapshot. Reuses the model
// editor's form components to render the complete frozen model (general, LLM,
// agents, environment, rules, interactions, events, widgets) — the same model
// that the run engine actually used, independent of later edits to the live
// model. Everything is presented non-interactively:
//   • each DETAIL form is wrapped in `.ro` (`inert` + `pointer-events-none`), so
//     fields are fully disabled;
//   • master lists stay selectable so every agent/rule/interaction/event is
//     browsable;
//   • action buttons (add/remove/canvas tools) are hidden via scoped CSS so no
//     management chrome is shown.
// The panel takes the already-normalized ModelResponse (arrays coerced) — see
// SimulationEditorView's loadSnapshot.
import { computed, ref } from 'vue'
import Message from 'primevue/message'
import Button from 'primevue/button'
import Tabs from 'primevue/tabs'
import TabList from 'primevue/tablist'
import Tab from 'primevue/tab'
import TabPanels from 'primevue/tabpanels'
import TabPanel from 'primevue/tabpanel'
import MasterDetailList from '@/shared/components/MasterDetailList.vue'
import GeneralForm from '@/modules/model/components/GeneralForm.vue'
import ModelLlmForm from '@/modules/model/components/ModelLlmForm.vue'
import AgentForm from '@/modules/model/components/AgentForm.vue'
import EnvironmentForm from '@/modules/model/components/EnvironmentForm.vue'
import RuleForm from '@/modules/model/components/RuleForm.vue'
import InteractionItemForm from '@/modules/model/components/InteractionItemForm.vue'
import EventItemForm from '@/modules/model/components/EventItemForm.vue'
import WidgetsSection from '@/modules/simulation/components/WidgetsSection.vue'
import { schemaFromModel } from '@/shared/components/widgets/widgetSchema'
import type { EnvironmentDTO, ModelResponse } from '@/shared/types'

const props = defineProps<{
  snapshot: ModelResponse | null
  loading: boolean
  error: string | null
}>()

const emit = defineEmits<{ retry: [] }>()

// The frozen model body. Guarded behind the loaded-state v-else, so non-null here.
const m = computed(() => props.snapshot!.model)

// Non-optional environment for EnvironmentForm (mirrors ModelEditorView): every
// model gets one; fall back to a blank so the form never sees undefined.
const environment = computed<EnvironmentDTO>(
  () =>
    m.value.environment ?? {
      uid: '',
      width: 0,
      height: 0,
      torus: false,
      shapes: [],
      global_properties: [],
    }
)

// Widget schema derived from the model (agents + env global properties). The
// model has no instances → previews show structure/empty, which is correct here.
const widgetSchema = computed(() => schemaFromModel(m.value))

// Inner-tab + per-list selection state (local; the snapshot is immutable).
const innerTab = ref<string>('general')
const agentSel = ref(0)
const ruleSel = ref(0)
const interactionSel = ref(0)
const eventSel = ref(0)
</script>

<style scoped>
/* Every detail form is fully disabled + non-interactive. `inert` alone handles
   focus/interaction; pointer-events-none is a belt-and-suspenders for the PixiJS
   canvas overlay. */
.ro {
  pointer-events: none;
}

/* Hide action/management buttons within the forms (add / remove / canvas tools /
   input steppers). Data is shown via Select / SelectButton / inputs / toggles /
   canvas — none of which are `.p-button` — so those stay visible. Scoped to the
   forms region only, so the loading/error Retry button is unaffected. */
.snapshot-forms :deep(.p-button) {
  display: none !important;
}

/* Tab header count chip (mirrors ModelEditorView). */
.tab-label {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
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
