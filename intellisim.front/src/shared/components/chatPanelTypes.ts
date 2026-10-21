// Minimal display contract — chat/model/simulation callers all map to this.
export interface ChatPanelMessage {
  id: string
  role: string
  content: string
  created_at: string
  error?: { code: string; message: string } | null
}
