export interface LLMTargetDTO {
  provider_uid: string
  model_uid: string
  effort?: string
}

export interface MessageErrorDTO {
  code: string
  message: string
}

export interface TokenUsage {
  provider_uid?: string
  model_uid?: string
  effort?: string
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  calls: number
  latency_ms: number
}

export interface TokenUsageSummary {
  total: TokenUsage
  by_model: TokenUsage[]
}

export type ChatStatus = 'idle' | 'thinking'
export type ChatMessageRole = 'user' | 'assistant' | 'system'

export interface ChatResponse {
  id: string
  type: string
  name: string
  description: string
  llm_target: LLMTargetDTO
  /** Live availability of the chat's model — false blocks sending. */
  model_available: boolean
  status: ChatStatus
  model_id?: string
  token_usage?: TokenUsageSummary
  created_at: string
  updated_at: string
  created_by: string
  updated_by: string
}

export interface ChatMessageResponse {
  id: string
  chat_id: string
  role: ChatMessageRole
  content: string
  llm_target?: LLMTargetDTO
  usage?: TokenUsage
  error?: MessageErrorDTO
  created_at: string
  created_by: string
}

export interface ListChatsResponse {
  chats: ChatResponse[]
  page: number
  page_size: number
  has_more: boolean
}

export interface MessagesPageResponse {
  /** Chronological (oldest → newest) within the page. */
  messages: ChatMessageResponse[]
  has_more: boolean
}

export interface SendMessageResponse {
  message: ChatMessageResponse
  chat_status: ChatStatus
}

export interface CreateChatRequest {
  name?: string
  description?: string
  llm_target: LLMTargetDTO
}

export interface UpdateChatRequest {
  name?: string
  description?: string
  llm_target?: LLMTargetDTO
}

// SSE payloads
export interface ChatStatusPayload {
  chat_id: string
  status: ChatStatus
}

export interface ChatErrorPayload {
  chat_id: string
  code: string
  message: string
}
