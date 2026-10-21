import { apiContext } from './api'
import { sweepE2EEntities } from './fixtures'

// Clears e2e- prefixed leftovers from previous crashed runs. Never touches
// non-prefixed data. Failures are non-fatal (stack may be partially up when
// running UI-only smoke specs).
// Gated behind E2E_SWEEP=1: concurrent playwright processes must not sweep
// each other's live fixtures. Run the sweep only on a full single run.
export default async function globalSetup(): Promise<void> {
  if (process.env.E2E_SWEEP !== '1') return
  try {
    const api = await apiContext()
    const removed = await sweepE2EEntities(api)
    if (removed.chats + removed.models + removed.sims > 0) {
      console.log(`[e2e] swept leftovers: ${removed.sims} sims, ${removed.models} models, ${removed.chats} chats`)
    }
    await api.dispose()
  } catch (err) {
    console.warn('[e2e] sweep skipped:', (err as Error).message)
  }
}
