<template>
  <!-- Chat Panel -->
  <div
    v-if="floatingStore.isOpen && !onGeneralChatPage"
    class="fixed bottom-20 right-6 w-96 h-[500px] bg-white rounded-xl shadow-2xl border border-zinc-200 flex flex-col z-50 overflow-hidden"
  >
    <!-- Header -->
    <div class="bg-zinc-900 px-4 py-2.5 flex items-center justify-between">
      <div class="flex items-center gap-2 min-w-0">
        <div class="w-6 h-6 bg-violet-500 rounded flex items-center justify-center text-xs font-bold text-white shrink-0">
          IS
        </div>
        <span class="text-sm font-medium text-white truncate">
          {{ session.chat.value?.name || 'Quick Chat' }}
        </span>
      </div>
      <div class="flex items-center gap-1">
        <button
          v-if="session.chatId.value"
          class="text-zinc-400 hover:text-white p-1"
          title="Open full page"
          @click="openFullPage"
        >
          <i class="pi pi-external-link text-sm" />
        </button>
        <button
          v-if="session.chatId.value"
          class="text-zinc-400 hover:text-white p-1"
          title="New quick chat"
          @click="resetChat"
        >
          <i class="pi pi-plus text-sm" />
        </button>
        <button class="text-zinc-400 hover:text-white p-1" title="Minimize" @click="floatingStore.close()">
          <i class="pi pi-minus text-sm" />
        </button>
      </div>
    </div>

    <!-- Unavailable model banner (compact) -->
    <div
      v-if="session.chat.value && !session.modelAvailable.value"
      class="bg-amber-50 border-b border-amber-200 px-3 py-2 text-xs text-amber-800"
    >
      <p class="mb-1">Model unavailable — pick another to continue:</p>
      <LlmTargetPicker :modelValue="session.chat.value.llm_target" @update:modelValue="onChangeModel" />
    </div>

    <!-- Model picker for a brand-new quick chat -->
    <div v-if="!session.chatId.value" class="px-3 py-2 border-b border-zinc-200 bg-zinc-50">
      <LlmTargetPicker v-model="newChatTarget" />
    </div>

    <ChatPanel
      :messages="session.messages.value"
      :loading="session.loading.value"
      :thinking="session.thinking.value"
      :has-more="session.hasMore.value"
      :loading-older="session.loadingOlder.value"
      :disabled="inputDisabled"
      :load-older="session.loadOlder"
      placeholder="Ask a quick question..."
      @send="handleSend"
    />
  </div>

  <!-- Floating Button -->
  <button
    v-if="!onGeneralChatPage"
    class="fixed bottom-6 right-6 w-14 h-14 bg-violet-500 hover:bg-violet-600 text-white rounded-full shadow-lg flex items-center justify-center z-50 transition-colors"
    title="Quick Chat"
    @click="floatingStore.toggleOpen()"
  >
    <i :class="floatingStore.isOpen ? 'pi pi-times' : 'pi pi-comment'" class="text-lg" />
  </button>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import ChatPanel from '@/shared/components/ChatPanel.vue'
import LlmTargetPicker from '@/shared/components/LlmTargetPicker.vue'
import { useFloatingChatStore } from '@/shared/stores/floatingChatStore'
import { useSidebarStore } from '@/shared/stores/sidebarStore'
import { useChatApi } from '@/modules/chat/composables/useChatApi'
import { useChatSession } from '@/modules/chat/composables/useChatSession'
import { useErrorHandler } from '@/shared/composables/useErrorHandler'
import { getDefaultLlmTarget, setDefaultLlmTarget } from '@/shared/llm/defaultTarget'
import { ApiError } from '@/shared/api/client'
import type { LLMTargetDTO } from '@/shared/types'

const router = useRouter()
const route = useRoute()
const floatingStore = useFloatingChatStore()

// The quick chat is redundant while the user is already on a general chat
// page — hide both the popup and its launcher button there.
const onGeneralChatPage = computed(() => route.name === 'chat' || route.name === 'chat-new')
const sidebarStore = useSidebarStore()
const chatApi = useChatApi()
const session = useChatSession()
const { handleError } = useErrorHandler()

const newChatTarget = ref<LLMTargetDTO | null>(getDefaultLlmTarget())
const creating = ref(false)

const inputDisabled = computed(() =>
  session.chat.value ? !session.modelAvailable.value : false
)

watch(session.deleted, (d) => {
  if (d) {
    if (session.chatId.value) sidebarStore.removeGeneralChat(session.chatId.value)
    resetChat()
  }
})

// Foreign chat (403): open() swallows the error into `forbidden` rather than
// throwing, so report it here and drop back to a fresh quick chat.
watch(session.forbidden, (f) => {
  if (f) {
    handleError(new ApiError("You don't have access to this chat.", 'forbidden', 403), 'Quick Chat')
    resetChat()
  }
})

// Open/close the live session with the popup; reconnect when the stored
// chat id changes (e.g. another part of the app pointed the popup somewhere).
watch(
  () => [floatingStore.isOpen, floatingStore.chatId] as const,
  async ([isOpen, chatId]) => {
    if (isOpen && chatId && chatId !== session.chatId.value) {
      try {
        await session.open(chatId)
      } catch (err) {
        handleError(err, 'Quick Chat')
        floatingStore.setChatId(null)
      }
    } else if (!isOpen) {
      session.close()
    } else if (isOpen && chatId && chatId === session.chatId.value) {
      // Re-opened on the same chat: resubscribe + refresh.
      try {
        await session.open(chatId)
      } catch {
        /* handled above on real failures */
      }
    }
  },
  { immediate: true }
)

async function handleSend(text: string) {
  if (!session.chatId.value) {
    if (creating.value) return
    creating.value = true
    try {
      const target = newChatTarget.value
      if (!target) {
        handleError(new Error('Select a model to start chatting'), 'Quick Chat')
        return
      }
      const chat = await chatApi.createChat({ name: 'Quick Chat', llm_target: target })
      setDefaultLlmTarget(target)
      sidebarStore.addGeneralChat(chat)
      floatingStore.setChatId(chat.id)
      await session.open(chat.id)
      await session.send(text)
    } catch (err) {
      if (err instanceof ApiError && err.code === 'model_unavailable' && session.chatId.value) {
        session.open(session.chatId.value).catch(() => {})
      }
      handleError(err, 'Quick Chat')
    } finally {
      creating.value = false
    }
  } else {
    try {
      await session.send(text)
    } catch (err) {
      if (err instanceof ApiError && err.code === 'model_unavailable' && session.chatId.value) {
        session.open(session.chatId.value).catch(() => {})
      }
      handleError(err, 'Quick Chat')
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

function openFullPage() {
  if (!session.chatId.value) return
  // Close the popup before navigating; on the chat page the launcher button
  // is hidden too, so the quick chat disappears entirely.
  floatingStore.close()
  router.push(`/chat/${session.chatId.value}`)
}

function resetChat() {
  session.close()
  session.chatId.value = null
  session.chat.value = null
  session.messages.value = []
  floatingStore.setChatId(null)
}
</script>
