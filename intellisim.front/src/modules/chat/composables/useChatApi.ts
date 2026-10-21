import apiClient from '@/shared/api/client'
import type {
  ChatResponse,
  CreateChatRequest,
  ListChatsResponse,
  MessagesPageResponse,
  SendMessageResponse,
  UpdateChatRequest,
} from '@/shared/types'

const BASE = '/chats'

export function useChatApi() {
  async function createChat(req: CreateChatRequest): Promise<ChatResponse> {
    const { data } = await apiClient.post<ChatResponse>(BASE, req)
    return data
  }

  async function listChats(page = 1, pageSize = 50): Promise<ListChatsResponse> {
    const { data } = await apiClient.get<ListChatsResponse>(BASE, {
      params: { page, page_size: pageSize },
    })
    return data
  }

  async function getChat(id: string): Promise<ChatResponse> {
    const { data } = await apiClient.get<ChatResponse>(`${BASE}/${id}`)
    return data
  }

  async function updateChat(id: string, req: UpdateChatRequest): Promise<ChatResponse> {
    const { data } = await apiClient.put<ChatResponse>(`${BASE}/${id}`, req)
    return data
  }

  async function deleteChat(id: string): Promise<void> {
    await apiClient.delete(`${BASE}/${id}`)
  }

  /** Latest page when `before` is omitted; older pages via the cursor. */
  async function listMessages(
    id: string,
    opts: { limit?: number; before?: string } = {}
  ): Promise<MessagesPageResponse> {
    const { data } = await apiClient.get<MessagesPageResponse>(`${BASE}/${id}/messages`, {
      params: { limit: opts.limit, before: opts.before },
    })
    return data
  }

  /** Async send: 202 with the persisted user message; reply arrives via SSE. */
  async function sendMessage(id: string, content: string): Promise<SendMessageResponse> {
    const { data } = await apiClient.post<SendMessageResponse>(`${BASE}/${id}/messages`, { content })
    return data
  }

  return { createChat, listChats, getChat, updateChat, deleteChat, listMessages, sendMessage }
}
