<template>
  <div class="space-y-2">
    <!-- Mode selector: Auto / Image / Figure -->
    <div>
      <label class="text-[11px] text-zinc-500 mb-0.5 block">Display</label>
      <SelectButton
        :modelValue="kind"
        :options="MODE_OPTIONS"
        optionLabel="label"
        optionValue="value"
        :allowEmpty="false"
        size="small"
        @update:modelValue="(v: DisplayKind | null) => v && setKind(v)"
      />
    </div>

    <!-- IMAGE mode -->
    <div v-if="kind === 'image'" class="space-y-2">
      <input
        ref="fileInput"
        type="file"
        :accept="ACCEPT"
        class="hidden"
        @change="onFilePicked"
      />
      <div class="flex flex-wrap items-center gap-2">
        <Button
          :label="hasImage ? 'Replace image' : 'Upload image'"
          icon="pi pi-image"
          size="small"
          severity="secondary"
          outlined
          @click="fileInput?.click()"
        />
        <Button
          v-if="hasImage"
          label="Remove"
          icon="pi pi-times"
          size="small"
          severity="danger"
          text
          @click="clearImage"
        />
      </div>

      <div v-if="hasImage" class="flex items-start gap-2">
        <img
          :src="previewSrc"
          alt="display sprite"
          class="w-16 h-16 object-contain rounded-md border border-zinc-200 bg-zinc-50"
        />
        <p class="text-[10px] text-zinc-400 leading-snug flex-1">
          Large images are compressed on save.
        </p>
      </div>
      <p v-else class="text-[10px] text-zinc-400 leading-snug">
        png/jpeg/webp/gif. Large images are compressed on save.
      </p>

      <Message v-if="error" severity="warn" :closable="true" class="!mt-1" @close="error = null">
        {{ error }}
      </Message>
    </div>

    <!-- FIGURE mode -->
    <div v-else-if="kind === 'figure'" class="space-y-2">
      <template v-if="allowDraw">
        <div class="flex flex-wrap items-center gap-2">
          <Button
            label="Draw figure on canvas"
            icon="pi pi-pencil"
            size="small"
            severity="secondary"
            outlined
            @click="emit('draw-figure')"
          />
          <Button
            v-if="figureCount"
            label="Clear"
            icon="pi pi-trash"
            size="small"
            severity="danger"
            text
            @click="clearFigure"
          />
        </div>
        <p class="text-[10px] text-zinc-400 leading-snug">
          {{ figureCount }} point(s){{ figureCount && figureCount < 3 ? ' — need at least 3' : '' }}.
          Points are centered on the entity.
        </p>
      </template>
      <template v-else>
        <!-- Agents have no world canvas: draw the figure on a mini-canvas. -->
        <p class="text-[10px] text-zinc-400 leading-snug">
          Draw the figure — click to add points, drag a point to move,
          double-click to remove. (0,0) is the agent center. Needs 3+ to render.
        </p>
        <PolygonDrawCanvas :modelValue="figurePoints" @update:modelValue="setFigure" />
      </template>
    </div>

    <!-- AUTO mode: nothing to configure -->
    <p v-else class="text-[10px] text-zinc-400 leading-snug">
      Auto — rendered as the entity body in its colour.
    </p>
  </div>
</template>

<script setup lang="ts">
// Shared display editor for an agent or shape's DisplayDTO (auto / image /
// figure). Reused in AgentForm (agent display) and the EnvironmentCanvas shape
// side-panel (shape display).
//
// - auto: emits undefined (default render).
// - image: uploads a file, strips the data-URL prefix so `data` is raw base64
//   (matching the backend contract), with a thumbnail + remove.
// - figure: on a host that owns a canvas (allowDraw=true, e.g. shapes), emits a
//   `draw-figure` request and shows the live point count + clear; on a host with
//   NO canvas (agents) it falls back to a minimal inline points editor.
//
// The figure points are anchor-relative (offsets from the entity center), stored
// as the same coordinate numbers the backend uses.
import { computed, ref } from 'vue'
import SelectButton from 'primevue/selectbutton'
import Button from 'primevue/button'
import Message from 'primevue/message'
import PolygonDrawCanvas from './PolygonDrawCanvas.vue'
import type { DisplayDTO, DisplayKind, PointDTO } from '@/shared/types'
import {
  IMAGE_MIME_TYPES,
  MAX_DISPLAY_IMAGE_BYTES,
  isAllowedImageMime,
} from '@/shared/validation/modelRules'

const props = withDefaults(
  defineProps<{
    modelValue: DisplayDTO | undefined
    /** When true the host owns a canvas → use the draw-on-canvas figure flow. */
    allowDraw?: boolean
  }>(),
  { allowDraw: false }
)

const emit = defineEmits<{
  'update:modelValue': [v: DisplayDTO | undefined]
  'draw-figure': []
}>()

const MODE_OPTIONS: { value: DisplayKind; label: string }[] = [
  { value: 'auto', label: 'Auto' },
  { value: 'image', label: 'Image' },
  { value: 'figure', label: 'Figure' },
]

const ACCEPT = IMAGE_MIME_TYPES.join(',')
const softMiB = Math.round(MAX_DISPLAY_IMAGE_BYTES / (1024 * 1024))

const fileInput = ref<HTMLInputElement | null>(null)
const error = ref<string | null>(null)

const kind = computed<DisplayKind>(() => props.modelValue?.kind ?? 'auto')
const hasImage = computed(() => !!props.modelValue?.image?.data)
const figurePoints = computed<PointDTO[]>(() => props.modelValue?.figure ?? [])
const figureCount = computed(() => figurePoints.value.length)

const previewSrc = computed(() => {
  const img = props.modelValue?.image
  if (!img?.data) return ''
  return `data:${img.mime_type || 'image/png'};base64,${img.data}`
})

function emitDisplay(next: DisplayDTO | undefined) {
  emit('update:modelValue', next)
}

function setKind(next: DisplayKind) {
  if (next === 'auto') {
    // Auto = default render → drop the whole display object.
    emitDisplay(undefined)
    return
  }
  emitDisplay({ ...(props.modelValue ?? {}), kind: next })
}

// --- image ---
function fileToBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => {
      const result = reader.result as string
      const comma = result.indexOf(',')
      resolve(comma >= 0 ? result.slice(comma + 1) : result)
    }
    reader.onerror = () => reject(reader.error ?? new Error('Failed to read file'))
    reader.readAsDataURL(file)
  })
}

async function onFilePicked(ev: Event) {
  error.value = null
  const input = ev.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = '' // allow re-picking the same file
  if (!file) return
  if (!isAllowedImageMime(file.type)) {
    error.value = `Unsupported type "${file.type || 'unknown'}". Allowed: png, jpeg, webp, gif.`
    return
  }
  // Soft size check — the backend is authoritative and compresses on save.
  if (file.size > MAX_DISPLAY_IMAGE_BYTES) {
    error.value = `"${file.name}" is over ${softMiB} MB; it will be compressed on save.`
  }
  try {
    const data = await fileToBase64(file)
    emitDisplay({
      ...(props.modelValue ?? {}),
      kind: 'image',
      image: { data, mime_type: file.type },
    })
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to read image.'
  }
}

function clearImage() {
  if (!props.modelValue) return
  emitDisplay({ ...props.modelValue, kind: 'image', image: undefined })
}

// --- figure ---
// allowDraw hosts (shapes) emit `draw-figure` to draw on the world canvas; the
// no-canvas host (agents) draws on the inline PolygonDrawCanvas via setFigure.
function setFigure(points: PointDTO[]) {
  emitDisplay({ ...(props.modelValue ?? {}), kind: 'figure', figure: points })
}

function clearFigure() {
  emitDisplay({ ...(props.modelValue ?? {}), kind: 'figure', figure: [] })
}
</script>
