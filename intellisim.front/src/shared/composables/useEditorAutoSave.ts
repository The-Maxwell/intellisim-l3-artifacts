import { onBeforeUnmount, onMounted } from 'vue'
import { onBeforeRouteLeave } from 'vue-router'

// Shared auto-save wiring for the model & simulation editors. Both editors keep
// their manual Save button and unsaved/saving indicator; this adds a safety net
// so the working copy is persisted at the two moments work is most easily lost:
// switching tabs and leaving the editor. There is deliberately NO periodic /
// debounced background save.
//
// The caller owns the actual persistence via `save()` — a function that resolves
// on success and REJECTS on failure (its own error UX, e.g. the non-blocking
// validation banner, runs inside it). This composable only decides WHEN to call
// it and what to do with a route-leave failure.
export interface EditorAutoSaveOptions {
  // True when there are unsaved edits worth persisting.
  isDirty: () => boolean
  // True when auto-save must stand down: a save is already in flight, an LLM
  // generation is running, or an external-update conflict is awaiting the user's
  // choice (auto-saving there would silently discard the incoming update, since
  // saveCurrent() clears the parked pendingExternal).
  isBlocked: () => boolean
  // Persist the working copy. Resolves on success; rejects on failure.
  save: () => Promise<void>
}

export interface EditorAutoSave {
  // Call from the tab bar's @update:value — fires only on genuine user tab
  // switches (programmatic activeTab changes, e.g. summary deep-links or a model
  // switch, do not emit update:value, so they never trigger a spurious save).
  autoSave: () => void
}

export function useEditorAutoSave(opts: EditorAutoSaveOptions): EditorAutoSave {
  const { isDirty, isBlocked, save } = opts

  // Tab switch: fire-and-forget. The tab has already changed (v-model), matching
  // "allow the switch, warn non-blocking" — save()'s own handler surfaces any
  // validation failure in the editor's banner without trapping the user.
  function autoSave(): void {
    if (isDirty() && !isBlocked()) void save()
  }

  // Leaving the editor route: try to persist first. On success, navigate. On a
  // hard-validation failure the edits can't be saved, so confirm before letting
  // the user discard them (never silently lose work).
  onBeforeRouteLeave(async () => {
    if (!isDirty() || isBlocked()) return true
    try {
      await save()
      return true
    } catch {
      return window.confirm(
        "Your unsaved changes couldn't be saved automatically because they have " +
          'validation errors. Leave and discard them?'
      )
    }
  })

  // Browser refresh/close: we can't reliably run an async save here, so fall back
  // to the native unsaved-changes prompt while dirty.
  function onBeforeUnload(e: BeforeUnloadEvent): void {
    if (isDirty()) {
      e.preventDefault()
      e.returnValue = ''
    }
  }
  onMounted(() => window.addEventListener('beforeunload', onBeforeUnload))
  onBeforeUnmount(() => window.removeEventListener('beforeunload', onBeforeUnload))

  return { autoSave }
}
