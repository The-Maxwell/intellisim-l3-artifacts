<template>
  <div class="h-full flex flex-col">
    <!-- Access denied: the chat exists but belongs to another user (403) -->
    <div
      v-if="session.forbidden.value"
      class="h-full flex flex-col items-center justify-center text-zinc-500 px-6 text-center"
    >
      <div class="w-16 h-16 bg-zinc-100 rounded-2xl flex items-center justify-center mb-4">
        <i class="pi pi-lock text-3xl text-zinc-400" />
      </div>
      <p class="text-lg font-medium text-zinc-700 mb-1">Access denied</p>
      <p class="text-sm text-zinc-500 mb-4">You don't have access to this chat.</p>
      <Button label="Go to new chat" icon="pi pi-plus" size="small" @click="router.replace('/chat/new')" />
    </div>

    <template v-else>
    <!-- Chat header: name + per-chat model picker -->
    <div v-if="session.chat.value" class="border-b border-zinc-200 bg-white px-6 h-14 shrink-0 flex items-center justify-between gap-4">
      <div class="min-w-0 flex items-center gap-2">
        <template v-if="renaming">
          <InputText
            v-model="renameText"
            :maxlength="CHAT_NAME_MAX"
            size="small"
            class="w-64"
            autofocus
            @keydown.enter="saveRename"
            @keydown.esc="renaming = false"
            @blur="saveRename"
          />
        </template>
        <template v-else>
          <h2 class="text-sm font-semibold text-zinc-800 truncate">{{ session.chat.value.name }}</h2>
          <button class="text-zinc-400 hover:text-zinc-600" title="Rename" @click="startRename">
            <i class="pi pi-pencil text-xs" />
          </button>
          <Button
            icon="pi pi-trash"
            severity="danger"
            text
            rounded
            size="small"
            aria-label="Delete chat"
            title="Delete chat"
            @click="confirmDeleteChat"
          />
        </template>
      </div>
      <LlmTargetPicker :modelValue="session.chat.value.llm_target" @update:modelValue="onChangeModel" />
    </div>

    <!-- Unavailable model banner -->
    <Message
      v-if="session.chat.value && !session.modelAvailable.value"
      severity="warn"
      :closable="false"
      class="mx-6 mt-3"
    >
      Model <b>{{ session.chat.value.llm_target.model_uid }}</b> (provider
      <b>{{ session.chat.value.llm_target.provider_uid }}</b>) is not available right now.
      Select another model above to continue this chat.
    </Message>

    <div class="flex-1 min-h-0 flex justify-center">
      <div class="w-full max-w-3xl h-full">
        <ChatPanel
          :messages="session.messages.value"
          :loading="session.loading.value"
          :thinking="session.thinking.value"
          :has-more="session.hasMore.value"
          :loading-older="session.loadingOlder.value"
          :disabled="inputDisabled"
          :load-older="session.loadOlder"
          placeholder="Ask anything..."
          @send="handleSend"
        >
          <template #empty>
            <div class="flex flex-col items-center justify-center h-full text-zinc-400">
              <div class="w-16 h-16 bg-violet-500 rounded-2xl flex items-center justify-center mb-4">
                <i class="pi pi-comments text-3xl text-white" />
              </div>
              <p class="text-lg font-medium text-zinc-700 mb-1">IntelliSim Chat</p>
              <p class="text-sm text-zinc-500 mb-4">Ask questions about agent-based modeling</p>
              <div v-if="!chatIdParam" class="mb-6">
                <LlmTargetPicker v-model="newChatTarget" />
              </div>
              <div class="flex flex-wrap gap-2 justify-center">
                <button
                  v-for="prompt in suggestedPrompts"
                  :key="prompt"
                  class="px-3 py-1.5 rounded-full border border-zinc-300 text-xs text-zinc-600 hover:bg-violet-50 hover:border-violet-300 transition-colors"
                  @click="handleSend(prompt)"
                >
                  {{ prompt }}
                </button>
              </div>
            </div>
          </template>
        </ChatPanel>
      </div>
    </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import Button from 'primevue/button'
import { useConfirm } from 'primevue/useconfirm'
import ChatPanel from '@/shared/components/ChatPanel.vue'
import LlmTargetPicker from '@/shared/components/LlmTargetPicker.vue'
import { useChatApi } from '../composables/useChatApi'
import { useChatSession } from '../composables/useChatSession'
import { useSidebarStore } from '@/shared/stores/sidebarStore'
import { useErrorHandler } from '@/shared/composables/useErrorHandler'
import { getDefaultLlmTarget, setDefaultLlmTarget } from '@/shared/llm/defaultTarget'
import { CHAT_NAME_MAX } from '@/shared/validation/chatRules'
import { ApiError } from '@/shared/api/client'
import type { LLMTargetDTO } from '@/shared/types'

const route = useRoute()
const router = useRouter()
const chatApi = useChatApi()
const session = useChatSession()
const sidebarStore = useSidebarStore()
const confirm = useConfirm()
const { handleError, handleSuccess } = useErrorHandler()

const newChatTarget = ref<LLMTargetDTO | null>(getDefaultLlmTarget())
const renaming = ref(false)
const renameText = ref('')
const creating = ref(false)

const chatIdParam = computed(() =>
  typeof route.params.id === 'string' && route.params.id ? route.params.id : null
)

const inputDisabled = computed(() => {
  if (session.chat.value) return !session.modelAvailable.value
  return false
})

const suggestedPrompts = [
  'What is agent-based modeling?',
  'How do agents interact in a simulation?',
  'What is emergent behavior in complex systems?',
  'Where is agent-based modeling used in the real world?',
]

onMounted(async () => {
  if (chatIdParam.value) await openChat(chatIdParam.value)
})

onUnmounted(() => session.close())

watch(chatIdParam, async (id) => {
  if (id && id !== session.chatId.value) {
    await openChat(id)
  } else if (!id) {
    session.close()
    session.chatId.value = null
    session.chat.value = null
    session.messages.value = []
    session.forbidden.value = false // clear the access-denied panel on /chat/new
  }
})

watch(session.deleted, (d) => {
  if (d && session.chatId.value) {
    sidebarStore.removeGeneralChat(session.chatId.value)
    router.replace('/chat/new')
  }
})

async function openChat(id: string) {
  try {
    await session.open(id)
  } catch (err) {
    handleError(err, 'Load Chat')
  }
}

async function handleSend(text: string) {
  if (!session.chatId.value) {
    if (creating.value) return
    creating.value = true
    try {
      const target = newChatTarget.value
      if (!target) {
        handleError(new Error('Select a model to start chatting'), 'New Chat')
        return
      }
      const chat = await chatApi.createChat({ llm_target: target })
      setDefaultLlmTarget(target)
      sidebarStore.addGeneralChat(chat)
      await session.open(chat.id)
      await session.send(text)
      router.replace(`/chat/${chat.id}`)
    } catch (err) {
      if (err instanceof ApiError && err.code === 'model_unavailable') {
        // Refresh availability so the banner appears.
        if (session.chatId.value) session.open(session.chatId.value).catch(() => {})
      }
      handleError(err, 'Send Message')
    } finally {
      creating.value = false
    }
  } else {
    try {
      await session.send(text)
    } catch (err) {
      if (err instanceof ApiError && err.code === 'model_unavailable') {
        // Refresh availability so the banner appears.
        if (session.chatId.value) session.open(session.chatId.value).catch(() => {})
      }
      handleError(err, 'Send Message')
    }
  }
}

async function onChangeModel(target: LLMTargetDTO) {
  try {
    await session.changeModel(target)
  } catch (err) {
    handleError(err, 'Change Model')
  }
}

function startRename() {
  renameText.value = session.chat.value?.name ?? ''
  renaming.value = true
}

async function saveRename() {
  if (!renaming.value) return
  renaming.value = false
  const name = renameText.value.trim()
  if (!name || name === session.chat.value?.name) return
  try {
    await session.rename(name)
    const updated = session.chat.value
    if (updated) {
      const inSidebar = sidebarStore.generalChats.find((c) => c.id === updated.id)
      if (inSidebar) inSidebar.name = updated.name
    }
  } catch (err) {
    handleError(err, 'Rename Chat')
  }
}

function confirmDeleteChat(event: MouseEvent) {
  const chat = session.chat.value
  if (!chat) return
  confirm.require({
    target: event.currentTarget as HTMLElement,
    message: `Delete "${chat.name || 'Untitled Chat'}"? This cannot be undone.`,
    header: 'Delete chat',
    icon: 'pi pi-exclamation-triangle',
    rejectProps: { label: 'Cancel', severity: 'secondary', size: 'small' },
    acceptProps: { label: 'Delete', severity: 'danger', size: 'small' },
    accept: async () => {
      try {
        await chatApi.deleteChat(chat.id)
        sidebarStore.removeGeneralChat(chat.id)
        handleSuccess('Chat deleted')
        router.replace('/chat/new')
      } catch (err) {
        handleError(err, 'Delete chat')
      }
    },
  })
}
</script>
