import type { LLMTargetDTO } from '@/shared/types'

const KEY = 'intellisim.defaultLlmTarget'

/** Last-used LLM target — the default for new chats. */
export function getDefaultLlmTarget(): LLMTargetDTO | null {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw) as LLMTargetDTO
    return parsed.provider_uid && parsed.model_uid ? parsed : null
  } catch {
    return null
  }
}

export function setDefaultLlmTarget(target: LLMTargetDTO): void {
  localStorage.setItem(KEY, JSON.stringify(target))
}
