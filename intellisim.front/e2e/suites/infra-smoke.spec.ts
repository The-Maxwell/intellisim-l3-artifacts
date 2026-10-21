import { test, expect } from '@playwright/test'
import { apiContext, apiJson, u } from '../_helpers/api'
import { minimalSpatial, seedModel, seedReadySim, runToCompletion, uniqueName, deleteQuietly } from '../_helpers/fixtures'

// INF smoke: proves the seeding/run/cleanup pipeline against the live stack.
test.describe('infra smoke', () => {
  test('seed model → sim ready → run finished → cascade delete', async () => {
    const api = await apiContext()
    const model = await seedModel(api, minimalSpatial(uniqueName('mdl-smoke')))
    try {
      const full = await apiJson<any>(api, 'get', `/models/${model.id}`, undefined, 200)
      expect(full.validation?.simulation_ready, JSON.stringify(full.validation?.errors)).toBe(true)

      const sim = await seedReadySim(api, model.id, { steps: 5, seed: 42 })
      const status = await runToCompletion(api, sim.id)
      expect(status).toBe('finished')

      // No `to` param → the backend returns the full recorded range (a 5-step run
      // fits in one page_size=10 page).
      const steps = await apiJson<any>(api, 'get', `/simulations/${sim.id}/steps?page=1&page_size=10`, undefined, 200)
      expect((steps.items ?? []).length).toBeGreaterThan(0)

      await apiJson(api, 'delete', `/simulations/${sim.id}`, undefined, 204)
      const after = await api.get(u(`/simulations/${sim.id}`))
      expect(after.status()).toBe(404)
    } finally {
      await deleteQuietly(api, `/models/${model.id}`)
      await api.dispose()
    }
  })
})
