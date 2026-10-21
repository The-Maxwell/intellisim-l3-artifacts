import { apiContext, apiJson } from './api'

export type LlmCapability =
  | 'chat'
  | 'model_generation'
  | 'simulation_generation'
  | 'simulation_execution'
  | 'simulation_decisions'
  | 'results_analysis'
  | 'vision_analysis'

export interface LlmTargetRef {
  provider_uid: string
  model_uid: string
}

interface ProviderModel {
  uid: string
  capabilities: LlmCapability[]
  reasoning: boolean
}
interface ProviderInfo {
  uid: string
  models: ProviderModel[]
}

// Cheapest-first preference. @llm tests always resolve through this so runs
// use the least expensive model advertising the required capability.
const CHEAP_ORDER = ['gpt-4.1-nano', 'gpt-4o-mini', 'gpt-4.1-mini', 'o4-mini', 'gpt-4.1', 'gpt-4o', 'o3']

let cache: ProviderInfo[] | null = null

async function providers(): Promise<ProviderInfo[]> {
  if (cache) return cache
  const api = await apiContext()
  try {
    const body = await apiJson<{ list: ProviderInfo[] }>(api, 'get', '/llms', undefined, 200)
    cache = body.list ?? []
    return cache
  } finally {
    await api.dispose()
  }
}

// Cheapest advertised model with the capability, or null when no provider is
// connected (lets core-tier tests degrade gracefully / @llm tests fail loudly).
export async function pickCheapModel(capability: LlmCapability): Promise<LlmTargetRef | null> {
  const list = await providers()
  const candidates: { provider: string; model: string }[] = []
  for (const p of list)
    for (const m of p.models)
      if (m.capabilities.includes(capability)) candidates.push({ provider: p.uid, model: m.uid })
  if (!candidates.length) return null
  candidates.sort((a, b) => {
    const ia = CHEAP_ORDER.indexOf(a.model)
    const ib = CHEAP_ORDER.indexOf(b.model)
    return (ia === -1 ? 99 : ia) - (ib === -1 ? 99 : ib)
  })
  return { provider_uid: candidates[0].provider, model_uid: candidates[0].model }
}

export async function requireCheapModel(capability: LlmCapability): Promise<LlmTargetRef> {
  const t = await pickCheapModel(capability)
  if (!t) throw new Error(`no connected LLM model advertises capability "${capability}" — is the provider running?`)
  return t
}
