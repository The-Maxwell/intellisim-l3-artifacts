<template>
  <Dialog
    :visible="visible"
    modal
    header="Duplicate model"
    :style="{ width: '440px' }"
    @update:visible="emit('update:visible', $event)"
  >
    <div>
      <label class="text-xs font-medium text-zinc-600 mb-1 block">Name</label>
      <InputText
        v-model="name"
        class="w-full"
        size="small"
        :maxlength="MODEL_NAME_MAX"
        placeholder="Copy name"
        @keydown.enter="confirm"
      />
      <p class="text-xs text-zinc-400 mt-1">
        The copy includes the model's companion chat (if any) with its full history.
      </p>
    </div>
    <template #footer>
      <div class="flex justify-end gap-2">
        <Button label="Cancel" severity="secondary" size="small" @click="emit('update:visible', false)" />
        <Button label="Duplicate" size="small" :loading="duplicating" :disabled="!modelId" @click="confirm" />
      </div>
    </template>
  </Dialog>
</template>

<script setup lang="ts">
// Reusable "duplicate model" dialog. Opened from the models list row and the
// model editor header. Self-contained: it calls the store (which hits the
// backend that also clones the companion chat) and emits `duplicated` with the
// new model — the caller decides what to do next (refresh the list / navigate).
import { ref, watch } from 'vue'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Button from 'primevue/button'
import { useModelStore } from '@/modules/model/stores/modelStore'
import { useErrorHandler } from '@/shared/composables/useErrorHandler'
import { MODEL_NAME_MAX } from '@/shared/validation/modelRules'
import type { ModelResponse } from '@/shared/types'

const props = defineProps<{
  visible: boolean
  modelId: string | null
  modelName: string
}>()
const emit = defineEmits<{
  'update:visible': [value: boolean]
  duplicated: [model: ModelResponse]
}>()

const store = useModelStore()
const { handleError } = useErrorHandler()

const name = ref('')
const duplicating = ref(false)

// Pre-fill the name with "<original> (copy)" each time the dialog opens.
watch(
  () => props.visible,
  (open) => {
    if (open) name.value = props.modelName ? `${props.modelName} (copy)` : ''
  }
)

async function confirm() {
  if (duplicating.value || !props.modelId) return
  duplicating.value = true
  try {
    const model = await store.duplicate(props.modelId, name.value.trim() || undefined)
    emit('duplicated', model)
    emit('update:visible', false)
  } catch (err) {
    handleError(err, 'Duplicate Model')
  } finally {
    duplicating.value = false
  }
}
</script>
