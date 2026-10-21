<template>
  <div class="h-full flex flex-col min-h-0">
    <!-- Header: title on the left; model filter + create on the right -->
    <div class="border-b border-zinc-200 bg-white px-6 h-14 shrink-0 flex items-center justify-between gap-4">
      <h1 class="text-lg font-semibold text-zinc-800">Simulations</h1>
      <div class="flex items-center gap-2">
        <MultiSelect
          v-model="filterModelIds"
          :options="filterModelOptions"
          optionLabel="label"
          optionValue="value"
          :loading="filterModelsLoading"
          placeholder="All models"
          filter
          showClear
          :showToggleAll="false"
          :maxSelectedLabels="2"
          selectedItemsLabel="{0} models"
          size="small"
          class="w-64"
          @change="onFilterChange"
        />
        <Button label="Create simulation" icon="pi pi-plus" size="small" @click="createDialog = true" />
      </div>
    </div>

    <div class="flex-1 min-h-0 flex flex-col p-6">
      <Message v-if="store.error" severity="error" :closable="false" class="mb-4 shrink-0">
        {{ store.error }}
      </Message>

      <DataTable
        :value="store.list"
        :loading="store.loading"
        dataKey="id"
        size="small"
        lazy
        paginator
        :rows="store.pageSize"
        :first="first"
        :totalRecords="store.total"
        :rowsPerPageOptions="[10, 20, 50, 100]"
        paginatorTemplate="FirstPageLink PrevPageLink PageLinks NextPageLink LastPageLink RowsPerPageDropdown CurrentPageReport"
        currentPageReportTemplate="Showing {first}–{last} of {totalRecords}"
        :sortField="store.sortField"
        :sortOrder="sortOrderNum"
        selectionMode="single"
        scrollable
        scrollHeight="flex"
        class="text-sm flex-1 min-h-0"
        @rowClick="onRowClick"
        @page="onPage"
        @sort="onSort"
      >
        <template #empty>
          <div class="text-center text-zinc-400 py-8 text-sm">
            {{ filterModelIds.length ? 'No simulations for the selected model(s).' : 'No simulations yet.' }}
          </div>
        </template>

        <Column field="name" header="Name" sortable>
          <template #body="{ data }">
            <span class="font-medium text-zinc-800">{{ data.name || 'Untitled simulation' }}</span>
          </template>
        </Column>

        <Column field="model_name" header="Model" sortable :style="{ width: '180px' }">
          <template #body="{ data }">
            <span class="text-zinc-600 truncate">{{ data.model_name || '—' }}</span>
          </template>
        </Column>

        <Column field="status" header="Status" sortable :style="{ width: '140px' }">
          <template #body="{ data }">
            <Tag :value="data.status" :severity="statusSeverity(data.status)" />
          </template>
        </Column>

        <Column field="steps" header="Steps" sortable :style="{ width: '90px' }">
          <template #body="{ data }">
            <span class="text-zinc-600">{{ data.steps }}</span>
          </template>
        </Column>

        <Column field="agent_count" header="Agents" sortable :style="{ width: '90px' }">
          <template #body="{ data }">
            <span class="text-zinc-600">{{ data.agent_count }}</span>
          </template>
        </Column>

        <Column field="total_tokens" header="Tokens" sortable :style="{ width: '110px' }">
          <template #body="{ data }">
            <span class="text-zinc-600">{{ formatNumber(data.total_tokens) }}</span>
          </template>
        </Column>

        <Column field="updated_at" header="Updated" sortable :style="{ width: '170px' }">
          <template #body="{ data }">
            <span class="text-zinc-500 text-xs">{{ formatDate(data.updated_at) }}</span>
          </template>
        </Column>

        <Column
          v-if="authStore.isAdmin"
          field="created_by"
          header="Owner"
          sortable
          :style="{ width: '180px' }"
        >
          <template #body="{ data }">
            <span class="text-zinc-500 text-xs truncate">{{ data.created_by }}</span>
          </template>
        </Column>

        <Column :style="{ width: '170px' }">
          <template #body="{ data }">
            <div class="flex items-center justify-end gap-1">
              <Button
                icon="pi pi-play"
                severity="success"
                text
                rounded
                size="small"
                aria-label="Run"
                title="Run simulation"
                :disabled="!canRun(data)"
                @click.stop="goToRun(data)"
              />
              <Button
                v-if="isTerminal(data)"
                icon="pi pi-eye"
                severity="secondary"
                text
                rounded
                size="small"
                aria-label="View results"
                title="View results"
                @click.stop="goToResults(data)"
              />
              <Button
                icon="pi pi-copy"
                severity="secondary"
                text
                rounded
                size="small"
                aria-label="Duplicate"
                @click.stop="openDuplicateDialog(data)"
              />
              <Button
                icon="pi pi-trash"
                severity="danger"
                text
                rounded
                size="small"
                aria-label="Delete"
                @click.stop="confirmDelete($event, data)"
              />
            </div>
          </template>
        </Column>
      </DataTable>
    </div>

    <!-- Create dialog (shared component; no preset → user picks the model) -->
    <CreateSimulationDialog v-model:visible="createDialog" />

    <!-- Duplicate dialog -->
    <Dialog v-model:visible="duplicateDialog" modal header="Duplicate simulation" :style="{ width: '420px' }">
      <div>
        <label class="text-xs font-medium text-zinc-600 mb-1 block">Name</label>
        <InputText
          v-model="duplicateName"
          class="w-full"
          size="small"
          placeholder="Copy name"
          @keydown.enter="duplicateSimulation"
        />
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <Button label="Cancel" severity="secondary" size="small" @click="duplicateDialog = false" />
          <Button label="Duplicate" size="small" :loading="duplicating" @click="duplicateSimulation" />
        </div>
      </template>
    </Dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import DataTable, {
  type DataTableRowClickEvent,
  type DataTablePageEvent,
  type DataTableSortEvent,
} from 'primevue/datatable'
import Column from 'primevue/column'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import MultiSelect from 'primevue/multiselect'
import InputText from 'primevue/inputtext'
import Tag from 'primevue/tag'
import Message from 'primevue/message'
import { useConfirm } from 'primevue/useconfirm'
import CreateSimulationDialog from '@/modules/simulation/components/CreateSimulationDialog.vue'
import { useSimulationStore } from '@/modules/simulation/stores/simulationStore'
import { useModelApi } from '@/modules/model/composables/useModelApi'
import { useAuthStore } from '@/modules/auth/stores/authStore'
import { useErrorHandler } from '@/shared/composables/useErrorHandler'
import { statusSeverity } from '@/modules/simulation/views/simulationStatus'
import type { ModelOption, SimulationSummary } from '@/shared/types'

const router = useRouter()
const route = useRoute()
const store = useSimulationStore()
const modelApi = useModelApi()
const authStore = useAuthStore()
const confirm = useConfirm()
const { handleError, handleSuccess } = useErrorHandler()

// --- Server-side lazy pagination + sort (PrimeVue DataTable) ---
// `first` (the paginator's active-page offset) is derived from the store page so
// it can never desync — including after the store clamps a paged-past-the-end
// load (e.g. the model filter shrank the result set) back to the last page.
const first = computed(() => (store.page - 1) * store.pageSize)
const sortOrderNum = computed(() => (store.sortOrder === 'asc' ? 1 : -1))

function onPage(e: DataTablePageEvent) {
  void store.loadList({ page: e.page + 1, pageSize: e.rows })
}

function onSort(e: DataTableSortEvent) {
  const field = typeof e.sortField === 'string' ? e.sortField : 'updated_at'
  void store.loadList({ page: 1, sortField: field, sortOrder: e.sortOrder === 1 ? 'asc' : 'desc' })
}

// --- Model filter ---
// Options are loaded once on mount (capped at 100 models — enough for this
// research tool's scale). The selection is mirrored to the route query
// (?model_id=…) so a filtered view is shareable / deep-linkable.
const filterModelIds = ref<string[]>([])
const filterModels = ref<ModelOption[]>([])
const filterModelsLoading = ref(false)
const filterModelOptions = computed(() =>
  filterModels.value.map((m) => ({ value: m.id, label: m.name || 'Untitled model' }))
)

function parseModelIds(q: unknown): string[] {
  if (Array.isArray(q)) return q.filter((x): x is string => typeof x === 'string' && x !== '')
  if (typeof q === 'string' && q) return [q]
  return []
}

function syncQuery() {
  const query = { ...route.query }
  if (filterModelIds.value.length) query.model_id = [...filterModelIds.value]
  else delete query.model_id
  void router.replace({ query })
}

function applyFilter() {
  // loadList({ page: 1 }) resets the store page, and `first` derives from it.
  void store.loadList({ page: 1, modelIds: [...filterModelIds.value] })
  syncQuery()
}

// Fires on option toggle and on the MultiSelect's built-in clear (showClear).
function onFilterChange() {
  applyFilter()
}

async function loadFilterModels() {
  filterModelsLoading.value = true
  try {
    filterModels.value = await modelApi.listModelOptions()
  } catch (err) {
    handleError(err, 'Load Models')
  } finally {
    filterModelsLoading.value = false
  }
}

// --- Create dialog (the form lives in CreateSimulationDialog) ---
const createDialog = ref(false)

// --- Duplicate dialog state ---
const duplicateDialog = ref(false)
const duplicating = ref(false)
const duplicateName = ref('')
const duplicateTarget = ref<SimulationSummary | null>(null)

onMounted(() => {
  if ('model_id' in route.query) {
    // Deep-link / shared URL drives the filter; reset to page 1.
    filterModelIds.value = parseModelIds(route.query.model_id)
    void store.loadList({ page: 1, modelIds: [...filterModelIds.value] })
  } else {
    // No URL filter: preserve whatever the store holds (page/sort/filter from a
    // previous visit), mirror it into the control, and reflect it in the URL so
    // the address bar never lies about an active filter.
    filterModelIds.value = [...store.modelIds]
    void store.loadList()
    if (filterModelIds.value.length) syncQuery()
  }
  void loadFilterModels()
})

function formatDate(iso: string): string {
  if (!iso) return '—'
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? '—' : d.toLocaleString()
}

function formatNumber(n: number): string {
  return (n ?? 0).toLocaleString()
}

function onRowClick(event: DataTableRowClickEvent) {
  const row = event.data as SimulationSummary
  router.push(`/simulations/${row.id}`)
}

// The Run action opens the run view. It's enabled for a ready simulation (start
// a fresh run) and for an active run status (view live); the summary carries no
// start_ready flag, so generating/generation_failed are excluded, as are
// terminal statuses (finished/stopped/failed) — those use the "View results"
// action instead. The run view + backend re-validate before an actual start.
function canRun(row: SimulationSummary): boolean {
  return row.status !== 'generating' && row.status !== 'generation_failed' && !isTerminal(row)
}

function isTerminal(row: SimulationSummary): boolean {
  return row.status === 'finished' || row.status === 'stopped' || row.status === 'failed'
}

function goToRun(row: SimulationSummary) {
  if (!canRun(row)) return
  router.push(`/simulations/${row.id}/run`)
}

// View results: opens the run screen in replay mode for a terminal simulation
// (Run is disabled for these — see canRun).
function goToResults(row: SimulationSummary) {
  router.push(`/simulations/${row.id}/run`)
}

// --- Duplicate ---
function openDuplicateDialog(row: SimulationSummary) {
  duplicateTarget.value = row
  duplicateName.value = row.name ? `${row.name} (copy)` : ''
  duplicateDialog.value = true
}

async function duplicateSimulation() {
  if (duplicating.value || !duplicateTarget.value) return
  duplicating.value = true
  try {
    const sim = await store.duplicate(duplicateTarget.value.id, duplicateName.value.trim() || undefined)
    duplicateDialog.value = false
    handleSuccess('Simulation duplicated')
    // Open the copy's editor (matches the simulation editor's own Duplicate action).
    router.push(`/simulations/${sim.id}`)
  } catch (err) {
    handleError(err, 'Duplicate Simulation')
  } finally {
    duplicating.value = false
  }
}

// --- Delete ---
function confirmDelete(event: MouseEvent, row: SimulationSummary) {
  confirm.require({
    target: event.currentTarget as HTMLElement,
    message: `Delete "${row.name || 'Untitled simulation'}"? This cannot be undone.`,
    header: 'Delete simulation',
    icon: 'pi pi-exclamation-triangle',
    rejectProps: { label: 'Cancel', severity: 'secondary', size: 'small' },
    acceptProps: { label: 'Delete', severity: 'danger', size: 'small' },
    accept: async () => {
      try {
        await store.remove(row.id)
        handleSuccess('Simulation deleted')
        await store.loadList()
      } catch (err) {
        handleError(err, 'Delete Simulation')
      }
    },
  })
}
</script>
