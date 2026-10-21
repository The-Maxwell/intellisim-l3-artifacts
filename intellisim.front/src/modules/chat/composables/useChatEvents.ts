import { fetchEventSource } from '@microsoft/fetch-event-source'
import type {
  ChatErrorPayload,
  ChatMessageResponse,
  ChatResponse,
  ChatStatusPayload,
} from '@/shared/types'

export interface ChatEventHandlers {
  onMessage?: (msg: ChatMessageResponse) => void
  onStatus?: (status: ChatStatusPayload) => void
  onChatUpdated?: (chat: ChatResponse) => void
  onChatDeleted?: () => void
  onChatError?: (err: ChatErrorPayload) => void
  /** Fired on a successful re-open after a drop — refetch state, no replay. */
  onReconnect?: () => void
}

const RETRY_MS = 3000

// Subscribes to GET /chats/:id/events with a real Authorization header
// (fetch-based SSE — native EventSource cannot send headers). Auto-retries
// forever until the returned unsubscribe function is called.
export function subscribeChatEvents(chatId: string, handlers: ChatEventHandlers): () => void {
  const ctrl = new AbortController()
  const base = import.meta.env.VITE_API_BASE_URL || '/api/v1/intellisim'
  let openedOnce = false

  ;(async () => {
    const { useAuthStore } = await import('@/modules/auth/stores/authStore')
    const auth = useAuthStore()

    await fetchEventSource(`${base}/chats/${chatId}/events`, {
      signal: ctrl.signal,
      openWhenHidden: true, // the popup chat must stay live in background tabs
      // Per-attempt fetch so every (re)connect carries the CURRENT token.
      fetch: async (input, init) => {
        const token = await auth.getValidAccessToken()
        const headers = new Headers(init?.headers)
        if (token) headers.set('Authorization', `Bearer ${token}`)
        return window.fetch(input, { ...init, headers })
      },
      async onopen(response) {
        if (response.ok) {
          if (openedOnce) handlers.onReconnect?.()
          openedOnce = true
          return
        }
        if (response.status === 401) await auth.renew() // next retry uses fresh token
        throw new Error(`SSE open failed: ${response.status}`)
      },
      onmessage(ev) {
        if (!ev.event || !ev.data) return
        const data: unknown = JSON.parse(ev.data)
        switch (ev.event) {
          case 'message.created':
            handlers.onMessage?.(data as ChatMessageResponse)
            break
          case 'chat.status':
            handlers.onStatus?.(data as ChatStatusPayload)
            break
          case 'chat.updated':
            handlers.onChatUpdated?.(data as ChatResponse)
            break
          case 'chat.deleted':
            handlers.onChatDeleted?.()
            break
          case 'chat.error':
            handlers.onChatError?.(data as ChatErrorPayload)
            break
        }
      },
      onerror() {
        return RETRY_MS // never give up; abort() is the only exit
      },
    }).catch(() => {
      /* aborted */
    })
  })()

  return () => ctrl.abort()
}
