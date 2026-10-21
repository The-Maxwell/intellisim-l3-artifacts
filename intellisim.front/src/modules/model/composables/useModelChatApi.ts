import apiClient from '@/shared/api/client'
import type {
  AddModelImageRequest,
  EditScope,
  ExecutionLlmTarget,
  GenerateEnvironmentRequest,
  MessagesPage,
  ModelChat,
  ModelResponse,
  ModelSendMessageResponse,
  SendModelMessageRequest,
  SetModelChatTargetRequest,
} from '@/shared/types'

const BASE = '/models'

// REST wrapper over the model-chat sub-resource (mirrors useChatApi for the
// chat half of the models module). The companion chat lives under /models/:id/chat.
export function useModelChatApi() {
  async function getChat(modelId: string): Promise<ModelChat> {
    const { data } = await apiClient.get<ModelChat>(`${BASE}/${modelId}/chat`)
    return data
  }

  async function setChatTarget(modelId: string, target: ExecutionLlmTarget): Promise<ModelChat> {
    const body: SetModelChatTargetRequest = { llm_target: target }
    const { data } = await apiClient.put<ModelChat>(`${BASE}/${modelId}/chat`, body)
    return data
  }

  /** Latest page when `before` is omitted; older pages via the cursor. */
  async function listMessages(
    modelId: string,
    opts: { limit?: number; before?: string } = {}
  ): Promise<MessagesPage> {
    const { data } = await apiClient.get<MessagesPage>(`${BASE}/${modelId}/chat/messages`, {
      params: { limit: opts.limit, before: opts.before },
    })
    return data
  }

  /** Async send: 202 with the persisted user message; reply arrives via SSE. */
  async function sendMessage(
    modelId: string,
    opts: { content: string; scope?: EditScope }
  ): Promise<ModelSendMessageResponse> {
    const body: SendModelMessageRequest = { content: opts.content, scope: opts.scope }
    const { data } = await apiClient.post<ModelSendMessageResponse>(
      `${BASE}/${modelId}/chat/messages`,
      body
    )
    return data
  }

  /** Add one image to the model's environment-generation library; returns the full updated model. */
  async function addModelImage(modelId: string, body: AddModelImageRequest): Promise<ModelResponse> {
    const { data } = await apiClient.post<ModelResponse>(`${BASE}/${modelId}/images`, body)
    return data
  }

  /** Delete one image (by UID) from the library; returns the full updated model. */
  async function deleteModelImage(modelId: string, imageId: string): Promise<ModelResponse> {
    const { data } = await apiClient.delete<ModelResponse>(`${BASE}/${modelId}/images/${imageId}`)
    return data
  }

  /**
   * Start environment generation from the model's STORED images. Async like
   * sendMessage: 202; the updated model arrives later via the SSE `model.updated` event.
   */
  async function generateEnvironment(
    modelId: string,
    opts: { llm_target?: ExecutionLlmTarget; prompt?: string }
  ): Promise<ModelSendMessageResponse> {
    const body: GenerateEnvironmentRequest = { llm_target: opts.llm_target, prompt: opts.prompt }
    const { data } = await apiClient.post<ModelSendMessageResponse>(
      `${BASE}/${modelId}/environment/generate`,
      body
    )
    return data
  }

  return {
    getChat,
    setChatTarget,
    listMessages,
    sendMessage,
    addModelImage,
    deleteModelImage,
    generateEnvironment,
  }
}
