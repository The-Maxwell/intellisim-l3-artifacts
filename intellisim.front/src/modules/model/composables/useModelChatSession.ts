import { ref } from 'vue'
import type {
  EditScope,
  ExecutionLlmTarget,
  ModelChat,
  ModelChatMessage,
} from '@/shared/types'
import { useModelChatApi } from './useModelChatApi'
import { subscribeModelChatEvents } from './useModelChatEvents'
import { useModelStore } from '@/modules/model/stores/modelStore'
import { ApiError } from '@/shared/api/client'

// One live model-companion chat. Mirrors useChatSession (chat module) but is the
// SINGLE SSE owner for an open model: besides chat events it also routes
// `model.updated` into the model store so the editor form/canvas/JSON refresh
// live. Keeps messages chronologically sorted and deduplicated by id (the sender
// receives its own user message twice: from the 202 response AND the SSE event).
//
// Instances are cached per modelId so the editor view (lifecycle owner) and the
// ModelChatPanel share the SAME session — guaranteeing exactly ONE SSE
// connection per open model. close() evicts the cache entry.
type ModelChatSession = ReturnType<typeof createModelChatSession>

const sessions = new Map<string, ModelChatSession>()

export function useModelChatSession(modelId: string): ModelChatSession {
  let session = sessions.get(modelId)
  if (!session) {
    session = createModelChatSession(modelId)
    sessions.set(modelId, session)
  }
  return session
}

function createModelChatSession(modelId: string) {
  const api = useModelChatApi()
  const store = useModelStore()

  const chat = ref<ModelChat | null>(null)
  const messages = ref<ModelChatMessage[]>([])
  const target = ref<ExecutionLlmTarget | null>(null)
  const thinking = ref(false)
  const hasMore = ref(false)
  const loading = ref(false)
  const loadingOlder = ref(false)
  const deleted = ref(false)
  const error = ref<string | null>(null)

  let unsubscribe: (() => void) | null = null
  let openedId: string | null = null
  // Monotonically-incrementing generation token for open(). Each call to open()
  // or close() bumps this counter so that any in-flight open() that suspended on
  // an await can detect it has been superseded and bail out before subscribing.
  let openGen = 0

  function asMessage(e: unknown, fallback: string): string {
    return e instanceof Error ? e.message : fallback
  }

  function compareMessages(a: ModelChatMessage, b: ModelChatMessage): number {
    // Go's RFC3339Nano trims trailing zeros, so the strings are NOT safely
    // lexicographically ordered across differing fractional precision —
    // compare real timestamps and tie-break on id.
    const byTime = Date.parse(a.created_at) - Date.parse(b.created_at)
    return byTime !== 0 ? byTime : a.id.localeCompare(b.id)
  }

  function upsertMessage(msg: ModelChatMessage) {
    if (messages.value.some((m) => m.id === msg.id)) return
    const idx = messages.value.findIndex((m) => compareMessages(msg, m) < 0)
    if (idx === -1) messages.value.push(msg)
    else messages.value.splice(idx, 0, msg)
  }

  function applyChat(fresh: ModelChat) {
    chat.value = fresh
    thinking.value = fresh.status === 'thinking'
    // Only adopt a real target (an empty get-or-create returns blank uids).
    if (fresh.llm_target?.provider_uid && fresh.llm_target?.model_uid) {
      target.value = fresh.llm_target
    }
  }

  async function open(id: string) {
    // Same model already open and subscribed — just refresh, avoid churn.
    if (id === openedId && unsubscribe !== null) return
    // Reset WITHOUT evicting from the cache: open() must keep this instance in
    // the map, otherwise a consumer mounting later (e.g. ModelChatPanel when
    // store.current resolves after the editor's openSession) would get a fresh
    // orphan session with no SSE/messages while this one stays live.
    reset()
    // Capture a generation token AFTER reset() (which already bumped openGen).
    // Any concurrent in-flight open() that resumes from an await after this
    // point will see myGen !== openGen and bail before subscribing.
    const myGen = ++openGen
    openedId = id
    deleted.value = false
    error.value = null
    loading.value = true
    try {
      // get-or-create with no target: returns the existing chat (with messages)
      // or a fresh empty one (blank target, no messages yet). A manual model with
      // no companion chat AND no target returns 409 (ErrNoChat) — the backend
      // cannot create a chat without a target. Treat that as an EMPTY session
      // (no chat, no messages, no target): the panel shows its "pick a target"
      // state, and setting a target / sending creates the chat. We still proceed
      // to subscribe so those later events flow.
      let fresh: ModelChat | null = null
      try {
        fresh = await api.getChat(id)
      } catch (e) {
        if (e instanceof ApiError && e.status === 409) {
          // No chat yet — empty session, not an error.
          fresh = null
        } else {
          throw e
        }
      }
      // Bail if a newer open() or close() superseded us while we were awaiting.
      if (myGen !== openGen) return
      if (fresh) applyChat(fresh)
      if (fresh?.chat_id) {
        const page = await api.listMessages(id)
        // Check again after the second await.
        if (myGen !== openGen) return
        messages.value = [...page.messages].sort(compareMessages)
        hasMore.value = page.has_more
      } else {
        chat.value = null
        target.value = null
        messages.value = []
        hasMore.value = false
      }
    } catch (e) {
      if (myGen !== openGen) return
      error.value = asMessage(e, 'Failed to open model chat')
    } finally {
      // Only clear loading if this generation is still current; otherwise the
      // newer open() is responsible for its own loading state.
      if (myGen === openGen) loading.value = false
    }
    // One final guard before subscribing: if close() was called between the
    // last await and here, do not create an SSE that nobody can abort.
    if (myGen !== openGen) return
    const sub = subscribeModelChatEvents(id, {
      onMessage: upsertMessage,
      onStatus: (s) => {
        thinking.value = s.status === 'thinking'
      },
      // The model body changed (generated/edited by the LLM, or an echo of the
      // user's own PUT): route through applyExternalModelUpdate so a stale echo
      // is ignored and an update arriving over unsaved edits is parked (not
      // clobbered) for the editor's accept/dismiss banner.
      onModelUpdated: (model) => store.applyExternalModelUpdate(model),
      onError: (err) => {
        thinking.value = false
        error.value = err.message
      },
      onModelDeleted: () => {
        deleted.value = true
      },
      onReconnect: () => {
        refresh().catch(() => {})
      },
    })
    // Between subscribeModelChatEvents (synchronous registration) and here,
    // close() could have run (extremely unlikely but guard it anyway).
    if (myGen !== openGen) {
      sub()
      return
    }
    unsubscribe = sub
  }

  async function refresh() {
    if (!openedId) return
    let fresh: ModelChat | null = null
    try {
      fresh = await api.getChat(openedId)
    } catch (e) {
      // Still no companion chat (no target yet) — stay an empty session.
      if (e instanceof ApiError && e.status === 409) return
      throw e
    }
    applyChat(fresh)
    if (fresh.chat_id) {
      const page = await api.listMessages(openedId)
      messages.value = [...page.messages].sort(compareMessages)
      hasMore.value = page.has_more
    }
  }

  // Tear down the live connection state (SSE + open bookkeeping) while KEEPING
  // this instance in the session cache — used by open() when re-opening.
  function reset() {
    // Invalidate any in-flight open() so it bails before subscribing.
    openGen++
    unsubscribe?.()
    unsubscribe = null
    openedId = null
  }

  // SINGLE OWNER: only the editor view (ModelEditorView) may call close().
  // ModelChatPanel is a read-only consumer of the shared keyed instance and must
  // NOT call close(). There is no ref-count — the editor is the sole owner.
  function close() {
    reset()
    // Evict so a later open() of this model starts from a fresh instance.
    sessions.delete(modelId)
  }

  async function send(content: string, scope?: EditScope) {
    const id = openedId ?? modelId
    const resp = await api.sendMessage(id, { content, scope })
    upsertMessage(resp.message) // optimistic: the SSE event will dedupe by id
    thinking.value = resp.chat_status === 'thinking'
  }

  async function setTarget(t: ExecutionLlmTarget) {
    const id = openedId ?? modelId
    const fresh = await api.setChatTarget(id, t)
    applyChat(fresh)
    target.value = t
  }

  async function loadOlder() {
    if (!openedId || !hasMore.value || loadingOlder.value || messages.value.length === 0) return
    loadingOlder.value = true
    try {
      const page = await api.listMessages(openedId, { before: messages.value[0]!.id })
      hasMore.value = page.has_more
      for (const m of page.messages) upsertMessage(m)
    } finally {
      loadingOlder.value = false
    }
  }

  return {
    chat,
    messages,
    target,
    thinking,
    hasMore,
    loading,
    loadingOlder,
    deleted,
    error,
    open,
    close,
    send,
    setTarget,
    loadOlder,
  }
}
