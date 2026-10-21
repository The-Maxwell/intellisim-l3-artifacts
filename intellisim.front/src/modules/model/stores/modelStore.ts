import { defineStore } from 'pinia'
import { ref } from 'vue'
import type {
  CreateModelRequest,
  Model,
  ModelImageDTO,
  ModelResponse,
  ModelSummary,
  ValidationResult,
} from '@/shared/types'
import { normalizeModelResponse } from '@/shared/types'
import { useModelApi } from '@/modules/model/composables/useModelApi'

// Owns the list of model summaries (sidebar/list view) and the single currently
// open model (editor view) with its validation + dirty tracking. Mirrors the
// existing setup-style Pinia store convention (sidebarStore/llmProviderStore).
export const useModelStore = defineStore('model', () => {
  const api = useModelApi()

  const models = ref<ModelSummary[]>([])
  // Server-side pagination + sort state for the list view (PrimeVue lazy DataTable).
  const total = ref(0)
  const page = ref(1)
  const pageSize = ref(20)
  const sortField = ref<string>('updated_at')
  const sortOrder = ref<'asc' | 'desc'>('desc')
  const current = ref<ModelResponse | null>(null)
  const validation = ref<ValidationResult | null>(null)
  const dirty = ref(false)
  const loading = ref(false)
  const error = ref<string | null>(null)
  // A model.updated arriving while the user has unsaved edits is parked here
  // (latest wins) instead of clobbering `current` — the editor surfaces an
  // accept/dismiss banner. Null when there is nothing pending.
  const pendingExternal = ref<ModelResponse | null>(null)

  function asMessage(e: unknown, fallback: string): string {
    return e instanceof Error ? e.message : fallback
  }

  // Server-side paged/sorted load. Any field left undefined keeps its current
  // value, so callers can update just page, just sort, etc. A null sortField
  // resets to the canonical default (updated_at desc). No-arg reload preserves
  // the current page/sort (used after create/duplicate/delete).
  async function loadList(opts: {
    page?: number
    pageSize?: number
    sortField?: string | null
    sortOrder?: 'asc' | 'desc'
  } = {}): Promise<void> {
    if (opts.page !== undefined) page.value = opts.page
    if (opts.pageSize !== undefined) pageSize.value = opts.pageSize
    if (opts.sortField !== undefined) sortField.value = opts.sortField ?? 'updated_at'
    if (opts.sortOrder !== undefined) sortOrder.value = opts.sortOrder
    loading.value = true
    error.value = null
    try {
      const resp = await api.listModels(page.value, pageSize.value, sortField.value, sortOrder.value)
      models.value = resp.models ?? []
      total.value = resp.total ?? 0
      // Clamp: if we paged past the end (e.g. rows were deleted, or a stale
      // deep-link points beyond the last page) snap to the last valid page and
      // refetch once. Bounded — the clamped page always returns rows.
      const lastPage = Math.max(1, Math.ceil(total.value / pageSize.value))
      if (page.value > lastPage) {
        page.value = lastPage
        const clamped = await api.listModels(page.value, pageSize.value, sortField.value, sortOrder.value)
        models.value = clamped.models ?? []
        total.value = clamped.total ?? 0
      }
    } catch (e) {
      error.value = asMessage(e, 'Failed to load models')
    } finally {
      loading.value = false
    }
  }

  async function create(req: CreateModelRequest): Promise<ModelResponse> {
    const resp = await api.createModel(req)
    applyModelUpdate(resp)
    return resp
  }

  // Duplicate a model (backend also clones its companion chat). Does not touch
  // the open model or the list — callers refresh/navigate as they see fit.
  async function duplicate(id: string, name?: string): Promise<ModelResponse> {
    return api.duplicateModel(id, name)
  }

  async function open(id: string): Promise<void> {
    loading.value = true
    error.value = null
    pendingExternal.value = null
    try {
      const resp = await api.getModel(id)
      applyModelUpdate(resp)
    } catch (e) {
      error.value = asMessage(e, 'Failed to open model')
      current.value = null
      validation.value = null
      // Re-throw so callers can inspect the ApiError (e.g. 403/404 panels);
      // store.error is still set for store-only consumers.
      throw e
    } finally {
      loading.value = false
    }
  }

  async function saveCurrent(): Promise<ModelResponse | null> {
    if (!current.value) return null
    const resp = await api.updateModel(current.value.id, current.value.model)
    applyModelUpdate(resp)
    // A save makes the just-persisted local state authoritative; drop any parked
    // external update. Saving over a dismissed LLM update is the user's explicit
    // choice, and the PUT response above carries the freshest updated_at.
    pendingExternal.value = null
    return resp
  }

  // Adopt a fresh ModelResponse (from create/open/save or the model.updated SSE
  // event) as the current model and refresh its validation + dirty state.
  // Normalize the model body first: the backend serializes empty Go slices as
  // JSON null, and the editor/form components iterate these arrays — so guarantee
  // every array field is a real array before storing (avoids null-array crashes).
  // This is the single adoption sink for create/open/save/applyModelUpdate.
  function applyModelUpdate(resp: ModelResponse): void {
    const normalized = normalizeModelResponse(resp)
    current.value = normalized
    validation.value = normalized.validation
    dirty.value = false
  }

  // Route an UNTRUSTED model.updated SSE event (which fires both for LLM edits
  // AND as an echo of the user's own PUT) through here rather than directly to
  // applyModelUpdate. Two guards protect in-progress local edits:
  //   1. Stale-echo guard: ignore any response not strictly newer than the one
  //      we already hold. The own-save echo carries the SAME updated_at as the
  //      PUT response saveCurrent already adopted, so it becomes a no-op.
  //   2. Dirty guard: with unsaved edits, park the (normalized) response in
  //      pendingExternal (latest wins) and leave current/dirty untouched so the
  //      editor can offer accept/dismiss; otherwise adopt it like applyModelUpdate.
  function applyExternalModelUpdate(resp: ModelResponse): void {
    if (current.value && Date.parse(resp.updated_at) <= Date.parse(current.value.updated_at)) {
      return
    }
    if (!dirty.value) {
      applyModelUpdate(resp)
      return
    }
    pendingExternal.value = normalizeModelResponse(resp)
  }

  // Graft a server-confirmed image library (from the add/delete image endpoints,
  // which return the full model) onto the currently open model WITHOUT touching
  // dirty/pendingExternal. The image library is managed independently of the
  // editable body, so adding/deleting an image must neither reset unsaved edits
  // nor raise the external-update banner. Advancing updated_at to the server value
  // makes the subsequent model.updated SSE echo a stale no-op.
  function applyImageLibrary(images: ModelImageDTO[] | null, updatedAt: string): void {
    if (!current.value) return
    current.value = {
      ...current.value,
      updated_at: updatedAt,
      // Coerce null (empty Go slice → JSON null) to [] so iterators stay safe.
      model: { ...current.value.model, images: images ?? [] },
    }
    // Order-independent neutralisation of this op's own SSE echo: if the
    // model.updated echo for this add/delete raced ahead of the HTTP response and
    // parked itself (only possible with unsaved edits → dirty), drop it. This op
    // already grafted the new library, and the parked echo is the same-or-older
    // server state, so keeping it would raise a spurious accept/dismiss banner. A
    // genuinely newer external update (later updated_at) is preserved.
    if (
      pendingExternal.value &&
      Date.parse(pendingExternal.value.updated_at) <= Date.parse(updatedAt)
    ) {
      pendingExternal.value = null
    }
  }

  // Discard local edits and adopt the parked external update via the normal sink.
  function acceptExternalUpdate(): void {
    if (!pendingExternal.value) return
    applyModelUpdate(pendingExternal.value)
    pendingExternal.value = null
  }

  // Keep local edits/dirty; drop the parked external update.
  function dismissExternalUpdate(): void {
    pendingExternal.value = null
  }

  function setDirty(v: boolean): void {
    dirty.value = v
  }

  // Replace the editable body of the currently open model (e.g. from the JSON
  // panel or a form section) and mark it dirty. Validation is refreshed on save.
  function setCurrentModel(model: Model): void {
    if (!current.value) return
    current.value = { ...current.value, model }
    dirty.value = true
  }

  async function remove(id: string): Promise<void> {
    await api.deleteModel(id)
    models.value = models.value.filter((m) => m.id !== id)
    if (current.value?.id === id) {
      current.value = null
      validation.value = null
      dirty.value = false
      pendingExternal.value = null
    }
  }

  return {
    models,
    total,
    page,
    pageSize,
    sortField,
    sortOrder,
    current,
    validation,
    dirty,
    loading,
    error,
    pendingExternal,
    loadList,
    create,
    duplicate,
    open,
    saveCurrent,
    applyModelUpdate,
    applyExternalModelUpdate,
    applyImageLibrary,
    acceptExternalUpdate,
    dismissExternalUpdate,
    setDirty,
    setCurrentModel,
    remove,
  }
})
