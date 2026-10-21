<template>
  <div v-if="isSpatial" class="mt-3 pt-3 border-t border-zinc-200 space-y-3">
    <p class="text-xs font-medium text-zinc-500">Environment from images</p>

    <!-- Managed image library: thumbnails + per-image labels + delete, plus an add tile. -->
    <div class="flex flex-wrap gap-3">
      <div v-for="img in images" :key="img.uid" class="w-20">
        <div class="relative">
          <img
            :src="`data:${img.mime_type};base64,${img.data}`"
            :alt="img.name || 'image'"
            class="w-20 h-20 object-cover rounded-md border border-zinc-200"
          />
          <button
            type="button"
            class="absolute -top-1.5 -right-1.5 w-5 h-5 rounded-full bg-zinc-700 text-white text-xs flex items-center justify-center hover:bg-zinc-900 disabled:opacity-50"
            :disabled="busy"
            title="Remove image"
            data-testid="remove-image"
            @click="removeImage(img.uid)"
          >
            <i class="pi pi-times text-[10px]" />
          </button>
        </div>
        <p class="mt-0.5 text-[10px] text-zinc-600 truncate" :title="img.name">{{ img.name || '—' }}</p>
        <p
          v-if="img.description"
          class="text-[10px] text-zinc-400 leading-tight line-clamp-2"
          :title="img.description"
        >
          {{ img.description }}
        </p>
      </div>

      <!-- Add tile -->
      <button
        v-if="images.length < MAX_MODEL_IMAGES"
        type="button"
        class="w-20 h-20 rounded-md border border-dashed border-zinc-300 text-zinc-400 hover:border-zinc-400 hover:text-zinc-600 flex flex-col items-center justify-center gap-1 disabled:opacity-50"
        :disabled="busy"
        data-testid="add-image-tile"
        @click="openAddDialog"
      >
        <i class="pi pi-plus text-sm" />
        <span class="text-[10px]">Add</span>
      </button>
    </div>

    <p class="text-[11px] text-zinc-400" data-testid="image-library-count">
      {{ images.length }}/{{ MAX_MODEL_IMAGES }} images · png/jpeg/webp/gif · ≤ {{ maxMiB }} MiB each
    </p>

    <!-- Vision-capable model picker. -->
    <div>
      <label class="text-xs text-zinc-500 mb-1 block">Vision model</label>
      <LlmExecutionConfig v-model="visionTarget" capability="vision_analysis" />
    </div>

    <!-- Optional prompt. -->
    <div>
      <label class="text-xs text-zinc-500 mb-1 block">Prompt (optional)</label>
      <Textarea
        v-model="prompt"
        class="w-full text-xs"
        rows="2"
        autoResize
        :maxlength="IMAGE_PROMPT_MAX"
        placeholder="e.g. Treat dark regions as obstacles and the green area as a food source."
      />
    </div>

    <!-- Over-limit mirror of the backend rule: the selected model can't take this many images. -->
    <Message
      v-if="overLimit"
      severity="warn"
      :closable="false"
      class="!mt-2"
      data-testid="image-over-limit"
    >
      The selected model accepts at most {{ effectiveMax }} image(s). Remove {{ excess }} image(s) to generate.
    </Message>
    <Message v-if="error" severity="error" :closable="true" class="!mt-2" @close="error = null">
      {{ error }}
    </Message>

    <div class="flex items-center gap-2">
      <Button
        label="Generate environment"
        icon="pi pi-sparkles"
        size="small"
        :loading="generating"
        :disabled="!canGenerate"
        data-testid="generate-environment"
        @click="generate"
      />
      <span v-if="generating" class="text-xs text-zinc-500">
        Analyzing images — the environment will update live…
      </span>
    </div>

    <!-- Add-image dialog: file + optional name + optional description in one POST. -->
    <Dialog v-model:visible="addOpen" modal header="Add image" :style="{ width: '24rem' }">
      <div class="space-y-3">
        <input ref="fileInput" type="file" :accept="ACCEPT" class="hidden" @change="onFilePicked" />
        <div class="flex items-center gap-2">
          <Button
            label="Choose file"
            icon="pi pi-image"
            size="small"
            severity="secondary"
            outlined
            @click="fileInput?.click()"
          />
          <span class="text-xs text-zinc-500 truncate">{{ pending?.fileName || 'No file chosen' }}</span>
        </div>
        <img
          v-if="pending"
          :src="`data:${pending.mimeType};base64,${pending.data}`"
          class="w-24 h-24 object-cover rounded-md border border-zinc-200"
        />
        <div>
          <label class="text-xs text-zinc-500 mb-1 block">Name (optional)</label>
          <InputText v-model="addName" class="w-full" :maxlength="MODEL_IMAGE_NAME_MAX" />
        </div>
        <div>
          <label class="text-xs text-zinc-500 mb-1 block">
            Description (optional — labels the image for the LLM)
          </label>
          <Textarea
            v-model="addDescription"
            class="w-full text-xs"
            rows="2"
            autoResize
            :maxlength="MODEL_IMAGE_DESCRIPTION_MAX"
            placeholder="e.g. obstacle layer"
          />
        </div>
        <Message v-if="addError" severity="error" :closable="false">{{ addError }}</Message>
      </div>
      <template #footer>
        <Button label="Cancel" severity="secondary" text size="small" :disabled="busy" @click="addOpen = false" />
        <Button
          label="Add"
          icon="pi pi-check"
          size="small"
          :loading="busy"
          :disabled="!pending || busy"
          data-testid="confirm-add-image"
          @click="confirmAdd"
        />
      </template>
    </Dialog>
  </div>
</template>

<script setup lang="ts">
// Spatial-only manager for a model's environment-generation image LIBRARY: add
// (compressed server-side) / delete images via dedicated endpoints, then generate
// the environment from ALL stored images in one vision call. The library lives on
// the model (store.current.model.images); add/delete responses are grafted via the
// narrow store.applyImageLibrary action (no dirty reset, no accept/dismiss banner).
// The resulting environment arrives later via the SSE model.updated event.
import { computed, onMounted, ref } from 'vue'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Textarea from 'primevue/textarea'
import Message from 'primevue/message'
import LlmExecutionConfig from '@/shared/components/LlmExecutionConfig.vue'
import { useModelChatApi } from '@/modules/model/composables/useModelChatApi'
import { useModelStore } from '@/modules/model/stores/modelStore'
import { useErrorHandler } from '@/shared/composables/useErrorHandler'
import { ApiError } from '@/shared/api/client'
import { fetchLLMProviders } from '@/shared/api/llmProviders'
import {
  IMAGE_MIME_TYPES,
  IMAGE_PROMPT_MAX,
  MAX_MODEL_IMAGES,
  MAX_IMAGE_BYTES,
  MODEL_IMAGE_NAME_MAX,
  MODEL_IMAGE_DESCRIPTION_MAX,
  isAllowedImageMime,
} from '@/shared/validation/modelRules'
import type { ExecutionLlmTarget, ImageMimeType, ModelImageDTO, ProviderInfo } from '@/shared/types'

const props = defineProps<{ modelId: string }>()

const api = useModelChatApi()
const store = useModelStore()
const { handleError, handleSuccess } = useErrorHandler()

const ACCEPT = IMAGE_MIME_TYPES.join(',')
const maxMiB = Math.round(MAX_IMAGE_BYTES / (1024 * 1024))

const isSpatial = computed(() => !!store.current?.model.is_spatial)
const images = computed<ModelImageDTO[]>(() => store.current?.model.images ?? [])

const visionTarget = ref<ExecutionLlmTarget | null>(null)
const prompt = ref('')
const busy = ref(false) // add/delete in flight
const generating = ref(false) // generate in flight
const error = ref<string | null>(null)

// --- vision model max-images (mirror of the backend over-limit rule) ---
const visionProviders = ref<ProviderInfo[]>([])

onMounted(async () => {
  try {
    const resp = await fetchLLMProviders('vision_analysis')
    visionProviders.value = resp.list ?? []
  } catch {
    // Non-fatal: without the list we fall back to the conservative max of 1.
  }
})

const hasVisionTarget = computed(
  () => !!visionTarget.value?.provider_uid && !!visionTarget.value?.model_uid
)

// effectiveMax mirrors the backend resolveMaxImages: a model that advertises 0 (or
// is unknown) resolves to a conservative single image.
const effectiveMax = computed(() => {
  const t = visionTarget.value
  if (!t?.provider_uid || !t?.model_uid) return MAX_MODEL_IMAGES
  const prov = visionProviders.value.find((p) => p.uid === t.provider_uid)
  const model = prov?.models.find((m) => m.uid === t.model_uid)
  const max = model?.max_images ?? 0
  return max > 0 ? max : 1
})

const excess = computed(() =>
  hasVisionTarget.value ? Math.max(0, images.value.length - effectiveMax.value) : 0
)
const overLimit = computed(() => excess.value > 0)

const canGenerate = computed(
  () =>
    !generating.value &&
    !busy.value &&
    images.value.length > 0 &&
    hasVisionTarget.value &&
    !overLimit.value
)

// --- add dialog state ---
const addOpen = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)
const pending = ref<{ fileName: string; mimeType: ImageMimeType; data: string } | null>(null)
const addName = ref('')
const addDescription = ref('')
const addError = ref<string | null>(null)

function openAddDialog() {
  pending.value = null
  addName.value = ''
  addDescription.value = ''
  addError.value = null
  addOpen.value = true
}

function fileToBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => {
      const result = reader.result as string
      // Strip the "data:<mime>;base64," prefix — the API wants raw base64.
      const comma = result.indexOf(',')
      resolve(comma >= 0 ? result.slice(comma + 1) : result)
    }
    reader.onerror = () => reject(reader.error ?? new Error('Failed to read file'))
    reader.readAsDataURL(file)
  })
}

async function onFilePicked(ev: Event) {
  addError.value = null
  const input = ev.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = '' // allow re-picking the same file
  if (!file) return
  if (!isAllowedImageMime(file.type)) {
    addError.value = `Unsupported type "${file.type || 'unknown'}". Allowed: png, jpeg, webp, gif.`
    return
  }
  if (file.size > MAX_IMAGE_BYTES) {
    addError.value = `"${file.name}" exceeds the ${maxMiB} MiB limit.`
    return
  }
  try {
    const data = await fileToBase64(file)
    pending.value = { fileName: file.name, mimeType: file.type as ImageMimeType, data }
    if (!addName.value) addName.value = file.name
  } catch (e) {
    addError.value = e instanceof Error ? e.message : 'Failed to read image.'
  }
}

async function confirmAdd() {
  if (!pending.value || busy.value) return
  busy.value = true
  addError.value = null
  try {
    const resp = await api.addModelImage(props.modelId, {
      name: addName.value.trim() || undefined,
      description: addDescription.value.trim() || undefined,
      mime_type: pending.value.mimeType,
      data: pending.value.data,
    })
    store.applyImageLibrary(resp.model.images, resp.updated_at)
    addOpen.value = false
  } catch (err) {
    addError.value = err instanceof Error ? err.message : 'Failed to add image.'
  } finally {
    busy.value = false
  }
}

async function removeImage(uid?: string) {
  if (!uid || busy.value) return
  busy.value = true
  error.value = null
  try {
    const resp = await api.deleteModelImage(props.modelId, uid)
    store.applyImageLibrary(resp.model.images, resp.updated_at)
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to remove image.'
    handleError(err, 'Remove image')
  } finally {
    busy.value = false
  }
}

async function generate() {
  if (!canGenerate.value) return
  generating.value = true
  error.value = null
  try {
    // Persist any unsaved model edits FIRST. The backend gates env generation on
    // the PERSISTED model, most importantly the `is_spatial` flag — so generating
    // with an unsaved spatial toggle 409s ("not spatial"). Saving here removes
    // that confusing failure without forcing a manual Save.
    if (store.dirty) {
      try {
        await store.saveCurrent()
      } catch (err) {
        error.value =
          err instanceof ApiError && (err.status === 400 || err.status === 422)
            ? 'Save your model before generating — it has validation errors. Fix the highlighted issues and try again.'
            : 'Could not save your model before generating. Fix the highlighted issues and try again.'
        return
      }
    }
    await api.generateEnvironment(props.modelId, {
      llm_target: hasVisionTarget.value ? visionTarget.value! : undefined,
      prompt: prompt.value.trim() || undefined,
    })
    handleSuccess('Generating environment from images — it will appear shortly.')
    prompt.value = ''
  } catch (err) {
    // Residual not-spatial 409: guide the user instead of showing the raw envelope.
    if (err instanceof ApiError && err.status === 409 && err.code === 'not_spatial') {
      error.value =
        'This model is not marked as spatial. Enable “Spatial model” in the General tab, then generate.'
    } else {
      error.value = err instanceof Error ? err.message : 'Failed to start generation.'
    }
    handleError(err, 'Image → Environment')
  } finally {
    generating.value = false
  }
}
</script>
