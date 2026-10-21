import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { ProviderInfo } from '@/shared/types'
import { fetchLLMProviders } from '@/shared/api/llmProviders'

export const useLlmProviderStore = defineStore('llmProvider', () => {
  const providers = ref<ProviderInfo[]>([])
  const selectedProviderId = ref<string | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  let inflight: Promise<void> | null = null

  async function fetchOnce() {
    loading.value = true
    error.value = null
    // The gRPC provider registers a few seconds after the backend boots, so an
    // early call can return an empty list. Retry a few times before giving up.
    const maxAttempts = 5
    try {
      for (let attempt = 1; attempt <= maxAttempts; attempt++) {
        const response = await fetchLLMProviders()
        providers.value = response.list ?? []
        if (providers.value.length > 0) break
        if (attempt < maxAttempts) await new Promise((r) => setTimeout(r, 1500))
      }
      const first = providers.value[0]
      if (!selectedProviderId.value && first) {
        selectedProviderId.value = first.uid
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to load providers'
    } finally {
      loading.value = false
    }
  }

  // Load providers at most once; concurrent callers share the same request.
  async function loadProviders(force = false): Promise<void> {
    if (force) {
      inflight = fetchOnce().finally(() => { inflight = null })
      return inflight
    }
    if (providers.value.length > 0) return
    if (!inflight) {
      inflight = fetchOnce().finally(() => { inflight = null })
    }
    return inflight
  }

  // Resolve a usable provider id, loading the list if needed.
  async function ensureProviderId(): Promise<string | undefined> {
    if (!selectedProviderId.value) await loadProviders()
    return selectedProviderId.value ?? undefined
  }

  function selectProvider(id: string) {
    selectedProviderId.value = id
  }

  return {
    providers,
    selectedProviderId,
    loading,
    error,
    loadProviders,
    ensureProviderId,
    selectProvider,
  }
})
