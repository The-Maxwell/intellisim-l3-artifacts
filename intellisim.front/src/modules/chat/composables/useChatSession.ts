import { computed, ref } from 'vue'
import type { ChatMessageResponse, ChatResponse, LLMTargetDTO } from '@/shared/types'
import { setDefaultLlmTarget } from '@/shared/llm/defaultTarget'
import { ApiError } from '@/shared/api/client'
import { useChatApi } from './useChatApi'
import { subscribeChatEvents } from './useChatEvents'

// One live chat: loads chat + latest page, subscribes to SSE, keeps messages
// chronologically sorted and deduplicated by id (the sender receives its own
// user message twice: from the 202 response AND the SSE event).
export function useChatSession() {
  const chatApi = useChatApi()

  const chatId = ref<string | null>(null)
  const chat = ref<ChatResponse | null>(null)
  const messages = ref<ChatMessageResponse[]>([])
  const thinking = ref(false)
  const hasMore = ref(false)
  const loading = ref(false)
  const loadingOlder = ref(false)
  const deleted = ref(false)
  // Set when the backend answers 403: the chat exists but belongs to someone
  // else and the user is not an admin. Distinct from `deleted` (404).
  const forbidden = ref(false)

  let unsubscribe: (() => void) | null = null

  const modelAvailable = computed(() => chat.value?.model_available ?? true)

  function compareMessages(a: ChatMessageResponse, b: ChatMessageResponse): number {
    // Go's RFC3339Nano trims trailing zeros, so the strings are NOT safely
    // lexicographically ordered across differing fractional precision —
    // compare real timestamps and tie-break on id.
    const byTime = Date.parse(a.created_at) - Date.parse(b.created_at)
    return byTime !== 0 ? byTime : a.id.localeCompare(b.id)
  }

  function upsertMessage(msg: ChatMessageResponse) {
    if (messages.value.some((m) => m.id === msg.id)) return
    const idx = messages.value.findIndex((m) => compareMessages(msg, m) < 0)
    if (idx === -1) messages.value.push(msg)
    else messages.value.splice(idx, 0, msg)
  }

  async function refresh() {
    if (!chatId.value) return
    const [freshChat, page] = await Promise.all([
      chatApi.getChat(chatId.value),
      chatApi.listMessages(chatId.value),
    ])
    chat.value = freshChat
    thinking.value = freshChat.status === 'thinking'
    messages.value = [...page.messages].sort(compareMessages)
    hasMore.value = page.has_more
  }

  async function open(id: string) {
    // Same chat already open and subscribed — just refresh, avoid churn.
    if (id === chatId.value && unsubscribe !== null) {
      await refresh()
      return
    }
    close()
    chatId.value = id
    deleted.value = false
    forbidden.value = false
    loading.value = true
    try {
      await refresh()
    } catch (err) {
      // Foreign chat (not owner, not admin): surface a distinct forbidden
      // state instead of bubbling a generic error, and never subscribe SSE.
      if (err instanceof ApiError && err.status === 403) {
        forbidden.value = true
        chat.value = null
        messages.value = []
        return
      }
      throw err
    } finally {
      loading.value = false
    }
    unsubscribe = subscribeChatEvents(id, {
      onMessage: upsertMessage,
      onStatus: (s) => {
        thinking.value = s.status === 'thinking'
      },
      onChatUpdated: (c) => {
        chat.value = c
      },
      onChatDeleted: () => {
        deleted.value = true
        close()
      },
      onChatError: () => {
        thinking.value = false
      },
      onReconnect: () => {
        refresh().catch(() => {})
      },
    })
  }

  function close() {
    unsubscribe?.()
    unsubscribe = null
  }

  async function send(content: string) {
    if (!chatId.value) throw new Error('no chat open')
    const resp = await chatApi.sendMessage(chatId.value, content)
    upsertMessage(resp.message)
    thinking.value = resp.chat_status === 'thinking'
  }

  async function loadOlder() {
    if (!chatId.value || !hasMore.value || loadingOlder.value || messages.value.length === 0) return
    loadingOlder.value = true
    try {
      const page = await chatApi.listMessages(chatId.value, { before: messages.value[0]!.id })
      hasMore.value = page.has_more
      for (const m of page.messages) upsertMessage(m)
    } finally {
      loadingOlder.value = false
    }
  }

  async function changeModel(target: LLMTargetDTO) {
    if (!chatId.value) return
    chat.value = await chatApi.updateChat(chatId.value, { llm_target: target })
    setDefaultLlmTarget(target)
  }

  async function rename(name: string) {
    if (!chatId.value) return
    chat.value = await chatApi.updateChat(chatId.value, { name })
  }

  return {
    chatId,
    chat,
    messages,
    thinking,
    hasMore,
    loading,
    loadingOlder,
    deleted,
    forbidden,
    modelAvailable,
    open,
    close,
    send,
    loadOlder,
    changeModel,
    rename,
  }
}
