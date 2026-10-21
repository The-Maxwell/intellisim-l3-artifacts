// Mirror of backend DTO validation (modules/chats/dtos.go) — single source of
// truth for UI-side limits. Backend remains the authority.
export const CHAT_NAME_MAX = 100
export const CHAT_DESCRIPTION_MAX = 500
export const MESSAGE_CONTENT_MAX = 10000
/** Show the character counter once the input reaches this length. */
export const MESSAGE_COUNTER_THRESHOLD = 9000
export const LLM_EFFORTS = ['minimal', 'low', 'medium', 'high', 'max'] as const
