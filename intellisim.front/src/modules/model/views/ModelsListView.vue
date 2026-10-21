<template>
  <div class="h-full flex flex-col min-h-0">
    <!-- Header -->
    <div class="border-b border-zinc-200 bg-white px-6 h-14 shrink-0 flex items-center justify-between gap-4">
      <h1 class="text-lg font-semibold text-zinc-800">Models</h1>
      <SplitButton
        label="New model"
        icon="pi pi-plus"
        size="small"
        :model="newMenu"
        @click="openBlankDialog"
      />
    </div>

    <div class="flex-1 min-h-0 flex flex-col p-6">
      <Message v-if="store.error" severity="error" :closable="false" class="mb-4 shrink-0">
        {{ store.error }}
      </Message>

      <DataTable
        :value="store.models"
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
          <div class="text-center text-zinc-400 py-8 text-sm">No models yet.</div>
        </template>

        <Column field="name" header="Name" sortable>
          <template #body="{ data }">
            <span class="font-medium text-zinc-800">{{ data.name || 'Untitled model' }}</span>
          </template>
        </Column>

        <!-- agent_count is computed at mapping time (not a DB-sortable field), so
             it stays non-sortable under server-side paging. -->
        <Column field="agent_count" header="Agents" :style="{ width: '90px' }">
          <template #body="{ data }">
            <span class="text-zinc-600">{{ data.agent_count }}</span>
          </template>
        </Column>

        <Column header="Spatial" :style="{ width: '100px' }">
          <template #body="{ data }">
            <Tag v-if="data.is_spatial" value="spatial" severity="info" />
            <span v-else class="text-zinc-300 text-xs">—</span>
          </template>
        </Column>

        <Column header="Status" :style="{ width: '140px' }">
          <template #body="{ data }">
            <Tag
              :value="data.simulation_ready ? 'ready' : 'draft'"
              :severity="data.simulation_ready ? 'success' : 'secondary'"
            />
          </template>
        </Column>

        <Column field="updated_at" header="Updated" sortable :style="{ width: '170px' }">
          <template #body="{ data }">
            <span class="text-zinc-500 text-xs">{{ formatDate(data.updated_at) }}</span>
          </template>
        </Column>

        <Column v-if="authStore.isAdmin" field="created_by" header="Owner" sortable :style="{ width: '180px' }">
          <template #body="{ data }">
            <span class="text-zinc-500 text-xs truncate">{{ data.created_by }}</span>
          </template>
        </Column>

        <Column :style="{ width: '170px' }">
          <template #body="{ data }">
            <div class="flex items-center justify-end gap-1">
              <Button
                icon="pi pi-list"
                severity="secondary"
                text
                rounded
                size="small"
                aria-label="View simulations"
                title="View simulations of this model"
                @click.stop="viewSimulations(data)"
              />
              <Button
                icon="pi pi-plus"
                severity="success"
                text
                rounded
                size="small"
                aria-label="Create simulation"
                :title="data.simulation_ready ? 'Create simulation from this model' : 'Model must be ready to simulate'"
                :disabled="!data.simulation_ready"
                @click.stop="openCreateSim(data)"
              />
              <Button
                icon="pi pi-copy"
                severity="secondary"
                text
                rounded
                size="small"
                aria-label="Duplicate"
                title="Duplicate model (and its chat)"
                @click.stop="openDuplicate(data)"
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

    <!-- Blank-model dialog: just a name -->
    <Dialog v-model:visible="blankDialog" modal header="New blank model" :style="{ width: '420px' }">
      <div class="space-y-3">
        <div>
          <label class="text-xs font-medium text-zinc-600 mb-1 block">Name</label>
          <InputText
            v-model="blankName"
            class="w-full"
            size="small"
            :maxlength="MODEL_NAME_MAX"
            placeholder="Untitled model"
            @keydown.enter="createBlank"
          />
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <Button label="Cancel" severity="secondary" size="small" @click="blankDialog = false" />
          <Button label="Create" size="small" :loading="creating" @click="createBlank" />
        </div>
      </template>
    </Dialog>

    <!-- With-LLM dialog: name + spatial + execution target (model_generation) + first message -->
    <Dialog v-model:visible="llmDialog" modal header="New model with LLM" :style="{ width: '560px' }">
      <div class="space-y-4">
        <div>
          <label class="text-xs font-medium text-zinc-600 mb-1 block">Name</label>
          <InputText
            v-model="llmName"
            class="w-full"
            size="small"
            :maxlength="MODEL_NAME_MAX"
            placeholder="Untitled model"
          />
        </div>

        <label class="flex items-center gap-2 text-xs text-zinc-600 cursor-pointer">
          <Checkbox v-model="llmSpatial" :binary="true" />
          <span>Spatial model (2D environment)</span>
        </label>

        <div>
          <label class="text-xs font-medium text-zinc-600 mb-1 block">Generation model</label>
          <LlmExecutionConfig v-model="llmTarget" capability="model_generation" />
        </div>

        <div>
          <label class="text-xs font-medium text-zinc-600 mb-1 block">First message</label>
          <Textarea
            v-model="llmMessage"
            class="w-full"
            rows="4"
            autoResize
            :maxlength="MESSAGE_CONTENT_MAX"
            placeholder="Describe the model you want to generate…"
          />
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <Button label="Cancel" severity="secondary" size="small" @click="llmDialog = false" />
          <Button
            label="Create & generate"
            size="small"
            :loading="creating"
            :disabled="!llmTarget || !llmMessage.trim()"
            @click="createWithLlm"
          />
        </div>
      </template>
    </Dialog>

    <!-- Create a simulation from a specific model (locked to that model) -->
    <CreateSimulationDialog
      v-model:visible="createSimDialog"
      :presetModelId="presetModel?.id"
      :presetModelName="presetModel?.name"
    />

    <!-- Duplicate a model (+ its companion chat) -->
    <DuplicateModelDialog
      v-model:visible="dupDialog"
      :modelId="dupTarget?.id ?? null"
      :modelName="dupTarget?.name ?? ''"
      @duplicated="onModelDuplicated"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import DataTable, {
  type DataTableRowClickEvent,
  type DataTablePageEvent,
  type DataTableSortEvent,
} from 'primevue/datatable'
import Column from 'primevue/column'
import Button from 'primevue/button'
import SplitButton from 'primevue/splitbutton'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Textarea from 'primevue/textarea'
import Checkbox from 'primevue/checkbox'
import Tag from 'primevue/tag'
import Message from 'primevue/message'
import { useConfirm } from 'primevue/useconfirm'
import type { MenuItem } from 'primevue/menuitem'
import LlmExecutionConfig from '@/shared/components/LlmExecutionConfig.vue'
import CreateSimulationDialog from '@/modules/simulation/components/CreateSimulationDialog.vue'
import DuplicateModelDialog from '@/modules/model/components/DuplicateModelDialog.vue'
import { useModelStore } from '@/modules/model/stores/modelStore'
import { useAuthStore } from '@/modules/auth/stores/authStore'
import { useErrorHandler } from '@/shared/composables/useErrorHandler'
import { MODEL_NAME_MAX, MESSAGE_CONTENT_MAX } from '@/shared/validation/modelRules'
import type { ExecutionLlmTarget, ModelResponse, ModelSummary } from '@/shared/types'

const router = useRouter()
const store = useModelStore()
const authStore = useAuthStore()
const confirm = useConfirm()
const { handleError, handleSuccess } = useErrorHandler()

const creating = ref(false)

// --- Server-side lazy pagination + sort (PrimeVue DataTable) ---
// `first` (the paginator's active-page offset) is derived from the store page so
// it can never desync — including after the store clamps a paged-past-the-end
// load back to the last valid page.
const first = computed(() => (store.page - 1) * store.pageSize)
const sortOrderNum = computed(() => (store.sortOrder === 'asc' ? 1 : -1))

function onPage(e: DataTablePageEvent) {
  void store.loadList({ page: e.page + 1, pageSize: e.rows })
}

function onSort(e: DataTableSortEvent) {
  const field = typeof e.sortField === 'string' ? e.sortField : 'updated_at'
  void store.loadList({ page: 1, sortField: field, sortOrder: e.sortOrder === 1 ? 'asc' : 'desc' })
}

// --- Blank dialog state ---
const blankDialog = ref(false)
const blankName = ref('')

// --- With-LLM dialog state ---
const llmDialog = ref(false)
const llmName = ref('')
const llmSpatial = ref(false)
const llmTarget = ref<ExecutionLlmTarget | null>(null)
const llmMessage = ref('')

// --- Create-simulation-from-model state ---
const createSimDialog = ref(false)
const presetModel = ref<{ id: string; name: string } | null>(null)

// --- Duplicate-model state ---
const dupDialog = ref(false)
const dupTarget = ref<{ id: string; name: string } | null>(null)

const newMenu: MenuItem[] = [
  { label: 'Blank', icon: 'pi pi-file', command: openBlankDialog },
  { label: 'With LLM', icon: 'pi pi-sparkles', command: openLlmDialog },
]

onMounted(() => store.loadList())

function formatDate(iso: string): string {
  if (!iso) return '—'
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? '—' : d.toLocaleString()
}

function onRowClick(event: DataTableRowClickEvent) {
  const row = event.data as ModelSummary
  router.push(`/models/${row.id}`)
}

// Jump to the simulations list filtered to this model (deep-link ?model_id=).
function viewSimulations(row: ModelSummary) {
  router.push({ path: '/simulations', query: { model_id: row.id } })
}

// Open the shared create-simulation dialog locked to this model.
function openCreateSim(row: ModelSummary) {
  presetModel.value = { id: row.id, name: row.name || 'Untitled model' }
  createSimDialog.value = true
}

// Open the duplicate-model dialog for this row.
function openDuplicate(row: ModelSummary) {
  dupTarget.value = { id: row.id, name: row.name || 'Untitled model' }
  dupDialog.value = true
}

// A duplicate landed → open the copy's editor (matches the model editor's own
// Duplicate action, so both entry points behave the same).
function onModelDuplicated(model: ModelResponse) {
  handleSuccess('Model duplicated')
  router.push(`/models/${model.id}`)
}

// --- Blank ---
function openBlankDialog() {
  blankName.value = ''
  blankDialog.value = true
}

async function createBlank() {
  if (creating.value) return
  creating.value = true
  try {
    const resp = await store.create({ name: blankName.value.trim() || 'Untitled model', is_spatial: false })
    blankDialog.value = false
    router.push(`/models/${resp.id}`)
  } catch (err) {
    handleError(err, 'Create Model')
  } finally {
    creating.value = false
  }
}

// --- With LLM (async generation; editor receives model.updated via SSE) ---
function openLlmDialog() {
  llmName.value = ''
  llmSpatial.value = false
  llmTarget.value = null
  llmMessage.value = ''
  llmDialog.value = true
}

async function createWithLlm() {
  if (creating.value || !llmTarget.value || !llmMessage.value.trim()) return
  creating.value = true
  try {
    const resp = await store.create({
      name: llmName.value.trim() || 'Untitled model',
      is_spatial: llmSpatial.value,
      llm_target: llmTarget.value,
      initial_message: llmMessage.value.trim(),
    })
    llmDialog.value = false
    router.push(`/models/${resp.id}`)
  } catch (err) {
    handleError(err, 'Create Model')
  } finally {
    creating.value = false
  }
}

// --- Delete ---
function confirmDelete(event: MouseEvent, row: ModelSummary) {
  confirm.require({
    target: event.currentTarget as HTMLElement,
    message: `Delete "${row.name || 'Untitled model'}"? This cannot be undone.`,
    header: 'Delete model',
    icon: 'pi pi-exclamation-triangle',
    rejectProps: { label: 'Cancel', severity: 'secondary', size: 'small' },
    acceptProps: { label: 'Delete', severity: 'danger', size: 'small' },
    accept: async () => {
      try {
        await store.remove(row.id)
        handleSuccess('Model deleted')
      } catch (err) {
        handleError(err, 'Delete Model')
      }
    },
  })
}
</script>
