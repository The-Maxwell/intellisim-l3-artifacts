<template>
  <div class="h-full flex flex-col min-h-0">
    <!-- Header: model-generation LLM target picker (capability-filtered). -->
    <div class="border-b border-zinc-200 bg-white px-4 py-3 space-y-2">
      <p class="text-xs font-medium text-zinc-500">Model-editing assistant</p>
      <LlmExecutionConfig
        :modelValue="session.target.value"
        capability="model_generation"
        @update:modelValue="onTargetChange"
      />
      <p v-if="!hasTarget" class="text-xs text-amber-600">
        Pick an execution model above before sending a message.
      </p>
    </div>

    <!-- Surfaced session error -->
    <Message
      v-if="session.error.value"
      severity="error"
      :closable="true"
      class="mx-4 mt-3"
      @close="session.error.value = null"
    >
      {{ session.error.value }}
    </Message>

    <!-- Conversation (display message only — never raw spec JSON). -->
    <div class="flex-1 min-h-0">
      <ChatPanel
        :messages="displayMessages"
        :loading="session.loading.value"
        :thinking="session.thinking.value"
        :has-more="session.hasMore.value"
        :loading-older="session.loadingOlder.value"
        :disabled="!hasTarget || session.thinking.value"
        :load-older="session.loadOlder"
        placeholder="Describe the model change you want…"
        @send="handleSend"
      >
        <template #input-prefix>
          <ScopeSelector v-model="scope" :model="props.model" />
        </template>
        <template #empty>
          <div class="flex flex-col items-center justify-center h-full text-zinc-400 px-6 text-center">
            <div class="w-14 h-14 bg-violet-500 rounded-2xl flex items-center justify-center mb-3">
              <i class="pi pi-comments text-2xl text-white" />
            </div>
            <p class="text-sm font-medium text-zinc-600 mb-1">Edit this model with the LLM</p>
            <p class="text-xs text-zinc-400">
              Ask to add agents, tweak rules, or refine the environment — changes apply live.
            </p>
          </div>
        </template>
      </ChatPanel>
    </div>
  </div>
</template>

<script setup lang="ts">
// Right-pane companion chat for the model editor. Consumes the SHARED
// useModelChatSession instance (keyed by modelId) whose lifecycle (open/close +
// the single SSE subscription + model.updated → store routing) is owned by the
// editor view. The panel renders only the display `content` of each message
// (never `raw`, the full spec JSON) and lets the user pick an EditScope + a
// model_generation execution target before sending.
import { computed, ref } from 'vue'
import ChatPanel from '@/shared/components/ChatPanel.vue'
import Message from 'primevue/message'
import LlmExecutionConfig from '@/shared/components/LlmExecutionConfig.vue'
import ScopeSelector from './ScopeSelector.vue'
import { useModelChatSession } from '@/modules/model/composables/useModelChatSession'
import { useErrorHandler } from '@/shared/composables/useErrorHandler'
import type { ChatPanelMessage } from '@/shared/components/chatPanelTypes'
import type { EditScope, ExecutionLlmTarget, Model } from '@/shared/types'

const props = defineProps<{ modelId: string; model: Model }>()

const { handleError } = useErrorHandler()
const session = useModelChatSession(props.modelId)

const scope = ref<EditScope | undefined>(undefined)

const hasTarget = computed(
  () => !!session.target.value?.provider_uid && !!session.target.value?.model_uid
)

// Map ModelChatMessage → the ChatPanel display contract. Show `content` only;
// `raw` (the full spec JSON) is deliberately never surfaced in the UI.
const displayMessages = computed<ChatPanelMessage[]>(() =>
  session.messages.value.map((m) => ({
    id: m.id,
    role: m.role,
    content: m.content,
    created_at: m.created_at,
    error: m.error ? { code: m.error.code, message: m.error.message } : null,
  }))
)

async function onTargetChange(t: ExecutionLlmTarget) {
  try {
    await session.setTarget(t)
  } catch (err) {
    handleError(err, 'Set Chat Model')
  }
}

async function handleSend(content: string) {
  if (!hasTarget.value) {
    handleError(new Error('Select an execution model first'), 'Model Chat')
    return
  }
  try {
    await session.send(content, scope.value)
  } catch (err) {
    handleError(err, 'Send Message')
  }
}
// Lifecycle (open/close + the single SSE subscription) is owned by the editor
// view; this panel only consumes the shared session for rendering and actions.
</script>
